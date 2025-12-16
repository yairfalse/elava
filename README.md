# Elava

**Stateless cloud resource scanner. Scan, emit, done.**

[![License](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)

---

## What is this?

Elava scans cloud resources and emits metrics. No state. No database. Just scanning.

```
Cloud APIs (AWS/GCP/Azure)
         |
         v
    ELAVA DAEMON
    (scan -> emit -> repeat)
         |
         v
   Prometheus / VictoriaMetrics / Grafana
         |
         v
   Your observability stack
```

**Drift detection happens at the query layer** (Prometheus/Grafana), not in Elava.

## Installation

### Binary

Download from [releases](https://github.com/yairfalse/elava/releases):

```bash
# Linux amd64
curl -LO https://github.com/yairfalse/elava/releases/latest/download/elava_linux_amd64.tar.gz
tar xzf elava_linux_amd64.tar.gz
./elava --version
```

### Docker

```bash
docker pull ghcr.io/yairfalse/elava:latest
docker run -v ~/.aws:/root/.aws ghcr.io/yairfalse/elava:latest --config /config/elava.toml
```

### Build from source

```bash
go build ./cmd/elava
./elava --version
```

## Quick Start

```bash
# Run with defaults (scans us-east-1 every 5 minutes)
./elava

# Run with config file
./elava --config elava.toml

# One-shot scan (scan once and exit)
./elava --config elava.toml  # with one_shot = true in config

# Debug mode
./elava --debug

# Custom metrics port
./elava --metrics :8080
```

## Configuration

Elava uses TOML configuration:

```toml
# elava.toml

[aws]
regions = ["us-east-1", "eu-west-1"]

[scanner]
interval = "5m"
one_shot = false

[otel]
service_name = "elava"
endpoint = "localhost:4317"   # Optional: OTLP endpoint for traces

[log]
level = "info"
```

## AWS Resources Scanned

29 resource types:

| Category | Resources |
|----------|-----------|
| Compute | EC2, Lambda, ECS, EKS, ASG |
| Database | RDS, DynamoDB, ElastiCache, Redshift |
| Storage | S3, EBS |
| Network | VPC, Subnet, Security Groups, ELB, NAT Gateway, EIP, Route53, CloudFront |
| Integration | SQS, SNS, Kinesis, API Gateway, Step Functions |
| Security | IAM Roles, Secrets Manager, ACM |
| Analytics | Glue, CloudWatch Logs |

## Metrics

Elava exposes Prometheus metrics at `:9090/metrics`:

```prometheus
# Resource counts
elava_scan_resources_total{provider="aws-us-east-1", resource_type="all"} 847

# Scan duration
elava_scan_duration_seconds{provider="aws-us-east-1", resource_type="all"} 12.5

# Errors
elava_scan_errors_total{provider="aws-us-east-1", resource_type="all"} 0
```

### Scrape with Prometheus/VictoriaMetrics

```yaml
# prometheus.yml
scrape_configs:
  - job_name: 'elava'
    static_configs:
      - targets: ['localhost:9090']
```

### Drift detection via PromQL

```promql
# Resources changed in last hour
changes(elava_scan_resources_total[1h]) > 0

# Scan taking too long
elava_scan_duration_seconds > 60
```

## AWS Permissions

Read-only access:

```json
{
  "Version": "2012-10-17",
  "Statement": [{
    "Effect": "Allow",
    "Action": [
      "ec2:Describe*",
      "rds:Describe*",
      "s3:List*",
      "lambda:List*",
      "eks:List*",
      "eks:Describe*",
      "ecs:List*",
      "ecs:Describe*",
      "autoscaling:Describe*",
      "dynamodb:List*",
      "dynamodb:Describe*",
      "sqs:List*",
      "elasticloadbalancing:Describe*",
      "route53:List*",
      "sns:List*",
      "logs:Describe*",
      "cloudfront:List*",
      "elasticache:Describe*",
      "secretsmanager:List*",
      "acm:List*",
      "apigateway:Get*",
      "kinesis:List*",
      "redshift:Describe*",
      "states:List*",
      "glue:Get*",
      "iam:List*"
    ],
    "Resource": "*"
  }]
}
```

## CLI Flags

| Flag | Default | Description |
|------|---------|-------------|
| `--config` | none | Path to TOML config file |
| `--metrics` | `:9090` | Metrics server address |
| `--debug` | false | Enable debug logging |
| `--version` | - | Show version and exit |

## Architecture

```
+-----------------------------------------------------------+
|                        ELAVA                               |
|                                                           |
|   +-------------+    +-----------+    +---------------+   |
|   |   Scanner   |--->| Normalize |--->|    Emitter    |   |
|   |  (Plugins)  |    | (Unified) |    | (Prometheus)  |   |
|   +-------------+    +-----------+    +---------------+   |
|                                                           |
|   State: NONE                                             |
|   Storage: NONE                                           |
|   Drift: QUERY LAYER                                      |
+-----------------------------------------------------------+
```

## Design

- **Stateless** - No database, no files, no persistence
- **Plugin-based** - Add cloud providers via plugins
- **OTEL-native** - Traces via OpenTelemetry, metrics via Prometheus
- **Daemon mode** - Runs continuously, scans on interval

## License

MIT

---

**Scan. Emit. Done.**
