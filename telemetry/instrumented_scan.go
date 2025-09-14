package telemetry

import (
	"context"
	"time"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/trace"
)

// Example of how to instrument the scan command with OTEL

// InstrumentedScan shows how to properly instrument a scan operation
func InstrumentedScan(ctx context.Context, region string, resourceCount int) error {
	// Start a span for the entire scan operation
	ctx, span := Tracer.Start(ctx, "ovi.scan",
		trace.WithAttributes(
			attribute.String("aws.region", region),
			attribute.String("operation.type", "full_scan"),
		),
		trace.WithSpanKind(trace.SpanKindInternal),
	)
	defer span.End()

	// Record start time for duration metric
	startTime := time.Now()

	// Simulate different phases of scanning
	if err := listResourcesWithTracing(ctx, region); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "Failed to list resources")
		return err
	}

	// Record metrics
	ResourcesScanned.Add(ctx, int64(resourceCount),
		metric.WithAttributes(
			attribute.String("region", region),
		),
	)

	// Find untracked resources
	untrackedCount := detectUntrackedWithTracing(ctx, resourceCount)

	UntrackedFound.Add(ctx, int64(untrackedCount),
		metric.WithAttributes(
			attribute.String("region", region),
			attribute.String("risk_level", "high"),
		),
	)

	// Store in MVCC
	if err := storeObservationsWithTracing(ctx, resourceCount); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "Failed to store observations")
		return err
	}

	// Record scan duration
	duration := time.Since(startTime).Seconds()
	ScanDuration.Record(ctx, duration,
		metric.WithAttributes(
			attribute.String("region", region),
			attribute.Bool("success", true),
		),
	)

	// Add summary to span
	span.SetAttributes(
		attribute.Int("resources.total", resourceCount),
		attribute.Int("resources.untracked", untrackedCount),
		attribute.Float64("duration.seconds", duration),
	)

	span.SetStatus(codes.Ok, "Scan completed successfully")
	return nil
}

// listResourcesWithTracing demonstrates child span creation
func listResourcesWithTracing(ctx context.Context, region string) error {
	_, span := Tracer.Start(ctx, "ovi.provider.list_resources",
		trace.WithAttributes(
			attribute.String("provider", "aws"),
			attribute.String("region", region),
		),
	)
	defer span.End()

	// Simulate AWS API calls
	time.Sleep(100 * time.Millisecond)

	span.AddEvent("Resources listed",
		trace.WithAttributes(
			attribute.Int("ec2.count", 50),
			attribute.Int("rds.count", 10),
			attribute.Int("s3.count", 25),
		),
	)

	return nil
}

// detectUntrackedWithTracing shows metric recording with tracing
func detectUntrackedWithTracing(ctx context.Context, total int) int {
	_, span := Tracer.Start(ctx, "ovi.scanner.detect_untracked")
	defer span.End()

	// Simulate detection logic
	untracked := total / 3 // 33% untracked

	span.SetAttributes(
		attribute.Int("untracked.count", untracked),
		attribute.Float64("untracked.percentage", float64(untracked)/float64(total)*100),
	)

	return untracked
}

// storeObservationsWithTracing demonstrates storage instrumentation
func storeObservationsWithTracing(ctx context.Context, count int) error {
	ctx, span := Tracer.Start(ctx, "ovi.storage.record_batch",
		trace.WithAttributes(
			attribute.Int("batch.size", count),
		),
	)
	defer span.End()

	// Record storage write
	StorageWrites.Add(ctx, 1,
		metric.WithAttributes(
			attribute.String("operation", "batch_write"),
			attribute.Int("size", count),
		),
	)

	// Update revision gauge
	StorageRevision.Record(ctx, 42) // Example revision
	ResourcesInStorage.Record(ctx, int64(count))

	span.AddEvent("Batch written to storage",
		trace.WithAttributes(
			attribute.Int("revision", 42),
		),
	)

	return nil
}
