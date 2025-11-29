# Elava

**Stateless cloud resource scanner. Scan, emit, done.**

[![License](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)

---

## What is this?

Elava scans cloud resources and emits metrics/logs. No state. No database. Just scanning.

```
Cloud APIs (AWS/GCP/Azure)
         |
         v
    ELAVA DAEMON
    (scan -> emit -> repeat)
         |
         v
   OTEL / Prometheus / NATS
         |
         v
   Your observability stack
```

**Drift detection happens at the query layer** (Prometheus/Grafana), not in Elava.

## Key Design

- **Stateless** - No database, no files, no persistence
- **Plugin-based** - Add cloud providers via plugins
- **Mega small** - ~2000 lines of Go
- **OTEL-native** - Emits metrics and logs via OpenTelemetry
- **Daemon mode** - Runs continuously, scans on interval

## Quick Start

```bash
# Build
go build ./cmd/elava

# Run daemon (scans every 5 minutes)
./elava --provider aws --region us-east-1

# One-shot scan
./elava scan --provider aws --region us-east-1

# With OTEL endpoint
./elava --otel-endpoint http://localhost:4317

# Standalone mode (bundled VictoriaMetrics)
./elava --standalone --port 9090
```

## What it scans

**AWS:**
- EC2, Lambda, EKS, ECS, Auto Scaling Groups
- RDS, Aurora, DynamoDB
- S3, EBS volumes
- VPCs, subnets, security groups, load balancers
- IAM roles (optional)

**GCP:** (coming soon)
- Compute instances, GKE, Cloud SQL, GCS

**Azure:** (coming soon)
- VMs, AKS, Azure SQL, Storage accounts

## Output

**Prometheus metrics:**
```prometheus
elava_resource_info{id="i-abc123", type="ec2", region="us-east-1", status="running"} 1
elava_resource_cpu_cores{id="i-abc123", type="ec2"} 2
elava_scan_duration_seconds{provider="aws", region="us-east-1"} 12.5
elava_scan_resources_total{provider="aws", region="us-east-1"} 847
```

**Drift detection via PromQL:**
```promql
# Resources that changed in last hour
changes(elava_resource_info[1h]) > 0

# Resources that disappeared
absent_over_time(elava_resource_info{id="i-abc123"}[10m])
```

## Modes

| Mode | Storage | Use Case |
|------|---------|----------|
| **External** | Your Prometheus/Loki | Production |
| **Standalone** | Bundled VictoriaMetrics | Quick start |
| **NATS** | Ahti integration | Enterprise |

## Configuration

```yaml
# elava.yaml
scan:
  interval: 5m
  timeout: 2m

providers:
  aws:
    enabled: true
    regions:
      - us-east-1
      - eu-west-1

output:
  otel:
    endpoint: http://localhost:4317
```

## AWS Permissions

Read-only access only:

```json
{
  "Version": "2012-10-17",
  "Statement": [{
    "Effect": "Allow",
    "Action": [
      "ec2:Describe*",
      "rds:Describe*",
      "s3:List*",
      "lambda:List*"
    ],
    "Resource": "*"
  }]
}
```

## Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                        ELAVA                                │
│                                                             │
│   ┌─────────────┐    ┌─────────────┐    ┌──────────────┐   │
│   │   Scanner   │───>│  Normalizer │───>│   Emitter    │   │
│   │  (Plugins)  │    │  (Unified   │    │  (OTEL/NATS) │   │
│   │             │    │   Format)   │    │              │   │
│   └─────────────┘    └─────────────┘    └──────────────┘   │
│                                                             │
│   State: NONE                                               │
│   Storage: NONE                                             │
│   Drift: QUERY LAYER                                        │
└─────────────────────────────────────────────────────────────┘
```

## Related Projects

- **TAPIO** - Kubernetes + eBPF observability
- **AHTI** - Universal graph-based observability backend
- **RAUTA** - Gateway API controller with WASM plugins
- **KULTA** - Progressive delivery controller

## Status

**Rewriting** - Simplifying to stateless architecture.

## Name

**Elava** = Finnish for "living" - your infrastructure, alive and visible.

## License

MIT

---

**Scan. Emit. Done.** 🇫🇮
