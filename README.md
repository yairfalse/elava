# Elava - Infrastructure Reconciliation Engine

Infrastructure reconciliation without state files. Elava continuously observes your cloud resources, detects drift, and explains changes through attribution.

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
Scans 15+ AWS resource types with full tag analysis:
- EC2 instances, Auto Scaling Groups, Load Balancers
- RDS databases (including Aurora clusters)
- S3 buckets, Lambda functions
- EBS volumes, Snapshots, AMIs
- VPCs, Subnets, Security Groups
- EKS clusters, ECR repositories
- KMS keys, Route53 zones

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

# Basic scan
./elava scan

# Scan specific region
./elava scan --region us-west-2

# Scan with attribution
./elava scan --explain-drift

# Filter resource types
./elava scan --filter ec2
./elava scan --filter rds

# Tiered scanning (fast → thorough)
./elava scan --tiers
```

## Example: Drift Attribution

```bash
$ ./elava explain i-mysterious-instance

📊 Attribution Report for i-mysterious-instance
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
Resource Type: EC2 Instance
Current State: Running
Tags: {} (no tags)

🕵️ Attribution:
Actor: john.doe@company.com
Action: RunInstances
Timestamp: 2024-09-21 03:15:23 UTC
Source: AWS Console (73.162.248.123)
Confidence: 0.92 (High)

📝 Context:
- Created outside business hours
- No associated Terraform/CloudFormation
- Similar to previous debug instances

🎯 Recommendation:
Contact john.doe@company.com for cleanup
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

```rego
# policies/cleanup.rego - Resource lifecycle management
package elava.cleanup

# Terminate expired temporary resources
terminate[action] {
    contains(input.resource.name, "temp-")
    resource_age_days > 7

    action := {
        "type": "terminate",
        "grace_period": "24h",
        "notification": input.resource.tags.owner
    }
}

# Calculate resource age
resource_age_days := days {
    now := time.now_ns()
    created := input.resource.created_at
    days := (now - created) / (24 * 60 * 60 * 1000000000)
}
```

### Enforcement Modes

```yaml
# Policy enforcement configuration
policies:
  enforcement_mode: notify  # monitor|notify|enforce|block

  # Per-environment settings
  environments:
    production:
      encryption_required: block
      tagging_required: enforce
    development:
      cleanup_temp_resources: enforce
      oversized_instances: notify
```

### CLI Integration

```bash
# Evaluate policies
./elava policy evaluate --resource i-123456

# Apply policies with dry-run
./elava policy apply --dry-run

# Show policy violations
./elava policy violations --last 24h

# Test policy against resource
./elava policy test policies/tagging.rego --resource-file resource.json
```

## Integration with FinOps Tools

Elava identifies waste patterns but doesn't calculate costs. Integration points:

```go
// Export waste patterns for cost tools
GET /api/waste-patterns

// Response format for FinOps tools
{
  "patterns": [
    {
      "type": "orphaned",
      "resource_ids": ["i-123", "i-456"],
      "reason": "No owner tags for 30+ days",
      "confidence": 0.95
    }
  ]
}
```

## Development

```bash
# Run tests
go test ./...

# Format and lint
go fmt ./...
go vet ./...
golangci-lint run

# Run with telemetry
OTEL_EXPORTER_OTLP_ENDPOINT=http://localhost:4317 ./elava scan
```

## Project Structure

```
elava/
├── analyzer/          # Drift, waste, pattern analysis
├── attribution/       # CloudTrail correlation
├── cmd/elava/        # CLI commands
├── config/           # Configuration management
├── executor/         # Action execution with safety
├── policy/           # OPA policy engine
├── providers/        # Cloud provider implementations
│   └── aws/         # AWS-specific code
├── reconciler/       # Reconciliation engine
├── storage/          # MVCC storage engine
├── telemetry/        # OpenTelemetry integration
└── types/           # Core type definitions
```

## Contributing

Key areas for contribution:
- Additional AWS resource types
- GCP and Azure provider implementations
- Policy templates for common scenarios
- Performance optimizations
- Documentation improvements

## License

MIT - See [LICENSE](LICENSE)
