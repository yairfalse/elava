# Elava Metric Naming Conventions

**Date**: 2025-10-25
**Status**: Design Phase
**Context**: Migrating from Prometheus-style names to OTEL semantic conventions

---

## Design Principles (From Tapio Research)

### 1. OTEL Metric Naming Convention
```
<namespace>.<component>.<metric_name>
```

Examples:
- `elava.daemon.reconciliations` (not `daemon_reconcile_runs_total`)
- `elava.storage.operations` (not `daemon_storage_writes_failed_total`)

### 2. Use Attributes for Dimensions (Not Separate Metrics)

**❌ Old Way (Prometheus)**:
```
daemon_reconcile_runs_total       # Success count
daemon_reconcile_failures_total   # Failure count
```
Problem: Two separate metrics for same thing

**✅ New Way (OTEL Semantic Conventions)**:
```
elava.daemon.reconciliations{status="success"}
elava.daemon.reconciliations{status="failure"}
```
Benefit: One metric, filterable by status

### 3. Unit in Description, Not Name

**❌ Old**: `daemon_reconcile_duration_seconds`
**✅ New**: `elava.daemon.reconciliation.duration` with `unit="s"`

OTEL SDK handles unit properly.

---

## Current Metrics → Semantic Convention Mapping

### Metric 1: Reconciliation Runs

**Current**:
```go
reconcileRunsTotal, err := meter.Int64Counter(
    "daemon_reconcile_runs_total",
    metric.WithDescription("Total number of reconciliation runs"),
    metric.WithUnit("{runs}"),
)

reconcileFailuresTotal, err := meter.Int64Counter(
    "daemon_reconcile_failures_total",
    metric.WithDescription("Total number of failed reconciliation runs"),
    metric.WithUnit("{failures}"),
)
```

**New (Semantic Convention)**:
```go
reconciliations, err := meter.Int64Counter(
    "elava.daemon.reconciliations",
    metric.WithDescription("Number of reconciliation runs"),
    metric.WithUnit("{reconciliation}"),
)

// Usage with attributes:
reconciliations.Add(ctx, 1,
    metric.WithAttributes(
        attribute.String("status", "success"),  // or "failure"
        attribute.String("cloud.provider", "aws"),
        attribute.String("cloud.region", "us-east-1"),
    ),
)
```

**Queries**:
```promql
# Success rate
rate(elava_daemon_reconciliations{status="success"}[5m])

# Failure rate
rate(elava_daemon_reconciliations{status="failure"}[5m])

# Error percentage
rate(elava_daemon_reconciliations{status="failure"}[5m])
/
rate(elava_daemon_reconciliations[5m])
```

---

### Metric 2: Reconciliation Duration

**Current**:
```go
reconcileDuration, err := meter.Float64Histogram(
    "daemon_reconcile_duration_seconds",
    metric.WithDescription("Reconciliation run duration in seconds"),
    metric.WithUnit("s"),
)
```

**New (Semantic Convention)**:
```go
reconciliationDuration, err := meter.Float64Histogram(
    "elava.daemon.reconciliation.duration",
    metric.WithDescription("Duration of reconciliation operations"),
    metric.WithUnit("s"),
    metric.WithExplicitBucketBoundaries(
        // Buckets: 1s, 5s, 10s, 30s, 60s, 120s, 300s
        1, 5, 10, 30, 60, 120, 300,
    ),
)

// Usage:
reconciliationDuration.Record(ctx, durationSeconds,
    metric.WithAttributes(
        attribute.String("status", "success"),
    ),
)
```

**Queries**:
```promql
# P95 latency
histogram_quantile(0.95,
    rate(elava_daemon_reconciliation_duration_bucket[5m])
)

# Average duration by status
rate(elava_daemon_reconciliation_duration_sum{status="success"}[5m])
/
rate(elava_daemon_reconciliation_duration_count{status="success"}[5m])
```

---

### Metric 3: Resources Discovered

**Current**:
```go
resourcesDiscovered, err := meter.Int64Gauge(
    "daemon_resources_discovered",
    metric.WithDescription("Number of resources discovered in last scan"),
    metric.WithUnit("{resources}"),
)
```

**New (Semantic Convention)**:
```go
resourcesDiscovered, err := meter.Int64Gauge(
    "elava.resources.discovered",
    metric.WithDescription("Number of cloud resources discovered"),
    metric.WithUnit("{resource}"),
)

// Usage with cardinality management:
resourcesDiscovered.Record(ctx, count,
    metric.WithAttributes(
        attribute.String("resource.type", "ec2"),     // ✅ Low cardinality
        attribute.String("cloud.provider", "aws"),
        attribute.String("cloud.region", "us-east-1"),
    ),
)
```

**Queries**:
```promql
# Total resources
sum(elava_resources_discovered)

# Resources by type
elava_resources_discovered{resource_type="ec2"}
elava_resources_discovered{resource_type="rds"}

# Resources by region
sum by (cloud_region) (elava_resources_discovered)
```

---

### Metric 4: Change Events

**Current**:
```go
changeEventsTotal, err := meter.Int64Counter(
    "daemon_change_events_total",
    metric.WithDescription("Total change events by type"),
    metric.WithUnit("{events}"),
)
```

**New (Semantic Convention)**:
```go
changeEvents, err := meter.Int64Counter(
    "elava.change_events",
    metric.WithDescription("Number of infrastructure change events detected"),
    metric.WithUnit("{event}"),
)

// Usage:
changeEvents.Add(ctx, 1,
    metric.WithAttributes(
        attribute.String("change.type", "created"),  // or "modified", "disappeared"
        attribute.String("resource.type", "ec2"),
        attribute.String("cloud.region", "us-east-1"),
    ),
)
```

**Queries**:
```promql
# Change rate by type
rate(elava_change_events{change_type="created"}[5m])
rate(elava_change_events{change_type="disappeared"}[5m])

# Changes by resource type
sum by (resource_type) (rate(elava_change_events[5m]))
```

---

### Metric 5: Storage Operations

**Current**:
```go
storageWritesFailed, err := meter.Int64Counter(
    "daemon_storage_writes_failed_total",
    metric.WithDescription("Total failed storage write operations"),
    metric.WithUnit("{failures}"),
)
```

**New (Semantic Convention)**:
```go
storageOperations, err := meter.Int64Counter(
    "elava.storage.operations",
    metric.WithDescription("Number of storage operations"),
    metric.WithUnit("{operation}"),
)

// Usage:
storageOperations.Add(ctx, 1,
    metric.WithAttributes(
        attribute.String("operation", "write"),      // or "read", "compact"
        attribute.String("status", "failure"),       // or "success"
        attribute.String("error.type", "disk_full"), // optional, on failure
    ),
)
```

**Queries**:
```promql
# Storage error rate
rate(elava_storage_operations{status="failure"}[5m])
/
rate(elava_storage_operations[5m])

# Operations by type
sum by (operation) (rate(elava_storage_operations[5m]))
```

---

## Complete Metric Inventory (New Conventions)

| Metric Name | Type | Unit | Attributes | Description |
|-------------|------|------|------------|-------------|
| `elava.daemon.reconciliations` | Counter | `{reconciliation}` | `status`, `cloud.provider`, `cloud.region` | Reconciliation runs |
| `elava.daemon.reconciliation.duration` | Histogram | `s` | `status` | Reconciliation duration |
| `elava.resources.discovered` | Gauge | `{resource}` | `resource.type`, `cloud.provider`, `cloud.region` | Resources found |
| `elava.change_events` | Counter | `{event}` | `change.type`, `resource.type`, `cloud.region` | Change events |
| `elava.storage.operations` | Counter | `{operation}` | `operation`, `status`, `error.type` (optional) | Storage operations |

---

## Attribute Standards

### Status Attribute
```go
attribute.String("status", value)

// Valid values:
"success"   // Operation completed successfully
"failure"   // Operation failed
"timeout"   // Operation timed out
"canceled"  // Operation was canceled
```

### Change Type Attribute
```go
attribute.String("change.type", value)

// Valid values:
"created"      // Resource appeared
"modified"     // Resource changed
"disappeared"  // Resource vanished
```

### Resource Type Attribute
```go
attribute.String("resource.type", value)

// Valid values (AWS):
"ec2", "rds", "s3", "elb", "vpc", "subnet", "sg", "iam_role", etc.

// Keep it low cardinality (~20 types)
```

### Cloud Provider Attribute
```go
attribute.String("cloud.provider", value)

// Valid values:
"aws", "gcp", "azure"
```

### Cloud Region Attribute
```go
attribute.String("cloud.region", value)

// Valid values (examples):
"us-east-1", "us-west-2", "eu-west-1"

// Low cardinality (~20 regions per provider)
```

---

## Migration Strategy

**Decision**: No migration needed - just switch to new names.

**Rationale**:
- Elava is 3 months old
- Never deployed to production
- No existing dashboards to migrate
- Clean break is simpler than dual emit

**Action**: Replace metrics.go with new semantic conventions, update daemon usage.

---

## Code Example (New Implementation)

```go
// internal/daemon/metrics.go
package daemon

import (
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
        metric.WithExplicitBucketBoundaries(1, 5, 10, 30, 60, 120, 300),
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
```

---

## Benefits of New Conventions

### 1. Standard Dashboards Work
Grafana templates expect `elava.*` pattern, not `daemon_*`.

### 2. Better Queries
```promql
# Before (two metrics)
daemon_reconcile_runs_total / (daemon_reconcile_runs_total + daemon_reconcile_failures_total)

# After (one metric, filtered)
rate(elava_daemon_reconciliations{status="success"}[5m])
/
rate(elava_daemon_reconciliations[5m])
```

### 3. Lower Cardinality
Attributes on one metric vs separate metrics = fewer time series.

### 4. OTEL Ecosystem Compatibility
Works with Grafana Cloud, Datadog, New Relic, Honeycomb out of the box.

---

## Open Questions

1. **Namespace**: `elava.*` or `elava.daemon.*`?
   - Recommend: `elava.daemon.*` for when we add `elava.scanner.*`, `elava.analyzer.*`

2. **Histogram buckets**: Are `[1s, 5s, 10s, 30s, 60s, 120s, 300s]` right?
   - Based on 5min reconciliation interval

3. **Resource types**: Which AWS resource types to support?
   - Start with: ec2, rds, s3, elb, vpc, subnet, sg, iam_role
   - Add more as needed

---

**Status**: Ready for review
**Next**: Implement Phase 1 (dual emit), update dashboards, deprecate old names
