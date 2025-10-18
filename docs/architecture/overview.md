# Elava Architecture Overview

## High-Level Design

Elava follows a **storage-first** architecture inspired by etcd's MVCC design. The MVCC storage engine is the core - everything else is just I/O. Multiple Elava instances can run simultaneously, all feeding observations into the shared temporal storage.

```
┌─────────────────┐    ┌─────────────────┐    ┌─────────────────┐
│   Elava Instance  │    │   Elava Instance  │    │   Elava Instance  │
│                 │    │                 │    │                 │
│  ┌───────────┐  │    │  ┌───────────┐  │    │  ┌───────────┐  │
│  │ Scanner + │  │    │  │ Scanner + │  │    │  │ Scanner + │  │
│  │ Analyzer  │  │    │  │ Analyzer  │  │    │  │ Analyzer  │  │
│  └───────────┘  │    │  └───────────┘  │    │  └───────────┘  │
└─────────────────┘    └─────────────────┘    └─────────────────┘
         │                       │                       │
         └───────────────────────┼───────────────────────┘
                                 │
                    ┌─────────────────────┐
                    │   THE BRAIN         │
                    │   (Shared Storage)  │
                    │                     │
                    │  ┌───────────────┐  │
                    │  │ MVCC Storage  │  │
                    │  │   (BadgerDB)  │  │
                    │  └───────────────┘  │
                    │                     │
                    │  ┌───────────────┐  │
                    │  │ WAL Logging   │  │
                    │  │  (audit)      │  │
                    │  └───────────────┘  │
                    └─────────────────────┘
```

## Core Principles

### 1. Storage-First Architecture
- **MVCC storage is the brain**: Not an addon - it's the core intelligence
- **Temporal awareness**: All observations stored with timestamps and revisions
- **Time-travel queries**: Query infrastructure state at any point in history
- **Complete memory**: Never forget what infrastructure looked like

### 2. Observability-Only
- **Read-only scanning**: Never modifies infrastructure
- **Reality tracking**: Observe actual cloud state continuously
- **Revision tracking**: Every observation gets a unique revision number
- **Drift detection**: Identify what changed between observations

### 3. Complete Auditability
- **WAL logging**: Every observation logged to write-ahead log
- **Immutable history**: Once written, audit trail cannot be modified
- **Historical analysis**: "When did this resource appear?" becomes answerable
- **Debug support**: Complete forensics of infrastructure changes

## System Components

### Scanner Engine
The eyes of Elava that continuously observe cloud infrastructure:

```go
type Scanner struct {
    providers []CloudProvider  // AWS, GCP, Azure scanners
    storage   MVCCStorage      // Temporal storage
    wal       WAL              // Audit logging
}
```

**Scanning Flow:**
1. **Poll** cloud providers (read-only APIs)
2. **Record** observations with timestamp + revision
3. **Store** to MVCC storage (immutable history)
4. **Log** to WAL (complete audit trail)

### Analyzer Engine
The intelligence of Elava that queries temporal patterns:

```go
type Analyzer struct {
    storage MVCCStorage  // Query historical data
    wal     WAL          // Read audit trail
}
```

**Analysis Capabilities:**
1. **Drift Detection** - Compare resource state across time
2. **Tag Compliance** - Identify missing or incorrect tags
3. **Orphan Detection** - Find disconnected resources
4. **Change Tracking** - "What changed in the last 24h?"
5. **Time-Travel Queries** - "What did this VPC look like yesterday?"

### MVCC Storage System
Inspired by etcd's multi-version concurrency control:

- **Revision-based**: Every observation has monotonic revision number
- **Time-travel**: Query state at any historical revision
- **Concurrent**: Multiple readers, serialized writers
- **Compaction**: Remove old revisions to control storage size

### Write-Ahead Log (WAL)
Complete audit trail for debugging and analysis:

- **Sequential logging**: All observations logged in order
- **Entry types**: `scan_started`, `resource_observed`, `scan_completed`, `analysis_run`
- **Crash recovery**: Resume from last known state
- **Replay debugging**: Understand complete observation history

### Provider Interface
Pluggable cloud provider abstraction (read-only):

```go
type CloudProvider interface {
    // Scanning operations (read-only)
    ListEC2Instances(ctx context.Context) ([]Resource, error)
    ListRDSInstances(ctx context.Context) ([]Resource, error)
    ListS3Buckets(ctx context.Context) ([]Resource, error)
    ListVPCs(ctx context.Context) ([]Resource, error)
    ListSubnets(ctx context.Context) ([]Resource, error)
    ListRouteTables(ctx context.Context) ([]Resource, error)
    ListNATGateways(ctx context.Context) ([]Resource, error)
    // ... more resource scanners

    // Provider metadata
    Name() string
    Region() string
    AccountID() string
}
```

**Key principle**: Providers are read-only scanners. No Create/Update/Delete operations.

## Data Flow

### 1. Scanning Phase
```
Cloud Provider (AWS/GCP) → Scanner → MVCC Storage
   (read-only APIs)                       ↓
                                    WAL Entry (observed)
```

**What happens:**
- Scanner polls cloud provider APIs (EC2, RDS, S3, etc.)
- Each resource observation recorded with timestamp + revision
- Stored immutably in MVCC storage
- WAL entry created for audit trail

### 2. Analysis Phase
```
MVCC Storage → Analyzer → Analysis Results
                   ↓
           WAL Entry (analysis_run)
```

**What happens:**
- Analyzer queries temporal storage
- Detects drift by comparing revisions
- Identifies missing tags, orphaned resources
- Results displayed to user (no actions taken)

### 3. Query Phase
```
User Query → MVCC Storage → Historical Results
(CLI/API)

Examples:
- "Show me EC2 instances as of yesterday"
- "What changed in the last 24 hours?"
- "When did this resource first appear?"
```

## Concurrency Model

Multiple Elava instances can run simultaneously without conflicts:

```go
// No locks needed - MVCC handles concurrent reads
// Writes are serialized by BadgerDB transaction layer
```

**Why it works:**
- **Read-only scanning**: Multiple instances can scan concurrently
- **MVCC writes**: BadgerDB serializes writes automatically
- **No resource claims**: Not needed - we're not modifying anything
- **WAL append-only**: Multiple writers append to WAL safely

## Error Handling

Since Elava is read-only, error handling focuses on resilience:

### Scan Failures
- **Graceful degradation**: Continue scanning other resources if one fails
- **Retry logic**: Exponential backoff for transient API errors
- **Partial results**: Store successful observations even if scan incomplete
- **Error logging**: All failures recorded in WAL for debugging

### Storage Failures
- **BadgerDB transactions**: Atomic writes ensure consistency
- **WAL durability**: Writes flushed to disk before acknowledgment
- **Compaction safety**: Old revisions safely removed after retention period

## Performance Characteristics

### Storage
- **In-memory index**: Fast lookups with BadgerDB's LSM tree
- **On-disk persistence**: BadgerDB for durability
- **Compaction**: Automatic LSM compaction + configurable revision retention
- **Concurrent reads**: Unlimited concurrent readers (MVCC snapshots)

### Scanning
- **Parallel scanning**: Scan multiple resource types concurrently
- **Regional isolation**: Each region scanned independently
- **Rate limiting**: Respect AWS/GCP API rate limits
- **Incremental**: Only fetch resources since last scan

### Querying
- **Time-travel**: O(log n) lookup by revision number
- **Range queries**: Efficient scans across time ranges
- **Index-backed**: Fast lookups by resource ID, type, tags
- **Streaming results**: Handle large result sets efficiently

### Scaling
- **Horizontal**: Multiple scanner instances feed same storage
- **Resource-level parallelism**: Scan resources concurrently
- **Provider-agnostic**: Pluggable cloud providers (AWS, GCP, Azure)
- **Stateless scanners**: Easy to containerize and scale

---

This architecture provides **infrastructure memory** - complete temporal awareness of your cloud without ever modifying it. Think "git log for infrastructure".