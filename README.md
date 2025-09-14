# Ovi - Day 2 Operations Scanner for AWS

Find untracked, untagged, and forgotten resources in your AWS account. Ovi scans your infrastructure and identifies what's not properly managed.

```
     Your AWS Account                    Ovi Scans                    Finds Problems
    ┌─────────────────┐                     │                       ┌──────────────┐
    │                 │                     ▼                       │ 🔴 HIGH RISK │
    │  EC2   RDS  S3  │    ─────────►  [Ovi Scanner]  ─────────►   │ 🟡 MEDIUM    │
    │  EBS   AMI  EIP │                                             │ 🟢 UNTRACKED │
    │                 │                                             └──────────────┘
    └─────────────────┘                                           
         500+ Resources            Reads tags & metadata            108 Issues Found
```

## What Ovi Does Best

### 🔍 **Discovers Everything**
Scans 10+ AWS resource types to build a complete picture of your infrastructure:
- EC2 instances, RDS databases, Load Balancers
- EBS volumes, Snapshots, AMIs
- S3 buckets, Lambda functions
- Elastic IPs, NAT Gateways

### 🏷️ **Finds Untagged Resources**
Identifies resources missing critical tags:
- No owner or team assignment
- Missing environment tags (prod/staging/dev)
- Resources without cost center tags
- Infrastructure not tracked in Terraform/CloudFormation

### 🧟 **Detects Zombie Resources**
Finds resources that are dead but still costing money:
- Stopped EC2 instances (why not terminated?)
- Unattached EBS volumes
- Unused Elastic IPs
- Old snapshots and AMIs
- Empty S3 buckets

### 🎯 **Pattern Recognition**
Intelligently identifies suspicious resources:
- Names containing "test", "temp", "old", "delete-me"
- Resources created at unusual times
- Infrastructure in unexpected regions
- Resources older than your retention policy

## Quick Start

```bash
# Build
go build ./cmd/ovi

# Scan everything in your default region
./ovi scan

# Scan specific region
./ovi scan --region us-west-2

# Focus on specific resource types
./ovi scan --filter snapshot   # Just snapshots
./ovi scan --filter ec2        # Just EC2 instances
./ovi scan --filter ebs        # Just EBS volumes

# Show only high-risk findings
./ovi scan --risk-only
```

## Real Example Output

```
🔍 Scanning AWS region us-east-1 for untracked resources...

📊 Scan Summary:
   Total resources: 342
   Tracked: 234 (68.4%)
   Untracked: 108 (31.6%)

🚨 Untracked Resources:
RESOURCE              TYPE      STATUS       RISK        ISSUES
snap-0abc123def       snapshot  completed    🔴 high     427 days old, named "temp-backup"
vol-0def456ghi        ebs       available    🔴 high     Unattached, 500GB, no tags
i-0ghi789jkl          ec2       stopped      🔴 high     Stopped 67 days ago, no owner
ami-backup-old-v2     ami       available    🟡 medium   Created 2022, 5 snapshots attached
test-lambda-func      lambda    active       🟡 medium   Last invoked 89 days ago
nat-12345678          nat_gw    available    🔴 high     Expensive resource, no tags

💡 Recommended Actions:
   • Clean up 23 stopped/dead resources
   • Add owner tags to 67 resources
   • Verify IaC management for 18 resources

🔒 Safety: Ovi operates read-only. We detect, you decide.
```

## How It Works

```
AWS Account → Ovi Scanner → Detection Rules → Risk Assessment → Report
     ↑                             ↓
     └──── Read-Only API ←─────────┘
```

1. **Connects** to AWS using your existing credentials
2. **Scans** resources using read-only API calls
3. **Analyzes** using smart detection rules
4. **Reports** findings with actionable recommendations

## Supported AWS Resources

- ✅ **Compute**: EC2 Instances, Lambda Functions
- ✅ **Storage**: S3 Buckets, EBS Volumes, Snapshots, AMIs
- ✅ **Database**: RDS Instances
- ✅ **Network**: Elastic IPs, NAT Gateways, Load Balancers
- 🔜 **Coming Soon**: CloudFormation Stacks, Security Groups, CloudWatch Logs

## Installation

```bash
# Clone and build
git clone https://github.com/yairfalse/ovi.git
cd ovi
go build ./cmd/ovi

# Configure AWS credentials (standard AWS CLI/SDK methods)
export AWS_REGION=us-east-1
export AWS_PROFILE=production

# Run your first scan
./ovi scan
```

## Required AWS Permissions

Ovi needs read-only access to scan your resources:

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
      "elasticloadbalancing:Describe*"
    ],
    "Resource": "*"
  }]
}
```

## Why Ovi?

- **🚀 Fast**: Parallel scanning gets results in seconds
- **🔒 Safe**: Read-only operations, never modifies anything
- **🎯 Accurate**: Smart pattern detection reduces false positives
- **💰 Saves Money**: Find waste without complex cost calculations
- **🛠️ Practical**: Focuses on real Day 2 operations problems

## Contributing

We'd love your help making Ovi better! Key areas:
- Adding more AWS resource types
- Improving detection patterns
- Supporting other clouds (GCP, Azure)
- Better output formats (JSON, CSV, HTML reports)

## License

MIT - See [LICENSE](LICENSE)

---

**Built for DevOps teams who want to know what's really in their AWS account.**