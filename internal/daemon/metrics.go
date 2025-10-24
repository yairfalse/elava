package daemon

import (
	"context"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

// DaemonMetrics holds operational metrics using OTEL semantic conventions
type DaemonMetrics struct {
	reconciliations        metric.Int64Counter
	reconciliationDuration metric.Float64Histogram
	resourcesDiscovered    metric.Int64Gauge
	changeEvents           metric.Int64Counter
	storageOperations      metric.Int64Counter
}

// NewDaemonMetrics creates daemon metrics following OTEL semantic conventions
func NewDaemonMetrics() (*DaemonMetrics, error) {
	meter := otel.Meter("elava.daemon")

	reconciliations, err := meter.Int64Counter(
		"elava.daemon.reconciliations",
		metric.WithDescription("Number of reconciliation runs"),
		metric.WithUnit("{reconciliation}"),
	)
	if err != nil {
		return nil, err
	}

	reconciliationDuration, err := meter.Float64Histogram(
		"elava.daemon.reconciliation.duration",
		metric.WithDescription("Duration of reconciliation operations"),
		metric.WithUnit("s"),
	)
	if err != nil {
		return nil, err
	}

	resourcesDiscovered, err := meter.Int64Gauge(
		"elava.resources.discovered",
		metric.WithDescription("Number of cloud resources discovered"),
		metric.WithUnit("{resource}"),
	)
	if err != nil {
		return nil, err
	}

	changeEvents, err := meter.Int64Counter(
		"elava.change_events",
		metric.WithDescription("Number of infrastructure change events detected"),
		metric.WithUnit("{event}"),
	)
	if err != nil {
		return nil, err
	}

	storageOperations, err := meter.Int64Counter(
		"elava.storage.operations",
		metric.WithDescription("Number of storage operations"),
		metric.WithUnit("{operation}"),
	)
	if err != nil {
		return nil, err
	}

	return &DaemonMetrics{
		reconciliations:        reconciliations,
		reconciliationDuration: reconciliationDuration,
		resourcesDiscovered:    resourcesDiscovered,
		changeEvents:           changeEvents,
		storageOperations:      storageOperations,
	}, nil
}

// RecordReconciliation records a reconciliation run with status
func (m *DaemonMetrics) RecordReconciliation(ctx context.Context, status string, provider string, region string) {
	m.reconciliations.Add(ctx, 1,
		metric.WithAttributes(
			attribute.String("status", status),
			attribute.String("cloud.provider", provider),
			attribute.String("cloud.region", region),
		),
	)
}

// RecordReconciliationDuration records reconciliation duration
func (m *DaemonMetrics) RecordReconciliationDuration(ctx context.Context, durationSeconds float64, status string) {
	m.reconciliationDuration.Record(ctx, durationSeconds,
		metric.WithAttributes(
			attribute.String("status", status),
		),
	)
}

// RecordResourcesDiscovered records number of resources found
func (m *DaemonMetrics) RecordResourcesDiscovered(ctx context.Context, count int64, resourceType string, provider string, region string) {
	m.resourcesDiscovered.Record(ctx, count,
		metric.WithAttributes(
			attribute.String("resource.type", resourceType),
			attribute.String("cloud.provider", provider),
			attribute.String("cloud.region", region),
		),
	)
}

// RecordChangeEvent records a change event
func (m *DaemonMetrics) RecordChangeEvent(ctx context.Context, changeType string, resourceType string, region string) {
	m.changeEvents.Add(ctx, 1,
		metric.WithAttributes(
			attribute.String("change.type", changeType),
			attribute.String("resource.type", resourceType),
			attribute.String("cloud.region", region),
		),
	)
}

// RecordStorageOperation records a storage operation
func (m *DaemonMetrics) RecordStorageOperation(ctx context.Context, operation string, status string, errorType string) {
	attrs := []attribute.KeyValue{
		attribute.String("operation", operation),
		attribute.String("status", status),
	}
	if errorType != "" {
		attrs = append(attrs, attribute.String("error.type", errorType))
	}

	m.storageOperations.Add(ctx, 1, metric.WithAttributes(attrs...))
}
