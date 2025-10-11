# Complete OpenTelemetry Solution for Elava

## Executive Summary

This document provides a complete, production-ready OpenTelemetry (OTEL) implementation design for Elava, based on official OTEL documentation and best practices.

**Key Findings:**
- ✅ **Traces**: Stable - Production ready
- ✅ **Metrics**: Stable - Production ready
- ⚠️  **Logs**: Beta - Production ready with caveats (use log bridges, not direct API)

## Architecture Overview

```
┌─────────────────────────────────────────────────────────────────┐
│                         Elava Application                        │
│  ┌────────────┐  ┌────────────┐  ┌────────────┐  ┌───────────┐ │
│  │ Reconciler │  │  Executor  │  │  Analyzer  │  │ Observers │ │
│  └─────┬──────┘  └─────┬──────┘  └─────┬──────┘  └─────┬─────┘ │
│        │                │                │                │       │
│        └────────────────┴────────────────┴────────────────┘       │
│                              │                                    │
│                    ┌─────────▼──────────┐                        │
│                    │  OTEL SDK (Go)     │                        │
│                    │  - Traces (Stable) │                        │
│                    │  - Metrics (Stable)│                        │
│                    │  - Logs (Bridge)   │                        │
│                    └─────────┬──────────┘                        │
└──────────────────────────────┼───────────────────────────────────┘
                               │ OTLP (gRPC/HTTP)
                               │
                    ┌──────────▼──────────┐
                    │  OTEL Collector     │
                    │  ┌───────────────┐  │
                    │  │  Receivers    │  │ ← OTLP, Prometheus
                    │  ├───────────────┤  │
                    │  │  Processors   │  │ ← Batch, Filter, Transform
                    │  ├───────────────┤  │
                    │  │  Exporters    │  │ ← Multiple backends
                    │  └───────────────┘  │
                    └──────────┬──────────┘
                               │
         ┌─────────────────────┼─────────────────────┐
         │                     │                     │
    ┌────▼────┐          ┌────▼────┐          ┌────▼────┐
    │Prometheus│         │  Loki   │         │ Jaeger  │
    │(Metrics) │         │ (Logs)  │         │(Traces) │
    └─────────┘          └─────────┘          └─────────┘
```

## Signal Implementation Strategy

### 1. Traces (✅ Stable - COMPLETE)

**Status**: Already implemented in PR #50

**What We Have:**
- Root reconciliation span
- Child spans: observe, detect, decide, execute
- Error recording with attributes
- Parent-child relationships via context

**What's Working:**
```go
ctx, reconSpan := telemetry.StartReconciliation(ctx, tracer, provider, region, scanType)
defer reconSpan.End()

ctx, observeSpan := telemetry.StartObserve(ctx, tracer, provider, region)
telemetry.EndObserve(observeSpan, resourcesScanned, durationSeconds)
```

**No Changes Needed** ✅

---

### 2. Metrics (✅ Stable - COMPLETE)

**Status**: Already implemented in PR #50

**What We Have:**
- Counters: changes.detected, decisions.made, violations.total
- Gauges: resources.current, resources.untagged, resources.blessed
- Histograms: reconcile.duration.ms, detect.duration.ms

**What's Working:**
```go
day2Metrics.RecordChangeDetected(ctx, "appeared", "ec2", "prod", "info", "aws", "us-east-1", 5)
day2Metrics.RecordDecisionMade(ctx, "notify", "ec2", "prod", false, 3)
day2Metrics.RecordReconcileDuration(ctx, "normal", "aws", "us-east-1", 1234.5)
```

**No Changes Needed** ✅

---

### 3. Logs (⚠️ Beta - NEEDS IMPLEMENTATION)

**Status**: NOT implemented - Was incorrectly skipped

**Official Stance:**
- **Logs API is Beta** - Production-ready for log bridges
- **No direct user-facing logs API** - Use log bridges instead
- **Recommended Approach**: Bridge existing logger (zerolog) to OTEL

**Current Situation:**
Elava uses `telemetry.Logger` which wraps `zerolog`. We need to bridge zerolog logs to OTEL.

**Implementation Strategy:**

#### Option A: Use OTEL Log Bridge for zerolog
```go
// Bridge zerolog to OTEL
import (
    "go.opentelemetry.io/otel/log/global"
    sdklog "go.opentelemetry.io/otel/sdk/log"
)

// Create OTEL log provider
logProvider := sdklog.NewLoggerProvider(
    sdklog.WithProcessor(sdklog.NewBatchProcessor(exporter)),
)
global.SetLoggerProvider(logProvider)

// Use zerolog with OTEL bridge hook
logger := zerolog.New(os.Stdout).Hook(otelZerologHook)
```

#### Option B: Structured Log Events via Traces
**RECOMMENDED for now** - Emit log events as span events:

```go
span.AddEvent("infrastructure.change.detected", trace.WithAttributes(
    attribute.String("change.type", "appeared"),
    attribute.String("resource.id", "i-1234"),
    attribute.String("resource.type", "ec2"),
    attribute.String("environment", "production"),
    attribute.String("severity", "info"),
))
```

**Why Option B:**
1. ✅ Traces are stable
2. ✅ Span events are correlated with traces automatically
3. ✅ No external dependencies
4. ✅ Works with existing OTEL setup
5. ✅ Can be filtered/queried in trace backends

**Implementation Plan:**
1. Add `RecordLogEvent()` helper in telemetry
2. Emit structured events as span events
3. Bridge to full OTEL logs when zerolog bridge is available

---

## Complete Implementation Checklist

### Phase 1: Log Events via Span Events (Current PR)
- [ ] Add `RecordLogEvent()` helper function
- [ ] Emit events for:
  - `infrastructure.change.detected`
  - `infrastructure.decision.made`
  - `infrastructure.action.executed`
  - `infrastructure.policy.violation`
  - `infrastructure.scan.completed`
- [ ] Update tests to verify span events
- [ ] Update PR #50 with log events

### Phase 2: Executor Instrumentation (Next PR)
- [ ] Add OTEL instrumentation to executor package
- [ ] Trace action execution spans
- [ ] Record action metrics (success/failure)
- [ ] Add executor-specific log events
- [ ] Tests for executor telemetry

### Phase 3: OTEL Collector Configs (Documentation)
- [ ] Create example collector configs
- [ ] Prometheus + Loki + Jaeger setup
- [ ] Datadog all-in-one setup
- [ ] Grafana Cloud setup
- [ ] Multi-destination routing example
- [ ] Security/filtering examples

### Phase 4: Documentation (README)
- [ ] OTEL architecture diagram
- [ ] Quick start guide
- [ ] Collector setup instructions
- [ ] Query examples (PromQL, LogQL, TraceQL)
- [ ] Troubleshooting guide

### Phase 5: Future Enhancements
- [ ] Evaluate zerolog OTEL bridge when available
- [ ] Migrate from span events to proper OTEL logs
- [ ] Add integration tests with real OTEL Collector
- [ ] Add Grafana dashboard templates
- [ ] Add alert rule examples

---

## Recommended Solution Design

### Core Telemetry Module

```go
// telemetry/otel.go
package telemetry

import (
    "go.opentelemetry.io/otel"
    "go.opentelemetry.io/otel/attribute"
    "go.opentelemetry.io/otel/metric"
    "go.opentelemetry.io/otel/trace"
)

// Complete OTEL setup
type OTELTelemetry struct {
    Tracer      trace.Tracer
    Meter       metric.Meter
    Day2Metrics *Day2Metrics
}

// Initialize with all signals
func InitOTEL(ctx context.Context, cfg Config) (*OTELTelemetry, func(context.Context) error, error) {
    // Setup trace provider (already done)
    traceShutdown, err := setupTraceProvider(ctx, cfg, res)

    // Setup metric provider (already done)
    metricShutdown, err := setupMetricProvider(ctx, cfg, res)

    // Initialize Day 2 metrics
    day2Metrics, err := InitDay2Metrics(meter)

    return &OTELTelemetry{
        Tracer:      tracer,
        Meter:       meter,
        Day2Metrics: day2Metrics,
    }, combinedShutdown, nil
}
```

### Log Events via Span Events

```go
// telemetry/log_events.go
package telemetry

// RecordChangeDetectedEvent emits a structured log event as a span event
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

// RecordDecisionMadeEvent emits a decision log event
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

// RecordActionExecutedEvent emits an action execution log event
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

// RecordPolicyViolationEvent emits a policy violation log event
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

// RecordScanCompletedEvent emits a scan completion log event
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
```

### Usage in Reconciler

```go
// reconciler/reconciler.go
func (e *Engine) detectAndDecide(ctx context.Context, current []types.Resource, config Config) ([]types.Decision, error) {
    ctx, detectSpan := telemetry.StartDetect(ctx, e.tracer, config.Provider, config.Region)

    changes, err := e.changeDetector.DetectChanges(ctx, current)
    if err != nil {
        telemetry.RecordError(detectSpan, err.Error(), "ChangeDetectionError")
        detectSpan.End()
        return nil, err
    }

    // Emit log events for each change
    for _, change := range changes {
        telemetry.RecordChangeDetectedEvent(
            detectSpan,
            string(change.Type),
            change.ResourceID,
            change.ResourceType,
            e.determineEnvironment(config),
            e.determineSeverity(change),
            config.Provider,
            config.Region,
            change.Description,
        )
    }

    telemetry.EndDetect(detectSpan, ...)

    // Continue with decisions...
}
```

---

## OTEL Collector Configuration Examples

### Example 1: Prometheus + Loki + Jaeger

```yaml
# otel-collector-config.yaml
receivers:
  otlp:
    protocols:
      grpc:
        endpoint: 0.0.0.0:4317
      http:
        endpoint: 0.0.0.0:4318

processors:
  batch:
    timeout: 10s
    send_batch_size: 1024

  # Add resource attributes
  resource:
    attributes:
      - key: service.name
        value: elava
        action: upsert
      - key: deployment.environment
        from_attribute: environment
        action: insert

  # Filter sensitive data
  attributes:
    actions:
      - key: aws.access_key
        action: delete
      - key: resource.tags.secret
        action: delete

exporters:
  # Metrics to Prometheus
  prometheus:
    endpoint: "0.0.0.0:8889"
    namespace: elava
    const_labels:
      service: elava

  # Logs to Loki
  loki:
    endpoint: http://loki:3100/loki/api/v1/push
    labels:
      resource:
        service.name: "service_name"
        environment: "environment"
      attributes:
        severity: "severity"
        resource.type: "resource_type"

  # Traces to Jaeger
  jaeger:
    endpoint: jaeger:14250
    tls:
      insecure: true

  # Debug exporter (for testing)
  debug:
    verbosity: detailed

service:
  pipelines:
    traces:
      receivers: [otlp]
      processors: [batch, resource, attributes]
      exporters: [jaeger, debug]

    metrics:
      receivers: [otlp]
      processors: [batch, resource]
      exporters: [prometheus, debug]

    logs:
      receivers: [otlp]
      processors: [batch, resource, attributes]
      exporters: [loki, debug]
```

### Example 2: Datadog All-in-One

```yaml
# otel-collector-datadog.yaml
receivers:
  otlp:
    protocols:
      grpc:
        endpoint: 0.0.0.0:4317
      http:
        endpoint: 0.0.0.0:4318

processors:
  batch:
    timeout: 10s
    send_batch_size: 1024

exporters:
  datadog:
    api:
      key: ${DD_API_KEY}
      site: datadoghq.com

    # Map service name
    hostname: elava-${HOSTNAME}

    # Send all signals
    traces:
      endpoint: https://trace.agent.datadoghq.com
    metrics:
      endpoint: https://api.datadoghq.com
    logs:
      endpoint: https://http-intake.logs.datadoghq.com

service:
  pipelines:
    traces:
      receivers: [otlp]
      processors: [batch]
      exporters: [datadog]

    metrics:
      receivers: [otlp]
      processors: [batch]
      exporters: [datadog]

    logs:
      receivers: [otlp]
      processors: [batch]
      exporters: [datadog]
```

### Example 3: Multi-Destination Routing

```yaml
# otel-collector-multi.yaml
receivers:
  otlp:
    protocols:
      grpc:
        endpoint: 0.0.0.0:4317

processors:
  batch:

  # Route critical logs to Slack
  filter/critical:
    logs:
      log_record:
        - 'attributes["severity"] == "critical"'

exporters:
  # Primary: Prometheus + Loki + Jaeger
  prometheus:
    endpoint: "0.0.0.0:8889"
  loki:
    endpoint: http://loki:3100/loki/api/v1/push
  jaeger:
    endpoint: jaeger:14250

  # Secondary: Long-term storage (S3)
  s3:
    region: us-east-1
    s3_bucket: elava-telemetry-archive
    s3_prefix: otel/

  # Alerts: Critical logs to Slack
  webhook/slack:
    endpoint: https://hooks.slack.com/services/YOUR/WEBHOOK/URL
    headers:
      Content-Type: application/json

service:
  pipelines:
    traces:
      receivers: [otlp]
      processors: [batch]
      exporters: [jaeger, s3]

    metrics:
      receivers: [otlp]
      processors: [batch]
      exporters: [prometheus, s3]

    logs:
      receivers: [otlp]
      processors: [batch]
      exporters: [loki, s3]

    logs/critical:
      receivers: [otlp]
      processors: [filter/critical]
      exporters: [webhook/slack]
```

---

## Benefits of Complete OTEL Implementation

### 1. **Vendor Neutrality**
- Route to 90+ observability vendors
- Switch backends without code changes
- Multi-cloud observability

### 2. **Unified Observability**
- Single SDK for all signals
- Correlated traces, metrics, logs
- Consistent semantic conventions

### 3. **Cost Optimization**
- Filter/sample in collector, not application
- Send to multiple backends simultaneously
- Archive to cheap storage (S3) for compliance

### 4. **Operational Simplicity**
- One configuration format (YAML)
- Centralized telemetry processing
- Easy to add new backends

### 5. **Production Ready**
- Industry standard (CNCF project)
- Battle-tested by major companies
- Active community and ecosystem

---

## Migration Path

### Current State (PR #50)
✅ Traces: Complete
✅ Metrics: Complete
❌ Logs: Missing

### Next Steps

**Week 1: Add Log Events**
- Implement `RecordLogEvent()` helpers
- Emit events as span events
- Update reconciler instrumentation
- Add tests

**Week 2: Executor Instrumentation**
- Add executor traces
- Add executor metrics
- Add executor log events
- Tests

**Week 3: OTEL Collector Setup**
- Create example configs
- Test with real collector
- Document setup

**Week 4: Documentation**
- Write README section
- Add query examples
- Create troubleshooting guide

---

## Testing Strategy

### Unit Tests
```go
func TestRecordChangeDetectedEvent(t *testing.T) {
    exporter := tracetest.NewInMemoryExporter()
    provider := sdktrace.NewTracerProvider(
        sdktrace.WithSyncer(exporter),
    )
    tracer := provider.Tracer("test")

    ctx, span := tracer.Start(context.Background(), "test")

    telemetry.RecordChangeDetectedEvent(
        span,
        "appeared",
        "i-123",
        "ec2",
        "prod",
        "info",
        "aws",
        "us-east-1",
        "New instance detected",
    )

    span.End()
    provider.ForceFlush(ctx)

    spans := exporter.GetSpans()
    require.Len(t, spans, 1)

    events := spans[0].Events
    require.Len(t, events, 1)
    assert.Equal(t, "infrastructure.change.detected", events[0].Name)
}
```

### Integration Tests
```go
func TestOTELCollectorIntegration(t *testing.T) {
    // Start OTEL Collector in Docker
    collector := startOTELCollector(t)
    defer collector.Stop()

    // Configure Elava to use collector
    cfg := telemetry.Config{
        OTELEndpoint: collector.Endpoint(),
        Insecure: true,
    }

    // Run reconciliation
    reconciler.Reconcile(ctx, config)

    // Verify metrics in Prometheus
    metrics := collector.GetMetrics()
    assert.Contains(t, metrics, "elava_changes_detected_total")

    // Verify traces in Jaeger
    traces := collector.GetTraces()
    assert.Contains(t, traces, "reconciliation")
}
```

---

## Conclusion

This complete OTEL solution provides:

1. ✅ **Production-ready telemetry** for all three signals
2. ✅ **Vendor-neutral architecture** - route to any backend
3. ✅ **Best practices** - follows official OTEL documentation
4. ✅ **Pragmatic approach** - uses span events for logs (stable) instead of beta logs API
5. ✅ **Migration path** - clear steps to complete implementation

**Key Decision: Use span events for structured logs** instead of waiting for OTEL logs API to stabilize. This gives us:
- ✅ Stable foundation (traces are stable)
- ✅ Automatic correlation with traces
- ✅ Works with existing backends
- ✅ Easy migration path when OTEL logs mature

**Next Action**: Implement log events via span events in PR #50.
