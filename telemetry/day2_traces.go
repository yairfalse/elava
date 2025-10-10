package telemetry

import (
	"context"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

// ReconciliationSpan represents a reconciliation cycle span
type ReconciliationSpan struct {
	ctx  context.Context
	span trace.Span
}

// StartReconciliation starts a new reconciliation span
func StartReconciliation(
	ctx context.Context,
	tracer trace.Tracer,
	provider string,
	region string,
	scanType string,
) (context.Context, *ReconciliationSpan) {
	ctx, span := tracer.Start(ctx, "reconciliation",
		trace.WithAttributes(
			attribute.String("provider", provider),
			attribute.String("region", region),
			attribute.String("scan.type", scanType),
		),
	)

	return ctx, &ReconciliationSpan{ctx: ctx, span: span}
}

// End ends the reconciliation span
func (r *ReconciliationSpan) End() {
	r.span.End()
}

// SetResourceCount sets the total resource count attribute
func (r *ReconciliationSpan) SetResourceCount(count int64) {
	r.span.SetAttributes(attribute.Int64("resources.total", count))
}

// SetChangeCount sets change count attributes
func (r *ReconciliationSpan) SetChangeCount(appeared, disappeared, tagDrift, statusChanged int64) {
	r.span.SetAttributes(
		attribute.Int64("changes.appeared", appeared),
		attribute.Int64("changes.disappeared", disappeared),
		attribute.Int64("changes.tag_drift", tagDrift),
		attribute.Int64("changes.status_changed", statusChanged),
	)
}

// StartObserve starts an observe phase span
func StartObserve(
	ctx context.Context,
	tracer trace.Tracer,
	provider string,
	region string,
) (context.Context, trace.Span) {
	return tracer.Start(ctx, "observe",
		trace.WithAttributes(
			attribute.String("provider", provider),
			attribute.String("region", region),
		),
	)
}

// EndObserve ends the observe span with metrics
func EndObserve(span trace.Span, resourcesScanned int64, durationSeconds float64) {
	span.SetAttributes(
		attribute.Int64("resources.scanned", resourcesScanned),
		attribute.Float64("duration.seconds", durationSeconds),
	)
	span.End()
}

// StartDetect starts a detect phase span
func StartDetect(
	ctx context.Context,
	tracer trace.Tracer,
	provider string,
	region string,
) (context.Context, trace.Span) {
	return tracer.Start(ctx, "detect",
		trace.WithAttributes(
			attribute.String("provider", provider),
			attribute.String("region", region),
		),
	)
}

// EndDetect ends the detect span with change counts
func EndDetect(
	span trace.Span,
	total, appeared, disappeared, tagDrift, statusChanged int64,
) {
	span.SetAttributes(
		attribute.Int64("changes.total", total),
		attribute.Int64("changes.appeared", appeared),
		attribute.Int64("changes.disappeared", disappeared),
		attribute.Int64("changes.tag_drift", tagDrift),
		attribute.Int64("changes.status_changed", statusChanged),
	)
	span.End()
}

// StartDecide starts a decide phase span
func StartDecide(ctx context.Context, tracer trace.Tracer) (context.Context, trace.Span) {
	return tracer.Start(ctx, "decide")
}

// EndDecide ends the decide span with decision counts
func EndDecide(
	span trace.Span,
	total, notify, alert, enforce int64,
	violationsDetected int64,
) {
	span.SetAttributes(
		attribute.Int64("decisions.total", total),
		attribute.Int64("decisions.notify", notify),
		attribute.Int64("decisions.alert", alert),
		attribute.Int64("decisions.enforce", enforce),
		attribute.Int64("violations.detected", violationsDetected),
	)
	span.End()
}

// StartExecute starts an execute phase span
func StartExecute(ctx context.Context, tracer trace.Tracer) (context.Context, trace.Span) {
	return tracer.Start(ctx, "execute")
}

// EndExecute ends the execute span with action counts
func EndExecute(
	span trace.Span,
	total, succeeded, failed int64,
	notificationsSent int64,
) {
	span.SetAttributes(
		attribute.Int64("actions.total", total),
		attribute.Int64("actions.succeeded", succeeded),
		attribute.Int64("actions.failed", failed),
		attribute.Int64("notifications.sent", notificationsSent),
	)
	span.End()
}

// RecordError records an error in a span
func RecordError(span trace.Span, errorMessage string, errorType string) {
	span.SetAttributes(
		attribute.String("error.message", errorMessage),
		attribute.String("error.type", errorType),
		attribute.Bool("error.occurred", true),
	)
}
