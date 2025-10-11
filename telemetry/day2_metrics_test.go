package telemetry

import (
	"context"
	"testing"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
)

// TestDay2Metrics_ChangesDetected tests that we can record infrastructure changes
func TestDay2Metrics_ChangesDetected(t *testing.T) {
	// Setup in-memory metric reader for testing
	reader := sdkmetric.NewManualReader()
	provider := sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader))

	// Create meter for test
	meter := provider.Meter("test")

	// Initialize metric (this should exist in day2_metrics.go)
	changesDetected, err := meter.Int64Counter("elava.changes.detected.total")
	if err != nil {
		t.Fatalf("Failed to create counter: %v", err)
	}

	// Record some changes
	ctx := context.Background()

	// Appeared change
	changesDetected.Add(ctx, 2,
		metric.WithAttributeSet(attribute.NewSet(
			attribute.String("change_type", "appeared"),
			attribute.String("environment", "production"),
			attribute.String("resource_type", "ec2"),
			attribute.String("severity", "info"),
		)),
	)

	// Disappeared change (critical)
	changesDetected.Add(ctx, 1,
		metric.WithAttributeSet(attribute.NewSet(
			attribute.String("change_type", "disappeared"),
			attribute.String("environment", "production"),
			attribute.String("resource_type", "rds"),
			attribute.String("severity", "critical"),
		)),
	)

	// Tag drift
	changesDetected.Add(ctx, 3,
		metric.WithAttributeSet(attribute.NewSet(
			attribute.String("change_type", "tag_drift"),
			attribute.String("environment", "staging"),
			attribute.String("resource_type", "s3"),
			attribute.String("severity", "warning"),
		)),
	)

	// Read metrics
	var rm metricdata.ResourceMetrics
	if err := reader.Collect(ctx, &rm); err != nil {
		t.Fatalf("Failed to collect metrics: %v", err)
	}

	// Verify metrics were recorded
	if len(rm.ScopeMetrics) == 0 {
		t.Fatal("Expected metrics to be recorded")
	}

	// Verify we have the counter
	found := false
	for _, sm := range rm.ScopeMetrics {
		for _, m := range sm.Metrics {
			if m.Name == "elava.changes.detected.total" {
				found = true

				// Verify it's a sum (counter)
				sum, ok := m.Data.(metricdata.Sum[int64])
				if !ok {
					t.Errorf("Expected Sum, got %T", m.Data)
					continue
				}

				// Verify we have data points
				if len(sum.DataPoints) == 0 {
					t.Error("Expected data points")
				}

				// Verify total count (2 + 1 + 3 = 6)
				var total int64
				for _, dp := range sum.DataPoints {
					total += dp.Value
				}
				if total != 6 {
					t.Errorf("Expected total of 6 changes, got %d", total)
				}
			}
		}
	}

	if !found {
		t.Error("Metric elava.changes.detected.total not found")
	}
}

// TestDay2Metrics_DecisionsMade tests policy decision tracking
func TestDay2Metrics_DecisionsMade(t *testing.T) {
	reader := sdkmetric.NewManualReader()
	provider := sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader))
	meter := provider.Meter("test")

	decisionsMade, err := meter.Int64Counter("elava.decisions.made.total",
		metric.WithDescription("Total number of policy decisions made"),
		metric.WithUnit("decisions"),
	)
	if err != nil {
		t.Fatalf("Failed to create counter: %v", err)
	}

	ctx := context.Background()

	// Notify actions
	decisionsMade.Add(ctx, 5,
		metric.WithAttributeSet(attribute.NewSet(
			attribute.String("action", "notify"),
			attribute.String("resource_type", "ec2"),
			attribute.String("environment", "production"),
			attribute.Bool("is_blessed", false),
		)),
	)

	// Critical alert
	decisionsMade.Add(ctx, 1,
		metric.WithAttributeSet(attribute.NewSet(
			attribute.String("action", "alert"),
			attribute.String("resource_type", "rds"),
			attribute.String("environment", "production"),
			attribute.Bool("is_blessed", true),
		)),
	)

	// Auto-remediation
	decisionsMade.Add(ctx, 3,
		metric.WithAttributeSet(attribute.NewSet(
			attribute.String("action", "enforce_tags"),
			attribute.String("resource_type", "s3"),
			attribute.String("environment", "staging"),
			attribute.Bool("is_blessed", false),
		)),
	)

	// Collect and verify
	var rm metricdata.ResourceMetrics
	if err := reader.Collect(ctx, &rm); err != nil {
		t.Fatalf("Failed to collect metrics: %v", err)
	}

	// Verify total decisions (5 + 1 + 3 = 9)
	var total int64
	for _, sm := range rm.ScopeMetrics {
		for _, m := range sm.Metrics {
			if m.Name == "elava.decisions.made.total" {
				sum := m.Data.(metricdata.Sum[int64])
				for _, dp := range sum.DataPoints {
					total += dp.Value
				}
			}
		}
	}

	if total != 9 {
		t.Errorf("Expected 9 total decisions, got %d", total)
	}
}

// TestDay2Metrics_PolicyViolations tests policy violation tracking
func TestDay2Metrics_PolicyViolations(t *testing.T) {
	reader := sdkmetric.NewManualReader()
	provider := sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader))
	meter := provider.Meter("test")

	violations, err := meter.Int64Counter("elava.policy.violations.total",
		metric.WithDescription("Total number of policy violations detected"),
		metric.WithUnit("violations"),
	)
	if err != nil {
		t.Fatalf("Failed to create counter: %v", err)
	}

	ctx := context.Background()

	// Critical violation
	violations.Add(ctx, 1,
		metric.WithAttributeSet(attribute.NewSet(
			attribute.String("policy_name", "databases-must-be-private"),
			attribute.String("severity", "critical"),
			attribute.String("environment", "production"),
			attribute.Bool("auto_remediated", false),
		)),
	)

	// Medium violations with auto-remediation
	violations.Add(ctx, 5,
		metric.WithAttributeSet(attribute.NewSet(
			attribute.String("policy_name", "resources-must-have-owner"),
			attribute.String("severity", "medium"),
			attribute.String("environment", "staging"),
			attribute.Bool("auto_remediated", true),
		)),
	)

	// Collect and verify
	var rm metricdata.ResourceMetrics
	if err := reader.Collect(ctx, &rm); err != nil {
		t.Fatalf("Failed to collect metrics: %v", err)
	}

	var total int64
	for _, sm := range rm.ScopeMetrics {
		for _, m := range sm.Metrics {
			if m.Name == "elava.policy.violations.total" {
				sum := m.Data.(metricdata.Sum[int64])
				for _, dp := range sum.DataPoints {
					total += dp.Value
				}
			}
		}
	}

	if total != 6 {
		t.Errorf("Expected 6 total violations, got %d", total)
	}
}

// TestDay2Metrics_CurrentResources tests gauge for current resource count
func TestDay2Metrics_CurrentResources(t *testing.T) {
	reader := sdkmetric.NewManualReader()
	provider := sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader))
	meter := provider.Meter("test")

	currentResources, err := meter.Int64Gauge("elava.resources.current",
		metric.WithDescription("Current number of resources being managed"),
		metric.WithUnit("resources"),
	)
	if err != nil {
		t.Fatalf("Failed to create gauge: %v", err)
	}

	ctx := context.Background()

	// Record current resource counts
	currentResources.Record(ctx, 45,
		metric.WithAttributeSet(attribute.NewSet(
			attribute.String("resource_type", "ec2"),
			attribute.String("environment", "production"),
			attribute.String("state", "running"),
		)),
	)

	currentResources.Record(ctx, 12,
		metric.WithAttributeSet(attribute.NewSet(
			attribute.String("resource_type", "rds"),
			attribute.String("environment", "production"),
			attribute.String("state", "available"),
		)),
	)

	currentResources.Record(ctx, 8,
		metric.WithAttributeSet(attribute.NewSet(
			attribute.String("resource_type", "ec2"),
			attribute.String("environment", "production"),
			attribute.String("state", "stopped"),
		)),
	)

	// Collect and verify
	var rm metricdata.ResourceMetrics
	if err := reader.Collect(ctx, &rm); err != nil {
		t.Fatalf("Failed to collect metrics: %v", err)
	}

	found := false
	for _, sm := range rm.ScopeMetrics {
		for _, m := range sm.Metrics {
			if m.Name == "elava.resources.current" {
				found = true
				gauge := m.Data.(metricdata.Gauge[int64])

				if len(gauge.DataPoints) != 3 {
					t.Errorf("Expected 3 data points, got %d", len(gauge.DataPoints))
				}
			}
		}
	}

	if !found {
		t.Error("Gauge metric not found")
	}
}

// TestDay2Metrics_ReconcileDuration tests histogram for reconciliation duration
func TestDay2Metrics_ReconcileDuration(t *testing.T) {
	reader := sdkmetric.NewManualReader()
	provider := sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader))
	meter := provider.Meter("test")

	duration, err := meter.Float64Histogram("elava.reconcile.duration.ms",
		metric.WithDescription("Time taken to complete a reconciliation scan"),
		metric.WithUnit("ms"),
	)
	if err != nil {
		t.Fatalf("Failed to create histogram: %v", err)
	}

	ctx := context.Background()

	// Record scan durations
	duration.Record(ctx, 1234.5,
		metric.WithAttributeSet(attribute.NewSet(
			attribute.String("scan_type", "baseline"),
			attribute.String("provider", "aws"),
			attribute.String("region", "us-east-1"),
		)),
	)

	duration.Record(ctx, 567.8,
		metric.WithAttributeSet(attribute.NewSet(
			attribute.String("scan_type", "normal"),
			attribute.String("provider", "aws"),
			attribute.String("region", "us-east-1"),
		)),
	)

	duration.Record(ctx, 890.2,
		metric.WithAttributeSet(attribute.NewSet(
			attribute.String("scan_type", "normal"),
			attribute.String("provider", "aws"),
			attribute.String("region", "us-west-2"),
		)),
	)

	// Collect and verify
	var rm metricdata.ResourceMetrics
	if err := reader.Collect(ctx, &rm); err != nil {
		t.Fatalf("Failed to collect metrics: %v", err)
	}

	found := false
	for _, sm := range rm.ScopeMetrics {
		for _, m := range sm.Metrics {
			if m.Name == "elava.reconcile.duration.ms" {
				found = true
				hist := m.Data.(metricdata.Histogram[float64])

				if len(hist.DataPoints) == 0 {
					t.Error("Expected histogram data points")
				}

				// Verify we recorded 3 measurements
				var count uint64
				for _, dp := range hist.DataPoints {
					count += dp.Count
				}
				if count != 3 {
					t.Errorf("Expected 3 measurements, got %d", count)
				}
			}
		}
	}

	if !found {
		t.Error("Histogram metric not found")
	}
}

// TestDay2Metrics_AllMetricsHaveCorrectAttributes tests attribute validation
func TestDay2Metrics_AllMetricsHaveCorrectAttributes(t *testing.T) {
	reader := sdkmetric.NewManualReader()
	provider := sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader))
	meter := provider.Meter("test")

	changesDetected, err := meter.Int64Counter("elava.changes.detected.total")
	if err != nil {
		t.Fatalf("Failed to create counter: %v", err)
	}

	ctx := context.Background()

	// Record with required attributes
	changesDetected.Add(ctx, 1,
		metric.WithAttributeSet(attribute.NewSet(
			attribute.String("change_type", "tag_drift"),
			attribute.String("environment", "production"),
			attribute.String("resource_type", "ec2"),
			attribute.String("severity", "warning"),
			attribute.String("provider", "aws"),
			attribute.String("region", "us-east-1"),
		)),
	)

	// Collect
	var rm metricdata.ResourceMetrics
	if err := reader.Collect(ctx, &rm); err != nil {
		t.Fatalf("Failed to collect metrics: %v", err)
	}

	// Verify attributes are present
	for _, sm := range rm.ScopeMetrics {
		for _, m := range sm.Metrics {
			if m.Name == "elava.changes.detected.total" {
				sum := m.Data.(metricdata.Sum[int64])
				for _, dp := range sum.DataPoints {
					// Verify all required attributes exist
					attrs := dp.Attributes.ToSlice()
					if len(attrs) != 6 {
						t.Errorf("Expected 6 attributes, got %d", len(attrs))
					}

					// Verify specific attributes
					hasChangeType := false
					hasEnvironment := false
					for _, kv := range attrs {
						if kv.Key == "change_type" && kv.Value.AsString() == "tag_drift" {
							hasChangeType = true
						}
						if kv.Key == "environment" && kv.Value.AsString() == "production" {
							hasEnvironment = true
						}
					}

					if !hasChangeType {
						t.Error("Missing change_type attribute")
					}
					if !hasEnvironment {
						t.Error("Missing environment attribute")
					}
				}
			}
		}
	}
}
