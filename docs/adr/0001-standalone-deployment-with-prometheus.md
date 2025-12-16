# ADR-0001: Standalone Deployment with Prometheus 3.8

**Status:** Proposed
**Date:** 2025-12-04
**Authors:** Elava Team
**Supersedes:** Initial VictoriaMetrics proposal

---

## Context

Elava is a stateless cloud resource scanner that emits metrics. The current deployment model requires a full observability stack (OTEL Collector + Prometheus + Loki + Jaeger + Grafana), which is heavyweight for users who want a simple "scan my cloud and show me what's there" experience.

We need a **standalone free tier** deployment that is:
- Easy to install
- Production-ready
- Minimal dependencies
- Works on Kubernetes and cloud VMs

### Current State

```
deployments/
└── otel-collector/
    └── local/
        └── docker-compose.yml  # 5 containers, ~200 lines YAML, ~2GB RAM
```

**Current flow:**
```
Elava → OTLP push → OTEL Collector → Prometheus/Loki/Jaeger → Grafana
```

This is overkill for a standalone free tier.

---

## Decision

We will create a **standalone deployment** using **Prometheus 3.8** as the metrics backend, leveraging its **native OTLP receiver** for direct push from Elava.

Deployment methods:
1. **Helm chart** for Kubernetes deployments
2. **Terraform modules** for cloud VM deployments (AWS, GCP, Azure)

### Why Prometheus 3.8 over VictoriaMetrics?

We initially considered VictoriaMetrics but reconsidered after Prometheus 3.0+ matured throughout 2025.

| Factor | VictoriaMetrics | Prometheus 3.8 |
|--------|-----------------|----------------|
| OTLP support | Via remote write | **Native receiver** (built-in) |
| Push model | Needs scrape config | **Direct OTLP push** |
| UI | vmui | **New modern UI** (PromLens-style) |
| Histograms | Good | **Native histograms stable** |
| Ecosystem | Growing | **Industry standard** |
| Resources | Low | **Improved in 3.x** |
| Long-term storage | Built-in | Needs Thanos (acceptable for free tier) |
| Learning curve | New tool | **Everyone knows Prometheus** |

**Key insight:** Prometheus 3.8's native OTLP receiver eliminates the need for scrape configuration. Elava already speaks OTLP - we just point it at Prometheus.

### Architecture

**Standalone flow (push model):**
```
Elava --OTLP push--> Prometheus 3.8 (/api/v1/otlp/v1/metrics) --> UI
```

**Components:**
- **Elava**: Scans cloud resources, pushes metrics via OTLP
- **Prometheus 3.8**: Receives OTLP, stores metrics, provides UI
- **No OTEL Collector needed**
- **No scrape configuration needed**

**Configuration:**
```toml
# elava.toml
[otel]
endpoint = "prometheus:4318"
insecure = true

[otel.metrics]
enabled = true
```

That's it. Push model. No scrape config.

---

## Deployment Options

### Option 1: Kubernetes (Helm Chart)

**Install:**
```bash
helm repo add elava https://yairfalse.github.io/elava
helm install elava-standalone elava/standalone
```

**Customization via values.yaml:**
```yaml
elava:
  config:
    aws:
      regions: ["us-east-1", "eu-west-1"]
    scanner:
      interval: "5m"
    otel:
      endpoint: "elava-prometheus:4318"
      insecure: true
      metrics:
        enabled: true

prometheus:
  version: "3.8.0"
  retention: "30d"
  storage:
    size: "10Gi"
  # OTLP receiver enabled by default

ingress:
  enabled: true
  host: elava.example.com
```

**Why Helm:**
- Industry standard for K8s packaging
- values.yaml provides clean customization
- Built-in upgrade/rollback
- Easy to extend (add Grafana, alerting, etc.)

### Option 2: Cloud VMs (Terraform)

**Install:**
```bash
cd deployments/standalone/terraform/aws
terraform init
terraform apply -var="region=us-east-1"
```

**What Terraform creates:**
- VM instance (t3.small or equivalent)
- Security group (ports 9090, 4318, 22)
- Docker + Docker Compose installed
- Elava + Prometheus 3.8 running
- Persistent EBS volume for metrics

**Why Terraform:**
- Infrastructure as Code (reproducible)
- Works across AWS/GCP/Azure with separate modules
- Production-ready (proper networking, storage)
- Easy to extend (add TLS, DNS, backups)

---

## Alternatives Considered

### 1. VictoriaMetrics (Rejected)

**Pros:** Lower resources, built-in long-term storage, vmui
**Cons:**
- Another tool to learn
- Prometheus 3.8 now has comparable UI
- No native OTLP receiver (needs scrape config)
- Less ecosystem support

**Verdict:** Prometheus 3.8's native OTLP receiver and industry-standard status wins.

### 2. OTEL Collector + Prometheus (Rejected)

**Pros:** Flexible pipeline, can fan-out to multiple backends
**Cons:**
- Extra component (OTEL Collector)
- More config files
- Prometheus 3.8 receives OTLP natively now

**Verdict:** Prometheus 3.8 eliminates need for OTEL Collector for metrics.

### 3. Docker Compose Only for Cloud (Rejected)

**Pros:** Simple, portable
**Cons:**
- Not production-ready alone
- No infrastructure management
- Manual VM setup required

**Verdict:** Terraform wrapper provides proper IaC.

### 4. Pre-baked AMI/VM Images (Rejected)

**Pros:** Easiest UX ("just launch")
**Cons:**
- Maintenance burden (rebuild for each update)
- Cloud-specific
- Version management nightmare

**Verdict:** Too much maintenance overhead.

### 5. Raw K8s Manifests (Rejected)

**Pros:** Simple, no Helm dependency
**Cons:**
- No clean customization
- No upgrade/rollback mechanism

**Verdict:** Helm is the standard; no reason to avoid it.

---

## Implementation

### Directory Structure

```
deployments/
├── otel-collector/              # Existing full stack (keep for traces/logs)
│   └── ...
│
└── standalone/                  # NEW
    ├── README.md                # Quick start guide
    │
    ├── helm/                    # Kubernetes
    │   ├── Chart.yaml
    │   ├── values.yaml
    │   └── templates/
    │       ├── _helpers.tpl
    │       ├── namespace.yaml
    │       ├── elava-deployment.yaml
    │       ├── elava-service.yaml
    │       ├── elava-configmap.yaml
    │       ├── prometheus-deployment.yaml
    │       ├── prometheus-service.yaml
    │       ├── prometheus-pvc.yaml
    │       └── prometheus-configmap.yaml
    │
    └── terraform/               # Cloud VMs
        ├── modules/
        │   └── elava-standalone/
        │       ├── main.tf
        │       ├── variables.tf
        │       ├── outputs.tf
        │       ├── docker-compose.yml
        │       └── cloud-init.yaml
        │
        ├── aws/
        │   ├── main.tf
        │   ├── variables.tf
        │   └── outputs.tf
        │
        ├── gcp/
        │   └── ...
        │
        └── azure/
            └── ...
```

### Prometheus 3.8 Configuration

```yaml
# prometheus.yml (minimal - OTLP receiver enabled by default in 3.8)
global:
  scrape_interval: 60s

# OTLP receiver is enabled by default on :4318
# No scrape_configs needed for Elava - it pushes via OTLP

# Optional: scrape Prometheus itself
scrape_configs:
  - job_name: 'prometheus'
    static_configs:
      - targets: ['localhost:9090']
```

### Docker Compose (used by Terraform)

```yaml
# docker-compose.yml
services:
  elava:
    image: ghcr.io/yairfalse/elava:latest
    volumes:
      - ~/.aws:/root/.aws:ro
      - ./elava.toml:/etc/elava.toml:ro
    command: ["--config", "/etc/elava.toml"]
    depends_on:
      - prometheus

  prometheus:
    image: prom/prometheus:v3.8.0
    command:
      - '--config.file=/etc/prometheus/prometheus.yml'
      - '--storage.tsdb.path=/prometheus'
      - '--storage.tsdb.retention.time=30d'
      - '--web.enable-otlp-receiver'
    volumes:
      - ./prometheus.yml:/etc/prometheus/prometheus.yml:ro
      - prometheus-data:/prometheus
    ports:
      - "9090:9090"   # UI + PromQL
      - "4318:4318"   # OTLP HTTP receiver

volumes:
  prometheus-data:
```

### User Experience

| Target | Install Command | Access |
|--------|-----------------|--------|
| K8s | `helm install elava ./helm` | `kubectl port-forward svc/elava-prometheus 9090` |
| AWS | `cd terraform/aws && terraform apply` | Output: `http://<ip>:9090` |
| GCP | `cd terraform/gcp && terraform apply` | Output: `http://<ip>:9090` |
| Azure | `cd terraform/azure && terraform apply` | Output: `http://<ip>:9090` |

---

## Prometheus 3.8 Features We Leverage

| Feature | How Elava Uses It |
|---------|-------------------|
| **Native OTLP receiver** | Direct push from Elava, no scrape config |
| **Native histograms** | Efficient scan duration metrics |
| **New UI** | Built-in visualization, no Grafana required |
| **UTF-8 support** | Clean OTel metric names |
| **Remote Write 2.0** | Future: federate to central Prometheus |
| **Unified AWS SD** | Future: auto-discover Elava instances |

---

## Consequences

### Positive

1. **Simpler architecture** - Push model, no scrape config
2. **No new tools** - Everyone knows Prometheus
3. **Standard ecosystem** - Grafana, Alertmanager, etc. all work
4. **Lower complexity** - 2 components, push model
5. **No OTEL Collector** - Prometheus receives OTLP natively
6. **Future-proof** - Native histograms, modern features

### Negative

1. **No built-in long-term storage** - 30d default retention
   - Acceptable for free tier
   - Can add Thanos later if needed
2. **Slightly higher resources than VM** - But improved in 3.x
3. **Two deployment systems** - Helm + Terraform to maintain

### Neutral

1. **Full stack still available** - otel-collector deployment remains for traces/logs
2. **Can add Grafana** - values.yaml can enable optional Grafana for dashboards
3. **Migration path** - Can switch to VictoriaMetrics later if needed (compatible)

---

## Migration from OTEL Collector Setup

For users on the existing full stack who want to simplify:

```bash
# Before: 5 containers
docker-compose -f deployments/otel-collector/local/docker-compose.yml down

# After: 2 containers
docker-compose -f deployments/standalone/docker-compose.yml up -d
```

Update elava.toml:
```toml
# Before
[otel]
endpoint = "otel-collector:4317"  # gRPC to collector

# After
[otel]
endpoint = "prometheus:4318"       # HTTP directly to Prometheus
```

---

## Future Considerations

1. **Grafana dashboards** - Pre-built dashboards for Elava metrics
2. **Alertmanager integration** - Drift detection alerts
3. **Thanos** - Long-term storage for paid tier
4. **Federation** - Central Prometheus aggregating multiple Elava instances
5. **AHTI integration** - Paid tier pushing to AHTI graph backend

---

## References

- [Prometheus 3.8.0 Release](https://github.com/prometheus/prometheus/releases/tag/v3.8.0)
- [Prometheus OTLP Receiver](https://prometheus.io/docs/prometheus/latest/configuration/configuration/#otlp)
- [Prometheus Native Histograms](https://prometheus.io/docs/concepts/native_histograms/)
- [Helm Best Practices](https://helm.sh/docs/chart_best_practices/)
- [Terraform Module Structure](https://developer.hashicorp.com/terraform/language/modules/develop/structure)

---

**Decision:** Approved / Pending Review
**Reviewers:**
