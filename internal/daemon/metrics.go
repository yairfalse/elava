package daemon

import (
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/metric"
)

// DaemonMetrics holds operational metrics for the daemon (Prometheus pattern)
type DaemonMetrics struct {
	// Reconciliation metrics
	reconcileRunsTotal     metric.Int64Counter
	reconcileFailuresTotal metric.Int64Counter
	reconcileDuration      metric.Float64Histogram

	// Resource tracking
	resourcesDiscovered metric.Int64Gauge
	changeEventsTotal   metric.Int64Counter

	// Storage health
	storageWritesFailed metric.Int64Counter
}

// NewDaemonMetrics creates daemon metrics following Prometheus naming
func NewDaemonMetrics() (*DaemonMetrics, error) {
	meter := otel.Meter("elava.daemon")

	reconcileRunsTotal, err := meter.Int64Counter(
		"daemon_reconcile_runs_total",
		metric.WithDescription("Total number of reconciliation runs"),
		metric.WithUnit("{runs}"),
	)
	if err != nil {
		return nil, err
	}

	reconcileFailuresTotal, err := meter.Int64Counter(
		"daemon_reconcile_failures_total",
		metric.WithDescription("Total number of failed reconciliation runs"),
		metric.WithUnit("{failures}"),
	)
	if err != nil {
		return nil, err
	}

	reconcileDuration, err := meter.Float64Histogram(
		"daemon_reconcile_duration_seconds",
		metric.WithDescription("Reconciliation run duration in seconds"),
		metric.WithUnit("s"),
	)
	if err != nil {
		return nil, err
	}

	resourcesDiscovered, err := meter.Int64Gauge(
		"daemon_resources_discovered",
		metric.WithDescription("Number of resources discovered in last scan"),
		metric.WithUnit("{resources}"),
	)
	if err != nil {
		return nil, err
	}

	changeEventsTotal, err := meter.Int64Counter(
		"daemon_change_events_total",
		metric.WithDescription("Total change events by type"),
		metric.WithUnit("{events}"),
	)
	if err != nil {
		return nil, err
	}

	storageWritesFailed, err := meter.Int64Counter(
		"daemon_storage_writes_failed_total",
		metric.WithDescription("Total failed storage write operations"),
		metric.WithUnit("{failures}"),
	)
	if err != nil {
		return nil, err
	}

	return &DaemonMetrics{
		reconcileRunsTotal:     reconcileRunsTotal,
		reconcileFailuresTotal: reconcileFailuresTotal,
		reconcileDuration:      reconcileDuration,
		resourcesDiscovered:    resourcesDiscovered,
		changeEventsTotal:      changeEventsTotal,
		storageWritesFailed:    storageWritesFailed,
	}, nil
}
