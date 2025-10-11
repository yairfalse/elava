# Elava OpenTelemetry Collector Setup

Complete observability for Elava using OpenTelemetry Collector with support for multiple backends.

## Quick Start - Local Development

The fastest way to get full observability locally:

```bash
cd deployments/otel-collector/local
docker-compose up -d
```

This starts:
- ✅ **OTEL Collector** - Receives telemetry from Elava
- ✅ **Prometheus** - Metrics storage (http://localhost:9090)
- ✅ **Loki** - Log aggregation
- ✅ **Jaeger** - Distributed tracing UI (http://localhost:16686)
- ✅ **Grafana** - Unified dashboard (http://localhost:3000)

### Configure Elava

Set these environment variables to send telemetry to the collector:

```bash
export OTEL_EXPORTER_OTLP_ENDPOINT=http://localhost:4317
export OTEL_SERVICE_NAME=elava
export OTEL_RESOURCE_ATTRIBUTES=deployment.environment=local
```

### Run Elava

```bash
# Run a reconciliation scan
elava reconcile --config config.yaml

# Or run continuously
elava watch --config config.yaml
```

### View Telemetry

- **Grafana**: http://localhost:3000 (admin/admin)
  - Pre-configured with Prometheus, Loki, and Jaeger data sources
  - Dashboards for metrics, logs, and traces

- **Prometheus**: http://localhost:9090
  - Query metrics: `elava_changes_detected_total`, `elava_decisions_made_total`
  - View targets and configuration

- **Jaeger**: http://localhost:16686
  - View distributed traces
  - Search by service, operation, tags
  - Analyze latency and dependencies

### Stop Stack

```bash
docker-compose down
```

To also remove data volumes:

```bash
docker-compose down -v
```

---

## Architecture

```
┌──────────────┐
│    Elava     │  Emits OTLP (gRPC/HTTP)
└──────┬───────┘
       │
       ▼
┌──────────────┐
│ OTEL         │  Receives, processes, exports
│ Collector    │
└──┬─────┬─────┘
   │     │
   ▼     ▼     ▼
┌─────┐ ┌────┐ ┌────────┐
│Prom │ │Loki│ │ Jaeger │
└─────┘ └────┘ └────────┘
   └──────┴───────┴────┐
                  ┌────▼────┐
                  │ Grafana │
                  └─────────┘
```

### Telemetry Signals

**1. Traces** (Distributed tracing)
- Reconciliation span (root)
- Observe phase span
- Detect phase span
- Decide phase span
- Execute phase span (if applicable)

**2. Metrics** (Time-series data)
- `elava_changes_detected_total` - Counter of infrastructure changes
- `elava_decisions_made_total` - Counter of policy decisions
- `elava_policy_violations_total` - Counter of violations
- `elava_actions_executed_total` - Counter of executed actions
- `elava_resources_current` - Gauge of current resources
- `elava_resources_untagged` - Gauge of untagged resources
- `elava_resources_blessed` - Gauge of blessed resources
- `elava_reconcile_duration_ms` - Histogram of reconciliation durations
- `elava_detect_duration_ms` - Histogram of change detection durations

**3. Logs** (Structured events via span events)
- `infrastructure.change.detected` - Resource appeared/disappeared/changed
- `infrastructure.decision.made` - Policy decision made
- `infrastructure.action.executed` - Action execution result
- `infrastructure.policy.violation` - Policy violation detected
- `infrastructure.scan.completed` - Scan completion summary

---

## Production Deployment

### Configuration

Use the production config with environment variables:

```bash
# Required environment variables
export ENVIRONMENT=production
export SERVICE_VERSION=v1.0.0
export K8S_CLUSTER_NAME=prod-cluster
export K8S_NAMESPACE=elava
export JAEGER_ENDPOINT=jaeger:4317
export LOKI_ENDPOINT=http://loki:3100/loki/api/v1/push
```

### Deploy Collector

**Using Docker:**

```bash
docker run -d \
  --name otel-collector \
  -p 4317:4317 \
  -p 4318:4318 \
  -p 8889:8889 \
  -v $(pwd)/production/collector-config.yaml:/etc/otel-collector-config.yaml \
  -e ENVIRONMENT=production \
  -e SERVICE_VERSION=v1.0.0 \
  otel/opentelemetry-collector-contrib:0.95.0 \
  --config=/etc/otel-collector-config.yaml
```

**Using Kubernetes:**

```bash
# Apply the collector deployment
kubectl apply -f production/kubernetes/

# Verify deployment
kubectl get pods -n observability
kubectl logs -n observability deployment/otel-collector
```

**Using Systemd:**

```bash
# Copy config
sudo cp production/collector-config.yaml /etc/otel-collector/config.yaml

# Install systemd service
sudo cp production/systemd/otel-collector.service /etc/systemd/system/

# Start service
sudo systemctl daemon-reload
sudo systemctl enable otel-collector
sudo systemctl start otel-collector

# Check status
sudo systemctl status otel-collector
sudo journalctl -u otel-collector -f
```

### Security Considerations

**1. TLS/SSL**
- Enable TLS for OTLP receivers
- Use mutual TLS for authentication
- Store certificates securely

**2. Authentication**
- Use bearer token auth for OTLP
- Rotate API keys regularly
- Use secrets management (Vault, K8s secrets)

**3. Data Filtering**
- Remove sensitive data (AWS keys, passwords)
- Hash PII (emails, user IDs)
- Redact resource tags with secrets

**4. Network Security**
- Firewall rules for collector ports
- VPC/private network for backends
- Rate limiting on receivers

### Resource Requirements

**Minimum (small deployments):**
- CPU: 2 cores
- Memory: 2 GB
- Disk: 10 GB (for local buffering)

**Recommended (medium deployments):**
- CPU: 4 cores
- Memory: 4 GB
- Disk: 50 GB

**Large scale:**
- CPU: 8+ cores
- Memory: 8+ GB
- Disk: 100+ GB
- Multiple collector instances with load balancing

### High Availability

**Multiple Collectors:**

```bash
# Deploy 3 collector instances
docker-compose -f production/docker-compose-ha.yaml up -d

# Use load balancer
export OTEL_EXPORTER_OTLP_ENDPOINT=http://lb.example.com:4317
```

**Persistent Queue:**

Enable in collector config:
```yaml
exporters:
  otlp:
    sending_queue:
      enabled: true
      num_consumers: 10
      queue_size: 5000
      storage: file_storage

extensions:
  file_storage:
    directory: /var/lib/otel/queue
    timeout: 10s
```

---

## SaaS Observability Platforms

### Datadog

Complete setup for Datadog (all signals in one platform):

```bash
# Set API key
export DD_API_KEY=your-datadog-api-key
export DD_SITE=datadoghq.com  # or datadoghq.eu for EU

# Start collector with Datadog config
docker run -d \
  --name otel-collector \
  -p 4317:4317 \
  -v $(pwd)/saas/datadog.yaml:/etc/otel-collector-config.yaml \
  -e DD_API_KEY=$DD_API_KEY \
  -e DD_SITE=$DD_SITE \
  otel/opentelemetry-collector-contrib:0.95.0 \
  --config=/etc/otel-collector-config.yaml
```

**View in Datadog:**
- Metrics: https://app.datadoghq.com/metric/explorer
- Traces: https://app.datadoghq.com/apm/traces
- Logs: https://app.datadoghq.com/logs

### New Relic

```bash
export NEW_RELIC_LICENSE_KEY=your-license-key

# Use New Relic exporter
# (Config similar to Datadog - contact support for details)
```

### Honeycomb

```bash
export HONEYCOMB_API_KEY=your-api-key
export HONEYCOMB_DATASET=elava

# Configure OTLP exporter to Honeycomb endpoint
```

---

## Query Examples

### PromQL (Prometheus Metrics)

```promql
# Total changes detected
sum(elava_changes_detected_total)

# Changes by type
sum by (change_type) (elava_changes_detected_total)

# Changes rate over 5 minutes
rate(elava_changes_detected_total[5m])

# 95th percentile reconciliation duration
histogram_quantile(0.95, rate(elava_reconcile_duration_ms_bucket[5m]))

# Current untagged resources
elava_resources_untagged

# Decisions made per region
sum by (region) (elava_decisions_made_total)
```

### LogQL (Loki Logs)

```logql
# All Elava logs
{service_name="elava"}

# Critical severity logs
{service_name="elava"} | json | severity="critical"

# Change detection events
{service_name="elava"} | json | event_type="infrastructure.change.detected"

# Appeared resources in production
{service_name="elava", deployment_environment="production"} | json | change_type="appeared"

# Policy violations
{service_name="elava"} | json | event_type="infrastructure.policy.violation"
```

### TraceQL (Jaeger Traces)

Search in Jaeger UI:
- Service: `elava`
- Operation: `reconciliation`, `observe`, `detect`, `decide`
- Tags: `provider=aws`, `region=us-east-1`, `environment=production`

---

## Troubleshooting

### Collector Not Receiving Data

**Check Elava config:**
```bash
# Verify OTEL endpoint
echo $OTEL_EXPORTER_OTLP_ENDPOINT

# Should be: http://localhost:4317 (local) or your collector endpoint
```

**Check collector health:**
```bash
curl http://localhost:13133/health
```

**Check collector logs:**
```bash
docker logs elava-otel-collector
```

### No Metrics in Prometheus

**Verify Prometheus scraping:**
```bash
# Check Prometheus targets
curl http://localhost:9090/api/v1/targets

# Should show otel-collector target as UP
```

**Test collector metrics endpoint:**
```bash
curl http://localhost:8889/metrics | grep elava
```

### No Traces in Jaeger

**Verify Jaeger connection:**
```bash
# Check Jaeger health
curl http://localhost:14269/health

# Check collector logs for trace export errors
docker logs elava-otel-collector | grep -i trace
```

**Verify trace sampling:**
```bash
# Check if traces are being dropped
curl http://localhost:8888/metrics | grep otelcol_processor_dropped_spans
```

### High Memory Usage

**Check collector metrics:**
```bash
curl http://localhost:8888/metrics | grep memory
```

**Adjust memory limiter:**
```yaml
processors:
  memory_limiter:
    limit_mib: 1024  # Reduce if needed
    spike_limit_mib: 256
```

**Enable batch processing:**
```yaml
processors:
  batch:
    timeout: 10s
    send_batch_size: 1024  # Increase for better batching
```

---

## Configuration Reference

### Collector Ports

| Port  | Protocol | Purpose                |
|-------|----------|------------------------|
| 4317  | gRPC     | OTLP receiver          |
| 4318  | HTTP     | OTLP HTTP receiver     |
| 8889  | HTTP     | Prometheus exporter    |
| 8888  | HTTP     | Collector metrics      |
| 13133 | HTTP     | Health check           |
| 55679 | HTTP     | zpages (diagnostics)   |
| 1777  | HTTP     | pprof (profiling)      |

### Environment Variables

**Required:**
```bash
OTEL_EXPORTER_OTLP_ENDPOINT=http://collector:4317
OTEL_SERVICE_NAME=elava
```

**Optional:**
```bash
OTEL_RESOURCE_ATTRIBUTES=deployment.environment=production,service.version=v1.0.0
OTEL_EXPORTER_OTLP_INSECURE=false
OTEL_EXPORTER_OTLP_TIMEOUT=10s
```

---

## Further Reading

- [OpenTelemetry Collector Documentation](https://opentelemetry.io/docs/collector/)
- [OTLP Specification](https://opentelemetry.io/docs/specs/otlp/)
- [Elava OTEL Design Doc](../../docs/design/otel-complete-solution.md)
- [Prometheus Query Language](https://prometheus.io/docs/prometheus/latest/querying/basics/)
- [Loki Query Language](https://grafana.com/docs/loki/latest/logql/)

---

**False Systems** - Building infrastructure tools that make sense 🇫🇮
