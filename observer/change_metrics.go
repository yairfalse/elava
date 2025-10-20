package observer

import (
	"context"
	"fmt"

	"github.com/yairfalse/elava/storage"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

// ChangeEventMetrics records change events as OTEL metrics
type ChangeEventMetrics struct {
	// Required OTEL fields (per CLAUDE.md)
	meter                metric.Meter
	resourcesCreated     metric.Int64Counter
	resourcesModified    metric.Int64Counter
	resourcesDisappeared metric.Int64Counter
	changeEventsTotal    metric.Int64Counter
}

// NewChangeEventMetrics creates metrics observer
func NewChangeEventMetrics() (*ChangeEventMetrics, error) {
	meter := otel.Meter("elava")

	created, err := meter.Int64Counter(
		"elava_resources_created_total",
		metric.WithDescription("Total resources created"),
		metric.WithUnit("{resource}"),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create counter: %w", err)
	}

	modified, err := meter.Int64Counter(
		"elava_resources_modified_total",
		metric.WithDescription("Total resources modified"),
		metric.WithUnit("{resource}"),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create counter: %w", err)
	}

	disappeared, err := meter.Int64Counter(
		"elava_resources_disappeared_total",
		metric.WithDescription("Total resources disappeared"),
		metric.WithUnit("{resource}"),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create counter: %w", err)
	}

	total, err := meter.Int64Counter(
		"elava_change_events_stored_total",
		metric.WithDescription("Total change events stored"),
		metric.WithUnit("{event}"),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create counter: %w", err)
	}

	return &ChangeEventMetrics{
		meter:                meter,
		resourcesCreated:     created,
		resourcesModified:    modified,
		resourcesDisappeared: disappeared,
		changeEventsTotal:    total,
	}, nil
}

// RecordChangeEvents records a batch of change events
func (m *ChangeEventMetrics) RecordChangeEvents(ctx context.Context, events []storage.ChangeEvent) {
	for _, event := range events {
		m.recordSingleEvent(ctx, event)
	}

	// Record total batch size
	m.changeEventsTotal.Add(ctx, int64(len(events)))
}

// recordSingleEvent records one event (small focused function)
func (m *ChangeEventMetrics) recordSingleEvent(ctx context.Context, event storage.ChangeEvent) {
	// Extract labels
	resourceType := extractResourceType(event)
	region := extractRegion(event)

	attrs := metric.WithAttributes(
		attribute.String("type", resourceType),
		attribute.String("region", region),
	)

	// Increment appropriate counter
	switch event.ChangeType {
	case "created":
		m.resourcesCreated.Add(ctx, 1, attrs)
	case "modified":
		m.resourcesModified.Add(ctx, 1, attrs)
	case "disappeared":
		m.resourcesDisappeared.Add(ctx, 1, attrs)
	}
}

// extractResourceType gets resource type from event (small helper)
func extractResourceType(event storage.ChangeEvent) string {
	if event.Current != nil {
		return event.Current.Type
	}
	if event.Previous != nil {
		return event.Previous.Type
	}
	return "unknown"
}

// extractRegion gets region from event (small helper)
func extractRegion(event storage.ChangeEvent) string {
	if event.Current != nil {
		return event.Current.Region
	}
	if event.Previous != nil {
		return event.Previous.Region
	}
	return "unknown"
}
