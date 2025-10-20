# OTEL Change Event Metrics Design

**Status:** Design Phase
**Date:** 2025-10-20
**Target:** Open Source - Prometheus integration

## Problem Statement

Open source Elava users need to monitor infrastructure changes via Prometheus without:
- Running CLI commands manually
- External backends (Ahti)
- Polling storage repeatedly

**Current State:**
- ✅ ChangeDetector generates change events
- ✅ Events stored in MVCC storage
- ❌ No metrics exposed to Prometheus

**Desired State:**
- Prometheus scrapes Elava metrics endpoint
- Dashboard shows: "10 EC2 created, 5 RDS modified, 2 S3 disappeared (last 1h)"
- Alerts fire on: "Security group opened to 0.0.0.0/0"

## Solution: OTEL Metrics for Change Events

### Metrics Schema

Following CLAUDE.md direct OTEL requirement:

```go
// Counter metrics (cumulative, monotonically increasing)
elava_resources_created_total{type="ec2", region="us-east-1"}
elava_resources_modified_total{type="rds", region="us-west-2"}
elava_resources_disappeared_total{type="s3_bucket", region="eu-west-1"}

// Total change events processed
elava_change_events_stored_total

// Last scan metadata
elava_last_scan_timestamp_seconds
elava_last_scan_resources_total
```

**Why these metrics?**
- Counters for Prometheus rate() queries: `rate(elava_resources_created_total[5m])`
- Labels: type + region (controlled cardinality, ~50 types × 20 regions = 1000 series)
- No high-cardinality labels (resource IDs, tags, names)

### Integration Point

Add metrics observer to existing scan pipeline:

```
cmd/elava/scan.go:
  handleStateChanges()
    ├─ storeObservations()         # Store resources
    ├─ detectChanges()             # Detect changes
    │   ├─ ChangeDetector.DetectChanges()
    │   └─ Storage.StoreChangeEventBatch()
    └─ NEW: observer.RecordChangeEvents(events)  # ← Add here
```

### Observer Pattern (Following CLAUDE.md)

```go
// observer/change_metrics.go

import (
    "context"
    "go.opentelemetry.io/otel"
    "go.opentelemetry.io/otel/metric"
    "github.com/yairfalse/elava/storage"
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

    // ... similar for modified, disappeared, total

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
```

### Wiring Into Scan Pipeline

```go
// cmd/elava/scan.go

type ScanCommand struct {
    // ... existing fields
    changeMetrics *observer.ChangeEventMetrics  // NEW
}

func (cmd *ScanCommand) Execute(ctx context.Context) error {
    // Initialize metrics observer
    metrics, err := observer.NewChangeEventMetrics()
    if err != nil {
        return fmt.Errorf("failed to init metrics: %w", err)
    }
    cmd.changeMetrics = metrics

    // ... existing scan logic
}

func (cmd *ScanCommand) handleStateChanges(ctx context.Context, storage *storage.MVCCStorage, resources []types.Resource) ChangeSet {
    // Store observations first
    revision, err := storeObservations(storage, resources)

    // Detect changes
    changes := detectChanges(ctx, storage, resources)

    // NEW: Record metrics
    if len(changes.New) > 0 || len(changes.Modified) > 0 || len(changes.Disappeared) > 0 {
        // Convert ChangeSet back to []ChangeEvent for metrics
        events := convertChangeSetToEvents(changes)
        cmd.changeMetrics.RecordChangeEvents(ctx, events)
    }

    // Report changes to user
    if len(changes.New) > 0 || len(changes.Modified) > 0 || len(changes.Disappeared) > 0 {
        cmd.reportChanges(changes)
    }

    return changes
}
```

## Prometheus Queries

### Rate of Changes (last 5 minutes)
```promql
# Resources created per minute
rate(elava_resources_created_total[5m]) * 60

# Resources disappeared per hour
rate(elava_resources_disappeared_total[1h]) * 3600

# Total change rate by type
sum by (type) (rate(elava_resources_created_total[5m]))
```

### Alerts
```yaml
# Alert on security group changes
- alert: SecurityGroupModified
  expr: rate(elava_resources_modified_total{type="security_group"}[5m]) > 0
  labels:
    severity: warning
  annotations:
    summary: "Security group modified"

# Alert on disappeared resources
- alert: ResourcesDisappeared
  expr: rate(elava_resources_disappeared_total[1h]) * 3600 > 10
  labels:
    severity: warning
  annotations:
    summary: "More than 10 resources disappeared in the last hour"
```

### Grafana Dashboard Panels

```json
{
  "title": "Infrastructure Changes (Last 24h)",
  "targets": [
    {
      "expr": "increase(elava_resources_created_total[24h])",
      "legendFormat": "Created: {{type}}"
    },
    {
      "expr": "increase(elava_resources_modified_total[24h])",
      "legendFormat": "Modified: {{type}}"
    },
    {
      "expr": "increase(elava_resources_disappeared_total[24h])",
      "legendFormat": "Disappeared: {{type}}"
    }
  ]
}
```

## Testing Strategy

### Unit Tests (TDD)

```go
// observer/change_metrics_test.go

func TestChangeEventMetrics_RecordCreated(t *testing.T) {
    metrics, err := NewChangeEventMetrics()
    require.NoError(t, err)

    event := storage.ChangeEvent{
        ChangeType: "created",
        Current: &types.Resource{
            Type:   "ec2",
            Region: "us-east-1",
        },
    }

    ctx := context.Background()
    metrics.RecordChangeEvents(ctx, []storage.ChangeEvent{event})

    // Verify counter incremented (using OTEL test helpers)
    // ... assertion code
}

func TestChangeEventMetrics_RecordBatch(t *testing.T) {
    metrics, err := NewChangeEventMetrics()
    require.NoError(t, err)

    events := []storage.ChangeEvent{
        {ChangeType: "created", Current: &types.Resource{Type: "ec2"}},
        {ChangeType: "modified", Current: &types.Resource{Type: "rds"}},
        {ChangeType: "disappeared", Previous: &types.Resource{Type: "s3_bucket"}},
    }

    ctx := context.Background()
    metrics.RecordChangeEvents(ctx, events)

    // Verify all counters incremented correctly
}

func TestExtractResourceType_BothCurrentAndPrevious(t *testing.T) {
    event := storage.ChangeEvent{
        Current:  &types.Resource{Type: "ec2"},
        Previous: &types.Resource{Type: "ec2"},
    }

    result := extractResourceType(event)
    assert.Equal(t, "ec2", result)
}

func TestExtractResourceType_OnlyCurrent(t *testing.T) {
    event := storage.ChangeEvent{
        Current: &types.Resource{Type: "ec2"},
    }

    result := extractResourceType(event)
    assert.Equal(t, "ec2", result)
}

func TestExtractResourceType_OnlyPrevious(t *testing.T) {
    event := storage.ChangeEvent{
        Previous: &types.Resource{Type: "ec2"},
    }

    result := extractResourceType(event)
    assert.Equal(t, "ec2", result)
}

func TestExtractResourceType_Missing(t *testing.T) {
    event := storage.ChangeEvent{}

    result := extractResourceType(event)
    assert.Equal(t, "unknown", result)
}
```

### Integration Test

```go
// cmd/elava/scan_metrics_test.go

func TestScanCommand_RecordsMetrics(t *testing.T) {
    // Setup temp storage
    tmpDir := t.TempDir()
    store, err := storage.NewMVCCStorage(tmpDir)
    require.NoError(t, err)
    defer store.Close()

    // Run first scan (baseline)
    resources1 := []types.Resource{
        {ID: "i-abc123", Type: "ec2", Region: "us-east-1"},
    }
    cmd := &ScanCommand{storage: store}
    cmd.handleStateChanges(context.Background(), store, resources1)

    // Run second scan (with changes)
    resources2 := []types.Resource{
        {ID: "i-abc123", Type: "ec2", Region: "us-east-1", Status: "stopped"}, // modified
        {ID: "i-def456", Type: "ec2", Region: "us-east-1"},                   // created
    }
    cmd.handleStateChanges(context.Background(), store, resources2)

    // Verify metrics recorded
    // Query OTEL metric reader to verify counters incremented
}
```

## File Structure

```
observer/
├── change_metrics.go           # NEW - OTEL metrics for change events
├── change_metrics_test.go      # NEW - Unit tests
├── observer.go                 # Existing - scan metrics
└── observer_test.go            # Existing - scan tests

cmd/elava/
├── scan.go                     # Modified - wire in metrics
└── scan_metrics_test.go        # NEW - Integration test
```

## Edge Cases

1. **Missing resource data**: Use "unknown" for type/region
2. **Metric registration failure**: Fail fast at startup
3. **High cardinality**: Limit labels to type + region only (no IDs, tags)
4. **Empty change batches**: Skip recording (no-op)
5. **Context cancellation**: Respect context in RecordChangeEvents

## Definition of Done

- [ ] Design documented ✅
- [ ] Functions are small (<50 lines)
- [ ] Direct OTEL (no wrappers) per CLAUDE.md
- [ ] Tests written (TDD - write first!)
- [ ] `go fmt` applied
- [ ] `go vet` passes
- [ ] `golangci-lint` passes
- [ ] 80%+ test coverage
- [ ] Error handling with context
- [ ] Integration test with real storage

## Success Criteria

1. **Prometheus scraping works**: `curl localhost:8080/metrics | grep elava_resources`
2. **Counters increment correctly**: Multiple scans show increasing values
3. **Labels correct**: type and region populated from events
4. **Zero overhead**: Metrics don't slow down scans significantly (<1ms)
5. **Grafana dashboard works**: Can visualize changes over time

## Next Steps

1. **RED Phase**: Write failing tests
2. **GREEN Phase**: Implement minimal code to pass
3. **REFACTOR Phase**: Extract helpers, add edge cases
4. **Integration**: Wire into scan pipeline
5. **Validation**: Test with Prometheus + Grafana

---

**Following CLAUDE.md principles:**
- ✅ Small focused functions (<30 lines each)
- ✅ Direct OTEL (no wrappers)
- ✅ Storage-first thinking (read change events)
- ✅ Observer pattern for decoupling
- ✅ TDD workflow (tests first)
- ✅ Clear interfaces
