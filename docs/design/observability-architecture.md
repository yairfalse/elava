# Observability Architecture Design

**Date**: 2025-10-25
**Status**: Design Phase - Informed by Tapio Research
**Question**: What happens when we run the daemon locally on kind k8s with Jaeger and Prometheus?

**Research Sources**:
- Tapio: OTEL redesign, Beyla patterns, Grafana/Prometheus research
- Elava: etcd MVCC learnings, daemon architecture, existing implementation

---

## Key Learnings from Tapio Research

### 1. Signals Serve Different Purposes (CRITICAL)
From `OTEL_OBSERVABILITY_REDESIGN.md`:

- **Metrics**: Aggregated measurements (rate, duration, counts)
- **Traces**: Operations across time (request → DB → cache)
- **Logs**: Individual events with context

**Our daemon events are METRICS, not traces.** Don't create fake point-in-time spans.

### 2. Semantic Conventions Are Mandatory
From `OTEL_ATTRIBUTE_STANDARDS.md`:

Use OTEL standard attributes:
- `service.name` not custom names
- Follow semconv package patterns
- Enables standard Grafana dashboards

### 3. Pre-Computed OTEL Attributes (Performance)
From `BEYLA_PATTERNS_IMPLEMENTATION.md`:

Computing attributes per-event: **100µs**
Pre-computing at startup: **1µs** (100x faster)

**Pattern**: Compute OTEL resource attributes once during daemon init.

### 4. Export Strategy Matters
From `ARCHITECTURE_RESEARCH_FINDINGS.md`:

- Prometheus prefers **pull** (ServiceMonitor)
- OTLP collector supports **push** (flexible backends)
- **Dual export** = both patterns (Beyla uses this)

### 5. Cardinality Management
From `OTEL_OBSERVABILITY_REDESIGN.md`:

- ✅ LOW: Component (~10), region (~20), event type (< 100)
- ❌ HIGH: Resource IDs (thousands), IPs (infinite)

**Use aggregation**: Map resource IDs → resource types

---

## Current State Analysis

### What We Have (Implemented)

**1. Daemon with Direct OTEL Metrics**
- Location: `internal/daemon/metrics.go`
- Meter: `otel.Meter("elava.daemon")`
- Export: Prometheus scraping via `promhttp.Handler()`
- Metrics: 6 daemon-specific metrics
  - `daemon_reconcile_runs_total`
  - `daemon_reconcile_failures_total`
  - `daemon_reconcile_duration_seconds`
  - `daemon_resources_discovered`
  - `daemon_change_events_total`
  - `daemon_storage_writes_failed_total`

**2. Global Telemetry Package**
- Location: `telemetry/otel.go`
- Meter: `otel.Meter("github.com/yairfalse/elava")`
- Export: OTLP to collector (localhost:4317)
- Metrics: Scan-focused
  - `ovi.resources.scanned.total`
  - `ovi.untracked.found.total`
  - `ovi.scan.duration.seconds`
  - `ovi.storage.writes.total`
  - `ovi.storage.revision.current`

**3. CLI Telemetry Init**
- Location: `cmd/elava/otel_init.go`
- Called by: Some commands (NOT daemon yet)
- Configures: OTLP exporter + MeterProvider

**4. Change Event Metrics**
- Location: `observer/change_metrics.go`
- Meter: `otel.Meter("elava.change-detector")`
- Used by: Daemon (already wired)

### The Architecture Conflict

We have **two competing export paths**:

```
Path 1: Daemon → OTEL Metrics → Prometheus Exporter → /metrics → Prometheus
Path 2: Daemon → OTEL Metrics → OTLP Exporter → Collector → Prometheus + Jaeger
```

**Problem**: These paths are mutually exclusive with current implementation.

---

## The Question Restated

### Scenario: kind + Jaeger + Prometheus

```bash
# In kind cluster:
kubectl apply -f jaeger-all-in-one.yaml  # OTLP receiver on :4317
kubectl apply -f prometheus.yaml          # Scrapes ServiceMonitors
kubectl apply -f elava-daemon.yaml        # Our daemon
```

**What would happen?**

### Current Behavior (Today)

**Prometheus**:
- ✅ Would scrape successfully
- ✅ See daemon metrics (6 metrics)
- ✅ See Go runtime metrics
- ❌ Would NOT see telemetry package metrics (different meter, no export)

**Jaeger**:
- ❌ Receives NOTHING
- No traces (not implemented in daemon)
- No metrics (daemon doesn't call `initTelemetry()`)

**Logs**:
- ✅ Console logs via zerolog (not OTEL)
- ❌ Not sent to collector

---

## Design Questions to Answer

### 1. Export Strategy: Push or Pull?

**Option A: Prometheus Pull (Current)**
```
Daemon exposes /metrics → Prometheus scrapes → Grafana queries Prometheus
```

Pros:
- ✅ Simple (already working)
- ✅ Standard Kubernetes pattern (ServiceMonitor)
- ✅ No external dependencies (daemon self-contained)
- ✅ Metrics always available even if Prometheus down

Cons:
- ❌ Daemon needs to be network-accessible
- ❌ No support for Jaeger/Tempo
- ❌ No OTLP compatibility

**Option B: OTLP Push**
```
Daemon → OTLP Exporter → Collector → Prometheus + Jaeger + Loki
```

Pros:
- ✅ Unified pipeline (metrics + traces + logs)
- ✅ Flexible backends (swap Prometheus for Grafana Cloud)
- ✅ Works with Jaeger out of the box
- ✅ OTEL standard

Cons:
- ❌ Requires collector always running
- ❌ Daemon loses metrics if collector down
- ❌ More moving parts

**Option C: Dual Export (Both)**
```
Daemon → Prometheus Exporter → /metrics → Prometheus (scrape)
       → OTLP Exporter → Collector → Jaeger (push)
```

Pros:
- ✅ Best of both worlds
- ✅ Prometheus works even if collector down
- ✅ Jaeger gets telemetry

Cons:
- ❌ Metrics exported twice (overhead)
- ❌ Complex configuration
- ❌ Two meters to maintain

### 2. Meter Hierarchy: Flat or Structured?

**Current State**: We have 3 different meters
- `otel.Meter("elava.daemon")` - Daemon metrics
- `otel.Meter("elava.change-detector")` - Change metrics
- `otel.Meter("github.com/yairfalse/elava")` - Telemetry package

**Option A: Single Global Meter**
```go
// One meter for everything
var Meter = otel.Meter("elava")

// Differentiate with labels
reconcileCounter := Meter.Int64Counter("elava.operations.total",
    metric.WithAttributes(attribute.String("component", "daemon")))
```

Pros:
- ✅ Simple
- ✅ One exporter configuration
- ✅ Consistent naming

Cons:
- ❌ Harder to disable components
- ❌ All metrics bundled together

**Option B: Hierarchical Meters (Recommended)**
```go
// Separate meters per component
daemonMeter := otel.Meter("elava.daemon")
detectorMeter := otel.Meter("elava.detector")
storageMeter := otel.Meter("elava.storage")
```

Pros:
- ✅ Clear ownership
- ✅ Can enable/disable per component
- ✅ Better organization

Cons:
- ❌ Slightly more verbose

**Decision**: Stick with hierarchical (what we have), but unify the export path.

### 3. MeterProvider Ownership: Who Initializes?

**Option A: CLI Layer (main.go)**
```go
func main() {
    // Initialize OTEL (global)
    shutdown := initTelemetry(ctx)
    defer shutdown()

    // Run command (uses global otel.Meter())
    Execute()
}
```

Pros:
- ✅ Single initialization point
- ✅ All commands get telemetry
- ✅ Easy to configure via env vars

Cons:
- ❌ Global state
- ❌ Harder to test
- ❌ Commands become dependent on main.go setup

**Option B: Daemon Owns Its Provider**
```go
func NewDaemon(config Config) (*Daemon, error) {
    // Daemon creates its own MeterProvider
    provider := metric.NewMeterProvider(...)
    otel.SetMeterProvider(provider)

    return &Daemon{provider: provider}
}
```

Pros:
- ✅ Daemon is self-contained
- ✅ Easier to test (inject provider)
- ✅ No global state

Cons:
- ❌ Sets global provider (still global)
- ❌ Other components can't share

**Option C: Dependency Injection**
```go
type Daemon struct {
    meterProvider metric.MeterProvider
    meter         metric.Meter
}

func NewDaemon(config Config, provider metric.MeterProvider) (*Daemon, error) {
    meter := provider.Meter("elava.daemon")
    return &Daemon{
        meterProvider: provider,
        meter: meter,
    }
}
```

Pros:
- ✅ No global state
- ✅ Easy to test (mock provider)
- ✅ Explicit dependencies

Cons:
- ❌ More boilerplate
- ❌ Need to wire everything

**Decision**: Start with Option A (CLI layer), migrate to Option C later if needed.

---

## Proposed Architecture (Based on Tapio Research)

### Core Principles

1. **Metrics-First** (not traces): Daemon generates aggregated metrics
2. **Semantic Conventions**: Use `semconv` package for all attributes
3. **Pre-Computed Resources**: Compute service metadata once at startup
4. **Dual Export**: Prometheus (pull) + OTLP (push) for flexibility
5. **Low Cardinality**: Aggregate by type/region, not by ID

### The Right Way: Beyla-Inspired Dual Export

```
┌───────────────────────────────────────────────────────────────────┐
│                         Elava Daemon                               │
│                                                                    │
│  ┌──────────────────────────────────────────────────────────────┐ │
│  │ Application Code                                              │ │
│  │ - Reconciliation loop                                         │ │
│  │ - Change detection                                            │ │
│  │ - Storage operations                                          │ │
│  └───────────────────┬──────────────────────────────────────────┘ │
│                      │                                             │
│                      │ Uses OTEL API                              │
│                      │                                             │
│  ┌───────────────────▼──────────────────────────────────────────┐ │
│  │ OTEL SDK (Initialized by CLI)                                │ │
│  │                                                                │ │
│  │  Meters:                                                      │ │
│  │  - otel.Meter("elava.daemon")                                │ │
│  │  - otel.Meter("elava.detector")                              │ │
│  │  - otel.Meter("elava.storage")                               │ │
│  │                                                                │ │
│  │  Tracers:                                                     │ │
│  │  - otel.Tracer("elava.daemon")                               │ │
│  │                                                                │ │
│  └────────────────────┬─────────────────────────────────────────┘ │
│                       │                                            │
│                       │ Export Path (configurable)                │
│                       │                                            │
│  ┌────────────────────┴─────────────────────────────────────────┐ │
│  │ MeterProvider with TWO exporters:                            │ │
│  │                                                                │ │
│  │  1. Prometheus Exporter (for /metrics endpoint)              │ │
│  │     └─> HTTP Handler on :2112/metrics                        │ │
│  │                                                                │ │
│  │  2. OTLP Exporter (for collector)                            │ │
│  │     └─> gRPC to collector:4317 (optional)                    │ │
│  │                                                                │ │
│  └──────────────────────────────────────────────────────────────┘ │
│                                                                    │
└────────────────────────────────────────────────────────────────────┘
                               │
                               │
        ┌──────────────────────┼──────────────────────┐
        │                      │                      │
        ▼                      ▼                      ▼
┌───────────────┐    ┌──────────────────┐    ┌──────────────┐
│  Prometheus   │    │ OTEL Collector   │    │   Direct     │
│   (scrape)    │    │   (push OTLP)    │    │  /metrics    │
└───────────────┘    └─────────┬────────┘    └──────────────┘
                               │
                    ┌──────────┴─────────┐
                    │                    │
                    ▼                    ▼
            ┌──────────────┐    ┌──────────────┐
            │  Prometheus  │    │    Jaeger    │
            │  (metrics)   │    │   (traces)   │
            └──────────────┘    └──────────────┘
```

### Configuration Strategy

**Environment Variables** (12-factor):
```bash
# Telemetry export mode
ELAVA_TELEMETRY_MODE=dual           # "prometheus" | "otlp" | "dual" | "disabled"

# OTLP collector endpoint
OTEL_EXPORTER_OTLP_ENDPOINT=localhost:4317

# Prometheus endpoint (always enabled in daemon)
ELAVA_METRICS_PORT=2112

# Service metadata
ELAVA_SERVICE_NAME=elava-daemon
ELAVA_ENVIRONMENT=production
```

**Code Implementation**:
```go
// cmd/elava/cmd_daemon.go
func runDaemon(cmd *cobra.Command, args []string) error {
    ctx := context.Background()

    // Initialize telemetry based on mode
    mode := os.Getenv("ELAVA_TELEMETRY_MODE")
    if mode == "" {
        mode = "prometheus" // Default: just Prometheus
    }

    var shutdownTelemetry func()
    if mode != "disabled" {
        shutdownTelemetry = initTelemetryForMode(ctx, mode)
        defer shutdownTelemetry()
    }

    // Create daemon (uses global otel.Meter())
    daemon, err := daemon.NewDaemon(config)
    // ... rest of daemon logic
}

func initTelemetryForMode(ctx context.Context, mode string) func() {
    switch mode {
    case "prometheus":
        // Just Prometheus exporter (current behavior)
        return initPrometheusOnly(ctx)
    case "otlp":
        // Just OTLP exporter (for cloud)
        return initOTLPOnly(ctx)
    case "dual":
        // Both exporters
        return initDualExport(ctx)
    default:
        return func() {}
    }
}
```

---

## Answer to Your Question

### Scenario: kind + Jaeger + Prometheus

With the proposed architecture:

**Setup**:
```yaml
# elava-daemon deployment
env:
- name: ELAVA_TELEMETRY_MODE
  value: "dual"
- name: OTEL_EXPORTER_OTLP_ENDPOINT
  value: "otel-collector:4317"
- name: ELAVA_METRICS_PORT
  value: "2112"
```

**What Would Happen**:

1. **Prometheus Scraping** ✅
   - Scrapes `elava-daemon:2112/metrics`
   - Gets all daemon metrics via Prometheus exporter
   - Works even if collector is down

2. **OTLP Push to Collector** ✅
   - Daemon pushes to `otel-collector:4317` every 10s
   - Collector receives metrics + traces
   - Collector forwards to Jaeger + Prometheus

3. **Jaeger** ✅
   - Receives traces (once we add spans)
   - Shows reconciliation spans with timing
   - Parent-child span relationships

4. **Metrics Consistency**
   - Same metrics in both Prometheus instances
   - Scrape-based has real-time data
   - OTLP-based has 10s delay (export interval)

**Trade-offs**:
- Metrics exported twice (2x network)
- Collector adds complexity
- BUT: Maximum flexibility and reliability

---

## Implementation Plan

### Phase 1: Add Telemetry Mode Support

1. **Refactor `otel_init.go`**
   - Add `mode` parameter
   - Implement `initPrometheusOnly()`, `initOTLPOnly()`, `initDualExport()`
   - Keep existing OTLP logic

2. **Wire into daemon command**
   - Call `initTelemetryForMode()` in `runDaemon()`
   - Pass context through

3. **Test all modes**
   - Test `prometheus` mode (current behavior)
   - Test `otlp` mode (collector required)
   - Test `dual` mode (both)

### Phase 2: Add Tracing (Optional but Valuable)

```go
// internal/daemon/daemon.go
func (d *Daemon) runReconciliation(ctx context.Context) {
    tracer := otel.Tracer("elava.daemon")
    ctx, span := tracer.Start(ctx, "reconciliation",
        trace.WithAttributes(
            attribute.Int64("run", runNum),
        ),
    )
    defer span.End()

    // Pass ctx through to sub-operations
    resources, err := d.listResources(ctx) // child span
    // ...
}
```

### Phase 3: Logs Bridge (Low Priority)

- Zerolog already works for console
- OTEL logs bridge only needed for collector ingestion
- Can add later if needed

---

## Tapio Patterns Applied to Elava

### Pattern 1: Pre-Computed Resource Attributes

**From Tapio**: `ComputeOTELAttributes()` at pod add/update (100x faster)
**For Elava**: Compute daemon resource attributes once at startup

```go
// internal/daemon/daemon.go
type Daemon struct {
    // ... existing fields
    resourceAttributes []attribute.KeyValue  // Pre-computed!
}

func NewDaemon(config Config) (*Daemon, error) {
    // Compute resource attributes ONCE
    resourceAttrs := computeResourceAttributes(config)

    // Create OTEL resource
    res := resource.NewWithAttributes(
        semconv.SchemaURL,
        resourceAttrs...,
    )

    return &Daemon{
        resourceAttributes: resourceAttrs,
        // ... other fields
    }
}

func computeResourceAttributes(config Config) []attribute.KeyValue {
    attrs := []attribute.KeyValue{
        semconv.ServiceName("elava-daemon"),
        semconv.ServiceVersion(version),
        semconv.DeploymentEnvironment(os.Getenv("ENVIRONMENT")),
        attribute.String("cloud.provider", config.Provider),
        attribute.String("cloud.region", config.Region),
    }

    // K8s context (if running in cluster)
    if podName := os.Getenv("POD_NAME"); podName != "" {
        attrs = append(attrs,
            semconv.K8SPodName(podName),
            semconv.K8SNamespaceName(os.Getenv("POD_NAMESPACE")),
            semconv.K8SNodeName(os.Getenv("NODE_NAME")),
        )
    }

    return attrs
}
```

**Benefit**: Attributes computed once, reused for all metrics. No per-metric overhead.

### Pattern 2: Semantic Conventions for Metrics

**From Tapio**: Use `semconv` package for all attributes
**For Elava**: Standardize metric names and attributes

```go
// Current (custom names)
daemon_reconcile_runs_total
daemon_reconcile_failures_total

// Proposed (semantic conventions)
elava.daemon.reconciliations.total{status="success"|"failure"}
elava.daemon.reconciliations.duration{unit="seconds"}
elava.resources.discovered{cloud.provider="aws", cloud.region="us-east-1"}
```

**Benefit**: Standard Grafana dashboards work immediately.

### Pattern 3: Low Cardinality via Aggregation

**From Tapio**: ❌ Don't use pod names (thousands), ✅ use deployment names (~500)
**For Elava**: ❌ Don't use resource IDs, ✅ use resource types

```go
// ❌ BAD: High cardinality (will crash Prometheus)
daemon_resources_total{resource_id="i-abc123"}  // Thousands of time series

// ✅ GOOD: Low cardinality
elava.resources.total{resource.type="ec2"}      // ~20 time series
elava.resources.total{resource.type="rds"}
elava.resources.total{resource.type="s3"}
```

**Benefit**: Prometheus doesn't OOM, queries stay fast.

### Pattern 4: Dual Export (Beyla Pattern)

**From Tapio/Beyla**: Run both Prometheus exporter + OTLP exporter
**For Elava**: Enable both, make OTLP optional

```go
// Config
type TelemetryConfig struct {
    PrometheusEnabled bool   // Default: true (always for /metrics)
    OTLPEnabled      bool   // Default: false (opt-in)
    OTLPEndpoint     string // Default: "localhost:4317"
}

// Dual export setup
func setupTelemetry(cfg TelemetryConfig) (*MeterProvider, error) {
    var readers []sdkmetric.Reader

    // Always include Prometheus (for /metrics endpoint)
    promReader := sdkmetric.NewManualReader()
    readers = append(readers, promReader)

    // Optional: OTLP push
    if cfg.OTLPEnabled {
        otlpReader := sdkmetric.NewPeriodicReader(
            otlpmetricgrpc.New(ctx, otlpOpts...),
            sdkmetric.WithInterval(10*time.Second),
        )
        readers = append(readers, otlpReader)
    }

    provider := sdkmetric.NewMeterProvider(
        sdkmetric.WithReader(readers...),
        sdkmetric.WithResource(resource),
    )

    return provider, nil
}
```

**Benefit**: Prometheus scraping always works, OTLP is bonus when available.

---

## Decision Matrix (Final - Do The Work Properly)

| Aspect | Decision | Rationale |
|--------|----------|-----------|
| **Export Strategy** | Dual (Prometheus + OTLP) | Beyla pattern - production ready |
| **Default Mode** | Dual enabled | Both work, OTLP optional via env var |
| **Metric Names** | Semantic conventions | Use `semconv` package (Tapio learning) |
| **Resource Attrs** | Pre-computed at startup | 100x faster (Beyla pattern) + full K8s context |
| **Cardinality** | Aggregate by type/region | Avoid resource IDs (Tapio learning) |
| **Meter Hierarchy** | Keep separate meters | Already works, no need to change |
| **Provider Ownership** | CLI layer (main.go) | Single init point |
| **Traces** | Yes - reconciliation + AWS API | Real operations (5-60s), diagnose slowness |
| **Logs** | Keep zerolog console | OTEL logs not needed yet |
| **Testing** | Integration test with testcontainers | Test both Prometheus + OTLP + Jaeger |

---

## Testing Strategy

### Local Development
```bash
# Just daemon (Prometheus mode)
elava daemon --interval 10s

# With collector (OTLP mode)
docker-compose up -d  # Starts collector + Jaeger + Prometheus
ELAVA_TELEMETRY_MODE=otlp elava daemon --interval 10s
```

### kind Cluster
```bash
# Deploy collector stack
kubectl apply -f otel-collector.yaml
kubectl apply -f jaeger.yaml
kubectl apply -f prometheus.yaml

# Deploy daemon (dual mode)
kubectl apply -f elava-daemon.yaml
```

### Verification
```bash
# Check Prometheus
curl localhost:2112/metrics | grep daemon_

# Check OTLP export
kubectl logs -l app=otel-collector | grep elava

# Check Jaeger UI
open http://localhost:16686
```

---

## Non-Goals (Scope Boundaries)

- ❌ **Not implementing**: Custom metric aggregation (use OTEL collector)
- ❌ **Not implementing**: Sampling strategies (use collector)
- ❌ **Not implementing**: Log correlation (nice-to-have, not MVP)
- ❌ **Not implementing**: Distributed tracing across services (only one service)

---

## Success Criteria

Phase 1 (Telemetry Mode Support):
- [ ] Can run daemon with `ELAVA_TELEMETRY_MODE=prometheus` (current behavior)
- [ ] Can run daemon with `ELAVA_TELEMETRY_MODE=otlp` (pushes to collector)
- [ ] Can run daemon with `ELAVA_TELEMETRY_MODE=dual` (both)
- [ ] Prometheus scraping still works in all modes
- [ ] OTLP export works when collector available
- [ ] Daemon doesn't crash if collector unavailable

Phase 2 (Tracing):
- [ ] Reconciliation loop has root span
- [ ] Sub-operations have child spans
- [ ] Spans visible in Jaeger UI
- [ ] Span attributes show useful info (run number, resource count)

---

## Open Questions

1. **Metric naming**: Keep `daemon_*` prefix or change to `elava.daemon.*` (OTEL convention)?
2. **Collector location**: Sidecar or separate service in k8s?
3. **OTLP HTTP vs gRPC**: Which protocol for collector?
4. **Sampling**: Sample all traces or add sampling later?

---

## Summary: Tapio Research → Elava Implementation

### What We Learned
1. **Metrics ≠ Traces**: Don't create fake spans (Tapio OTEL redesign)
2. **Pre-compute attributes**: 100x performance gain (Beyla pattern)
3. **Semantic conventions**: Standard dashboards work immediately (Tapio standards)
4. **Dual export**: Prometheus + OTLP = flexibility (Beyla architecture)
5. **Low cardinality**: Aggregate by type, not ID (Prometheus best practices)

### What We're Doing
1. **Keep current Prometheus export** (already works, don't break it)
2. **Add optional OTLP export** (enable with env var for kind/k8s)
3. **Pre-compute resource attributes** (compute once at daemon startup)
4. **Use semantic conventions** (migration path: old names still work)
5. **Add reconciliation traces** (real operations, not point-in-time)

### What We're NOT Doing (Scope Boundaries)
- ❌ Custom metric aggregation (use OTEL collector)
- ❌ Log correlation (nice-to-have, not MVP)
- ❌ Sampling strategies (use collector)
- ❌ Breaking existing metrics (migration, not replacement)

---

**Status**: Ready for implementation
**Next**: Design review, then Phase 1 TDD implementation

**Research References**:
- `/Users/yair/projects/tapio/docs/OTEL_OBSERVABILITY_REDESIGN.md`
- `/Users/yair/projects/tapio/docs/BEYLA_PATTERNS_IMPLEMENTATION.md`
- `/Users/yair/projects/tapio/docs/ARCHITECTURE_RESEARCH_FINDINGS.md`
- `/Users/yair/projects/elava/docs/design/etcd-mvcc-learnings.md`
