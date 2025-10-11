# OpenTelemetry Schema for Elava Day 2 Operations

## Overview

Elava uses OpenTelemetry (OTEL) as the **universal telemetry layer** for all observability data. This allows users to route metrics, logs, and traces to ANY backend without changing Elava's code.

### Design Philosophy

```
┌─────────────────────────────────────────────────────────┐
│  "Elava emits OTEL, users decide where it goes"         │
└─────────────────────────────────────────────────────────┘

Elava (simple) → OTEL Collector (powerful) → Anywhere

Benefits:
  ✅ No vendor lock-in
  ✅ Route to multiple backends simultaneously
  ✅ Change routing without redeploying Elava
  ✅ Standard observability stack
  ✅ Follows CLAUDE.md (OTEL only, no wrappers)
```

### Supported Destinations

Via OTEL Collector, Elava data can go to:
- **Metrics**: Prometheus, Datadog, Grafana Cloud, InfluxDB, Victoria Metrics
- **Logs**: Loki, CloudWatch, Splunk, Elasticsearch, stdout, files
- **Traces**: Jaeger, Tempo, Zipkin, Datadog APM
- **Webhooks**: Slack, Discord, PagerDuty, custom APIs
- **Your Device**: Any HTTP endpoint you control

## Architecture

```
┌────────────────────────────────────────────────────────────┐
│                    Elava Components                         │
│  ┌──────────┐  ┌──────────┐  ┌──────────┐  ┌──────────┐  │
│  │ Observer │  │ Detector │  │  Policy  │  │ Executor │  │
│  └────┬─────┘  └────┬─────┘  └────┬─────┘  └────┬─────┘  │
│       │             │               │             │         │
│       └─────────────┴───────────────┴─────────────┘         │
│                           │                                  │
│                    Emit OTEL (OTLP)                         │
└─────────────────────────────┬──────────────────────────────┘
                              │
                    ┌─────────▼──────────┐
                    │  OTEL Collector    │
                    │  (User configures) │
                    └─────────┬──────────┘
                              │
        ┌─────────────────────┼─────────────────────┐
        │                     │                     │
   ┌────▼────┐          ┌─────▼─────┐        ┌─────▼─────┐
   │ Metrics │          │   Logs    │        │  Traces   │
   └────┬────┘          └─────┬─────┘        └─────┬─────┘
        │                     │                     │
   ┌────┼────┐          ┌─────┼─────┐        ┌─────┼─────┐
   ▼    ▼    ▼          ▼     ▼     ▼        ▼     ▼     ▼
 Prom  DD  Grafana    Loki  CW   stdout   Jaeger Tempo Zipkin

                      ┌───────────────┐
                      │   Webhooks    │
                      │  (processors) │
                      └───────┬───────┘
                              │
                      ┌───────┼───────┐
                      ▼       ▼       ▼
                   Slack  Discord  Your API
```

## Metrics Schema

### Resource Attributes (Global)

All metrics include these resource attributes:

```go
service.name = "elava"
service.version = "1.0.0"
deployment.environment = "production"
cloud.provider = "aws"
cloud.region = "us-east-1"
elava.instance.id = "elava-prod-01"
```

### Counter Metrics

#### `elava.changes.detected.total`
Total number of infrastructure changes detected.

**Type**: Counter
**Unit**: changes
**Attributes**:
- `change_type` (string): baseline|appeared|disappeared|modified|tag_drift|status_changed|unmanaged
- `environment` (string): production|staging|development|unknown
- `resource_type` (string): ec2|rds|s3|elb|sg|...
- `severity` (string): critical|warning|info|debug
- `provider` (string): aws|gcp|azure
- `region` (string): us-east-1|eu-west-1|...

**Example Queries**:
```promql
# Total changes in last hour
sum(rate(elava_changes_detected_total[1h]))

# Critical changes by environment
sum by(environment) (elava_changes_detected_total{severity="critical"})

# Tag drift rate
rate(elava_changes_detected_total{change_type="tag_drift"}[5m])
```

#### `elava.decisions.made.total`
Total number of policy decisions made.

**Type**: Counter
**Unit**: decisions
**Attributes**:
- `action` (string): notify|alert|protect|enforce_tags|enforce_policy|auto_tag|ignore|audit
- `resource_type` (string): ec2|rds|s3|...
- `environment` (string): production|staging|...
- `is_blessed` (bool): true|false

**Example Queries**:
```promql
# Alerts sent per hour
sum(rate(elava_decisions_made_total{action="alert"}[1h]))

# Auto-remediation rate
sum(rate(elava_decisions_made_total{action=~"enforce.*|auto_tag"}[5m]))
```

#### `elava.actions.executed.total`
Total number of actions executed.

**Type**: Counter
**Unit**: actions
**Attributes**:
- `action` (string): notify|alert|protect|...
- `status` (string): success|failed|skipped
- `resource_type` (string): ec2|rds|...

**Example Queries**:
```promql
# Success rate
sum(rate(elava_actions_executed_total{status="success"}[5m]))
/
sum(rate(elava_actions_executed_total[5m]))

# Failed actions
sum by(action) (elava_actions_executed_total{status="failed"})
```

#### `elava.policy.violations.total`
Total number of policy violations detected.

**Type**: Counter
**Unit**: violations
**Attributes**:
- `policy_name` (string): databases-must-be-private|volumes-must-be-encrypted|...
- `severity` (string): critical|high|medium|low
- `environment` (string): production|staging|...
- `auto_remediated` (bool): true|false

**Example Queries**:
```promql
# Critical violations in production
sum(elava_policy_violations_total{severity="critical",environment="production"})

# Auto-remediation effectiveness
sum(rate(elava_policy_violations_total{auto_remediated="true"}[1h]))
```

#### `elava.resources.observed.total`
Total number of resources observed in scans.

**Type**: Counter
**Unit**: resources
**Attributes**:
- `resource_type` (string): ec2|rds|...
- `environment` (string): production|staging|...
- `provider` (string): aws|gcp|...
- `region` (string): us-east-1|...
- `has_owner` (bool): true|false
- `is_blessed` (bool): true|false

### Gauge Metrics

#### `elava.resources.current`
Current number of resources being managed.

**Type**: Gauge
**Unit**: resources
**Attributes**:
- `resource_type` (string): ec2|rds|...
- `environment` (string): production|...
- `state` (string): running|stopped|available|...

**Example Queries**:
```promql
# Total EC2 instances
elava_resources_current{resource_type="ec2"}

# Running production instances
elava_resources_current{resource_type="ec2",environment="production",state="running"}
```

#### `elava.resources.untagged`
Number of resources missing required tags.

**Type**: Gauge
**Unit**: resources
**Attributes**:
- `resource_type` (string): ec2|rds|...
- `missing_tag` (string): owner|environment|team|...

**Example Queries**:
```promql
# Resources missing owner tag
sum by(resource_type) (elava_resources_untagged{missing_tag="owner"})
```

#### `elava.resources.blessed`
Number of protected/blessed resources.

**Type**: Gauge
**Unit**: resources
**Attributes**:
- `resource_type` (string): ec2|rds|...
- `environment` (string): production|...

### Histogram Metrics

#### `elava.reconcile.duration.ms`
Time taken to complete a reconciliation scan.

**Type**: Histogram
**Unit**: milliseconds
**Attributes**:
- `scan_type` (string): baseline|normal
- `provider` (string): aws|gcp|...
- `region` (string): us-east-1|...

**Example Queries**:
```promql
# P95 reconciliation time
histogram_quantile(0.95, rate(elava_reconcile_duration_ms_bucket[5m]))

# Average scan duration by type
avg by(scan_type) (rate(elava_reconcile_duration_ms_sum[5m])
/
rate(elava_reconcile_duration_ms_count[5m]))
```

#### `elava.detect.duration.ms`
Time taken to detect changes.

**Type**: Histogram
**Unit**: milliseconds
**Attributes**:
- `resource_count` (int): Number of resources processed

#### `elava.policy.evaluation.duration.ms`
Time taken to evaluate policies.

**Type**: Histogram
**Unit**: milliseconds
**Attributes**:
- `policy_count` (int): Number of policies evaluated

## Log Events Schema

All log events are structured JSON following OTEL log data model.

### Common Fields

Every log event includes:

```json
{
  "timestamp": "2025-10-10T14:23:45.123Z",
  "level": "debug|info|warn|error",
  "service": "elava-reconciler",
  "trace_id": "5b8aa5a2d2c872e8321cf37308d69df2",
  "span_id": "051581bf3cb55c13",
  "resource": {
    "service.name": "elava",
    "service.version": "1.0.0",
    "deployment.environment": "production",
    "elava.instance.id": "elava-prod-01"
  },
  "attributes": { /* event-specific */ },
  "body": "Human-readable message"
}
```

### Event Type: `infrastructure.change.detected`

Emitted when a change is detected in infrastructure.

**Level**: `info` (baseline/appeared), `warn` (drift/modified), `error` (disappeared blessed)

```json
{
  "timestamp": "2025-10-10T14:23:45.123Z",
  "level": "warn",
  "service": "elava-reconciler",
  "event_type": "infrastructure.change.detected",
  "body": "Tag drift detected on i-abc123def",

  "attributes": {
    "elava.change.type": "tag_drift",
    "elava.change.severity": "warning",
    "elava.scan.id": "scan-2025-10-10-142345",
    "elava.scan.type": "normal",

    "elava.resource.id": "i-abc123def",
    "elava.resource.type": "ec2",
    "elava.resource.provider": "aws",
    "elava.resource.region": "us-east-1",
    "elava.resource.environment": "production",
    "elava.resource.owner": "team-backend",
    "elava.resource.is_blessed": false,
    "elava.resource.state": "running",

    "elava.change.previous.tags.owner": "team-backend",
    "elava.change.current.tags.owner": "team-frontend",
    "elava.change.diff.tags.owner.old": "team-backend",
    "elava.change.diff.tags.owner.new": "team-frontend"
  }
}
```

**OTEL Collector Routing**:
```yaml
# Route critical changes to PagerDuty
processors:
  filter/critical_changes:
    logs:
      log_record:
        - 'attributes["elava.change.severity"] == "critical"'

exporters:
  webhook/pagerduty:
    endpoint: https://events.pagerduty.com/v2/enqueue

pipelines:
  logs/critical:
    receivers: [otlp]
    processors: [filter/critical_changes]
    exporters: [webhook/pagerduty]
```

### Event Type: `infrastructure.decision.made`

Emitted when a policy decision is made.

**Level**: `info`

```json
{
  "timestamp": "2025-10-10T14:23:46.456Z",
  "level": "info",
  "service": "elava-reconciler",
  "event_type": "infrastructure.decision.made",
  "body": "Decision: notify team about tag drift on i-abc123def",

  "attributes": {
    "elava.decision.action": "notify",
    "elava.decision.reason": "Tag drift detected on production resource",

    "elava.resource.id": "i-abc123def",
    "elava.resource.type": "ec2",
    "elava.resource.is_blessed": false,

    "elava.policy.matched": true,
    "elava.policy.name": "production-tag-enforcement",
    "elava.policy.rule": "owner_tag_required",
    "elava.policy.confidence": 0.95,

    "elava.scan.id": "scan-2025-10-10-142345",

    "elava.metadata.change_type": "tag_drift",
    "elava.metadata.notification_channels": "slack,email"
  }
}
```

### Event Type: `infrastructure.action.executed`

Emitted when an action is executed.

**Level**: `info` (success), `error` (failed)

```json
{
  "timestamp": "2025-10-10T14:23:47.789Z",
  "level": "info",
  "service": "elava-executor",
  "event_type": "infrastructure.action.executed",
  "body": "Notification sent to #team-backend about i-abc123def",

  "attributes": {
    "elava.action.type": "notify",
    "elava.action.status": "success",
    "elava.action.duration_ms": 234,

    "elava.resource.id": "i-abc123def",

    "elava.result.notification_sent": true,
    "elava.result.channels": "slack",
    "elava.result.message_id": "slack-msg-123",
    "elava.result.recipients": "#team-backend,@oncall",

    "elava.scan.id": "scan-2025-10-10-142345"
  }
}
```

**OTEL Collector Routing**:
```yaml
# Route all notifications to Slack
processors:
  filter/notifications:
    logs:
      log_record:
        - 'attributes["elava.action.type"] == "notify"'
        - 'attributes["elava.action.status"] == "success"'

exporters:
  webhook/slack:
    endpoint: https://hooks.slack.com/services/XXX
    headers:
      Content-Type: application/json

pipelines:
  logs/notifications:
    receivers: [otlp]
    processors: [filter/notifications]
    exporters: [webhook/slack]
```

### Event Type: `infrastructure.policy.violation`

Emitted when a policy violation is detected.

**Level**: `warn` (medium/low), `error` (critical/high)

```json
{
  "timestamp": "2025-10-10T14:23:45.123Z",
  "level": "error",
  "service": "elava-policy-engine",
  "event_type": "infrastructure.policy.violation",
  "body": "Policy violation: databases-must-be-private on db-public-01",

  "attributes": {
    "elava.violation.policy_name": "databases-must-be-private",
    "elava.violation.rule": "publicly_accessible == false",
    "elava.violation.severity": "critical",

    "elava.resource.id": "db-public-01",
    "elava.resource.type": "rds",
    "elava.resource.environment": "production",
    "elava.resource.owner": "team-data",
    "elava.resource.is_blessed": true,
    "elava.resource.state": "available",

    "elava.violation.publicly_accessible": true,
    "elava.violation.expected": false,
    "elava.violation.actual": true,
    "elava.violation.risk_score": 9.5,
    "elava.violation.compliance_frameworks": "SOC2,HIPAA,PCI-DSS",

    "elava.remediation.auto_remediate": false,
    "elava.remediation.suggested_action": "Make database private",
    "elava.remediation.runbook_url": "https://wiki.company.com/rds-security",
    "elava.remediation.action_taken": "alert",

    "elava.scan.id": "scan-2025-10-10-142345"
  }
}
```

### Event Type: `infrastructure.scan.completed`

Emitted when a scan completes.

**Level**: `info`

```json
{
  "timestamp": "2025-10-10T14:24:00.000Z",
  "level": "info",
  "service": "elava-reconciler",
  "event_type": "infrastructure.scan.completed",
  "body": "Scan completed: 127 resources, 5 changes, 3 violations",

  "attributes": {
    "elava.scan.id": "scan-2025-10-10-142345",
    "elava.scan.type": "normal",
    "elava.scan.duration_ms": 15000,
    "elava.scan.started_at": "2025-10-10T14:23:45.000Z",
    "elava.scan.completed_at": "2025-10-10T14:24:00.000Z",

    "elava.resources.total": 127,
    "elava.resources.ec2": 45,
    "elava.resources.rds": 12,
    "elava.resources.s3": 23,
    "elava.resources.elb": 15,
    "elava.resources.sg": 32,
    "elava.resources.production": 89,
    "elava.resources.staging": 28,
    "elava.resources.development": 10,
    "elava.resources.untagged": 8,
    "elava.resources.blessed": 12,

    "elava.changes.total": 5,
    "elava.changes.appeared": 2,
    "elava.changes.disappeared": 1,
    "elava.changes.modified": 0,
    "elava.changes.tag_drift": 2,
    "elava.changes.status_changed": 0,

    "elava.decisions.total": 5,
    "elava.decisions.notify": 3,
    "elava.decisions.alert": 1,
    "elava.decisions.audit": 1,

    "elava.violations.total": 3,
    "elava.violations.critical": 1,
    "elava.violations.high": 0,
    "elava.violations.medium": 2,
    "elava.violations.low": 0,

    "cloud.provider": "aws",
    "cloud.region": "us-east-1"
  }
}
```

## Trace Spans Schema

### Root Span: `reconciler.reconcile`

**Span Name**: `reconciler.reconcile`
**Span Kind**: `SERVER`
**Duration**: Full reconciliation cycle

**Attributes**:
```go
elava.scan.type = "baseline" | "normal"
elava.scan.id = "scan-2025-10-10-142345"
cloud.provider = "aws"
cloud.region = "us-east-1"
elava.instance.id = "elava-prod-01"
```

**Events**:
- `scan.started` (timestamp: start)
- `scan.completed` (timestamp: end)

**Metrics** (on span):
- `resources.found` = 127
- `changes.detected` = 5
- `decisions.made` = 5

**Child Spans**:
- `observer.observe`
- `change_detector.detect_changes`
- `policy.evaluate`
- `decision_maker.decide`
- `storage.record_observations`
- `executor.execute_decisions`

### Child Span: `observer.observe`

**Span Name**: `observer.observe`
**Span Kind**: `CLIENT`
**Duration**: Time to observe cloud resources

**Attributes**:
```go
cloud.provider = "aws"
cloud.region = "us-east-1"
elava.resource.types = ["ec2", "rds", "s3"]
```

**Metrics**:
- `resources.found` = 127
- `api.calls` = 15
- `api.throttled` = 2

### Child Span: `change_detector.detect_changes`

**Span Name**: `change_detector.detect_changes`
**Span Kind**: `INTERNAL`
**Duration**: Time to detect changes

**Attributes**:
```go
elava.resource.count = 127
elava.scan.is_first = false
```

**Metrics**:
- `changes.detected` = 5
- `changes.appeared` = 2
- `changes.disappeared` = 1
- `changes.drift` = 2
- `mvcc.queries` = 127

### Child Span: `policy.evaluate`

**Span Name**: `policy.evaluate`
**Span Kind**: `INTERNAL`
**Duration**: Time to evaluate policies

**Attributes**:
```go
elava.policy.count = 25
elava.resource.count = 127
```

**Metrics**:
- `evaluations` = 3175
- `violations.found` = 3
- `violations.critical` = 1

### Child Span: `executor.execute_decisions`

**Span Name**: `executor.execute_decisions`
**Span Kind**: `INTERNAL`
**Duration**: Time to execute all decisions

**Attributes**:
```go
elava.decision.count = 5
```

**Metrics**:
- `executed` = 5
- `succeeded` = 5
- `failed` = 0
- `skipped` = 0

**Child Spans**:
- `executor.execute_single` (per decision)

### Child Span: `executor.execute_single`

**Span Name**: `executor.execute_single`
**Span Kind**: `CLIENT`
**Duration**: Time to execute one decision

**Attributes**:
```go
elava.decision.action = "notify"
elava.resource.id = "i-abc123def"
```

**Events**:
- `action.started`
- `notification.sent` (if applicable)
- `action.completed` or `action.failed`

## Semantic Conventions

Elava follows OpenTelemetry semantic conventions and extends them with custom attributes.

### Standard OTEL Attributes

```go
// Service attributes (resource level)
service.name = "elava"
service.version = "1.0.0"
service.instance.id = "elava-prod-01"

// Deployment attributes
deployment.environment = "production"

// Cloud attributes
cloud.provider = "aws"
cloud.region = "us-east-1"
cloud.account.id = "123456789012"
```

### Elava Custom Attributes

All custom attributes use `elava.*` namespace:

```go
// Resource attributes
elava.resource.id         // i-abc123def
elava.resource.type       // ec2, rds, s3
elava.resource.provider   // aws, gcp, azure
elava.resource.region     // us-east-1
elava.resource.environment // production, staging
elava.resource.owner      // team-backend
elava.resource.is_blessed // true, false
elava.resource.state      // running, stopped, available

// Change attributes
elava.change.type         // appeared, disappeared, tag_drift
elava.change.severity     // critical, warning, info
elava.change.diff.*       // Diff details

// Decision attributes
elava.decision.action     // notify, alert, protect
elava.decision.reason     // Human-readable reason

// Policy attributes
elava.policy.matched      // true, false
elava.policy.name         // databases-must-be-private
elava.policy.rule         // publicly_accessible == false
elava.policy.confidence   // 0.95

// Scan attributes
elava.scan.id             // scan-2025-10-10-142345
elava.scan.type           // baseline, normal

// Instance attributes
elava.instance.id         // elava-prod-01

// Violation attributes
elava.violation.policy_name
elava.violation.severity
elava.violation.risk_score

// Remediation attributes
elava.remediation.auto_remediate
elava.remediation.action_taken
```

## Example OTEL Collector Configurations

### Route to Prometheus + Loki + Slack

```yaml
receivers:
  otlp:
    protocols:
      grpc:
        endpoint: 0.0.0.0:4317
      http:
        endpoint: 0.0.0.0:4318

processors:
  # Filter critical events
  filter/critical:
    logs:
      log_record:
        - 'attributes["elava.change.severity"] == "critical"'
        - 'attributes["elava.violation.severity"] == "critical"'

  # Add batch processing for efficiency
  batch:
    timeout: 10s
    send_batch_size: 100

exporters:
  # Metrics to Prometheus
  prometheus:
    endpoint: "0.0.0.0:9090"
    namespace: elava

  # Logs to Loki
  loki:
    endpoint: http://loki:3100/loki/api/v1/push
    labels:
      resource:
        service.name: "service_name"
      attributes:
        level: "level"

  # Critical alerts to Slack
  webhook/slack:
    endpoint: https://hooks.slack.com/services/YOUR/WEBHOOK/URL
    headers:
      Content-Type: application/json
    timeout: 5s

service:
  pipelines:
    # Metrics pipeline
    metrics:
      receivers: [otlp]
      processors: [batch]
      exporters: [prometheus]

    # All logs to Loki
    logs:
      receivers: [otlp]
      processors: [batch]
      exporters: [loki]

    # Critical logs to Slack
    logs/critical:
      receivers: [otlp]
      processors: [filter/critical]
      exporters: [webhook/slack]
```

### Route to Datadog

```yaml
receivers:
  otlp:
    protocols:
      grpc:
        endpoint: 0.0.0.0:4317

exporters:
  datadog:
    api:
      site: datadoghq.com
      key: ${DD_API_KEY}

    host_metadata:
      enabled: true

    metrics:
      send_monotonic_counter: true

service:
  pipelines:
    metrics:
      receivers: [otlp]
      exporters: [datadog]

    logs:
      receivers: [otlp]
      exporters: [datadog]

    traces:
      receivers: [otlp]
      exporters: [datadog]
```

### Route to Your Custom API

```yaml
receivers:
  otlp:
    protocols:
      grpc:
        endpoint: 0.0.0.0:4317

processors:
  # Transform to your API format
  attributes/add_custom:
    actions:
      - key: api.version
        value: "v1"
        action: insert

exporters:
  webhook/your_api:
    endpoint: https://your-api.company.com/elava/events
    headers:
      Authorization: "Bearer ${YOUR_API_TOKEN}"
      Content-Type: "application/json"
    timeout: 10s
    retry_on_failure:
      enabled: true
      initial_interval: 1s
      max_interval: 30s

service:
  pipelines:
    logs:
      receivers: [otlp]
      processors: [attributes/add_custom]
      exporters: [webhook/your_api]
```

### Multi-Destination Setup

```yaml
receivers:
  otlp:
    protocols:
      grpc:
        endpoint: 0.0.0.0:4317

processors:
  batch:
    timeout: 10s

exporters:
  # Metrics
  prometheus:
    endpoint: "0.0.0.0:9090"
  datadog:
    api:
      key: ${DD_API_KEY}

  # Logs
  loki:
    endpoint: http://loki:3100/loki/api/v1/push
  file:
    path: /var/log/elava/events.jsonl
  stdout:
    verbosity: detailed

  # Traces
  jaeger:
    endpoint: jaeger:14250
    tls:
      insecure: true

service:
  pipelines:
    # Metrics to BOTH Prometheus AND Datadog
    metrics:
      receivers: [otlp]
      processors: [batch]
      exporters: [prometheus, datadog]

    # Logs to Loki, file, AND stdout
    logs:
      receivers: [otlp]
      processors: [batch]
      exporters: [loki, file, stdout]

    # Traces to Jaeger
    traces:
      receivers: [otlp]
      processors: [batch]
      exporters: [jaeger]
```

## Benefits of This Approach

### For Elava Development

✅ **Simple Code**: Just emit structured logs and metrics via OTEL
✅ **No Integrations**: No Slack SDK, no Datadog SDK, no custom clients
✅ **No Configuration**: Users configure OTEL Collector, not Elava
✅ **Standards Compliant**: Following CLAUDE.md (OTEL only, no wrappers)

### For Users

✅ **Flexibility**: Route to ANY backend
✅ **Multi-Destination**: Send same data to 10 places simultaneously
✅ **No Lock-In**: Switch backends without changing Elava
✅ **Standard Stack**: Use existing observability infrastructure
✅ **Dynamic Routing**: Change routing without redeploying Elava

### For Operations

✅ **Rich Context**: Traces show full reconciliation flow with timings
✅ **Queryable Metrics**: Dashboard anything in Grafana
✅ **Structured Logs**: Easy to parse and analyze
✅ **Correlation**: Trace IDs link metrics, logs, and traces

## Implementation Checklist

- [ ] Add OTEL instrumentation to reconciler package
- [ ] Add OTEL instrumentation to executor package
- [ ] Define all metrics with proper attributes
- [ ] Emit structured log events for all changes
- [ ] Add trace spans for reconciliation flow
- [ ] Create example OTEL Collector configs
- [ ] Document metric queries (PromQL examples)
- [ ] Test with real OTEL Collector
- [ ] Add integration tests for telemetry
- [ ] Document for users in README

## References

- [OpenTelemetry Specification](https://opentelemetry.io/docs/specs/otel/)
- [OTEL Semantic Conventions](https://opentelemetry.io/docs/specs/semconv/)
- [OTEL Collector Documentation](https://opentelemetry.io/docs/collector/)
- [Go OTEL SDK](https://opentelemetry.io/docs/instrumentation/go/)
