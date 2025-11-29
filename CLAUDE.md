# Elava - Stateless Cloud Resource Scanner

**ELAVA = Scan. Emit. Done.**

---

## CRITICAL: Project Nature

**THIS IS A STATELESS SCANNER**
- **Goal**: Scan cloud resources, emit metrics/logs, repeat
- **Language**: 100% Go
- **Status**: Rewriting to stateless architecture
- **Approach**: Mega small (~2000 lines), plugin-based, zero state

---

## PROJECT MISSION

**Mission**: Build the simplest possible cloud resource scanner that emits to OTEL/Prometheus.

**Core Value Proposition:**

**"Scan cloud resources. Emit metrics. Let your observability stack handle the rest."**

**The Differentiator: Stateless**
- Terraform: State files everywhere
- AWS Config: Complex, expensive
- Elava: No state. Scan and emit. Drift detection via PromQL.

**Why This Matters:**
- No state = No state management problems
- Plugin-based = Add any cloud provider
- OTEL-native = Works with your existing stack
- Mega small = Easy to understand and maintain

---

## ARCHITECTURE

```
┌─────────────────────────────────────────────────────────────┐
│                        ELAVA DAEMON                         │
│                                                             │
│   ┌─────────────────────────────────────────────────────┐  │
│   │                   PLUGIN SYSTEM                      │  │
│   │                                                      │  │
│   │   ┌─────────┐  ┌─────────┐  ┌─────────┐            │  │
│   │   │   AWS   │  │   GCP   │  │  Azure  │  ...       │  │
│   │   │ Plugin  │  │ Plugin  │  │ Plugin  │            │  │
│   │   └────┬────┘  └────┬────┘  └────┬────┘            │  │
│   │        └────────────┴────────────┘                  │  │
│   │                     │                                │  │
│   │                     ▼                                │  │
│   │          ┌─────────────────────┐                    │  │
│   │          │  Unified Resource   │                    │  │
│   │          │      Emitter        │                    │  │
│   │          └──────────┬──────────┘                    │  │
│   └─────────────────────┼───────────────────────────────┘  │
│                         │                                   │
│   State: NONE           ▼                                   │
│   Storage: NONE  ┌─────────────┐                           │
│   Drift: BACKEND │ OTEL / NATS │                           │
│                  └─────────────┘                           │
└─────────────────────────────────────────────────────────────┘
```

**Key Points:**
- Daemon runs continuously (ticker-based)
- Plugins implement the Scanner interface
- Emitter outputs to OTEL/Prometheus/NATS
- NO state in Elava - backends handle history

---

## CORE PHILOSOPHY

- **Stateless** - No database, no files, no persistence in Elava
- **Plugin-based** - Cloud providers are plugins implementing Scanner interface
- **Mega small** - Target ~2000 lines of Go
- **OTEL-native** - Direct OpenTelemetry, no wrappers
- **Drift at query layer** - PromQL detects changes, not Elava
- **TDD mandatory** - Tests first, code second
- **Typed everything** - No map[string]interface{}

---

## PLUGIN INTERFACE

The entire plugin contract:

```go
// plugin.go - ~50 lines

type Plugin interface {
    // Name returns plugin identifier
    Name() string  // "aws", "gcp", "azure"

    // Scan returns all resources from this provider
    Scan(ctx context.Context) ([]Resource, error)
}

type Resource struct {
    ID       string            // "i-abc123"
    Type     string            // "ec2", "rds", "s3"
    Provider string            // "aws", "gcp", "azure"
    Region   string            // "us-east-1"
    Status   string            // "running", "stopped"
    Labels   map[string]string // Normalized tags
    Attrs    map[string]string // Provider-specific
}
```

**Adding a new provider:**
```go
type MyCloudPlugin struct{}

func (p *MyCloudPlugin) Name() string { return "mycloud" }

func (p *MyCloudPlugin) Scan(ctx context.Context) ([]Resource, error) {
    // Call MyCloud API
    // Return normalized resources
}
```

---

## DAEMON LOOP

The entire daemon (~100 lines):

```go
func main() {
    ctx := setupSignalHandler()
    plugins := loadPlugins(config)
    emitter := newEmitter(config)

    ticker := time.NewTicker(config.Interval)
    defer ticker.Stop()

    // Initial scan
    scan(ctx, plugins, emitter)

    for {
        select {
        case <-ticker.C:
            scan(ctx, plugins, emitter)
        case <-ctx.Done():
            log.Info().Msg("shutting down")
            return
        }
    }
}

func scan(ctx context.Context, plugins []Plugin, emitter Emitter) {
    for _, p := range plugins {
        resources, err := p.Scan(ctx)
        if err != nil {
            log.Error().Err(err).Str("plugin", p.Name()).Msg("scan failed")
            continue
        }
        emitter.Emit(resources)
    }
}
```

**That's it.** No supervisor. No complex orchestration. K8s handles restarts.

---

## TDD WORKFLOW (MANDATORY)

**ALL CODE MUST FOLLOW TEST-DRIVEN DEVELOPMENT**

### RED Phase: Write Failing Test First
```go
func TestAWSPlugin_ScanEC2(t *testing.T) {
    plugin := NewAWSPlugin(mockClient)  // Doesn't exist yet

    resources, err := plugin.Scan(context.Background())
    require.NoError(t, err)
    require.Len(t, resources, 2)

    assert.Equal(t, "i-abc123", resources[0].ID)
    assert.Equal(t, "ec2", resources[0].Type)
}
// $ go test → FAILS (RED confirmed)
```

### GREEN Phase: Minimal Implementation
```go
type AWSPlugin struct {
    client EC2Client
}

func NewAWSPlugin(client EC2Client) *AWSPlugin {
    return &AWSPlugin{client: client}
}

func (p *AWSPlugin) Scan(ctx context.Context) ([]Resource, error) {
    instances, err := p.client.DescribeInstances(ctx)
    if err != nil {
        return nil, fmt.Errorf("describe instances: %w", err)
    }

    var resources []Resource
    for _, inst := range instances {
        resources = append(resources, Resource{
            ID:       inst.InstanceID,
            Type:     "ec2",
            Provider: "aws",
            Status:   inst.State,
        })
    }
    return resources, nil
}
// $ go test → PASS (GREEN confirmed)
```

### REFACTOR Phase: Improve Quality
```go
// Add edge cases, extract helpers, improve naming
func TestAWSPlugin_ScanEC2_Empty(t *testing.T) { ... }
func TestAWSPlugin_ScanEC2_Error(t *testing.T) { ... }
// $ go test → STILL PASS (REFACTOR complete)
```

### TDD Checklist
- [ ] **RED**: Write failing test first
- [ ] **GREEN**: Write minimal implementation
- [ ] **REFACTOR**: Add edge cases, improve design
- [ ] **Commit**: Small commits (<30 lines)

---

## BANNED PATTERNS - AUTOMATIC REJECTION

### map[string]interface{} IS BANNED
```go
// NEVER - INSTANT REJECTION
func Process(data map[string]interface{}) error

// ALWAYS - TYPED STRUCTS
type Resource struct {
    ID     string
    Type   string
    Status string
}
```

### NO TODOs OR STUBS
```go
// INSTANT REJECTION
func Scan() error {
    // TODO: implement
    return nil
}

// COMPLETE IMPLEMENTATION ONLY
func Scan(ctx context.Context) ([]Resource, error) {
    instances, err := client.List(ctx)
    if err != nil {
        return nil, fmt.Errorf("list instances: %w", err)
    }
    return normalize(instances), nil
}
```

### NO STATE IN ELAVA
```go
// INSTANT REJECTION
type Scanner struct {
    db        *sql.DB        // NO!
    lastState map[string]Resource  // NO!
}

// CORRECT - STATELESS
type Scanner struct {
    client CloudClient
    emitter Emitter
}
```

---

## OTEL STANDARDS (MANDATORY)

### Direct OTEL Only - NO WRAPPERS
```go
// BANNED
import "custom/telemetry/wrapper"

// REQUIRED
import (
    "go.opentelemetry.io/otel"
    "go.opentelemetry.io/otel/metric"
)

// Metric naming
elava_resource_info{...}           // Resource state
elava_scan_duration_seconds{...}   // Scan timing
elava_scan_resources_total{...}    // Resource count
elava_scan_errors_total{...}       // Error count
```

---

## PACKAGE STRUCTURE

```
elava/
├── cmd/elava/
│   └── main.go              # ~100 lines (daemon loop)
│
├── internal/
│   ├── plugin/
│   │   ├── plugin.go        # ~50 lines (interface)
│   │   ├── aws/
│   │   │   ├── plugin.go    # ~200 lines
│   │   │   ├── ec2.go       # ~100 lines
│   │   │   ├── rds.go       # ~100 lines
│   │   │   └── s3.go        # ~100 lines
│   │   ├── gcp/
│   │   │   └── plugin.go
│   │   └── azure/
│   │       └── plugin.go
│   │
│   ├── emitter/
│   │   ├── emitter.go       # ~50 lines (interface)
│   │   ├── otel.go          # ~100 lines
│   │   ├── prometheus.go    # ~100 lines
│   │   └── nats.go          # ~100 lines (paid)
│   │
│   └── config/
│       └── config.go        # ~100 lines
│
├── pkg/
│   └── resource/
│       └── resource.go      # ~50 lines (unified model)
│
└── deployments/
    └── kubernetes/
        └── deployment.yaml
```

**Size target:**
- Core: ~350 lines
- AWS plugin: ~500 lines
- Total with AWS: ~1000 lines
- Total with all providers: ~2000 lines

---

## ERROR HANDLING

```go
// BAD - No context
return err

// BAD - Ignored error
_ = client.Close()

// GOOD - Contextual error
if err := client.DescribeInstances(ctx); err != nil {
    return nil, fmt.Errorf("describe instances in %s: %w", region, err)
}

// GOOD - Proper cleanup
defer func() {
    if err := client.Close(); err != nil {
        log.Error().Err(err).Msg("failed to close client")
    }
}()
```

---

## VERIFICATION (BEFORE EVERY COMMIT)

```bash
# Quick check
go fmt ./...
go vet ./...
go test ./... -race

# Full verification
golangci-lint run
go test ./... -cover  # Must be >80%
```

---

## DEFINITION OF DONE

A feature is complete when:
- [ ] Tests written FIRST (TDD)
- [ ] All tests passing with -race
- [ ] Coverage >= 80%
- [ ] NO TODOs or stubs
- [ ] NO map[string]interface{}
- [ ] NO state in Elava
- [ ] Functions < 50 lines
- [ ] `go fmt && go vet && golangci-lint run` passes

---

## INSTANT REJECTION CRITERIA

Your code will be REJECTED for:
1. **ANY state/storage in Elava** (databases, files, caches)
2. **ANY map[string]interface{}**
3. **ANY TODO/FIXME/stub**
4. **Missing tests or <80% coverage**
5. **Ignored errors** (`_ = func()`)
6. **Commits > 30 lines**
7. **Functions > 50 lines**
8. **Custom telemetry wrappers** (use direct OTEL)

---

## DEPLOYMENT

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: elava
spec:
  replicas: 1
  template:
    spec:
      containers:
      - name: elava
        image: elava:latest
        env:
        - name: OTEL_ENDPOINT
          value: "http://otel-collector:4317"
        - name: SCAN_INTERVAL
          value: "5m"
        - name: AWS_REGION
          value: "us-east-1"
        resources:
          requests:
            memory: "64Mi"
            cpu: "50m"
          limits:
            memory: "128Mi"
            cpu: "200m"
      # No PVC needed - stateless!
```

---

## DRIFT DETECTION

Elava doesn't detect drift. The backend does.

**PromQL examples:**
```promql
# Resources that changed status in last hour
changes(elava_resource_info{type="ec2"}[1h]) > 0

# Resources that disappeared
absent_over_time(elava_resource_info{id="i-abc123"}[10m])

# New resources (appeared recently)
elava_resource_info unless elava_resource_info offset 1h
```

**Grafana alerts:**
- "EC2 instance status changed"
- "Resource disappeared"
- "Untagged resource detected"

---

## PRODUCT TIERS

| Feature | Free | Paid |
|---------|------|------|
| AWS scanning | Y | Y |
| GCP scanning | Y | Y |
| Azure scanning | Y | Y |
| OTEL export | Y | Y |
| Standalone mode | Y | Y |
| Grafana dashboard | Y | Y |
| NATS -> AHTI | N | Y |
| Cross-system correlation | N | Y |

---

## RELATED PROJECTS

- **TAPIO** - K8s + eBPF observability (what's happening inside the cluster)
- **AHTI** - Universal graph backend (correlation engine)
- **RAUTA** - Gateway API controller (traffic routing)
- **KULTA** - Progressive delivery (canary rollouts)

**Ecosystem fit:**
```
TAPIO (K8s/eBPF) ──┐
                   ├──> AHTI (correlation) ──> Insights
ELAVA (Cloud)   ───┘
```

---

## FINAL MANIFESTO

**Elava is the simplest possible cloud resource scanner.**

**We DO:**
- Scan cloud resources
- Emit metrics/logs
- Support multiple providers via plugins
- Stay mega small (~2000 lines)

**We DON'T:**
- Store state
- Detect drift (backend's job)
- Build complex features
- Over-engineer

**Scan. Emit. Done.**

---

**False Systems** 🇫🇮
