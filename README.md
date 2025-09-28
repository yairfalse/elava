# Elava - Living Infrastructure Engine

Infrastructure reconciliation without state files. Your cloud IS the state. Elava continuously observes resources, detects drift, and makes intelligent decisions through temporal awareness.

```
    Cloud Infrastructure              Elava Engine                     Intelligence
   ┌──────────────────┐                   │                      ┌─────────────────┐
   │ AWS Resources    │                   ▼                      │ Drift Detection │
   │ • EC2 • RDS      │ ──────► ┌─────────────────┐ ──────►      │ Attribution     │
   │ • S3  • Lambda   │         │ MVCC Storage    │              │ Waste Analysis  │
   │ • EKS • VPC      │ ◄────── │ (Living Memory) │ ◄──────      │ OPA Policies    │
   └──────────────────┘         └─────────────────┘              └─────────────────┘
     Actual State                 Temporal Awareness              Operational Actions
```

## What Elava Does

### 🔍 Resource Discovery
Scans 20+ AWS resource types with full tag analysis:
- **Compute**: EC2, Lambda, EKS, ECS, Auto Scaling Groups
- **Storage**: S3, EBS volumes, Snapshots, AMIs
- **Database**: RDS instances, Aurora clusters, DynamoDB, ElastiCache
- **Network**: VPCs, Subnets, Security Groups, NAT Gateways, EIPs
- **Identity**: IAM roles, KMS keys, ECR repositories
- **DNS**: Route53 zones

### 🕵️ Drift Attribution
Explains WHO changed WHAT and WHY through CloudTrail integration:
- Correlates resource changes with API calls
- Identifies the actor (human/service/automation)
- Provides confidence scoring for attribution
- Falls back to heuristics when CloudTrail unavailable

### 📊 Operational Intelligence
Analyzes patterns without calculating costs:
- Detects orphaned resources (no owner tags)
- Identifies idle and oversized resources
- Tracks resource lifecycle patterns
- Provides waste analysis for FinOps tools to price

### 🎯 Policy Enforcement
Uses Open Policy Agent (OPA) for Day-2 operations:
- Enforce tagging requirements
- Block non-compliant resources
- Auto-remediate violations
- Custom policy definitions

## Core Philosophy

**"Storage is the brain, everything else is I/O"**

- **No State Files**: AWS/GCP is your source of truth
- **Living Infrastructure**: Continuous reconciliation loop
- **Temporal Awareness**: Not just "what is" but "what was, when, and why"
- **Friendly Operations**: Notifies before destroying, explains decisions

## Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                    MVCC Storage (Brain)                     │
├─────────────────────────────────────────────────────────────┤
│ • Resource Observations  • Change Events                    │
│ • Attribution Data       • Policy Decisions                 │
│ • 30-day Rolling Window  • Temporal Queries                 │
└─────────────────────────────────────────────────────────────┘
                              │
        ┌─────────────────────┼─────────────────────┐
        ▼                     ▼                     ▼
┌──────────────┐    ┌──────────────┐    ┌──────────────┐
│  Providers   │    │   Analyzer   │    │ Attribution  │
├──────────────┤    ├──────────────┤    ├──────────────┤
│ • AWS        │    │ • Drift      │    │ • CloudTrail │
│ • GCP*       │    │ • Waste      │    │ • Correlation│
│ • Azure*     │    │ • Patterns   │    │ • Confidence │
└──────────────┘    └──────────────┘    └──────────────┘
                              │
                    ┌─────────▼─────────┐
                    │   Reconciler      │
                    ├───────────────────┤
                    │ • Compare State   │
                    │ • Make Decisions  │
                    │ • Execute Actions │
                    └───────────────────┘

* = Planned
```

## Quick Start

```bash
# Build
go build ./cmd/elava

# Basic scan (discovers all resource types)
./elava scan

# Scan specific region
./elava scan --region us-west-2

# Show only high-risk untracked resources
./elava scan --risk-only

# Tiered scanning for large environments
./elava scan --tiers critical,production
```

## Core Components

### Storage Engine
MVCC (Multi-Version Concurrency Control) storage provides:
- Living memory of infrastructure state
- Temporal queries across time ranges
- Change event tracking
- No state file conflicts

### Attribution Service
CloudTrail integration for drift explanation:
- API call correlation
- Actor identification
- Confidence scoring
- Heuristic fallbacks

### Analyzer Package
Pattern detection without cost calculation:
- **DriftAnalyzer**: Detects configuration changes
- **WasteAnalyzer**: Identifies unused resources
- **PatternDetector**: Finds recurring behaviors
- **QueryEngine**: Temporal data queries

### Policy Engine
OPA integration for enforcement:
- YAML policy definitions
- Monitor/Notify/Enforce modes
- Custom rule creation
- Automated remediation

## Configuration

```yaml
# elava.yaml
providers:
  aws:
    regions:
      - us-east-1
      - us-west-2

storage:
  path: /var/lib/elava
  retention: 30d

analyzer:
  drift:
    enabled: true
    attribution: true
  waste:
    enabled: true
    idle_threshold: 7d

policies:
  enforcement_mode: notify  # monitor|notify|enforce
  path: ./policies/
```

## AWS Permissions

```json
{
  "Version": "2012-10-17",
  "Statement": [{
    "Effect": "Allow",
    "Action": [
      "ec2:Describe*",
      "rds:Describe*",
      "s3:List*",
      "s3:GetBucketTagging",
      "lambda:List*",
      "eks:List*",
      "eks:Describe*",
      "elasticloadbalancing:Describe*",
      "autoscaling:Describe*",
      "kms:List*",
      "kms:Describe*",
      "cloudtrail:LookupEvents"
    ],
    "Resource": "*"
  }]
}
```

## OPA Policy Integration

Elava uses Open Policy Agent for policy-driven Day-2 operations:

### Policy Examples

```rego
# policies/tagging.rego - Enforce resource tagging
package elava.tagging

# Automatically tag orphaned resources
tag_orphan[action] {
    input.resource.tags.owner == ""
    input.attribution.actor != ""

    action := {
        "type": "tag",
        "tags": {
            "owner": input.attribution.actor,
            "elava_managed": "true"
        },
        "reason": "Auto-tagged based on CloudTrail attribution"
    }
}

# Block unencrypted production databases
deny[msg] {
    input.resource.type == "rds"
    input.resource.tags.environment == "production"
    input.resource.metadata.encrypted == false

    msg := "Unencrypted RDS in production is forbidden"
}
```


## Development

### Project Structure

```
elava/
├── analyzer/          # Drift, waste, pattern analysis
├── attribution/       # CloudTrail correlation
├── cmd/elava/        # CLI commands
├── config/           # Configuration management
├── executor/         # Action execution with safety
├── orchestrator/     # Integration flow coordination
├── policy/           # OPA policy engine
├── providers/        # Cloud provider implementations
│   └── aws/         # AWS provider (modular design)
│       ├── compute.go    # Lambda, EKS, ECS, ASG
│       ├── storage.go    # S3 buckets
│       ├── network.go    # VPC, SG, EIPs, NAT
│       ├── volumes.go    # EBS, snapshots, AMIs
│       ├── identity.go   # IAM, KMS, Route53
│       └── databases.go  # RDS, Aurora, DynamoDB
├── reconciler/       # Reconciliation engine
├── scanner/          # Tiered resource scanning
├── storage/          # MVCC storage engine
├── telemetry/        # OpenTelemetry integration
├── types/            # Core type definitions
└── wal/             # Write-ahead logging
```

### Design Principles

- **Small Functions**: Max 50 lines per function
- **Clear Interfaces**: Provider-agnostic core logic
- **Pluggable Everything**: Easy to add new providers
- **Storage First**: Every feature considers temporal data
- **Test Driven**: Tests before implementation

## Contributing

Key areas for contribution:
- Additional AWS resource types
- GCP and Azure provider implementations
- Policy templates for common scenarios
- Performance optimizations
- Documentation improvements

## License

MIT - See [LICENSE](LICENSE)
