# Ovi Architecture Overview

## High-Level Design

Ovi follows a **stateless-with-memory** architecture inspired by etcd's MVCC design. Multiple Ovi instances can run simultaneously without traditional state file conflicts.

```
┌─────────────────┐    ┌─────────────────┐    ┌─────────────────┐
│   Ovi Instance  │    │   Ovi Instance  │    │   Ovi Instance  │
│                 │    │                 │    │                 │
│  ┌───────────┐  │    │  ┌───────────┐  │    │  ┌───────────┐  │
│  │Reconciler │  │    │  │Reconciler │  │    │  │Reconciler │  │
│  │  Engine   │  │    │  │  Engine   │  │    │  │  Engine   │  │
│  └───────────┘  │    │  └───────────┘  │    │  └───────────┘  │
└─────────────────┘    └─────────────────┘    └─────────────────┘
         │                       │                       │
         └───────────────────────┼───────────────────────┘
                                 │
                    ┌─────────────────────┐
                    │   Shared Storage    │
                    │                     │
                    │  ┌───────────────┐  │
                    │  │ MVCC Storage  │  │
                    │  │   (bbolt)     │  │
                    │  └───────────────┘  │
                    │                     │
                    │  ┌───────────────┐  │
                    │  │ WAL Logging   │  │
                    │  │  (audit)      │  │
                    │  └───────────────┘  │
                    └─────────────────────┘
```

## Core Principles

### 1. Stateless with Memory
- **No state files**: Ovi doesn't maintain traditional state files
- **MVCC storage**: All observations stored with revision numbers
- **Time-travel queries**: Can view infrastructure state at any point in history
- **Coordination**: Multiple instances coordinate through claims and WAL

### 2. Observation-Driven
- **Reality first**: Always starts by observing actual cloud state
- **Revision tracking**: Every observation gets a unique revision number  
- **Batch operations**: Atomic recording of multiple resource observations
- **Drift detection**: Compares current reality with desired configuration

### 3. Complete Auditability
- **WAL logging**: Every operation logged to write-ahead log
- **Immutable history**: Once written, audit trail cannot be modified
- **Replay capability**: Can replay decisions from any point in time
- **Debug support**: "Why did Ovi do X?" becomes answerable

## System Components

### Reconciler Engine
The heart of Ovi that orchestrates the reconciliation process:

```go
type Engine struct {
    observer      Observer      // Polls cloud providers
    comparator    Comparator    // Finds state differences  
    decisionMaker DecisionMaker // Generates safe actions
    coordinator   Coordinator   // Prevents conflicts
    storage       MVCCStorage   // Persistent observations
    wal           WAL           // Audit logging
}
```

**Reconciliation Flow:**
1. **Observe** current cloud state
2. **Store** observations with revision
3. **Compare** current vs desired state
4. **Decide** what actions to take
5. **Log** decisions to WAL
6. **Execute** actions (separate component)

### MVCC Storage System
Inspired by etcd's multi-version concurrency control:

- **Revision-based**: Every observation has monotonic revision number
- **Time-travel**: Query state at any historical revision
- **Concurrent**: Multiple readers, serialized writers
- **Compaction**: Remove old revisions to control storage size

### Write-Ahead Log (WAL)
Complete audit trail for debugging and recovery:

- **Sequential logging**: All operations logged in order
- **Entry types**: `observed`, `decided`, `executing`, `executed`, `failed`
- **Crash recovery**: Resume from last known state
- **Replay debugging**: Understand why decisions were made

### Provider Interface
Pluggable cloud provider abstraction:

```go
type CloudProvider interface {
    ListResources(ctx context.Context, filter ResourceFilter) ([]Resource, error)
    CreateResource(ctx context.Context, spec ResourceSpec) (*Resource, error)
    UpdateResource(ctx context.Context, resource Resource) error
    DeleteResource(ctx context.Context, resourceID string) error
}
```

## Data Flow

### 1. Observation Phase
```
Cloud Provider → Observer → MVCC Storage
                     ↓
                 WAL Entry (observed)
```

### 2. Comparison Phase  
```
MVCC Storage → Comparator → Diff List
Config File  ↗
```

### 3. Decision Phase
```
Diff List → Decision Maker → Decision List
                    ↓
              WAL Entry (decided)
```

### 4. Execution Phase
```
Decision List → Executor → Cloud Provider
                   ↓
            WAL Entry (executed)
```

## Coordination Model

### Resource Claims
To prevent multiple Ovi instances from conflicting:

```go
type Claim struct {
    ResourceID string
    InstanceID string  
    ClaimedAt  time.Time
    ExpiresAt  time.Time
}
```

### Claim Lifecycle
1. **Claim**: Instance claims resources before acting
2. **Execute**: Perform actions on claimed resources
3. **Release**: Release claims when done
4. **Expire**: Claims auto-expire to handle crashes

## Safety Features

### Blessed Resources
Resources tagged with `elava:blessed: true` are protected:
- Cannot be deleted automatically
- Generate notification decisions instead
- Require explicit human intervention

### Destructive Action Controls
- **Skip destructive**: Configuration option to avoid deletions
- **Dry run mode**: Preview decisions without executing
- **Confirmation required**: Some actions need approval

### Error Handling
- **Graceful degradation**: Continue processing other resources if one fails
- **Retry logic**: Exponential backoff for transient failures  
- **Error logging**: All failures recorded in WAL

## Performance Characteristics

### Storage
- **In-memory index**: Fast lookups with btree
- **On-disk persistence**: bbolt for durability
- **Compaction**: Control storage growth
- **Concurrent reads**: Multiple readers supported

### Scaling
- **Horizontal**: Multiple Ovi instances
- **Resource-level parallelism**: Process resources concurrently
- **Provider-agnostic**: Pluggable cloud providers
- **Stateless**: Easy to containerize and scale

---

This architecture enables Ovi to be both powerful and safe, providing complete visibility into infrastructure changes while preventing the coordination issues that plague traditional infrastructure tools.