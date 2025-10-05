# Elava

Infrastructure scanner with temporal memory. Scans AWS resources, stores observations over time, detects what changed.

## What it does

**Scans AWS resources** - Discovers 30+ resource types (EC2, RDS, S3, Lambda, etc.)

**Tracks changes over time** - MVCC storage remembers every observation with timestamps

**Detects waste** - Finds orphaned resources, idle instances, unattached volumes

**Analyzes drift** - Shows what changed between scans

That's it. Read-only. No modifications to your infrastructure.

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
┌─────────────────────────────────────────────────────────────┐
│                      MVCC Storage (BadgerDB)                │
│  • Every observation has timestamp + revision number        │
│  • Query: "Show me what changed in the last 24 hours"       │
│  • Tombstones track disappeared resources                   │
└─────────────────────────────────────────────────────────────┘
                              ▲
                              │
                    ┌─────────┴─────────┐
                    │   elava scan      │
                    └─────────┬─────────┘
                              │
                              ▼
                    ┌───────────────────┐
                    │   AWS Provider    │
                    │  (30 resource     │
                    │   types)          │
                    └───────────────────┘
                              │
                              ▼
                    ┌───────────────────┐
                    │  Your AWS Account │
                    │  (Read-only API   │
                    │   calls)          │
                    └───────────────────┘
```

1. Calls AWS APIs (read-only)
2. Stores observations in BadgerDB with timestamps
3. Compares to previous observations to detect changes
4. Identifies resources without proper tags

## What it scans

**Compute**: EC2, Lambda, EKS, ECS, Auto Scaling Groups

**Databases**: RDS, Aurora, DynamoDB, Redshift, MemoryDB

**Storage**: S3, EBS volumes/snapshots, AMIs

**Network**: Load balancers, Elastic IPs, NAT Gateways, Security Groups, VPC endpoints

**Other**: CloudWatch logs, IAM roles, ECR, Route53, KMS, SQS

## AWS permissions needed

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
      "s3:GetBucketTagging",
      "lambda:List*",
      "eks:List*",
      "eks:Describe*",
      "ecs:List*",
      "ecs:Describe*",
      "elasticloadbalancing:Describe*",
      "autoscaling:Describe*",
      "iam:List*",
      "kms:List*",
      "kms:Describe*",
      "logs:Describe*",
      "route53:List*",
      "ecr:Describe*",
      "dynamodb:List*",
      "dynamodb:Describe*",
      "redshift:Describe*",
      "memorydb:Describe*",
      "sqs:List*",
      "sqs:GetQueueAttributes",
      "sqs:ListQueueTags"
    ],
    "Resource": "*"
  }]
}
```

## Configuration

```yaml
# elava.yaml
version: "1.0"
provider: aws
region: us-east-1

# Optional: Policy enforcement with OPA
policies:
  path: ./policies/examples  # Omit to disable policy enforcement

scanning:
  enabled: true
  tiers:
    critical:
      scan_interval: 5m
      patterns:
        - type: rds
          tags:
            environment: production

    standard:
      scan_interval: 1h
```

**Policy enforcement is optional** - Elava works with or without OPA policies. Without policies, it only scans and stores observations.

## What's inside

```
elava/
├── providers/aws/    # AWS resource discovery
├── storage/          # MVCC storage (BadgerDB)
├── scanner/          # Tiered scanning + untracked detection
├── analyzer/         # Waste detection, drift analysis
├── cmd/elava/        # CLI commands
└── types/            # Core types
```

## Why "Elava"?

Finnish word for "living" or "door". Infrastructure should be living, not frozen in state files.

## License

MIT
