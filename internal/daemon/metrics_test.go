package daemon

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
)

// TestDaemonMetrics_SemanticConventions verifies metric names follow OTEL conventions
func TestDaemonMetrics_SemanticConventions(t *testing.T) {
	reader := sdkmetric.NewManualReader()
	provider := sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader))

	// This will fail until we implement semantic conventions
	// Expected: elava.daemon.reconciliations
	// Current: daemon_reconcile_runs_total

	meter := provider.Meter("elava.daemon")

	reconciliations, err := meter.Int64Counter(
		"elava.daemon.reconciliations",
		// Note: WithUnit should use {reconciliation} not {runs}
	)
	require.NoError(t, err)
	require.NotNil(t, reconciliations)

	// Record with attributes (not separate metrics)
	ctx := context.Background()
	reconciliations.Add(ctx, 1,
		metric.WithAttributes(
			attribute.String("status", "success"),
			attribute.String("cloud.provider", "aws"),
			attribute.String("cloud.region", "us-east-1"),
		),
	)

	// Verify metric recorded
	var rm metricdata.ResourceMetrics
	err = reader.Collect(ctx, &rm)
	require.NoError(t, err)

	// Check metric name and attributes
	require.Len(t, rm.ScopeMetrics, 1)
	require.Len(t, rm.ScopeMetrics[0].Metrics, 1)

	metricData := rm.ScopeMetrics[0].Metrics[0]
	assert.Equal(t, "elava.daemon.reconciliations", metricData.Name)
}

// TestDaemonMetrics_RecordReconciliation tests helper method
func TestDaemonMetrics_RecordReconciliation(t *testing.T) {
	reader := sdkmetric.NewManualReader()
	provider := sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader))

	// Create metrics with provider (will fail until implemented)
	dm, err := newDaemonMetricsWithProvider(provider)
	require.NoError(t, err)

	ctx := context.Background()

	// Test success recording
	dm.RecordReconciliation(ctx, "success", "aws", "us-east-1")

	var rm metricdata.ResourceMetrics
	err = reader.Collect(ctx, &rm)
	require.NoError(t, err)

	// Verify attributes are present
	require.Len(t, rm.ScopeMetrics, 1)
	require.Len(t, rm.ScopeMetrics[0].Metrics, 1)

	metricData := rm.ScopeMetrics[0].Metrics[0]
	sum := metricData.Data.(metricdata.Sum[int64])
	require.Len(t, sum.DataPoints, 1)

	dp := sum.DataPoints[0]
	assert.Equal(t, int64(1), dp.Value)

	// Check attributes
	attrs := dp.Attributes.ToSlice()
	assert.Contains(t, attrs, attribute.String("status", "success"))
	assert.Contains(t, attrs, attribute.String("cloud.provider", "aws"))
	assert.Contains(t, attrs, attribute.String("cloud.region", "us-east-1"))
}

// TestDaemonMetrics_RecordReconciliationDuration tests duration histogram
func TestDaemonMetrics_RecordReconciliationDuration(t *testing.T) {
	reader := sdkmetric.NewManualReader()
	provider := sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader))

	dm, err := newDaemonMetricsWithProvider(provider)
	require.NoError(t, err)

	ctx := context.Background()

	// Record duration
	dm.RecordReconciliationDuration(ctx, 5.5, "success")

	var rm metricdata.ResourceMetrics
	err = reader.Collect(ctx, &rm)
	require.NoError(t, err)

	// Find duration metric
	var foundDuration bool
	for _, sm := range rm.ScopeMetrics {
		for _, m := range sm.Metrics {
			if m.Name == "elava.daemon.reconciliation.duration" {
				foundDuration = true

				hist := m.Data.(metricdata.Histogram[float64])
				require.Len(t, hist.DataPoints, 1)

				dp := hist.DataPoints[0]
				assert.Equal(t, float64(5.5), dp.Sum)
				assert.Equal(t, uint64(1), dp.Count)

				// Check status attribute
				attrs := dp.Attributes.ToSlice()
				assert.Contains(t, attrs, attribute.String("status", "success"))
			}
		}
	}
	assert.True(t, foundDuration, "duration metric not found")
}

// TestDaemonMetrics_RecordResourcesDiscovered tests gauge metric
func TestDaemonMetrics_RecordResourcesDiscovered(t *testing.T) {
	reader := sdkmetric.NewManualReader()
	provider := sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader))

	dm, err := newDaemonMetricsWithProvider(provider)
	require.NoError(t, err)

	ctx := context.Background()

	// Record discovered resources
	dm.RecordResourcesDiscovered(ctx, 42, "ec2", "aws", "us-east-1")

	var rm metricdata.ResourceMetrics
	err = reader.Collect(ctx, &rm)
	require.NoError(t, err)

	// Find resources discovered metric
	var foundResources bool
	for _, sm := range rm.ScopeMetrics {
		for _, m := range sm.Metrics {
			if m.Name == "elava.resources.discovered" {
				foundResources = true

				gauge := m.Data.(metricdata.Gauge[int64])
				require.Len(t, gauge.DataPoints, 1)

				dp := gauge.DataPoints[0]
				assert.Equal(t, int64(42), dp.Value)

				// Check attributes
				attrs := dp.Attributes.ToSlice()
				assert.Contains(t, attrs, attribute.String("resource.type", "ec2"))
				assert.Contains(t, attrs, attribute.String("cloud.provider", "aws"))
				assert.Contains(t, attrs, attribute.String("cloud.region", "us-east-1"))
			}
		}
	}
	assert.True(t, foundResources, "resources discovered metric not found")
}

// TestDaemonMetrics_RecordChangeEvent tests change event counter
func TestDaemonMetrics_RecordChangeEvent(t *testing.T) {
	reader := sdkmetric.NewManualReader()
	provider := sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader))

	dm, err := newDaemonMetricsWithProvider(provider)
	require.NoError(t, err)

	ctx := context.Background()

	// Record change events
	dm.RecordChangeEvent(ctx, "created", "ec2", "us-east-1")
	dm.RecordChangeEvent(ctx, "modified", "ec2", "us-east-1")
	dm.RecordChangeEvent(ctx, "disappeared", "ec2", "us-east-1")

	var rm metricdata.ResourceMetrics
	err = reader.Collect(ctx, &rm)
	require.NoError(t, err)

	// Find change events metric
	var foundEvents bool
	for _, sm := range rm.ScopeMetrics {
		for _, m := range sm.Metrics {
			if m.Name == "elava.change_events" {
				foundEvents = true

				sum := m.Data.(metricdata.Sum[int64])

				// Should have 3 data points (one per change type)
				assert.Len(t, sum.DataPoints, 3)

				// Verify each change type
				changeTypes := make(map[string]int64)
				for _, dp := range sum.DataPoints {
					attrs := dp.Attributes.ToSlice()
					for _, attr := range attrs {
						if attr.Key == "change.type" {
							changeTypes[attr.Value.AsString()] = dp.Value
						}
					}
				}

				assert.Equal(t, int64(1), changeTypes["created"])
				assert.Equal(t, int64(1), changeTypes["modified"])
				assert.Equal(t, int64(1), changeTypes["disappeared"])
			}
		}
	}
	assert.True(t, foundEvents, "change events metric not found")
}

// TestDaemonMetrics_RecordStorageOperation tests storage operations counter
func TestDaemonMetrics_RecordStorageOperation(t *testing.T) {
	reader := sdkmetric.NewManualReader()
	provider := sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader))

	dm, err := newDaemonMetricsWithProvider(provider)
	require.NoError(t, err)

	ctx := context.Background()

	// Record storage operations
	dm.RecordStorageOperation(ctx, "write", "success", "")
	dm.RecordStorageOperation(ctx, "write", "failure", "disk_full")

	var rm metricdata.ResourceMetrics
	err = reader.Collect(ctx, &rm)
	require.NoError(t, err)

	// Find storage operations metric
	var foundOps bool
	for _, sm := range rm.ScopeMetrics {
		for _, m := range sm.Metrics {
			if m.Name == "elava.storage.operations" {
				foundOps = true

				sum := m.Data.(metricdata.Sum[int64])
				assert.Len(t, sum.DataPoints, 2)

				// Check failure has error.type attribute
				for _, dp := range sum.DataPoints {
					attrs := dp.Attributes.ToSlice()
					hasStatus := false
					for _, attr := range attrs {
						if attr.Key == "status" && attr.Value.AsString() == "failure" {
							hasStatus = true
							// Check error.type is present
							hasErrorType := false
							for _, a := range attrs {
								if a.Key == "error.type" {
									hasErrorType = true
									assert.Equal(t, "disk_full", a.Value.AsString())
								}
							}
							assert.True(t, hasErrorType, "failure should have error.type attribute")
						}
					}
					if hasStatus {
						break
					}
				}
			}
		}
	}
	assert.True(t, foundOps, "storage operations metric not found")
}

// TestDaemonMetrics_HistogramBuckets tests explicit bucket boundaries
func TestDaemonMetrics_HistogramBuckets(t *testing.T) {
	reader := sdkmetric.NewManualReader()

	// Configure histogram view with explicit buckets
	view := sdkmetric.NewView(
		sdkmetric.Instrument{Name: "elava.daemon.reconciliation.duration"},
		sdkmetric.Stream{
			Aggregation: sdkmetric.AggregationExplicitBucketHistogram{
				Boundaries: []float64{1, 5, 10, 30, 60, 120, 300},
			},
		},
	)

	provider := sdkmetric.NewMeterProvider(
		sdkmetric.WithReader(reader),
		sdkmetric.WithView(view),
	)

	dm, err := newDaemonMetricsWithProvider(provider)
	require.NoError(t, err)

	ctx := context.Background()

	// Record various durations
	durations := []float64{0.5, 3.0, 8.0, 25.0, 45.0, 90.0, 180.0}
	for _, d := range durations {
		dm.RecordReconciliationDuration(ctx, d, "success")
	}

	var rm metricdata.ResourceMetrics
	err = reader.Collect(ctx, &rm)
	require.NoError(t, err)

	// Find duration histogram
	for _, sm := range rm.ScopeMetrics {
		for _, m := range sm.Metrics {
			if m.Name == "elava.daemon.reconciliation.duration" {
				hist := m.Data.(metricdata.Histogram[float64])
				require.Len(t, hist.DataPoints, 1)

				dp := hist.DataPoints[0]

				// Verify explicit buckets match design doc: 1, 5, 10, 30, 60, 120, 300
				expectedBuckets := []float64{1, 5, 10, 30, 60, 120, 300}
				assert.Equal(t, expectedBuckets, dp.Bounds)

				// Verify bucket counts
				assert.Equal(t, uint64(7), dp.Count)
			}
		}
	}
}

// Helper function to create DaemonMetrics with custom provider (for testing)
// This will fail until we implement it
func newDaemonMetricsWithProvider(provider *sdkmetric.MeterProvider) (*DaemonMetrics, error) {
	meter := provider.Meter("elava.daemon")

	reconciliations, err := meter.Int64Counter(
		"elava.daemon.reconciliations",
		// Implementation will go here
	)
	if err != nil {
		return nil, err
	}

	reconciliationDuration, err := meter.Float64Histogram(
		"elava.daemon.reconciliation.duration",
		// Implementation will go here
	)
	if err != nil {
		return nil, err
	}

	resourcesDiscovered, err := meter.Int64Gauge(
		"elava.resources.discovered",
		// Implementation will go here
	)
	if err != nil {
		return nil, err
	}

	changeEvents, err := meter.Int64Counter(
		"elava.change_events",
		// Implementation will go here
	)
	if err != nil {
		return nil, err
	}

	storageOperations, err := meter.Int64Counter(
		"elava.storage.operations",
		// Implementation will go here
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
