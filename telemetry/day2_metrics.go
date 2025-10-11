package telemetry

import (
	"context"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

// Day2Metrics holds all Day 2 operations metrics
type Day2Metrics struct {
	// Counters
	ChangesDetected  metric.Int64Counter
	DecisionsMade    metric.Int64Counter
	PolicyViolations metric.Int64Counter
	ActionsExecuted  metric.Int64Counter

	// Gauges
	ResourcesCurrent  metric.Int64Gauge
	ResourcesUntagged metric.Int64Gauge
	ResourcesBlessed  metric.Int64Gauge

	// Histograms
	ReconcileDuration metric.Float64Histogram
	DetectDuration    metric.Float64Histogram
}

// InitDay2Metrics initializes all Day 2 operations metrics
func InitDay2Metrics(meter metric.Meter) (*Day2Metrics, error) {
	m := &Day2Metrics{}

	if err := m.initCounters(meter); err != nil {
		return nil, err
	}

	if err := m.initGauges(meter); err != nil {
		return nil, err
	}

	if err := m.initHistograms(meter); err != nil {
		return nil, err
	}

	return m, nil
}

// initCounters initializes counter metrics
func (m *Day2Metrics) initCounters(meter metric.Meter) error {
	var err error

	m.ChangesDetected, err = meter.Int64Counter(
		"elava.changes.detected.total",
		metric.WithDescription("Total number of infrastructure changes detected"),
		metric.WithUnit("changes"),
	)
	if err != nil {
		return err
	}

	m.DecisionsMade, err = meter.Int64Counter(
		"elava.decisions.made.total",
		metric.WithDescription("Total number of policy decisions made"),
		metric.WithUnit("decisions"),
	)
	if err != nil {
		return err
	}

	m.PolicyViolations, err = meter.Int64Counter(
		"elava.policy.violations.total",
		metric.WithDescription("Total number of policy violations detected"),
		metric.WithUnit("violations"),
	)
	if err != nil {
		return err
	}

	m.ActionsExecuted, err = meter.Int64Counter(
		"elava.actions.executed.total",
		metric.WithDescription("Total number of actions executed"),
		metric.WithUnit("actions"),
	)
	if err != nil {
		return err
	}

	return nil
}

// initGauges initializes gauge metrics
func (m *Day2Metrics) initGauges(meter metric.Meter) error {
	var err error

	m.ResourcesCurrent, err = meter.Int64Gauge(
		"elava.resources.current",
		metric.WithDescription("Current number of resources being managed"),
		metric.WithUnit("resources"),
	)
	if err != nil {
		return err
	}

	m.ResourcesUntagged, err = meter.Int64Gauge(
		"elava.resources.untagged",
		metric.WithDescription("Current number of untagged resources"),
		metric.WithUnit("resources"),
	)
	if err != nil {
		return err
	}

	m.ResourcesBlessed, err = meter.Int64Gauge(
		"elava.resources.blessed",
		metric.WithDescription("Current number of blessed resources"),
		metric.WithUnit("resources"),
	)
	if err != nil {
		return err
	}

	return nil
}

// initHistograms initializes histogram metrics
func (m *Day2Metrics) initHistograms(meter metric.Meter) error {
	var err error

	m.ReconcileDuration, err = meter.Float64Histogram(
		"elava.reconcile.duration.ms",
		metric.WithDescription("Time taken to complete a reconciliation scan"),
		metric.WithUnit("ms"),
	)
	if err != nil {
		return err
	}

	m.DetectDuration, err = meter.Float64Histogram(
		"elava.detect.duration.ms",
		metric.WithDescription("Time taken to detect changes"),
		metric.WithUnit("ms"),
	)
	if err != nil {
		return err
	}

	return nil
}

// RecordChangeDetected records an infrastructure change detection
func (m *Day2Metrics) RecordChangeDetected(
	ctx context.Context,
	changeType string,
	resourceType string,
	environment string,
	severity string,
	provider string,
	region string,
	count int64,
) {
	m.ChangesDetected.Add(ctx, count,
		metric.WithAttributeSet(attribute.NewSet(
			attribute.String("change_type", changeType),
			attribute.String("resource_type", resourceType),
			attribute.String("environment", environment),
			attribute.String("severity", severity),
			attribute.String("provider", provider),
			attribute.String("region", region),
		)),
	)
}

// RecordDecisionMade records a policy decision
func (m *Day2Metrics) RecordDecisionMade(
	ctx context.Context,
	action string,
	resourceType string,
	environment string,
	isBlessed bool,
	count int64,
) {
	m.DecisionsMade.Add(ctx, count,
		metric.WithAttributeSet(attribute.NewSet(
			attribute.String("action", action),
			attribute.String("resource_type", resourceType),
			attribute.String("environment", environment),
			attribute.Bool("is_blessed", isBlessed),
		)),
	)
}

// RecordPolicyViolation records a policy violation
func (m *Day2Metrics) RecordPolicyViolation(
	ctx context.Context,
	policyName string,
	severity string,
	environment string,
	autoRemediated bool,
	count int64,
) {
	m.PolicyViolations.Add(ctx, count,
		metric.WithAttributeSet(attribute.NewSet(
			attribute.String("policy_name", policyName),
			attribute.String("severity", severity),
			attribute.String("environment", environment),
			attribute.Bool("auto_remediated", autoRemediated),
		)),
	)
}

// RecordActionExecuted records an action execution
func (m *Day2Metrics) RecordActionExecuted(
	ctx context.Context,
	actionType string,
	resourceType string,
	environment string,
	status string,
	count int64,
) {
	m.ActionsExecuted.Add(ctx, count,
		metric.WithAttributeSet(attribute.NewSet(
			attribute.String("action_type", actionType),
			attribute.String("resource_type", resourceType),
			attribute.String("environment", environment),
			attribute.String("status", status),
		)),
	)
}

// RecordCurrentResources records current resource count
func (m *Day2Metrics) RecordCurrentResources(
	ctx context.Context,
	resourceType string,
	environment string,
	state string,
	count int64,
) {
	m.ResourcesCurrent.Record(ctx, count,
		metric.WithAttributeSet(attribute.NewSet(
			attribute.String("resource_type", resourceType),
			attribute.String("environment", environment),
			attribute.String("state", state),
		)),
	)
}

// RecordReconcileDuration records reconciliation duration
func (m *Day2Metrics) RecordReconcileDuration(
	ctx context.Context,
	scanType string,
	provider string,
	region string,
	durationMs float64,
) {
	m.ReconcileDuration.Record(ctx, durationMs,
		metric.WithAttributeSet(attribute.NewSet(
			attribute.String("scan_type", scanType),
			attribute.String("provider", provider),
			attribute.String("region", region),
		)),
	)
}
