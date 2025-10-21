# Elava Daemon Architecture Design

**Status:** Design Phase
**Date:** 2025-10-22
**Problem:** CLI tool doesn't fit "Living Infrastructure" philosophy - need continuous reconciliation daemon

---

## Design Session Checklist

- [ ] What problem are we solving?
- [ ] How will this interact with storage (read/write/query)?
- [ ] What historical data do we need to track?
- [ ] What's the simplest solution?
- [ ] Can we break it into smaller functions?
- [ ] What interfaces do we need?
- [ ] What can go wrong?
- [ ] Draw the flow (ASCII or diagram)

---

## 1. What Problem Are We Solving?

### Current State (CLI Tool)
```bash
$ elava scan
Scanning...
Done! (exits)
```

**Problems:**
- ❌ One-shot execution doesn't match "Living Infrastructure" philosophy
- ❌ Prometheus can't scrape metrics (process exits immediately)
- ❌ No continuous reconciliation loop
- ❌ Users have to cron/schedule scans manually
- ❌ Relics from IaC (Terraform) mindset

### Desired State (Daemon)
```bash
$ elava serve
2025-10-22 10:00:00 | Starting Elava daemon...
2025-10-22 10:00:00 | Metrics server listening on :2112
2025-10-22 10:00:00 | Reconciliation loop started (interval: 5m)
2025-10-22 10:05:00 | [Scan #1] Found 42 resources, 3 changes
2025-10-22 10:10:00 | [Scan #2] Found 43 resources, 1 change
^C
2025-10-22 10:12:00 | Graceful shutdown initiated...
2025-10-22 10:12:01 | Daemon stopped
```

**Benefits:**
- ✅ Continuous reconciliation (like Kubernetes controllers)
- ✅ Prometheus can scrape `/metrics` endpoint
- ✅ Real-time infrastructure monitoring
- ✅ Graceful shutdown handling
- ✅ Health checks for orchestration (K8s, systemd)

---

## 2. How Will This Interact with Storage?

### Storage Interaction Pattern

**Daemon Lifecycle:**
```
1. Startup:
   - Open MVCC storage (read-only mode initially)
   - Load last known state
   - Initialize metrics counters

2. Reconciliation Loop (every 5 minutes):
   - Scan cloud resources (AWS API calls)
   - Store observations → MVCC storage (new revision)
   - Detect changes → ChangeDetector analyzer
   - Store change events → MVCC storage
   - Record metrics → OTEL

3. Shutdown:
   - Flush pending writes to storage
   - Close storage cleanly
   - Shutdown metrics server
```

**Storage Access Pattern:**
```go
// Each reconciliation loop iteration:
func (d *Daemon) reconciliationLoop(ctx context.Context) {
    // 1. Scan cloud (read-only, no storage)
    resources := d.scanner.ScanAll(ctx)

    // 2. Store observations (write)
    revision := d.storage.RecordObservationBatch(resources)

    // 3. Detect changes (read previous + current)
    changes := d.detector.DetectChanges(ctx, resources)

    // 4. Store change events (write)
    d.storage.StoreChangeEventBatch(ctx, changes)

    // 5. Record metrics (in-memory, exposed via HTTP)
    d.metrics.RecordChangeEvents(ctx, changes)
}
```

---

## 3. What Historical Data Do We Need to Track?

**Daemon State (In-Memory):**
- Last successful scan timestamp
- Total scans completed
- Last error (if any)
- Current reconciliation loop iteration

**Persistent Storage (MVCC):**
- Resource observations (already tracked)
- Change events (already tracked)
- Scan metadata:
  - Scan start/end timestamps
  - Resources scanned count
  - Errors encountered
  - Duration

**New Storage Requirement:**
```go
// storage/scan_metadata.go
type ScanMetadata struct {
    ScanID      int64     `json:"scan_id"`
    Revision    int64     `json:"revision"`
    StartTime   time.Time `json:"start_time"`
    EndTime     time.Time `json:"end_time"`
    Duration    int64     `json:"duration_ms"`
    ResourceCount int     `json:"resource_count"`
    ErrorCount    int     `json:"error_count"`
    Status      string    `json:"status"` // "success", "partial", "failed"
}
```

---

## 4. What's the Simplest Solution?

### Core Components (Keep It Simple!)

**1. Daemon Service**
```go
// cmd/elava/serve.go
type ServeCommand struct {
    Interval     time.Duration `help:"Reconciliation interval" default:"5m"`
    MetricsPort  int           `help:"Metrics HTTP port" default:"2112"`
    Region       string        `help:"AWS region" default:"us-east-1"`
}

func (cmd *ServeCommand) Run() error {
    // 1. Setup infrastructure
    daemon := NewDaemon(cmd)

    // 2. Start background workers
    daemon.Start()

    // 3. Wait for shutdown signal
    <-daemon.shutdown

    // 4. Cleanup
    return daemon.Stop()
}
```

**2. Reconciliation Loop (Small Focused Function)**
```go
// internal/daemon/reconciler.go
func (d *Daemon) reconciliationLoop(ctx context.Context) {
    ticker := time.NewTicker(d.interval)
    defer ticker.Stop()

    for {
        select {
        case <-ctx.Done():
            return
        case <-ticker.C:
            d.runSingleReconciliation(ctx)
        }
    }
}

func (d *Daemon) runSingleReconciliation(ctx context.Context) {
    // Small function - one reconciliation iteration
    // < 30 lines
}
```

**3. Metrics Server (Already Have Pattern from Tapio)**
```go
// From Tapio research: Simple HTTP server
http.Handle("/metrics", prometheusExporter)
http.HandleFunc("/health", healthCheck)
http.ListenAndServe(":2112", nil)
```

**4. Graceful Shutdown (Signal Handling)**
```go
// Use oklog/run pattern (from Tapio research)
var g run.Group

// Reconciliation loop actor
g.Add(func() error {
    return d.reconciliationLoop(ctx)
}, func(error) {
    cancel() // Stop reconciliation
})

// Metrics server actor
g.Add(func() error {
    return metricsServer.ListenAndServe()
}, func(error) {
    metricsServer.Shutdown(ctx)
})

// Signal handler actor
g.Add(run.SignalHandler(ctx, syscall.SIGINT, syscall.SIGTERM))
```

---

## 5. Can We Break It Into Smaller Functions?

### Function Breakdown (< 30 lines each)

**Daemon Lifecycle:**
```go
// daemon.go
func NewDaemon(config Config) *Daemon           // Constructor
func (d *Daemon) Start() error                  // Start all components
func (d *Daemon) Stop() error                   // Graceful shutdown
func (d *Daemon) Health() HealthStatus          // Health check

// reconciler.go
func (d *Daemon) reconciliationLoop(ctx)        // Main loop
func (d *Daemon) runSingleReconciliation(ctx)   // One iteration
func (d *Daemon) scanResources(ctx)             // Scan cloud
func (d *Daemon) detectChanges(ctx, resources)  // Detect changes
func (d *Daemon) recordMetrics(ctx, changes)    // Record metrics

// metrics_server.go
func NewMetricsServer(port) *MetricsServer      // Create server
func (s *MetricsServer) Start() error           // Start HTTP server
func (s *MetricsServer) Shutdown(ctx) error     // Graceful shutdown

// health.go
func (d *Daemon) healthCheck(w, r)              // HTTP handler
func (d *Daemon) computeHealth() HealthStatus   // Calculate health
```

**All functions < 30 lines following CLAUDE.md!**

---

## 6. What Interfaces Do We Need?

### Daemon Interface
```go
// internal/daemon/daemon.go
type Daemon interface {
    Start(ctx context.Context) error
    Stop() error
    Health() HealthStatus
}
```

### Reconciler Interface (Strategy Pattern)
```go
// internal/daemon/reconciler.go
type Reconciler interface {
    Reconcile(ctx context.Context) (ReconcileResult, error)
}

type ReconcileResult struct {
    ResourcesScanned int
    ChangesDetected  int
    Duration         time.Duration
    Errors           []error
}

// Implementations:
type AWSReconciler struct { }      // For AWS
type GCPReconciler struct { }      // For GCP (future)
```

### MetricsServer Interface
```go
// observer/metrics_server.go
type MetricsServer interface {
    Start() error
    Shutdown(ctx context.Context) error
    Handler() http.Handler
}
```

---

## 7. What Can Go Wrong?

### Failure Scenarios

**1. Cloud API Rate Limiting**
```
Problem: AWS throttles API calls during scan
Solution:
  - Exponential backoff
  - Configurable scan interval
  - Continue with partial results
  - Log errors, don't crash
```

**2. Storage Corruption/Full Disk**
```
Problem: BadgerDB write fails
Solution:
  - Continue reconciliation (skip this iteration)
  - Alert via metrics (storage_write_errors_total)
  - Health check reports "degraded"
  - Auto-recovery on next iteration
```

**3. Memory Leak in Long-Running Process**
```
Problem: Daemon grows unbounded memory
Solution:
  - Resource pooling (reuse HTTP clients)
  - Bounded work queues
  - Clear caches after each scan
  - Memory profiling endpoint (pprof)
```

**4. Stuck Reconciliation (Deadlock)**
```
Problem: Reconciliation hangs, never completes
Solution:
  - Per-iteration timeout (e.g., 10 minutes)
  - Context cancellation
  - Metrics: reconciliation_timeout_total
  - Skip iteration, continue loop
```

**5. Shutdown During Scan**
```
Problem: SIGTERM arrives mid-scan
Solution:
  - Context cancellation propagates
  - Flush MVCC writes (best effort)
  - Timeout: max 30s graceful shutdown
  - Force exit if exceeded
```

---

## 8. Flow Diagram

### Daemon Architecture

```
┌─────────────────────────────────────────────────────────┐
│                    Elava Daemon                          │
│                                                          │
│  ┌────────────────────────────────────────────────────┐ │
│  │ Reconciliation Loop (Goroutine)                    │ │
│  │                                                     │ │
│  │  ┌──────────────────────────────────────────────┐  │ │
│  │  │ Ticker (5 minutes)                           │  │ │
│  │  └──────────────────────────────────────────────┘  │ │
│  │           ↓                                         │ │
│  │  ┌──────────────────────────────────────────────┐  │ │
│  │  │ Scan Cloud (AWS SDK)                         │  │ │
│  │  │ - List EC2, RDS, S3, etc.                    │  │ │
│  │  └──────────────────────────────────────────────┘  │ │
│  │           ↓                                         │ │
│  │  ┌──────────────────────────────────────────────┐  │ │
│  │  │ Store Observations (MVCC Storage)            │  │ │
│  │  │ - New revision created                       │  │ │
│  │  └──────────────────────────────────────────────┘  │ │
│  │           ↓                                         │ │
│  │  ┌──────────────────────────────────────────────┐  │ │
│  │  │ Detect Changes (ChangeDetector)              │  │ │
│  │  │ - Compare N vs N-1                           │  │ │
│  │  └──────────────────────────────────────────────┘  │ │
│  │           ↓                                         │ │
│  │  ┌──────────────────────────────────────────────┐  │ │
│  │  │ Store Change Events (MVCC Storage)           │  │ │
│  │  └──────────────────────────────────────────────┘  │ │
│  │           ↓                                         │ │
│  │  ┌──────────────────────────────────────────────┐  │ │
│  │  │ Record Metrics (OTEL)                        │  │ │
│  │  └──────────────────────────────────────────────┘  │ │
│  │                                                     │ │
│  └─────────────────────────────────────────────────────┘ │
│                                                          │
│  ┌────────────────────────────────────────────────────┐ │
│  │ Metrics HTTP Server (Goroutine)                    │ │
│  │                                                     │ │
│  │  :2112/metrics  → Prometheus exporter             │ │
│  │  :2112/health   → Health check                    │ │
│  │  :2112/debug/pprof → Memory profiling             │ │
│  │                                                     │ │
│  └─────────────────────────────────────────────────────┘ │
│                                                          │
│  ┌────────────────────────────────────────────────────┐ │
│  │ Signal Handler (Goroutine)                         │ │
│  │                                                     │ │
│  │  SIGINT/SIGTERM → Graceful shutdown               │ │
│  │                                                     │ │
│  └─────────────────────────────────────────────────────┘ │
│                                                          │
└──────────────────────────────────────────────────────────┘

           ↓ Metrics Scraping
   ┌────────────────────┐
   │   Prometheus       │
   │   (in K8s)         │
   └────────────────────┘
           ↓
   ┌────────────────────┐
   │   Grafana          │
   │   (Dashboards)     │
   └────────────────────┘
```

### Reconciliation Loop State Machine

```
┌────────────┐
│  Sleeping  │ ◄──────┐
└────────────┘        │
      │               │
      │ Ticker fires  │
      ↓               │
┌────────────┐        │
│  Scanning  │        │
└────────────┘        │
      │               │
      │ Success       │
      ↓               │
┌────────────┐        │
│  Storing   │        │
└────────────┘        │
      │               │
      │ Success       │
      ↓               │
┌────────────┐        │
│ Detecting  │        │
└────────────┘        │
      │               │
      │ Success       │
      ↓               │
┌────────────┐        │
│  Metrics   │        │
└────────────┘        │
      │               │
      │ Complete      │
      └───────────────┘

Error at any stage:
  - Log error
  - Increment error metrics
  - Return to Sleeping
  - Retry next iteration
```

---

## 9. Deployment Patterns

### Systemd Service (Linux)
```ini
# /etc/systemd/system/elava.service
[Unit]
Description=Elava Living Infrastructure Daemon
After=network.target

[Service]
Type=simple
User=elava
ExecStart=/usr/local/bin/elava serve --interval=5m --region=us-east-1
Restart=always
RestartSec=10
StandardOutput=journal
StandardError=journal

[Install]
WantedBy=multi-user.target
```

### Kubernetes Deployment
```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: elava
spec:
  replicas: 1  # Single daemon per region
  selector:
    matchLabels:
      app: elava
  template:
    metadata:
      labels:
        app: elava
      annotations:
        prometheus.io/scrape: "true"
        prometheus.io/port: "2112"
        prometheus.io/path: "/metrics"
    spec:
      serviceAccountName: elava
      containers:
      - name: elava
        image: elava:latest
        command: ["elava", "serve"]
        args:
          - --interval=5m
          - --region=us-east-1
        ports:
        - name: metrics
          containerPort: 2112
        livenessProbe:
          httpGet:
            path: /health
            port: 2112
          initialDelaySeconds: 30
          periodSeconds: 10
        readinessProbe:
          httpGet:
            path: /health
            port: 2112
          initialDelaySeconds: 5
          periodSeconds: 5
        resources:
          requests:
            memory: "256Mi"
            cpu: "100m"
          limits:
            memory: "512Mi"
            cpu: "500m"
```

### Docker Compose (Local Dev)
```yaml
version: '3.8'
services:
  elava:
    build: .
    command: serve --interval=1m
    ports:
      - "2112:2112"
    volumes:
      - ./data:/data
    environment:
      - AWS_REGION=us-east-1
      - AWS_ACCESS_KEY_ID=${AWS_ACCESS_KEY_ID}
      - AWS_SECRET_ACCESS_KEY=${AWS_SECRET_ACCESS_KEY}
    restart: unless-stopped
```

---

## 10. Configuration

### Config File (YAML)
```yaml
# elava.yaml
daemon:
  reconciliation_interval: 5m
  metrics_port: 2112
  graceful_shutdown_timeout: 30s

cloud:
  provider: aws
  region: us-east-1

storage:
  path: /var/lib/elava/data

logging:
  level: info
  format: json
```

### Environment Variables (12-Factor App)
```bash
ELAVA_RECONCILIATION_INTERVAL=5m
ELAVA_METRICS_PORT=2112
ELAVA_CLOUD_PROVIDER=aws
ELAVA_CLOUD_REGION=us-east-1
ELAVA_STORAGE_PATH=/data
ELAVA_LOG_LEVEL=info
```

---

## 11. Metrics

### Daemon-Specific Metrics (NEW)
```go
// Reconciliation metrics
daemon_reconciliation_total          // Counter: total reconciliations
daemon_reconciliation_duration_ms    // Histogram: reconciliation duration
daemon_reconciliation_errors_total   // Counter: failed reconciliations
daemon_last_reconciliation_timestamp // Gauge: Unix timestamp

// Resources scanned
daemon_resources_scanned_total       // Counter: total resources seen
daemon_resources_current             // Gauge: current resource count

// Daemon health
daemon_uptime_seconds                // Gauge: daemon uptime
daemon_goroutines                    // Gauge: active goroutines
daemon_memory_usage_bytes            // Gauge: memory usage
```

### Existing Metrics (ChangeEventMetrics)
```
elava_resources_created_total{type, region}
elava_resources_modified_total{type, region}
elava_resources_disappeared_total{type, region}
elava_change_events_stored_total
```

---

## 12. Health Check

### Health Status
```go
type HealthStatus struct {
    Status            string    `json:"status"` // "healthy", "degraded", "unhealthy"
    Uptime            int64     `json:"uptime_seconds"`
    LastReconciliation time.Time `json:"last_reconciliation"`
    ReconciliationErrors int    `json:"reconciliation_errors"`
    StorageHealthy    bool      `json:"storage_healthy"`
}

// HTTP handler
func (d *Daemon) healthCheck(w http.ResponseWriter, r *http.Request) {
    health := d.computeHealth()

    statusCode := http.StatusOK
    if health.Status == "unhealthy" {
        statusCode = http.StatusServiceUnavailable
    }

    w.WriteHeader(statusCode)
    json.NewEncoder(w).Encode(health)
}
```

---

## 13. Testing Strategy

### Unit Tests (< 30 lines each)
```go
// daemon_test.go
func TestDaemon_Start(t *testing.T)
func TestDaemon_Stop(t *testing.T)
func TestDaemon_GracefulShutdown(t *testing.T)

// reconciler_test.go
func TestReconciler_SingleIteration(t *testing.T)
func TestReconciler_ErrorHandling(t *testing.T)
func TestReconciler_ContextCancellation(t *testing.T)

// metrics_server_test.go
func TestMetricsServer_StartStop(t *testing.T)
func TestMetricsServer_HealthCheck(t *testing.T)
func TestMetricsServer_PrometheusMetrics(t *testing.T)
```

### Integration Test
```go
// daemon_integration_test.go
func TestDaemon_FullLifecycle(t *testing.T) {
    // 1. Start daemon
    // 2. Wait for 2 reconciliation iterations
    // 3. Verify metrics incremented
    // 4. Send SIGTERM
    // 5. Verify graceful shutdown
}
```

---

## 14. Migration Path

### Phase 1: Add `serve` Command (Keep CLI)
```bash
elava scan   # Still works (existing users)
elava serve  # New daemon mode
```

### Phase 2: Deprecate `scan` (Warnings)
```bash
$ elava scan
⚠️  WARNING: 'scan' command is deprecated. Use 'elava serve' for continuous monitoring.
Scanning...
```

### Phase 3: Remove `scan` (Major Version)
```bash
$ elava scan
Error: 'scan' command removed in v2.0. Use 'elava serve'.
```

---

## 15. Definition of Done

- [ ] Design documented ✅
- [ ] Functions are small (<30 lines)
- [ ] Interfaces defined
- [ ] Tests planned (TDD - write first!)
- [ ] Error handling strategy clear
- [ ] Deployment patterns documented
- [ ] Metrics defined
- [ ] Health checks defined
- [ ] Migration path clear

---

## 16. Next Steps (TDD Workflow)

### RED Phase (Write Failing Tests)
1. `daemon_test.go` - Daemon lifecycle tests
2. `reconciler_test.go` - Reconciliation loop tests
3. `metrics_server_test.go` - HTTP server tests

### GREEN Phase (Minimal Implementation)
1. `internal/daemon/daemon.go` - Core daemon
2. `internal/daemon/reconciler.go` - Reconciliation loop
3. `observer/metrics_server.go` - Metrics HTTP server
4. `cmd/elava/serve.go` - CLI command

### REFACTOR Phase
1. Extract smaller functions
2. Add error handling
3. Add graceful shutdown
4. Verify all functions < 30 lines

---

**Following CLAUDE.md principles:**
- ✅ Design session first
- ✅ Small focused functions (<30 lines)
- ✅ Clear interfaces
- ✅ Storage-first thinking
- ✅ TDD workflow (tests before code)
- ✅ Fail fast, error handling
- ✅ No magic, obvious code

Ready to move to implementation!
