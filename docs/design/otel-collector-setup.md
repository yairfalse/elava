# OTEL Collector Configuration Design

## Design Session Checklist

- [x] What problem are we solving?
- [x] How will this interact with storage (read/write/query)?
- [x] What historical data do we need to track?
- [x] What's the simplest solution?
- [x] Can we break it into smaller functions?
- [x] What interfaces do we need?
- [x] What can go wrong?
- [x] Draw the flow (ASCII or diagram)

## Problem Statement

**What are we solving?**
- Elava now emits OTEL telemetry (traces, metrics, log events)
- Need to collect, process, and route this telemetry to observability backends
- Must support multiple deployment scenarios (local dev, production, cloud)
- Should be simple to configure and test

**Current State:**
- ✅ Elava emits OTLP via gRPC/HTTP
- ❌ No collector configs for users to deploy
- ❌ No local testing setup
- ❌ No examples for popular backends

## Architecture Flow

```
┌──────────────────────────────────────────────────────────────┐
│                        Elava Application                      │
│                                                               │
│  Emits OTLP signals:                                         │
│  - Traces (reconciliation spans)                             │
│  - Metrics (changes, decisions, durations)                   │
│  - Logs (span events for infrastructure changes)             │
│                                                               │
└────────────────────────┬─────────────────────────────────────┘
                         │
                         │ OTLP (gRPC: 4317 or HTTP: 4318)
                         │
                         ▼
┌──────────────────────────────────────────────────────────────┐
│                    OTEL Collector                             │
│                                                               │
│  ┌────────────────────────────────────────────────────────┐  │
│  │ Receivers:                                             │  │
│  │  - otlp (gRPC/HTTP) ← Primary                         │  │
│  │  - prometheus (scrape) ← Optional                     │  │
│  └────────────────────────────────────────────────────────┘  │
│                         │                                     │
│  ┌────────────────────────────────────────────────────────┐  │
│  │ Processors:                                            │  │
│  │  - batch (performance)                                │  │
│  │  - resource (add metadata)                            │  │
│  │  - attributes (filter sensitive data)                 │  │
│  │  - filter (route by condition)                        │  │
│  └────────────────────────────────────────────────────────┘  │
│                         │                                     │
│  ┌────────────────────────────────────────────────────────┐  │
│  │ Exporters:                                             │  │
│  │  - prometheus (metrics)                               │  │
│  │  - loki (logs)                                        │  │
│  │  - jaeger (traces)                                    │  │
│  │  - datadog (all-in-one)                               │  │
│  │  - debug (testing)                                    │  │
│  └────────────────────────────────────────────────────────┘  │
│                                                               │
└────────────┬──────────────┬──────────────┬───────────────────┘
             │              │              │
    ┌────────▼───┐  ┌──────▼──────┐  ┌───▼────────┐
    │ Prometheus │  │    Loki     │  │   Jaeger   │
    │ (Metrics)  │  │   (Logs)    │  │  (Traces)  │
    └────────────┘  └─────────────┘  └────────────┘
                      │
              ┌───────▼────────┐
              │    Grafana     │
              │  (Dashboards)  │
              └────────────────┘
```

## Configuration Strategy

### Approach: Multiple Config Files for Different Scenarios

**Why multiple configs?**
1. **Local Development** - Simple, all-in-one, docker-compose
2. **Production** - Scalable, multi-backend, security features
3. **Cloud-Specific** - AWS CloudWatch, GCP Cloud Monitoring, Azure Monitor
4. **SaaS** - Datadog, New Relic, Honeycomb

### File Structure

```
deployments/otel-collector/
├── README.md                          # Setup guide
├── local/
│   ├── docker-compose.yml            # Full local stack
│   ├── collector-config.yaml         # Local collector config
│   ├── prometheus.yml                # Prometheus config
│   └── grafana/
│       └── dashboards/               # Pre-built dashboards
├── production/
│   ├── collector-config.yaml         # Production setup
│   ├── kubernetes/
│   │   ├── deployment.yaml           # K8s deployment
│   │   └── service.yaml              # K8s service
│   └── systemd/
│       └── otel-collector.service    # Systemd service
├── cloud/
│   ├── aws-cloudwatch.yaml           # AWS setup
│   ├── gcp-monitoring.yaml           # GCP setup
│   └── azure-monitor.yaml            # Azure setup
└── saas/
    ├── datadog.yaml                  # Datadog all-in-one
    ├── newrelic.yaml                 # New Relic
    └── honeycomb.yaml                # Honeycomb
```

## Configuration Designs

### 1. Local Development Config (Simplest)

**Use Case:** Developer wants to test Elava with full observability locally

**Requirements:**
- Single `docker-compose up` command
- All backends included (Prometheus, Loki, Jaeger, Grafana)
- Pre-configured dashboards
- Debug exporter enabled

**Components:**
```yaml
services:
  - otel-collector
  - prometheus
  - loki
  - jaeger
  - grafana (pre-configured with all data sources)
```

**Storage:** None needed (in-memory for testing)

### 2. Production Config

**Use Case:** Production deployment with reliability and security

**Requirements:**
- Persistent storage
- Resource limits
- Security (TLS, authentication)
- High availability (multiple collector instances)
- Sampling for cost control
- Sensitive data filtering

**Key Features:**
- Batch processing for efficiency
- Retry logic for reliability
- Multiple exporters (primary + backup)
- Health checks

### 3. Cloud-Specific Configs

**Use Case:** Use cloud-native observability

**AWS:**
- CloudWatch for metrics/logs
- X-Ray for traces
- S3 for long-term storage

**GCP:**
- Cloud Monitoring
- Cloud Logging
- Cloud Trace

**Azure:**
- Azure Monitor
- Application Insights

### 4. SaaS Configs

**Use Case:** Send to managed observability platforms

**Datadog:**
- All signals to Datadog (simplest)
- API key configuration
- Hostname mapping

**New Relic:**
- License key configuration
- Data ingest API

**Honeycomb:**
- API key + dataset
- Optimized for trace sampling

## What Can Go Wrong?

### 1. Configuration Issues
- **Problem:** Invalid YAML syntax
- **Solution:** Validate with `otel-collector validate --config=config.yaml`

### 2. Network Connectivity
- **Problem:** Collector can't reach Elava or backends
- **Solution:** Health checks, debug exporter, network diagnostics

### 3. Resource Exhaustion
- **Problem:** Collector OOMs or CPU spikes
- **Solution:** Memory limits, batch processing, sampling

### 4. Data Loss
- **Problem:** Telemetry dropped during outages
- **Solution:** Persistent queue, retry logic, multiple collectors

### 5. Security
- **Problem:** Sensitive data in logs/traces
- **Solution:** Attribute filtering, redaction processors

### 6. Cost
- **Problem:** High ingestion costs
- **Solution:** Sampling, filtering, tail-based sampling

## Simplest Solution

**Start with:**
1. ✅ Local docker-compose setup (for testing)
2. ✅ Production config template (for real deployment)
3. ✅ Datadog config (popular SaaS)
4. ⏳ Documentation with examples

**Skip for now:**
- Kubernetes deployment (can add later)
- Cloud-specific configs (user-specific)
- Advanced features (tail sampling, load balancing)

## Interfaces Needed

### OTEL Collector Config Interface
```yaml
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
  prometheus:
    endpoint: "0.0.0.0:8889"

service:
  pipelines:
    metrics:
      receivers: [otlp]
      processors: [batch]
      exporters: [prometheus]
```

### Environment Variables Interface
```bash
# Elava config
OTEL_EXPORTER_OTLP_ENDPOINT=http://otel-collector:4317
OTEL_SERVICE_NAME=elava
OTEL_RESOURCE_ATTRIBUTES=deployment.environment=production,service.version=v1.0.0

# Collector config (for SaaS)
DD_API_KEY=your-datadog-api-key
NR_LICENSE_KEY=your-newrelic-license
```

## Breaking It Into Smaller Pieces

### Phase 1: Local Setup (This PR)
- [ ] Create `deployments/otel-collector/` directory
- [ ] Write `local/docker-compose.yml`
- [ ] Write `local/collector-config.yaml`
- [ ] Write `local/prometheus.yml`
- [ ] Write `deployments/otel-collector/README.md`
- [ ] Test: `docker-compose up` + run Elava + verify in Grafana

### Phase 2: Production Config (This PR)
- [ ] Write `production/collector-config.yaml`
- [ ] Document resource requirements
- [ ] Add security considerations

### Phase 3: SaaS Config (This PR)
- [ ] Write `saas/datadog.yaml`
- [ ] Document API key setup

### Phase 4: Documentation (This PR)
- [ ] Quick start guide
- [ ] Configuration reference
- [ ] Troubleshooting

## Implementation Plan

### Files to Create:

```
deployments/otel-collector/
├── README.md (~200 lines)
├── local/
│   ├── docker-compose.yml (~150 lines)
│   ├── collector-config.yaml (~100 lines)
│   └── prometheus.yml (~30 lines)
├── production/
│   └── collector-config.yaml (~150 lines)
└── saas/
    └── datadog.yaml (~80 lines)
```

**Total: ~710 lines** across 6 files

### Function Requirements:

No code needed - pure configuration files. But we need:
1. ✅ Valid YAML syntax
2. ✅ Working docker-compose
3. ✅ Tested configs (at least local)
4. ✅ Clear documentation

## Success Criteria

### Must Have:
- [ ] Local setup works with `docker-compose up`
- [ ] Can see Elava metrics in Grafana
- [ ] Can see Elava traces in Jaeger
- [ ] README has clear setup instructions
- [ ] All configs are valid YAML

### Nice to Have:
- [ ] Pre-built Grafana dashboard
- [ ] Health check endpoints documented
- [ ] Query examples (PromQL, LogQL, TraceQL)
- [ ] Cost optimization tips

## Next Steps

1. Create directory structure
2. Write local docker-compose (simplest, test first)
3. Write collector configs
4. Test with Elava
5. Write documentation
6. Commit and push

---

**Decision: Start with local setup** - Gives immediate value for developers and provides foundation for other configs.
