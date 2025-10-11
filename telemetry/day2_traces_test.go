package telemetry

import (
	"context"
	"testing"

	"go.opentelemetry.io/otel/attribute"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	"go.opentelemetry.io/otel/trace"
)

// TestDay2Traces_ReconciliationFlow tests the full reconciliation trace span flow
func TestDay2Traces_ReconciliationFlow(t *testing.T) {
	// Setup in-memory span recorder
	exporter := tracetest.NewInMemoryExporter()
	provider := sdktrace.NewTracerProvider(
		sdktrace.WithSampler(sdktrace.AlwaysSample()),
		sdktrace.WithSyncer(exporter),
	)
	tracer := provider.Tracer("test")

	ctx := context.Background()

	// Start root reconciliation span
	ctx, rootSpan := tracer.Start(ctx, "reconciliation",
		trace.WithAttributes(
			attribute.String("provider", "aws"),
			attribute.String("region", "us-east-1"),
			attribute.String("scan.type", "normal"),
		),
	)

	// Child span: observe
	_, observeSpan := tracer.Start(ctx, "observe",
		trace.WithAttributes(
			attribute.String("operation", "list_resources"),
			attribute.Int64("resources.count", 156),
		),
	)
	observeSpan.End()

	// Child span: detect
	_, detectSpan := tracer.Start(ctx, "detect",
		trace.WithAttributes(
			attribute.String("operation", "detect_changes"),
			attribute.Int64("changes.appeared", 2),
			attribute.Int64("changes.disappeared", 1),
			attribute.Int64("changes.tag_drift", 3),
		),
	)
	detectSpan.End()

	// Child span: decide
	_, decideSpan := tracer.Start(ctx, "decide",
		trace.WithAttributes(
			attribute.String("operation", "make_decisions"),
			attribute.Int64("decisions.notify", 4),
			attribute.Int64("decisions.alert", 1),
			attribute.Int64("decisions.enforce", 1),
		),
	)
	decideSpan.End()

	// Child span: execute
	_, executeSpan := tracer.Start(ctx, "execute",
		trace.WithAttributes(
			attribute.String("operation", "execute_actions"),
			attribute.Int64("actions.executed", 6),
			attribute.Int64("actions.succeeded", 5),
			attribute.Int64("actions.failed", 1),
		),
	)
	executeSpan.End()

	rootSpan.End()

	// Force flush
	_ = provider.ForceFlush(ctx)

	// Verify spans were recorded
	spans := exporter.GetSpans()
	if len(spans) == 0 {
		t.Fatal("Expected spans to be recorded")
	}

	// We should have 5 spans: root + 4 children
	if len(spans) != 5 {
		t.Errorf("Expected 5 spans (1 root + 4 children), got %d", len(spans))
	}

	// Verify root span
	var rootSpanRecorded *tracetest.SpanStub
	for i := range spans {
		if spans[i].Name == "reconciliation" {
			rootSpanRecorded = &spans[i]
			break
		}
	}

	if rootSpanRecorded == nil {
		t.Fatal("Root reconciliation span not found")
	}

	// Verify root span has required attributes
	hasProvider := false
	hasRegion := false
	for _, attr := range rootSpanRecorded.Attributes {
		if attr.Key == "provider" && attr.Value.AsString() == "aws" {
			hasProvider = true
		}
		if attr.Key == "region" && attr.Value.AsString() == "us-east-1" {
			hasRegion = true
		}
	}

	if !hasProvider {
		t.Error("Root span missing provider attribute")
	}
	if !hasRegion {
		t.Error("Root span missing region attribute")
	}
}

// TestDay2Traces_ObservePhase tests the observe phase span
func TestDay2Traces_ObservePhase(t *testing.T) {
	exporter := tracetest.NewInMemoryExporter()
	provider := sdktrace.NewTracerProvider(
		sdktrace.WithSampler(sdktrace.AlwaysSample()),
		sdktrace.WithSyncer(exporter),
	)
	tracer := provider.Tracer("test")

	ctx := context.Background()

	// Start observe span
	_, span := tracer.Start(ctx, "observe",
		trace.WithAttributes(
			attribute.String("provider", "aws"),
			attribute.String("region", "us-east-1"),
			attribute.String("resource.type", "ec2"),
		),
	)

	// Simulate observation completion
	span.SetAttributes(
		attribute.Int64("resources.scanned", 45),
		attribute.Float64("duration.seconds", 2.345),
	)

	span.End()

	// Force flush
	_ = provider.ForceFlush(ctx)

	// Verify span
	spans := exporter.GetSpans()
	if len(spans) != 1 {
		t.Fatalf("Expected 1 span, got %d", len(spans))
	}

	observeSpan := spans[0]
	if observeSpan.Name != "observe" {
		t.Errorf("Expected span name 'observe', got '%s'", observeSpan.Name)
	}

	// Verify attributes
	var hasResourcesScanned bool
	for _, attr := range observeSpan.Attributes {
		if attr.Key == "resources.scanned" {
			if attr.Value.AsInt64() == 45 {
				hasResourcesScanned = true
			}
		}
	}

	if !hasResourcesScanned {
		t.Error("Observe span missing resources.scanned attribute")
	}
}

// TestDay2Traces_DetectPhase tests the detect phase span
func TestDay2Traces_DetectPhase(t *testing.T) {
	exporter := tracetest.NewInMemoryExporter()
	provider := sdktrace.NewTracerProvider(
		sdktrace.WithSampler(sdktrace.AlwaysSample()),
		sdktrace.WithSyncer(exporter),
	)
	tracer := provider.Tracer("test")

	ctx := context.Background()

	// Start detect span
	_, span := tracer.Start(ctx, "detect",
		trace.WithAttributes(
			attribute.String("provider", "aws"),
			attribute.String("region", "us-east-1"),
		),
	)

	// Simulate change detection
	span.SetAttributes(
		attribute.Int64("changes.total", 6),
		attribute.Int64("changes.appeared", 2),
		attribute.Int64("changes.disappeared", 1),
		attribute.Int64("changes.tag_drift", 3),
		attribute.Int64("changes.status_changed", 0),
	)

	span.End()

	// Force flush
	_ = provider.ForceFlush(ctx)

	// Verify span
	spans := exporter.GetSpans()
	if len(spans) != 1 {
		t.Fatalf("Expected 1 span, got %d", len(spans))
	}

	detectSpan := spans[0]
	if detectSpan.Name != "detect" {
		t.Errorf("Expected span name 'detect', got '%s'", detectSpan.Name)
	}

	// Verify we captured change counts
	var totalChanges int64
	for _, attr := range detectSpan.Attributes {
		if attr.Key == "changes.total" {
			totalChanges = attr.Value.AsInt64()
		}
	}

	if totalChanges != 6 {
		t.Errorf("Expected 6 total changes, got %d", totalChanges)
	}
}

// TestDay2Traces_DecidePhase tests the decide phase span
func TestDay2Traces_DecidePhase(t *testing.T) {
	exporter := tracetest.NewInMemoryExporter()
	provider := sdktrace.NewTracerProvider(
		sdktrace.WithSampler(sdktrace.AlwaysSample()),
		sdktrace.WithSyncer(exporter),
	)
	tracer := provider.Tracer("test")

	ctx := context.Background()

	// Start decide span
	_, span := tracer.Start(ctx, "decide")

	// Simulate decision making
	span.SetAttributes(
		attribute.Int64("decisions.total", 10),
		attribute.Int64("decisions.notify", 7),
		attribute.Int64("decisions.alert", 2),
		attribute.Int64("decisions.enforce", 1),
		attribute.Int64("violations.detected", 3),
	)

	span.End()

	// Force flush
	_ = provider.ForceFlush(ctx)

	// Verify span
	spans := exporter.GetSpans()
	if len(spans) != 1 {
		t.Fatalf("Expected 1 span, got %d", len(spans))
	}

	decideSpan := spans[0]
	if decideSpan.Name != "decide" {
		t.Errorf("Expected span name 'decide', got '%s'", decideSpan.Name)
	}
}

// TestDay2Traces_ExecutePhase tests the execute phase span
func TestDay2Traces_ExecutePhase(t *testing.T) {
	exporter := tracetest.NewInMemoryExporter()
	provider := sdktrace.NewTracerProvider(
		sdktrace.WithSampler(sdktrace.AlwaysSample()),
		sdktrace.WithSyncer(exporter),
	)
	tracer := provider.Tracer("test")

	ctx := context.Background()

	// Start execute span
	_, span := tracer.Start(ctx, "execute")

	// Simulate action execution
	span.SetAttributes(
		attribute.Int64("actions.total", 10),
		attribute.Int64("actions.succeeded", 8),
		attribute.Int64("actions.failed", 2),
		attribute.Int64("notifications.sent", 8),
	)

	span.End()

	// Force flush
	_ = provider.ForceFlush(ctx)

	// Verify span
	spans := exporter.GetSpans()
	if len(spans) != 1 {
		t.Fatalf("Expected 1 span, got %d", len(spans))
	}

	executeSpan := spans[0]
	if executeSpan.Name != "execute" {
		t.Errorf("Expected span name 'execute', got '%s'", executeSpan.Name)
	}

	// Verify success/failure tracking
	var succeeded, failed int64
	for _, attr := range executeSpan.Attributes {
		if attr.Key == "actions.succeeded" {
			succeeded = attr.Value.AsInt64()
		}
		if attr.Key == "actions.failed" {
			failed = attr.Value.AsInt64()
		}
	}

	if succeeded != 8 {
		t.Errorf("Expected 8 succeeded actions, got %d", succeeded)
	}
	if failed != 2 {
		t.Errorf("Expected 2 failed actions, got %d", failed)
	}
}

// TestDay2Traces_SpanHierarchy tests that child spans are properly nested
func TestDay2Traces_SpanHierarchy(t *testing.T) {
	exporter := tracetest.NewInMemoryExporter()
	provider := sdktrace.NewTracerProvider(
		sdktrace.WithSampler(sdktrace.AlwaysSample()),
		sdktrace.WithSyncer(exporter),
	)
	tracer := provider.Tracer("test")

	ctx := context.Background()

	// Start parent span
	ctx, parentSpan := tracer.Start(ctx, "reconciliation")

	// Start child span
	_, childSpan := tracer.Start(ctx, "observe")
	childSpan.End()

	parentSpan.End()

	// Force flush
	_ = provider.ForceFlush(ctx)

	// Verify hierarchy
	spans := exporter.GetSpans()
	if len(spans) != 2 {
		t.Fatalf("Expected 2 spans, got %d", len(spans))
	}

	// Find parent and child
	var parent, child *tracetest.SpanStub
	for i := range spans {
		if spans[i].Name == "reconciliation" {
			parent = &spans[i]
		} else if spans[i].Name == "observe" {
			child = &spans[i]
		}
	}

	if parent == nil || child == nil {
		t.Fatal("Could not find parent and child spans")
	}

	// Verify child's parent is the parent span
	if child.Parent.SpanID() != parent.SpanContext.SpanID() {
		t.Error("Child span does not have correct parent SpanID")
	}

	// Verify they share the same trace
	if child.SpanContext.TraceID() != parent.SpanContext.TraceID() {
		t.Error("Child and parent spans do not share the same TraceID")
	}
}

// TestDay2Traces_ErrorRecording tests that errors are properly recorded in spans
func TestDay2Traces_ErrorRecording(t *testing.T) {
	exporter := tracetest.NewInMemoryExporter()
	provider := sdktrace.NewTracerProvider(
		sdktrace.WithSampler(sdktrace.AlwaysSample()),
		sdktrace.WithSyncer(exporter),
	)
	tracer := provider.Tracer("test")

	ctx := context.Background()

	// Start span and record an error
	_, span := tracer.Start(ctx, "execute")

	// Simulate an error with attributes
	span.SetAttributes(
		attribute.String("error.message", "Failed to tag resource"),
		attribute.String("error.type", "TaggingError"),
		attribute.Bool("error.occurred", true),
	)

	span.End()

	// Force flush
	_ = provider.ForceFlush(ctx)

	// Verify span has error attributes
	spans := exporter.GetSpans()
	if len(spans) != 1 {
		t.Fatalf("Expected 1 span, got %d", len(spans))
	}

	errorSpan := spans[0]
	var hasErrorMessage bool
	for _, attr := range errorSpan.Attributes {
		if attr.Key == "error.message" {
			hasErrorMessage = true
		}
	}

	if !hasErrorMessage {
		t.Error("Expected error.message attribute in span")
	}
}
