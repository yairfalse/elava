# Elava MVCC Storage Architecture

## 🏗️ Architecture Overview

Elava uses a **lean MVCC (Multi-Version Concurrency Control) storage engine** that perfectly aligns with Day 2 operations requirements: tracking infrastructure changes over time without traditional state files.

```
┌─────────────────────────────────────────────────────────────┐
│                     AWS/GCP Cloud APIs                       │
└─────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────┐
│                      Elava Scanner Loop                        │
│  • Polls cloud APIs every N minutes                          │
│  • Discovers all resources in region                         │
│  • Detects changes, new resources, disappearances            │
└─────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────┐
│                    MVCC Storage Engine                       │
├─────────────────────────────────────────────────────────────┤
│  ┌───────────────────┐        ┌──────────────────────────┐ │
│  │  In-Memory BTree   │◄──────►│     BoltDB Storage       │ │
│  │                    │        │                          │ │
│  │  • Current State   │        │  • Revisioned Log        │ │
│  │  • Fast Lookups    │        │  • Atomic Batches        │ │
│  │  • O(log n) Filter │        │  • Tombstone Markers     │ │
│  └───────────────────┘        └──────────────────────────┘ │
└─────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────┐
│                    Temporal Queries                          │
│  • "Show me this resource's history"                         │
│  • "What disappeared between rev 100-200?"                   │
│  • "When did this untagged resource first appear?"           │
└─────────────────────────────────────────────────────────────┘
```

## 🎯 Why MVCC for Day 2 Operations?

### Traditional IaC Problem
```yaml
# Terraform state file - what SHOULD exist
resource "aws_instance" "web" {
  instance_type = "t3.micro"
  tags = {
    Owner = "platform-team"
  }
}
```

### Elava's MVCC Solution
```go
// What ACTUALLY exists, with full history
Revision 1: EC2 instance i-abc123 appeared (no tags)
Revision 2: EC2 instance i-abc123 still exists (no tags) 
Revision 3: EC2 instance i-def456 appeared (tagged: test)
Revision 4: EC2 instance i-abc123 disappeared
```

## 🔧 Implementation Excellence

### 1. **Hybrid Storage Architecture**
```go
type MVCCStorage struct {
    // Lightning-fast in-memory index
    index *btree.BTreeG[*ResourceState]  // O(log n) lookups
    
    // Durable on-disk storage
    db *bbolt.DB                         // Atomic, crash-safe
    
    // Global revision counter
    currentRev int64                     // Monotonic time
}
```

### 2. **Resource State Tracking**
```go
type ResourceState struct {
    ResourceID     string   // i-abc123def
    Owner          string   // platform-team
    Type           string   // ec2
    FirstSeenRev   int64    // When first discovered
    LastSeenRev    int64    // Last time confirmed alive
    DisappearedRev int64    // When it vanished (tombstone)
    Exists         bool     // Current existence
}
```

### 3. **Key Design Decisions**

| Decision | Implementation | Benefit |
|----------|---------------|---------|
| **Fixed-width keys** | `fmt.Sprintf("%016d-%s", rev, id)` | Lexicographic ordering for range scans |
| **Tombstone markers** | Never delete, mark disappeared | Temporal queries, audit trail |
| **Batch atomicity** | Single BoltDB transaction | Consistent snapshots |
| **In-memory index** | BTree with resource state | Sub-millisecond lookups |
| **Append-only log** | Revisions never overwritten | Time-travel debugging |

## 📊 Use Cases Enabled

### 1. **Drift Detection**
```go
// Find resources that appeared without IaC
rev100 := storage.GetStateAtRevision(100)
rev200 := storage.GetStateAtRevision(200)
drift := findNewResources(rev100, rev200)
```

### 2. **Zombie Hunting**
```go
// Find resources that disappeared but might still cost money
disappeared := storage.GetDisappearedBetween(lastWeek, today)
for _, resource := range disappeared {
    if resource.Type == "ebs_volume" {
        alert("Detached volume might still be billing!")
    }
}
```

### 3. **Ownership Timeline**
```go
// Track when resources lost their owner tags
history := storage.GetResourceHistory("i-abc123")
for _, observation := range history {
    if observation.Tags["Owner"] == "" {
        fmt.Printf("Lost owner at revision %d\n", observation.Rev)
    }
}
```

## 🚀 Performance Characteristics

| Operation | Complexity | Typical Latency |
|-----------|------------|-----------------|
| Current state lookup | O(log n) | < 1ms |
| Filter by owner/type | O(n) scan | 10-50ms |
| Record observation | O(log n) + disk | 5-10ms |
| Batch record (100 items) | O(1) transaction | 20-30ms |
| Point-in-time query | O(log n) + cursor | 10-20ms |
| Compaction | O(n) full scan | 1-5 seconds |

## 🔄 Compaction Strategy

```go
// Keep last 30 days of detailed history
// Older than 30 days: keep only daily snapshots
func (s *MVCCStorage) CompactionPolicy() {
    cutoff := time.Now().Add(-30 * 24 * time.Hour)
    s.CompactBefore(cutoff, KeepDailySnapshots)
}
```

## 🎨 Future Enhancements

### Near-term
- [ ] Async background compaction
- [ ] Prometheus metrics export
- [ ] Resource relationship tracking
- [ ] Change event streaming

### Long-term  
- [ ] Distributed consensus (Raft)
- [ ] Multi-region federation
- [ ] SQL query interface
- [ ] Rust port for performance

## 📚 Comparison with Other Systems

| System | Storage Model | Use Case | Elava Advantage |
|--------|--------------|----------|---------------|
| Terraform State | Single version JSON | Desired state | Elava tracks actual state with history |
| CloudTrail | Event log | Audit trail | Elava provides resource-centric view |
| AWS Config | Snapshot + changes | Compliance | Elava is simpler, focused on waste |
| Prometheus | Time-series | Metrics | Elava tracks resource lifecycle |

## 🏆 Why This Architecture Rocks

1. **No State File Conflicts**: Unlike Terraform, no locking/merging issues
2. **Time Travel**: Query any point in time for debugging
3. **Audit Trail**: Complete history of every resource
4. **Fast Queries**: In-memory index for current state
5. **Crash Safe**: BoltDB ensures durability
6. **Bounded Growth**: Compaction keeps disk usage reasonable

## 🔑 Key Takeaway

> **"Your cloud IS the state. Elava just remembers what it saw."**

Traditional IaC tools manage desired state. Elava observes actual state over time, making it the perfect companion for Day 2 operations where you need to understand what's REALLY happening in your infrastructure.

---

*Built with production-quality MVCC storage because Day 2 operations deserve real engineering.*