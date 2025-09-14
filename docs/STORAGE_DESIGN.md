# Ovi Storage Design: Change-First Architecture

## 🎯 Design Requirements (From User Research)

Based on real user needs analysis:

1. **Scale**: Hundreds of resources per organization
2. **Scanning**: Adaptive - frequent during work hours, less at night  
3. **Primary Query**: "What's cooking in my infra?!?" (change detection)
4. **Retention**: User-configurable (30 days to 2 years)
5. **Performance**: Slow queries are worst enemy, debugging must be fast

## 🏗️ Change-First Storage Model

### Core Insight
> **Users care about CHANGES more than snapshots. Optimize for "what's different?" not "what exists?"**

### Primary Storage: Recent Changes (Hot Path)
```go
type RecentChanges struct {
    Revision         int64                    `json:"revision"`
    Timestamp        time.Time               `json:"timestamp"`  
    ScanDuration     time.Duration           `json:"scan_duration"`
    
    // Changes since last scan
    NewResources         []Resource           `json:"new"`         // Appeared
    ModifiedResources    []ResourceChange     `json:"modified"`    // Changed
    DisappearedResources []string             `json:"disappeared"` // Vanished
    
    // Context for debugging
    ScanMetadata     ScanMetadata            `json:"scan_metadata"`
    ChangeContext    []RelatedChange         `json:"context,omitempty"`
}

type ResourceChange struct {
    Resource      Resource                  `json:"resource"`       // Current state
    PreviousState Resource                  `json:"previous_state"` // What it was
    ChangedFields []string                  `json:"changed_fields"` // ["status", "owner"]
    ChangeType    string                   `json:"change_type"`    // "tags", "status", "size"
}
```

### Secondary Storage: Current State Snapshot (Context)
```go
type CurrentStateSnapshot struct {
    Revision        int64                    `json:"revision"`
    Timestamp       time.Time               `json:"timestamp"`
    AllResources    map[string]Resource     `json:"resources"`     // resourceID -> Resource
    ResourcesByType map[string][]string     `json:"by_type"`       // Fast type filtering
    ResourcesByOwner map[string][]string    `json:"by_owner"`      // Fast owner filtering
    
    // Pre-computed aggregates for speed
    TotalCount      int                     `json:"total_count"`
    UntrackedCount  int                     `json:"untracked_count"`
    WasteByType     map[string]int          `json:"waste_by_type"`
}
```

### Archive Storage: Historical Summaries (Cold Path)
```go
type HistoricalSummary struct {
    Date            time.Time               `json:"date"`
    Revision        int64                   `json:"revision"`
    
    // High-level metrics
    TotalCount      int                     `json:"total_count"`
    ChangeCount     int                     `json:"change_count"`
    WasteMetrics    WasteMetrics            `json:"waste_metrics"`
    
    // Most significant changes (for debugging)
    TopChanges      []ImportantChange       `json:"top_changes"`
    CriticalEvents  []string                `json:"critical_events"`  // Disappearances, etc.
}
```

## 🚀 Query Patterns (Performance Optimized)

### FAST Queries (Primary Use Cases)
```go
// #1 User Question: "What's cooking?"
func GetLatestChanges() RecentChanges {
    // Returns: What changed in last scan (< 1ms)
}

// Context for changes
func GetResourceContext(resourceID string) Resource {
    // Returns: Current state from snapshot (< 1ms)
}

// Filter current state  
func GetResourcesByOwner(owner string) []Resource {
    // Uses pre-computed index (< 10ms)
}
```

### MEDIUM Queries (Debugging)
```go
// Timeline for troubleshooting
func GetResourceTimeline(resourceID string, duration time.Duration) []ResourceChange {
    // Returns: Changes for specific resource over time (< 100ms)
    // Queries recent changes + archived summaries
}

// Pattern detection
func GetSimilarChanges(change ResourceChange) []ResourceChange {
    // Find related changes for context (< 500ms)  
}
```

### SLOW Queries (Rare, Compliance)
```go
// Point-in-time reconstruction  
func GetStateAtDate(date time.Time) (*CurrentStateSnapshot, error) {
    // Rebuilds state from archived data (< 5s)
    // Used for compliance audits
}
```

## ⚡ Adaptive Scanning Strategy

### Smart Scan Intervals
```go
func GetScanInterval() time.Duration {
    now := time.Now()
    
    if isWorkingHours(now) {
        return 15 * time.Minute  // High activity, watch closely
    }
    
    if isWeekend(now) {
        return 4 * time.Hour     // Very quiet
    }
    
    return 2 * time.Hour         // Off-hours baseline
}

func isWorkingHours(t time.Time) bool {
    // 9 AM - 6 PM local time  
    hour := t.Hour()
    return hour >= 9 && hour <= 18 && t.Weekday() >= time.Monday && t.Weekday() <= time.Friday
}
```

### Change Detection Logic
```go
func DetectChanges(current, previous []Resource) RecentChanges {
    changes := RecentChanges{
        Timestamp: time.Now(),
        Revision:  getNextRevision(),
    }
    
    // Build lookup maps
    currentMap := mapByID(current)
    previousMap := mapByID(previous)
    
    // Find new resources
    for id, resource := range currentMap {
        if _, existed := previousMap[id]; !existed {
            changes.NewResources = append(changes.NewResources, resource)
        }
    }
    
    // Find disappeared resources  
    for id := range previousMap {
        if _, exists := currentMap[id]; !exists {
            changes.DisappearedResources = append(changes.DisappearedResources, id)
        }
    }
    
    // Find modified resources
    for id, current := range currentMap {
        if previous, existed := previousMap[id]; existed {
            if changed := detectResourceChanges(current, previous); changed != nil {
                changes.ModifiedResources = append(changes.ModifiedResources, *changed)
            }
        }
    }
    
    return changes
}
```

## 💾 Storage Implementation Strategy

### Storage Layers
```
┌─────────────────────────────────────────────────────────────┐
│                    Storage Layers                           │
├─────────────────────────────────────────────────────────────┤
│  In-Memory Cache    │ Last 2 scans for instant diff         │
│  (Redis/Local)      │ GetLatestChanges() - sub-millisecond  │
├─────────────────────┼─────────────────────────────────────────┤
│  Recent Changes DB  │ Last 7 days of changes                │  
│  (BoltDB)           │ GetResourceTimeline() - fast          │
├─────────────────────┼─────────────────────────────────────────┤
│  Archive DB         │ Historical summaries                   │
│  (BoltDB/Compressed)│ GetStateAtDate() - slower but rare    │
└─────────────────────┴─────────────────────────────────────────┘
```

### Data Flow
```
AWS Scan → Detect Changes → Update Cache → Store Recent → Archive Old
    ↓           ↓              ↓              ↓             ↓
 Resources   RecentChanges   FastQueries   Debugging   Compliance
```

### Retention Policy (User Configurable)
```yaml
# ovi.yaml
storage:
  recent_changes_days: 7      # Keep full change history
  archive_summary_days: 90    # Keep daily summaries  
  compliance_years: 2         # Keep monthly summaries for audit
  
  # Performance tuning
  cache_last_scans: 2         # In-memory for instant diff
  compress_archives: true     # Save disk space
```

## 🔍 Example User Workflows

### "What's cooking in my infra?"
```bash
$ ovi changes
🔍 Latest Changes (15 minutes ago):

🆕 NEW RESOURCES (3):
  i-abc123def    ec2      running    ❌ No owner tag
  vol-456ghi     ebs      available  ❌ Unattached
  
🔄 MODIFIED (2):  
  i-old789       ec2      running→stopped  ⚠️  Been stopped 3 hours
  rds-prod       rds      owner: "" → "john"  ✅ Tagged!

💀 DISAPPEARED (1):
  test-instance  ec2      Was stopped 2 days  ✅ Cleanup!

💡 Recommendations: 2 new untagged resources need attention
```

### "Debug this resource"
```bash  
$ ovi timeline i-abc123def --hours 24
📈 Resource Timeline: i-abc123def

2024-01-15 09:30  🆕 APPEARED     running, no tags
2024-01-15 11:15  🏷️  TAGGED      owner="" → "john"
2024-01-15 14:45  ⏸️  STOPPED     running → stopped  
2024-01-15 17:20  ⚠️  FLAGGED     wasteful (stopped 3h)

🔍 Similar changes today: 3 other instances also stopped around 14:45
💡 Possible batch operation or outage?
```

### "Compliance audit"  
```bash
$ ovi audit --date 2024-01-01 --resource i-abc123def
📋 Compliance Report: 2024-01-01

Resource: i-abc123def  
Status: running ✅
Owner: john ✅  
Environment: prod ✅
Compliant: YES

📄 Audit trail available for download
```

## 🎯 Why This Design Works for Ovi

1. **Change-First**: Optimized for "what's different?" - your #1 question
2. **Fast Debugging**: Recent changes stored with full context  
3. **Adaptive**: Scans more during active hours, less at night
4. **Scalable**: Hundreds of resources × hourly scans = manageable
5. **Flexible**: User-configurable retention for different compliance needs
6. **Simple**: No complex event sourcing or reconstruction for common queries

## 🚧 Implementation Priority

1. **Phase 1**: Basic change detection (new/modified/disappeared)
2. **Phase 2**: Rich context and timeline queries
3. **Phase 3**: Adaptive scanning and smart intervals
4. **Phase 4**: Advanced pattern detection and insights

---

**Built for Day 2 Operations: Because infrastructure changes, and you need to know about it.**