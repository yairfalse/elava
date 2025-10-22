# etcd MVCC Implementation Learnings

**Date**: 2025-10-22
**Status**: Research Complete
**Purpose**: Document production-grade MVCC patterns from etcd for applying to Elava

---

## 📚 Study Overview

Studied etcd's MVCC implementation to understand production-grade patterns for Elava's storage layer. etcd has handled billions of transactions in production Kubernetes clusters - their patterns are battle-tested.

**Files analyzed**:
- `/server/storage/mvcc/revision.go` - Revision numbering system
- `/server/storage/mvcc/key_index.go` - Key history tracking with generations
- `/server/storage/mvcc/kvstore.go` - Storage coordination and restoration
- `/server/storage/wal/wal.go` - Write-ahead logging for durability

---

## 🔑 Key Learnings

### 1. Two-Part Revision System

```go
// etcd's approach: Main + Sub
type Revision struct {
    Main int64  // Transaction/batch number (monotonic)
    Sub  int64  // Operation within transaction
}

// Example: Recording 3 EC2 instances in one scan
// All get Main=42 (scan #42), but different Sub values:
Revision{Main: 42, Sub: 0}  // i-abc123
Revision{Main: 42, Sub: 1}  // i-def456
Revision{Main: 42, Sub: 2}  // i-ghi789
```

**Why this matters for Elava**:
- Each scan cycle = one Main revision
- Each resource observation = Sub within that scan
- Enables atomic batch recording of entire scan results
- Can query "show me ALL resources at scan #42"

**Current Elava implementation**: Uses single `int64` revision. Could benefit from Main+Sub for batch atomicity.

### 2. Generations for Key History

```go
// etcd tracks key history as "generations" separated by tombstones
type keyIndex struct {
    key         []byte
    modified    Revision
    generations []generation  // List of generations for this key
}

type generation struct {
    ver     int64       // Version number within generation
    created Revision    // When generation started
    revs    []Revision  // All revisions in this generation
}

// Example: EC2 instance lifecycle
// put(1.0); put(2.0); tombstone(3.0); put(4.0); tombstone(5.0)
// Creates 3 generations:
// Gen 0: {1.0, 2.0, 3.0(tombstone)}  ← First lifecycle
// Gen 1: {4.0, 5.0(tombstone)}        ← Recreated with same ID
// Gen 2: {empty}                       ← Current (doesn't exist)
```

**Why this matters for Elava**:
- EC2 instances can be terminated and recreated with same tags
- Generations track separate lifecycles
- Tombstones mark end of generation, not deletion from index
- Compaction can remove old generations but keeps latest

**Current Elava implementation**: Uses `DisappearedRev` field but no generation concept. Adding generations would enable:
- Tracking resource recreation with same ID
- Better compaction (remove old generations entirely)
- "How many times was this volume attached/detached?"

### 3. Dual-Lock Pattern

```go
// etcd separates locks for different access patterns
type store struct {
    // General operations lock (read lock for txns, write lock for changes)
    mu sync.RWMutex

    // Revision tracking lock (separate to minimize contention)
    revMu sync.RWMutex
    currentRev     int64
    compactMainRev int64
}

// Pattern: Lock revMu FIRST, then mu (strict ordering prevents deadlock)
func (s *store) operation() {
    s.revMu.RLock()
    currentRev := s.currentRev
    s.revMu.RUnlock()

    s.mu.RLock()
    // Do work with index
    s.mu.RUnlock()
}
```

**Why this matters for Elava**:
- Separate hot paths (reading currentRev) from slow paths (index updates)
- Reduces lock contention on high-frequency operations
- Reader queries don't block each other when checking revision

**Current Elava implementation**: Single `sync.RWMutex` for everything. Could benefit from split for high-throughput scanning.

### 4. Chunked Restoration with Concurrency

```go
// etcd restores 10,000 keys at a time, rebuilds index concurrently
func (s *store) restore() error {
    rkvc, revc := restoreIntoIndex(s.lg, s.kvindex)  // Channels for streaming

    for {
        keys, vals := tx.UnsafeRange(schema.Key, min, max, int64(restoreChunkKeys))
        if len(keys) == 0 {
            break
        }
        restoreChunk(s.lg, rkvc, keys, vals, keyToLease)  // Send to indexer
        if len(keys) < restoreChunkKeys {
            break  // Last chunk
        }
        // Update min for next chunk
        newMin := BytesToRev(keys[len(keys)-1][:revBytesLen])
        newMin.Sub++
        min = RevToBytes(newMin, min)
    }
    close(rkvc)
    s.currentRev = <-revc  // Get final revision from indexer goroutine
}
```

**Why this matters for Elava**:
- Large databases (100k+ resources) need efficient startup
- Concurrent indexing overlaps I/O with computation
- Bounded memory (10k chunks prevent OOM)
- Progress tracking per chunk

**Current Elava implementation**: Loads all at once. For large infrastructures (AWS accounts with 50k+ resources), chunked loading would improve startup time.

### 5. WAL Durability Guarantees

```go
// etcd uses segmented WAL files with explicit fsync
const SegmentSizeBytes int64 = 64 * 1000 * 1000  // 64MB segments

func (w *WAL) Save(st HardState, ents []Entry) error {
    w.mu.Lock()
    defer w.mu.Unlock()

    mustSync := MustSync(st, w.state, len(ents))

    for i := range ents {
        w.saveEntry(&ents[i])
    }
    w.saveState(&st)

    if mustSync {
        return w.sync()  // Explicit fsync
    }
    return nil
}

func (w *WAL) sync() error {
    if w.unsafeNoSync {
        return nil
    }

    start := time.Now()
    err := fileutil.Fdatasync(w.tail().File)

    took := time.Since(start)
    if took > warnSyncDuration {  // Warn if fsync takes >1s
        w.lg.Warn("slow fdatasync", zap.Duration("took", took))
    }
    return err
}
```

**Why this matters for Elava**:
- BoltDB handles fsync internally, but we should monitor it
- Slow fsync (>1s) indicates disk problems or overload
- WAL segments (64MB) prevent unbounded file growth
- Separate WAL from data improves crash recovery

**Current Elava implementation**: Relies on BoltDB's fsync. Consider adding:
- Fsync latency metrics (detect slow disks)
- WAL-style append-only log for change events (separate from MVCC storage)
- Segment rotation for bounded log growth

### 6. Compaction with Hash Verification

```go
// etcd tracks compaction state across restarts
func (s *store) updateCompactRev(rev int64) error {
    s.revMu.Lock()
    if rev <= s.compactMainRev {
        return ErrCompacted
    }
    s.compactMainRev = rev

    SetScheduledCompact(s.b.BatchTx(), rev)
    s.b.ForceCommit()  // Persist before starting compaction
    s.revMu.Unlock()

    return nil
}

// Separate scheduled vs finished tracking handles crashes
func (s *store) checkPrevCompactionCompleted() bool {
    scheduledCompact := UnsafeReadScheduledCompact(tx)
    finishedCompact := UnsafeReadFinishedCompact(tx)
    return scheduledCompact == finishedCompact
}
```

**Why this matters for Elava**:
- Compaction can take minutes for large databases
- Crash during compaction = corrupt state
- Track "scheduled" vs "finished" separately
- On restart, check if previous compaction completed

**Current Elava implementation**: No compaction tracking. For production, add:
- Scheduled/finished compaction markers
- Resume interrupted compactions on restart
- Hash verification after compaction

### 7. Tombstone Handling

```go
// etcd never deletes from index, marks as tombstone
func (ki *keyIndex) tombstone(lg *zap.Logger, main, sub int64) error {
    if ki.generations[len(ki.generations)-1].isEmpty() {
        return ErrRevisionNotFound
    }
    ki.put(lg, main, sub)  // Add tombstone revision
    ki.generations = append(ki.generations, generation{})  // New empty generation
    return nil
}

// Tombstones have marker byte in storage
func isTombstone(b []byte) bool {
    return len(b) == markedRevBytesLen && b[markBytePosition] == markTombstone
}
```

**Why this matters for Elava**:
- Distinguish "disappeared" from "never scanned this region"
- Audit trail: "When did this $10k/month RDS instance disappear?"
- Compaction can remove old tombstones after retention period

**Current Elava implementation**: Uses `DisappearedRev` field. Consider explicit tombstone marker in BoltDB keys for faster range queries.

---

## 🎯 Recommendations for Elava

### Immediate (Low Effort, High Value)

1. **Add fsync latency metrics** - Detect slow disks before they cause outages
2. **Split revision lock** - Separate `revMu` for currentRev from main index lock
3. **Chunked restoration** - Load resources in 10k batches on startup

### Medium Term (Design + Implementation)

4. **Main+Sub revision system** - Enable atomic batch recording of scan results
5. **Generations** - Track resource recreation lifecycles
6. **Compaction tracking** - Scheduled/finished markers with crash recovery

### Long Term (Major Features)

7. **Separate WAL** - Append-only change event log independent of MVCC storage
8. **Hash verification** - Detect corruption during compaction
9. **Segment rotation** - Bounded log file growth with automatic cleanup

---

## 📊 Pattern Comparison

| Pattern | etcd Implementation | Elava Current | Recommendation |
|---------|-------------------|---------------|----------------|
| **Revision** | Main+Sub (atomic batches) | Single int64 | Add Sub for scan atomicity |
| **Key History** | Generations with tombstones | Single lifecycle | Add generations for recreated resources |
| **Locking** | Dual locks (data + revision) | Single RWMutex | Split for high throughput |
| **Restoration** | 10k chunks, concurrent | Load all at once | Chunk for large infrastructures |
| **Durability** | WAL with explicit fsync | BoltDB internal | Monitor fsync latency |
| **Compaction** | Scheduled/finished tracking | Not implemented | Add for production safety |

---

## 🔬 Code Examples for Elava

### Example 1: Main+Sub Revision

```go
// Current Elava
type ResourceState struct {
    ResourceID     string
    LastSeenRev    int64  // Single revision
}

// Proposed: etcd-style
type ResourceState struct {
    ResourceID     string
    LastSeenRev    Revision  // Main=scan cycle, Sub=order in batch
}

type Revision struct {
    Main int64  // Scan cycle number
    Sub  int64  // Position in scan batch
}

// Recording a scan with 3 resources
batch := storage.BeginBatch()  // Main revision = 100
batch.Record("i-abc", Revision{Main: 100, Sub: 0})
batch.Record("i-def", Revision{Main: 100, Sub: 1})
batch.Record("i-ghi", Revision{Main: 100, Sub: 2})
batch.Commit()  // Atomic: all at revision 100
```

### Example 2: Chunked Restoration

```go
func (s *MVCCStorage) Restore() error {
    const chunkSize = 10000

    resultCh := make(chan ResourceState, chunkSize)
    errCh := make(chan error, 1)

    // Background goroutine rebuilds index from channel
    go func() {
        for resource := range resultCh {
            s.index.ReplaceOrInsert(&resource)
        }
        errCh <- nil
    }()

    // Load from BoltDB in chunks
    err := s.db.View(func(tx *bolt.Tx) error {
        c := tx.Bucket([]byte("resources")).Cursor()

        count := 0
        for k, v := c.First(); k != nil; k, v = c.Next() {
            var resource ResourceState
            if err := json.Unmarshal(v, &resource); err != nil {
                return err
            }
            resultCh <- resource
            count++

            // Log progress every 10k
            if count%chunkSize == 0 {
                s.lg.Info("restoration progress", zap.Int("loaded", count))
            }
        }
        return nil
    })

    close(resultCh)
    return <-errCh
}
```

### Example 3: Fsync Latency Monitoring

```go
// Add to MVCCStorage
type MVCCStorage struct {
    // ... existing fields ...
    fsyncLatency prometheus.Histogram
}

// Wrap BoltDB commits with metrics
func (s *MVCCStorage) RecordObservationBatch(resources []Resource) error {
    start := time.Now()

    err := s.db.Update(func(tx *bolt.Tx) error {
        // ... write operations ...
        return nil
    })

    // BoltDB calls fsync in Update()
    fsyncDuration := time.Since(start)
    s.fsyncLatency.Observe(fsyncDuration.Seconds())

    if fsyncDuration > time.Second {
        s.lg.Warn("slow fsync detected",
            zap.Duration("took", fsyncDuration),
            zap.String("warning", "disk may be slow or overloaded"),
        )
    }

    return err
}
```

---

## ✅ Validation

These patterns are proven in production:
- **etcd**: Manages Kubernetes cluster state for thousands of companies
- **10+ years**: Battle-tested since 2013
- **High scale**: Handles 10k+ writes/sec in large clusters
- **Crash safety**: Survives power loss, disk failures, network partitions

Applying these patterns to Elava ensures production-grade reliability for infrastructure observability.

---

## 📝 Next Steps

1. **Add metrics**: Implement fsync latency monitoring (1 hour)
2. **Design doc**: Main+Sub revision system for atomic scan batches (2 hours)
3. **Prototype**: Chunked restoration for large infrastructures (1 day)
4. **Testing**: Crash recovery scenarios for compaction (2 days)

---

**References**:
- etcd MVCC: `/Users/yair/projects/etcd/server/storage/mvcc/`
- etcd WAL: `/Users/yair/projects/etcd/server/storage/wal/`
- Elava MVCC: `/Users/yair/projects/elava/storage/`
