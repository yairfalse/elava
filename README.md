# Elava

Infrastructure scanner with memory. Scans your account, tracks changes over time.

## What it does

Scans AWS resources and remembers what changed.

- **Discovers resources** - EC2, RDS, S3, Lambda, VPCs, and more
- **Tracks changes** - Stores every observation with timestamps
- **Detects drift** - Shows what changed between scans
- **Finds waste** - Orphaned volumes, idle instances, untagged resources

Read-only. No modifications to your infrastructure.

## Quick start

```bash
# Build
go build ./cmd/elava

# Scan your AWS account
./elava scan

# Scan specific region
./elava scan --region us-west-2

# Show untracked resources only
./elava scan --risk-only
```

## How it works

```
  AWS Account
      │
      │ Read-only API calls
      ▼
  elava scan
      │
      ▼
  BadgerDB (MVCC)
  • Timestamp per observation
  • Revision history
  • Tombstones for disappeared resources
```

1. Scan AWS resources via read-only APIs
2. Store observations with timestamps
3. Compare to previous scans
4. Detect changes, drift, and waste

## What it scans

- **Compute**: EC2, Lambda, EKS, ECS, Auto Scaling Groups
- **Databases**: RDS, Aurora, DynamoDB
- **Storage**: S3, EBS volumes, snapshots
- **Network**: VPCs, subnets, load balancers, NAT gateways, security groups
- **Other**: IAM roles, CloudWatch logs, KMS keys

## AWS permissions

Read-only access. Attach `ReadOnlyAccess` managed policy or use this minimal policy:

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
      "eks:Describe*",
      "elasticloadbalancing:Describe*",
      "autoscaling:Describe*",
      "iam:List*",
      "dynamodb:Describe*"
    ],
    "Resource": "*"
  }]
}
```

## Configuration

Optional `elava.yaml`:

```yaml
provider: aws
region: us-east-1

scanning:
  interval: 15m

policies:
  path: ./policies  # Optional: OPA policy enforcement
```

Defaults work fine. Config is optional.

## Policy enforcement (optional)

Elava includes OPA policy support. Policies in `policies/` directory are loaded automatically.

Example policies included:
- `ownership.rego` - Require owner tags
- `security.rego` - Enforce encryption, public access rules
- `waste.rego` - Detect idle resources
- `compliance.rego` - Tag standards, naming conventions

Skip the `policies.path` config to disable policy enforcement.

## Status

Early development. Storage and core types are stable. AWS scanner in progress.

## License

MIT
