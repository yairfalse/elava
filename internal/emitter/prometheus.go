package emitter

import (
	"context"
	"fmt"
	"sync"

	"github.com/rs/zerolog/log"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"

	"github.com/yairfalse/elava/pkg/resource"
)

// PrometheusEmitter emits metrics in Prometheus format via OTEL.
type PrometheusEmitter struct {
	meter metric.Meter

	// Metrics
	resourceInfo         metric.Int64ObservableGauge
	scanDuration         metric.Float64Histogram
	scanResourcesTotal   metric.Int64Counter
	scanErrorsTotal      metric.Int64Counter
	resourceChangesTotal metric.Int64Counter

	// State for observable gauge
	mu        sync.RWMutex
	resources []resource.Resource

	// Diff tracking
	diffTracker *DiffTracker
}

// NewPrometheusEmitter creates a Prometheus emitter.
func NewPrometheusEmitter() (*PrometheusEmitter, error) {
	meter := otel.Meter("elava")

	e := &PrometheusEmitter{
		meter:       meter,
		resources:   make([]resource.Resource, 0),
		diffTracker: NewDiffTracker(),
	}

	if err := e.initMetrics(); err != nil {
		return nil, fmt.Errorf("init metrics: %w", err)
	}

	return e, nil
}

func (e *PrometheusEmitter) initMetrics() error {
	var err error

	// Resource info gauge - shows current resources
	e.resourceInfo, err = e.meter.Int64ObservableGauge(
		"elava_resource_info",
		metric.WithDescription("Cloud resource information"),
		metric.WithInt64Callback(e.observeResources),
	)
	if err != nil {
		return fmt.Errorf("create resource_info gauge: %w", err)
	}

	// Scan duration histogram
	e.scanDuration, err = e.meter.Float64Histogram(
		"elava_scan_duration_seconds",
		metric.WithDescription("Time taken to scan resources"),
		metric.WithUnit("s"),
	)
	if err != nil {
		return fmt.Errorf("create scan_duration histogram: %w", err)
	}

	// Resources scanned counter
	e.scanResourcesTotal, err = e.meter.Int64Counter(
		"elava_scan_resources_total",
		metric.WithDescription("Total resources scanned"),
	)
	if err != nil {
		return fmt.Errorf("create scan_resources counter: %w", err)
	}

	// Scan errors counter
	e.scanErrorsTotal, err = e.meter.Int64Counter(
		"elava_scan_errors_total",
		metric.WithDescription("Total scan errors"),
	)
	if err != nil {
		return fmt.Errorf("create scan_errors counter: %w", err)
	}

	// Resource changes counter
	e.resourceChangesTotal, err = e.meter.Int64Counter(
		"elava_resource_changes_total",
		metric.WithDescription("Total resource changes detected"),
	)
	if err != nil {
		return fmt.Errorf("create resource_changes counter: %w", err)
	}

	return nil
}

// Emit records the scan result as metrics.
func (e *PrometheusEmitter) Emit(ctx context.Context, result resource.ScanResult) error {
	attrs := []attribute.KeyValue{
		attribute.String("provider", result.Provider),
		attribute.String("region", result.Region),
	}

	// Record scan duration
	e.scanDuration.Record(ctx, result.Duration.Seconds(), metric.WithAttributes(attrs...))

	// Record error if any
	if result.Error != nil {
		e.scanErrorsTotal.Add(ctx, 1, metric.WithAttributes(attrs...))
		log.Error().
			Err(result.Error).
			Str("provider", result.Provider).
			Str("region", result.Region).
			Msg("scan error")
		return nil // Don't fail on scan errors
	}

	// Record resource count
	e.scanResourcesTotal.Add(ctx, int64(len(result.Resources)), metric.WithAttributes(attrs...))

	// Compute and emit diffs
	e.emitDiffs(ctx, result)

	// Update resources for observable gauge
	e.mu.Lock()
	e.resources = result.Resources
	e.mu.Unlock()

	// Update diff tracker state
	e.diffTracker.Update(result.Resources)

	log.Info().
		Str("provider", result.Provider).
		Str("region", result.Region).
		Int("resources", len(result.Resources)).
		Dur("duration", result.Duration).
		Msg("scan complete")

	return nil
}

// emitDiffs computes diffs and emits metrics/logs for changes.
func (e *PrometheusEmitter) emitDiffs(ctx context.Context, result resource.ScanResult) {
	diffs := e.diffTracker.ComputeDiff(result.Resources)
	if diffs == nil {
		// First scan - baseline established
		return
	}

	for _, diff := range diffs {
		attrs := []attribute.KeyValue{
			attribute.String("provider", diff.Resource.Provider),
			attribute.String("type", diff.Resource.Type),
			attribute.String("region", diff.Resource.Region),
			attribute.String("change_type", string(diff.Type)),
		}
		e.resourceChangesTotal.Add(ctx, 1, metric.WithAttributes(attrs...))

		// Log the change
		logEvent := log.Info().
			Str("id", diff.Resource.ID).
			Str("type", diff.Resource.Type).
			Str("provider", diff.Resource.Provider).
			Str("region", diff.Resource.Region).
			Str("change", string(diff.Type))

		// Add change details for modifications
		if diff.Type == resource.DiffModified {
			for field, change := range diff.Changes {
				logEvent = logEvent.
					Str(field+".from", change.Previous).
					Str(field+".to", change.Current)
			}
		}

		logEvent.Msg("resource changed")
	}
}

// observeResources is the callback for the resource_info gauge.
func (e *PrometheusEmitter) observeResources(_ context.Context, o metric.Int64Observer) error {
	e.mu.RLock()
	defer e.mu.RUnlock()

	for _, r := range e.resources {
		attrs := []attribute.KeyValue{
			attribute.String("id", r.ID),
			attribute.String("type", r.Type),
			attribute.String("provider", r.Provider),
			attribute.String("region", r.Region),
			attribute.String("status", r.Status),
		}

		// Add name if present
		if r.Name != "" {
			attrs = append(attrs, attribute.String("name", r.Name))
		}

		// Add common labels
		for k, v := range r.Labels {
			if v != "" {
				attrs = append(attrs, attribute.String("label_"+k, v))
			}
		}

		o.Observe(1, metric.WithAttributes(attrs...))
	}

	return nil
}

// Close is a no-op for Prometheus emitter.
func (e *PrometheusEmitter) Close() error {
	return nil
}
