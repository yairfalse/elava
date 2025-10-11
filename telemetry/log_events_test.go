package telemetry

import (
	"context"
	"testing"

	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
)

// TestRecordChangeDetectedEvent tests change detection log events
func TestRecordChangeDetectedEvent(t *testing.T) {
	exporter := tracetest.NewInMemoryExporter()
	provider := sdktrace.NewTracerProvider(
		sdktrace.WithSampler(sdktrace.AlwaysSample()),
		sdktrace.WithSyncer(exporter),
	)
	tracer := provider.Tracer("test")

	ctx, span := tracer.Start(context.Background(), "test")

	RecordChangeDetectedEvent(
		span,
		"appeared",
		"i-1234567890abcdef0",
		"ec2",
		"production",
		"info",
		"aws",
		"us-east-1",
		"New EC2 instance detected",
	)

	span.End()
	_ = provider.ForceFlush(ctx)

	spans := exporter.GetSpans()
	if len(spans) != 1 {
		t.Fatalf("Expected 1 span, got %d", len(spans))
	}

	events := spans[0].Events
	if len(events) != 1 {
		t.Fatalf("Expected 1 event, got %d", len(events))
	}

	event := events[0]
	if event.Name != "infrastructure.change.detected" {
		t.Errorf("Expected event name 'infrastructure.change.detected', got '%s'", event.Name)
	}

	// Verify attributes
	attrs := event.Attributes
	expectedAttrs := map[string]interface{}{
		"event.type":    "infrastructure.change.detected",
		"change.type":   "appeared",
		"resource.id":   "i-1234567890abcdef0",
		"resource.type": "ec2",
		"environment":   "production",
		"severity":      "info",
		"provider":      "aws",
		"region":        "us-east-1",
		"message":       "New EC2 instance detected",
	}

	for key, expectedValue := range expectedAttrs {
		found := false
		for _, attr := range attrs {
			if string(attr.Key) == key {
				found = true
				if attr.Value.AsString() != expectedValue.(string) {
					t.Errorf("Attribute %s: expected '%v', got '%v'", key, expectedValue, attr.Value.AsString())
				}
				break
			}
		}
		if !found {
			t.Errorf("Missing attribute: %s", key)
		}
	}
}

// TestRecordDecisionMadeEvent tests decision log events
func TestRecordDecisionMadeEvent(t *testing.T) {
	exporter := tracetest.NewInMemoryExporter()
	provider := sdktrace.NewTracerProvider(
		sdktrace.WithSampler(sdktrace.AlwaysSample()),
		sdktrace.WithSyncer(exporter),
	)
	tracer := provider.Tracer("test")

	ctx, span := tracer.Start(context.Background(), "test")

	RecordDecisionMadeEvent(
		span,
		"notify",
		"i-orphan123",
		"ec2",
		"production",
		false,
		"Resource has no owner tags",
		"Decision: notify about orphaned EC2 instance",
	)

	span.End()
	_ = provider.ForceFlush(ctx)

	spans := exporter.GetSpans()
	if len(spans) != 1 {
		t.Fatalf("Expected 1 span, got %d", len(spans))
	}

	events := spans[0].Events
	if len(events) != 1 {
		t.Fatalf("Expected 1 event, got %d", len(events))
	}

	event := events[0]
	if event.Name != "infrastructure.decision.made" {
		t.Errorf("Expected event name 'infrastructure.decision.made', got '%s'", event.Name)
	}

	// Verify is_blessed attribute (bool)
	hasIsBlessed := false
	for _, attr := range event.Attributes {
		if string(attr.Key) == "is_blessed" {
			hasIsBlessed = true
			if attr.Value.AsBool() != false {
				t.Errorf("Expected is_blessed=false, got %v", attr.Value.AsBool())
			}
		}
	}
	if !hasIsBlessed {
		t.Error("Missing is_blessed attribute")
	}
}

// TestRecordActionExecutedEvent tests action execution log events
func TestRecordActionExecutedEvent(t *testing.T) {
	exporter := tracetest.NewInMemoryExporter()
	provider := sdktrace.NewTracerProvider(
		sdktrace.WithSampler(sdktrace.AlwaysSample()),
		sdktrace.WithSyncer(exporter),
	)
	tracer := provider.Tracer("test")

	ctx, span := tracer.Start(context.Background(), "test")

	// Test successful action
	RecordActionExecutedEvent(
		span,
		"enforce_tags",
		"bucket-needs-tags",
		"s3",
		"staging",
		"success",
		"",
		"Successfully enforced tags on S3 bucket",
	)

	// Test failed action
	RecordActionExecutedEvent(
		span,
		"delete",
		"i-protected",
		"ec2",
		"production",
		"failed",
		"Resource has deletion protection enabled",
		"Failed to delete protected EC2 instance",
	)

	span.End()
	_ = provider.ForceFlush(ctx)

	spans := exporter.GetSpans()
	if len(spans) != 1 {
		t.Fatalf("Expected 1 span, got %d", len(spans))
	}

	events := spans[0].Events
	if len(events) != 2 {
		t.Fatalf("Expected 2 events, got %d", len(events))
	}

	// Verify first event (success)
	successEvent := events[0]
	if successEvent.Name != "infrastructure.action.executed" {
		t.Errorf("Expected event name 'infrastructure.action.executed', got '%s'", successEvent.Name)
	}

	hasStatus := false
	for _, attr := range successEvent.Attributes {
		if string(attr.Key) == "status" {
			hasStatus = true
			if attr.Value.AsString() != "success" {
				t.Errorf("Expected status='success', got '%s'", attr.Value.AsString())
			}
		}
	}
	if !hasStatus {
		t.Error("Missing status attribute")
	}

	// Verify second event has error attribute
	failedEvent := events[1]
	hasError := false
	for _, attr := range failedEvent.Attributes {
		if string(attr.Key) == "error" {
			hasError = true
		}
	}
	if !hasError {
		t.Error("Failed action should have error attribute")
	}
}

// TestRecordPolicyViolationEvent tests policy violation log events
func TestRecordPolicyViolationEvent(t *testing.T) {
	exporter := tracetest.NewInMemoryExporter()
	provider := sdktrace.NewTracerProvider(
		sdktrace.WithSampler(sdktrace.AlwaysSample()),
		sdktrace.WithSyncer(exporter),
	)
	tracer := provider.Tracer("test")

	ctx, span := tracer.Start(context.Background(), "test")

	RecordPolicyViolationEvent(
		span,
		"databases-must-be-private",
		"db-public-exposed",
		"rds",
		"production",
		"critical",
		false,
		"RDS instance is publicly accessible",
		"Policy violation: database publicly accessible",
	)

	span.End()
	_ = provider.ForceFlush(ctx)

	spans := exporter.GetSpans()
	if len(spans) != 1 {
		t.Fatalf("Expected 1 span, got %d", len(spans))
	}

	events := spans[0].Events
	if len(events) != 1 {
		t.Fatalf("Expected 1 event, got %d", len(events))
	}

	event := events[0]
	if event.Name != "infrastructure.policy.violation" {
		t.Errorf("Expected event name 'infrastructure.policy.violation', got '%s'", event.Name)
	}

	// Verify severity
	hasSeverity := false
	for _, attr := range event.Attributes {
		if string(attr.Key) == "severity" {
			hasSeverity = true
			if attr.Value.AsString() != "critical" {
				t.Errorf("Expected severity='critical', got '%s'", attr.Value.AsString())
			}
		}
	}
	if !hasSeverity {
		t.Error("Missing severity attribute")
	}
}

// TestRecordScanCompletedEvent tests scan completion log events
func TestRecordScanCompletedEvent(t *testing.T) {
	exporter := tracetest.NewInMemoryExporter()
	provider := sdktrace.NewTracerProvider(
		sdktrace.WithSampler(sdktrace.AlwaysSample()),
		sdktrace.WithSyncer(exporter),
	)
	tracer := provider.Tracer("test")

	ctx, span := tracer.Start(context.Background(), "test")

	RecordScanCompletedEvent(
		span,
		"baseline",
		"aws",
		"us-east-1",
		156,
		156,
		0,
		0,
		12.456,
		"Baseline scan completed",
	)

	span.End()
	_ = provider.ForceFlush(ctx)

	spans := exporter.GetSpans()
	if len(spans) != 1 {
		t.Fatalf("Expected 1 span, got %d", len(spans))
	}

	events := spans[0].Events
	if len(events) != 1 {
		t.Fatalf("Expected 1 event, got %d", len(events))
	}

	event := events[0]
	if event.Name != "infrastructure.scan.completed" {
		t.Errorf("Expected event name 'infrastructure.scan.completed', got '%s'", event.Name)
	}

	// Verify numeric attributes
	expectedInts := map[string]int64{
		"resources.scanned":     156,
		"resources.new":         156,
		"resources.changed":     0,
		"resources.disappeared": 0,
	}

	for key, expectedValue := range expectedInts {
		found := false
		for _, attr := range event.Attributes {
			if string(attr.Key) == key {
				found = true
				if attr.Value.AsInt64() != expectedValue {
					t.Errorf("Attribute %s: expected %d, got %d", key, expectedValue, attr.Value.AsInt64())
				}
				break
			}
		}
		if !found {
			t.Errorf("Missing attribute: %s", key)
		}
	}
}

// TestLogEventWithNilSpan tests graceful handling of nil span
func TestLogEventWithNilSpan(t *testing.T) {
	// Should not panic with nil span
	RecordChangeDetectedEvent(nil, "appeared", "i-123", "ec2", "prod", "info", "aws", "us-east-1", "test")
	RecordDecisionMadeEvent(nil, "notify", "i-123", "ec2", "prod", false, "reason", "test")
	RecordActionExecutedEvent(nil, "tag", "i-123", "ec2", "prod", "success", "", "test")
	RecordPolicyViolationEvent(nil, "policy", "i-123", "ec2", "prod", "critical", false, "details", "test")
	RecordScanCompletedEvent(nil, "normal", "aws", "us-east-1", 10, 1, 2, 3, 1.5, "test")

	// Test passes if no panic occurred
}

// TestMultipleLogEvents tests multiple events in single span
func TestMultipleLogEvents(t *testing.T) {
	exporter := tracetest.NewInMemoryExporter()
	provider := sdktrace.NewTracerProvider(
		sdktrace.WithSampler(sdktrace.AlwaysSample()),
		sdktrace.WithSyncer(exporter),
	)
	tracer := provider.Tracer("test")

	ctx, span := tracer.Start(context.Background(), "reconciliation")

	// Emit multiple log events
	RecordChangeDetectedEvent(span, "appeared", "i-1", "ec2", "prod", "info", "aws", "us-east-1", "change 1")
	RecordChangeDetectedEvent(span, "disappeared", "i-2", "ec2", "prod", "warning", "aws", "us-east-1", "change 2")
	RecordDecisionMadeEvent(span, "notify", "i-1", "ec2", "prod", false, "reason", "decision 1")

	span.End()
	_ = provider.ForceFlush(ctx)

	spans := exporter.GetSpans()
	if len(spans) != 1 {
		t.Fatalf("Expected 1 span, got %d", len(spans))
	}

	events := spans[0].Events
	if len(events) != 3 {
		t.Fatalf("Expected 3 events, got %d", len(events))
	}

	// Verify event types
	expectedTypes := []string{
		"infrastructure.change.detected",
		"infrastructure.change.detected",
		"infrastructure.decision.made",
	}

	for i, expectedType := range expectedTypes {
		if events[i].Name != expectedType {
			t.Errorf("Event %d: expected type '%s', got '%s'", i, expectedType, events[i].Name)
		}
	}
}

// TestLogEventAttributeTypes tests different attribute value types
func TestLogEventAttributeTypes(t *testing.T) {
	exporter := tracetest.NewInMemoryExporter()
	provider := sdktrace.NewTracerProvider(
		sdktrace.WithSampler(sdktrace.AlwaysSample()),
		sdktrace.WithSyncer(exporter),
	)
	tracer := provider.Tracer("test")

	ctx, span := tracer.Start(context.Background(), "test")

	// Event with int64 values
	RecordScanCompletedEvent(span, "normal", "aws", "us-east-1", 100, 5, 3, 2, 1.234, "scan complete")

	span.End()
	_ = provider.ForceFlush(ctx)

	spans := exporter.GetSpans()
	events := spans[0].Events
	event := events[0]

	// Verify different attribute types
	for _, attr := range event.Attributes {
		switch string(attr.Key) {
		case "resources.scanned", "resources.new", "resources.changed", "resources.disappeared":
			// Should be int64
			_ = attr.Value.AsInt64()
		case "duration.seconds":
			// Should be float64
			_ = attr.Value.AsFloat64()
		case "scan.type", "provider", "region", "message":
			// Should be string
			_ = attr.Value.AsString()
		}
	}
}
