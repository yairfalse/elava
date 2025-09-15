# Elava Scanning Strategy: Configurable & Scalable

## 🎯 The Challenge
- Small orgs: 100s of resources - can scan everything frequently
- Large orgs: 1000s-10,000s resources - need smart sampling
- Solution: **Configurable tiered scanning**

## 🎛️ Configuration Design

### Default Configuration (ovi.yaml)
```yaml
# Scanning strategy configuration
scanning:
  # Global settings
  enabled: true
  adaptive_hours: true  # Scan more during work hours
  
  # Tiered scanning strategy
  tiers:
    critical:
      description: "Production databases, NAT gateways, expensive resources"
      scan_interval: 15m
      full_history: true
      patterns:
        - type: "rds"
          tags: 
            environment: "production"
        - type: "nat_gateway"
        - type: "ec2"
          size_pattern: "*xlarge"  # Expensive instances
      
    important:
      description: "Regular production resources"
      scan_interval: 1h
      history_days: 30
      patterns:
        - tags:
            environment: "production"
        - status: "running"
      
    standard:
      description: "Development and staging resources"  
      scan_interval: 4h
      history_days: 7
      patterns:
        - tags:
            environment: ["staging", "development"]
    
    low_priority:
      description: "Everything else - snapshots, AMIs, stopped resources"
      scan_interval: 24h
      history_days: 3
      patterns:
        - type: ["snapshot", "ami"]
        - status: "stopped"
  
  # Change detection settings
  change_detection:
    enabled: true
    check_interval: 5m  # How often to check for changes
    interesting_changes:  # What changes to always track
      - status_changes: true
      - tag_changes: ["Owner", "Environment", "Project"]
      - new_resources: true
      - disappeared_resources: true
  
  # Performance tuning
  performance:
    batch_size: 1000  # Process in chunks
    parallel_workers: 4
    rate_limit: 100  # API calls per second
    
  # Storage optimization
  storage:
    mode: "smart"  # "full", "changes_only", "smart"
    compression: true
    retention:
      hot_days: 7      # Full detail
      warm_days: 30    # Daily summaries
      cold_days: 90    # Weekly summaries
      archive_days: 365 # Monthly summaries
```

## 🏗️ Implementation Architecture

### Resource Classification Engine
```go
type TierClassifier struct {
    tiers []TierConfig
}

func (tc *TierClassifier) ClassifyResource(resource Resource) string {
    // Match resource against tier patterns
    for _, tier := range tc.tiers {
        if tc.matchesTier(resource, tier) {
            return tier.Name
        }
    }
    return "standard" // Default tier
}

func (tc *TierClassifier) matchesTier(resource Resource, tier TierConfig) bool {
    for _, pattern := range tier.Patterns {
        if pattern.Matches(resource) {
            return true
        }
    }
    return false
}
```

### Adaptive Scan Scheduler
```go
type AdaptiveScheduler struct {
    config   ScanConfig
    tiers    map[string]TierConfig
    schedule map[string]time.Time  // Next scan time per tier
}

func (s *AdaptiveScheduler) GetNextScanBatch() []Resource {
    now := time.Now()
    
    // Adjust for work hours if enabled
    if s.config.AdaptiveHours {
        now = s.adjustForWorkHours(now)
    }
    
    // Collect resources due for scanning
    var toScan []Resource
    for tierName, nextScan := range s.schedule {
        if now.After(nextScan) {
            resources := s.getResourcesInTier(tierName)
            toScan = append(toScan, resources...)
            s.schedule[tierName] = now.Add(s.tiers[tierName].Interval)
        }
    }
    
    return s.batchResources(toScan, s.config.BatchSize)
}

func (s *AdaptiveScheduler) adjustForWorkHours(t time.Time) time.Time {
    if isWorkingHours(t) {
        // Scan 2x more frequently during work hours
        return t.Add(-30 * time.Minute)
    }
    return t
}
```

### Smart Change Detection
```go
type ChangeDetector struct {
    config         ChangeConfig
    lastKnownState map[string]Resource
}

func (cd *ChangeDetector) DetectInterestingChanges(current []Resource) []Change {
    var changes []Change
    
    for _, resource := range current {
        previous, exists := cd.lastKnownState[resource.ID]
        
        // New resource
        if !exists && cd.config.TrackNewResources {
            changes = append(changes, Change{
                Type:     "appeared",
                Resource: resource,
                Priority: "high",
            })
            continue
        }
        
        // Check for interesting changes
        if exists {
            change := cd.compareResources(previous, resource)
            if cd.isInteresting(change) {
                changes = append(changes, change)
            }
        }
    }
    
    // Find disappeared resources
    if cd.config.TrackDisappeared {
        changes = append(changes, cd.findDisappeared(current)...)
    }
    
    return changes
}

func (cd *ChangeDetector) isInteresting(change Change) bool {
    // Status changes always interesting
    if change.HasStatusChange && cd.config.TrackStatusChanges {
        return true
    }
    
    // Check if important tags changed
    for _, tag := range cd.config.ImportantTags {
        if change.TagsChanged[tag] {
            return true
        }
    }
    
    // Waste indicators
    if change.BecameWasteful {
        return true
    }
    
    return false
}
```

## 📊 Storage Optimization by Tier

### Storage Strategy per Tier
```go
type TieredStorage struct {
    strategies map[string]StorageStrategy
}

func (ts *TieredStorage) StoreResource(resource Resource, tier string) {
    strategy := ts.strategies[tier]
    
    switch strategy.Mode {
    case "full":
        // Store complete resource every scan
        ts.storeFull(resource)
        
    case "changes_only":
        // Only store if changed
        if ts.hasChanged(resource) {
            ts.storeChange(resource)
        }
        
    case "smart":
        // Store based on importance and change frequency
        if tier == "critical" || ts.isSignificantChange(resource) {
            ts.storeFull(resource)
        } else {
            ts.storeMinimal(resource)
        }
    }
}
```

## 🎯 Example Configurations

### Small Organization (< 500 resources)
```yaml
scanning:
  tiers:
    all:
      scan_interval: 30m
      full_history: true
      patterns:
        - "*"  # Scan everything
```

### Large Enterprise (10,000+ resources)
```yaml
scanning:
  tiers:
    critical:
      scan_interval: 10m
      patterns:
        - tags: {tier: "tier-0"}
        - type: ["rds", "elasticache"]
          tags: {env: "prod"}
    
    production:
      scan_interval: 30m
      patterns:
        - tags: {env: "prod"}
    
    staging:
      scan_interval: 2h
      patterns:
        - tags: {env: "staging"}
    
    development:
      scan_interval: 6h
      patterns:
        - tags: {env: "dev"}
    
    archive:
      scan_interval: 24h
      patterns:
        - type: ["snapshot", "ami"]
        - status: ["stopped", "terminated"]
```

### Cost-Conscious Configuration
```yaml
scanning:
  # Only track expensive or wasteful resources
  tiers:
    expensive:
      scan_interval: 15m
      patterns:
        - size_pattern: "*large"  # All large instances
        - type: ["nat_gateway", "rds"]
    
    wasteful:
      scan_interval: 30m
      patterns:
        - status: "stopped"
        - tags: {Owner: ""}  # Untagged
        - name_pattern: "*test*"
```

## 📈 Performance Impact

### Scanning Load by Configuration

| Config Type | Resources | Scan Frequency | Daily API Calls | Storage/Day |
|-------------|-----------|----------------|-----------------|-------------|
| Simple (all) | 500 | Every 30m | 24,000 | 50MB |
| Tiered (smart) | 10,000 | Variable | 50,000 | 20MB |
| Critical Only | 10,000 | Top 100 @ 15m | 9,600 | 5MB |
| Change Focus | 10,000 | Changes @ 5m | 30,000 | 10MB |

## 🚀 Benefits of Configurable Scanning

1. **Scalability**: Works for 100 or 10,000 resources
2. **Cost Control**: Reduce API calls and storage
3. **Focus**: Track what matters most closely
4. **Flexibility**: Adjust as infrastructure grows
5. **Performance**: Maintain fast queries even at scale

## 🔧 CLI Usage

```bash
# Use default configuration
ovi scan

# Use custom config
ovi scan --config production.yaml

# Override specific tier
ovi scan --tier critical --interval 5m

# Dry run to see what would be scanned
ovi scan --dry-run --show-plan

# Show current scanning schedule
ovi status --schedule
```

## 📊 Monitoring Scanner Performance

```yaml
# Metrics exposed for monitoring
metrics:
  ovi_scan_duration_seconds{tier="critical"}
  ovi_resources_scanned_total{tier="critical"}
  ovi_changes_detected_total{type="new"}
  ovi_api_calls_total{provider="aws"}
  ovi_storage_bytes_written{tier="critical"}
```

---

**Built for scale: From startups to enterprises, Elava adapts to your infrastructure.**