package telemetry

import (
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

// RecordChangeDetectedEvent emits a structured log event for infrastructure changes
func RecordChangeDetectedEvent(
	span trace.Span,
	changeType string,
	resourceID string,
	resourceType string,
	environment string,
	severity string,
	provider string,
	region string,
	message string,
) {
	if span == nil {
		return
	}

	span.AddEvent("infrastructure.change.detected", trace.WithAttributes(
		attribute.String("event.type", "infrastructure.change.detected"),
		attribute.String("change.type", changeType),
		attribute.String("resource.id", resourceID),
		attribute.String("resource.type", resourceType),
		attribute.String("environment", environment),
		attribute.String("severity", severity),
		attribute.String("provider", provider),
		attribute.String("region", region),
		attribute.String("message", message),
	))
}

// RecordDecisionMadeEvent emits a structured log event for policy decisions
func RecordDecisionMadeEvent(
	span trace.Span,
	action string,
	resourceID string,
	resourceType string,
	environment string,
	isBlessed bool,
	reason string,
	message string,
) {
	if span == nil {
		return
	}

	span.AddEvent("infrastructure.decision.made", trace.WithAttributes(
		attribute.String("event.type", "infrastructure.decision.made"),
		attribute.String("decision.action", action),
		attribute.String("resource.id", resourceID),
		attribute.String("resource.type", resourceType),
		attribute.String("environment", environment),
		attribute.Bool("is_blessed", isBlessed),
		attribute.String("reason", reason),
		attribute.String("message", message),
	))
}

// RecordActionExecutedEvent emits a structured log event for action execution
func RecordActionExecutedEvent(
	span trace.Span,
	actionType string,
	resourceID string,
	resourceType string,
	environment string,
	status string,
	errorMsg string,
	message string,
) {
	if span == nil {
		return
	}

	attrs := []attribute.KeyValue{
		attribute.String("event.type", "infrastructure.action.executed"),
		attribute.String("action.type", actionType),
		attribute.String("resource.id", resourceID),
		attribute.String("resource.type", resourceType),
		attribute.String("environment", environment),
		attribute.String("status", status),
		attribute.String("message", message),
	}

	if errorMsg != "" {
		attrs = append(attrs, attribute.String("error", errorMsg))
	}

	span.AddEvent("infrastructure.action.executed", trace.WithAttributes(attrs...))
}

// RecordPolicyViolationEvent emits a structured log event for policy violations
func RecordPolicyViolationEvent(
	span trace.Span,
	policyName string,
	resourceID string,
	resourceType string,
	environment string,
	severity string,
	autoRemediated bool,
	violationDetails string,
	message string,
) {
	if span == nil {
		return
	}

	span.AddEvent("infrastructure.policy.violation", trace.WithAttributes(
		attribute.String("event.type", "infrastructure.policy.violation"),
		attribute.String("policy.name", policyName),
		attribute.String("resource.id", resourceID),
		attribute.String("resource.type", resourceType),
		attribute.String("environment", environment),
		attribute.String("severity", severity),
		attribute.Bool("auto_remediated", autoRemediated),
		attribute.String("violation.details", violationDetails),
		attribute.String("message", message),
	))
}

// RecordScanCompletedEvent emits a structured log event for scan completion
func RecordScanCompletedEvent(
	span trace.Span,
	scanType string,
	provider string,
	region string,
	resourcesScanned int64,
	resourcesNew int64,
	resourcesChanged int64,
	resourcesDisappeared int64,
	durationSeconds float64,
	message string,
) {
	if span == nil {
		return
	}

	span.AddEvent("infrastructure.scan.completed", trace.WithAttributes(
		attribute.String("event.type", "infrastructure.scan.completed"),
		attribute.String("scan.type", scanType),
		attribute.String("provider", provider),
		attribute.String("region", region),
		attribute.Int64("resources.scanned", resourcesScanned),
		attribute.Int64("resources.new", resourcesNew),
		attribute.Int64("resources.changed", resourcesChanged),
		attribute.Int64("resources.disappeared", resourcesDisappeared),
		attribute.Float64("duration.seconds", durationSeconds),
		attribute.String("message", message),
	))
}
