# Elava Tiered Scanning: Smart, Scalable, Configurable

## Overview

Elava's tiered scanning system intelligently manages infrastructure monitoring at scale. Instead of scanning all resources equally, it prioritizes what matters most to you - scanning critical resources frequently while checking archives daily.

## The Problem It Solves

- **Small orgs (100s resources)**: Can scan everything frequently
- **Large orgs (10,000+ resources)**: Hit API rate limits, waste money
- **Solution**: Configurable tiered scanning that adapts to your scale

## How It Works

### 1. Resource Classification

Resources are automatically classified into tiers based on configurable patterns:

```yaml
tiers:
  critical:       # Scanned every 15 minutes
  production:     # Scanned hourly
  standard:       # Scanned every 4 hours  
  archive:        # Scanned daily
```

### 2. Pattern Matching

Each tier matches resources using flexible patterns:

- **By Type**: `type: "rds"` or `types: ["snapshot", "ami"]`
- **By Status**: `status: "running"` or `status: "stopped"`
- **By Tags**: `tags: {environment: "production", owner: "platform"}`
- **By Instance Size**: `instance_type_pattern: "*xlarge"`

### 3. Adaptive Scanning

- **Work Hours Boost**: 2x more frequent scanning 9am-6pm weekdays
- **Smart Scheduling**: Only scans tiers when due
- **Efficient Batching**: Groups resources by tier for API efficiency

## Configuration

### Quick Start

Create `elava.yaml` in your project root:

```yaml
version: "1.0"
provider: "aws"
region: "us-east-1"

scanning:
  enabled: true
  adaptive_hours: true  # Scan more during work hours
  
  tiers:
    critical:
      description: "Production databases and expensive resources"
      scan_interval: 15m
      patterns:
        - type: "rds"
          tags: 
            environment: "production"
        - type: "nat_gateway"  # $45/month minimum!
        - type: "ec2"
          instance_type_pattern: "*xlarge"
```

### Default Tiers

If no configuration is provided, Elava uses sensible defaults:

| Tier | Interval | What It Matches |
|------|----------|-----------------|
| **Critical** | 15 min | Production RDS, NAT Gateways, Large EC2 (*xlarge) |
| **Production** | 1 hour | All production-tagged resources, running instances |
| **Standard** | 4 hours | Development/staging resources |
| **Archive** | 24 hours | Snapshots, AMIs, stopped instances |

### Advanced Configuration

```yaml
scanning:
  # Performance tuning
  performance:
    batch_size: 1000      # Process in chunks
    parallel_workers: 4   # Concurrent API calls
    rate_limit: 100      # Max API calls/second
  
  # Change detection
  change_detection:
    enabled: true
    check_interval: 5m
    alert_on:
      new_untagged_resources: true
      status_changes: true
      disappeared_resources: true
      tag_changes: ["Owner", "Environment", "CostCenter"]
  
  # Custom tiers for your organization
  tiers:
    mission_critical:
      scan_interval: 5m
      patterns:
        - tags: {tier: "tier-0"}
        
    expensive:
      scan_interval: 10m
      patterns:
        - instance_type_pattern: "*8xlarge"
        - type: "nat_gateway"
        
    development:
      scan_interval: 6h
      patterns:
        - tags: {environment: "dev"}
        - name_pattern: "*-test-*"
```

## Usage Examples

### View Scanning Status

```bash
$ elava scan --status

Tiered Scanning Status:
  ✓ critical: Production databases and expensive resources (23 resources)
    Last: 14:45, Next: 15:00
  ⏰ production: Regular production resources (156 resources)
    Last: 14:00, Next: 15:00
  ✓ standard: Development and staging (89 resources)
    Last: 12:30, Next: 16:30
  ✓ archive: Rarely changing resources (234 resources)
    Never scanned
```

### Scan Specific Tier

```bash
# Scan only critical resources
$ elava scan --tier critical

Scanning critical tier (23 resources)...
Found 2 changes:
  New: i-abc123 (ec2, m5.xlarge) - No owner tag!
  Modified: rds-prod (status: available → backing-up)
```

### Dry Run to See What Would Be Scanned

```bash
$ elava scan --dry-run

Would scan these tiers:
  critical: 23 resources (due now)
  production: 156 resources (due in 12 min)
  
Estimated API calls: 179
Estimated time: ~3 seconds
```

## Pattern Matching Examples

### Match Production Databases
```yaml
patterns:
  - type: "rds"
    tags:
      environment: "production"
```

### Match Large Instances
```yaml
patterns:
  - type: "ec2"
    instance_type_pattern: "*xlarge"  # m5.xlarge, c5.2xlarge, etc
```

### Match Untagged Resources
```yaml
patterns:
  - tags:
      owner: ""  # Empty owner tag
```

### Match Multiple Conditions (AND)
```yaml
patterns:
  - type: "ec2"
    status: "running"
    tags:
      environment: "production"
```

### Match Multiple Options (OR)
```yaml
patterns:
  - types: ["snapshot", "ami", "backup"]
  - status: "stopped"
```

## Implementation Details

### Architecture

```
Config Loader → Resource Classifier → Tier Scheduler → Targeted Scanner
      ↓                ↓                    ↓               ↓
   YAML Config    Pattern Matcher    Due Calculation    API Calls
```

### Key Components

1. **TieredScanner** (`scanner/tiered.go`)
   - Classifies resources into tiers
   - Determines which tiers need scanning
   - Tracks last scan times

2. **Config System** (`config/config.go`)
   - Loads YAML configuration
   - Provides defaults if no config exists
   - Validates tier patterns

3. **Pattern Matching**
   - Type matching (exact or array)
   - Tag matching (key-value pairs)
   - Glob patterns for flexible matching
   - Status-based filtering

### Performance Characteristics

| Organization Size | Resources | Scan Strategy | Daily API Calls | vs. Full Scan |
|------------------|-----------|---------------|-----------------|---------------|
| Small | 500 | All every 30m | 24,000 | Same |
| Medium | 2,000 | 4 tiers | 15,000 | -70% |
| Large | 10,000 | 4 tiers | 30,000 | -88% |
| Enterprise | 50,000 | 5+ custom tiers | 50,000 | -95% |

## Benefits

1. **Scalable**: Works efficiently from 100 to 50,000+ resources
2. **Cost-Effective**: Reduces API calls by up to 95% for large infrastructures
3. **Flexible**: Fully customizable to your organization's needs
4. **Smart**: Adapts scanning frequency based on resource importance
5. **Simple**: Works out of the box with sensible defaults

## Tips for Configuration

### For Small Startups
```yaml
tiers:
  all:
    scan_interval: 30m
    patterns: [{}]  # Match everything
```

### For Cost-Conscious Teams
```yaml
tiers:
  expensive:
    scan_interval: 10m
    patterns:
      - type: "nat_gateway"
      - instance_type_pattern: "*large"
  
  cheap:
    scan_interval: 24h
    patterns:
      - instance_type_pattern: "t3.*"
```

### For Compliance-Focused Organizations
```yaml
tiers:
  regulated:
    scan_interval: 5m
    patterns:
      - tags: {compliance: "pci"}
      - tags: {compliance: "hipaa"}
  
  standard:
    scan_interval: 1h
    patterns: [{}]  # Everything else
```

## Troubleshooting

### Resources Not Being Classified Correctly

Check pattern matching:
```bash
$ elava scan --debug-classification i-abc123
Resource: i-abc123 (ec2)
  Type: ec2 ✓
  Status: running
  Tags: {environment: "prod"}
  
Checking tiers:
  critical: 
    Pattern 1: type="rds" ✗
    Pattern 2: type="nat_gateway" ✗
    Pattern 3: instance_type="*xlarge" ✗
  production:
    Pattern 1: tags.environment="production" ✗
  standard:
    Pattern 1: {} ✓
    
Result: Classified as "standard"
```

### Scanning Too Frequently/Infrequently

Adjust intervals:
```bash
# View current intervals
$ elava config --show-scanning

# Test with different interval
$ elava scan --override-interval critical=5m
```

## Future Enhancements

- **Dynamic Adjustment**: Auto-adjust tiers based on change frequency
- **Cost Optimization**: Suggest tier adjustments to reduce API costs
- **Anomaly Detection**: Alert when resources move between tiers unexpectedly
- **Multi-Region**: Different tier strategies per AWS region

---

**Built for scale**: From startups to enterprises, Elava's tiered scanning adapts to your infrastructure.