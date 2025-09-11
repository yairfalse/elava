# Ovi Documentation

**Ovi** is a living infrastructure reconciliation engine that acts as a "benevolent dictator" for cloud resources.

## Overview

Ovi continuously observes your cloud infrastructure, compares it with desired state, and makes intelligent decisions about what actions to take. Unlike traditional tools, Ovi is stateless but has memory through an etcd-inspired MVCC storage system.

## Key Principles

- **Stateless with Memory**: No state files, but MVCC provides coordination
- **Observation-Based**: Records what it sees, makes decisions based on reality  
- **Auditable**: Complete WAL trail of every observation → decision → execution
- **Safe**: Blessed resources are protected, destructive actions require confirmation
- **Coordinated**: Multiple instances can run without conflicts

## Documentation Structure

### Architecture
- [**Overall Architecture**](architecture/overview.md) - High-level system design
- [**MVCC Storage**](architecture/mvcc-storage.md) - Multi-version storage system
- [**WAL System**](architecture/wal-system.md) - Write-ahead logging for audit

### Design
- [**Reconciler Engine**](design/reconciler-engine.md) - Core reconciliation logic
- [**Provider Interface**](design/provider-interface.md) - Cloud provider abstraction
- [**Type System**](design/type-system.md) - Structured types (no maps!)

### API Reference
- [**Storage API**](api/storage.md) - MVCC storage operations
- [**WAL API**](api/wal.md) - Write-ahead log operations  
- [**Reconciler API**](api/reconciler.md) - Reconciliation engine

## Quick Start

```bash
# Clone and build
git clone https://github.com/yairfalse/ovi
cd ovi
go build ./cmd/ovi

# Run reconciliation
./ovi reconcile --config infrastructure.yaml
```

## What Makes Ovi Different

| Feature | Terraform | Ovi |
|---------|-----------|-----|
| State Management | State files | MVCC storage |
| Coordination | Locking | Claims + WAL |
| Observability | Plan output | Complete audit trail |
| Safety | Plan review | Blessed resources |
| Memory | Stateful | Stateless with memory |

## Core Components

1. **Observer** - Polls cloud providers for current state
2. **Storage** - MVCC system for tracking observations over time  
3. **Comparator** - Identifies differences between current/desired state
4. **Decision Maker** - Generates safe actions based on differences
5. **Executor** - Performs actions with full audit logging
6. **WAL** - Immutable log of all operations for debugging/recovery

## Test Coverage

Ovi has comprehensive test coverage with 67+ passing tests across all components:

```bash
go test ./... -v
```

- **Storage Tests**: Time-travel queries, concurrent access, compaction
- **WAL Tests**: Audit trails, replay functionality, data integrity  
- **Reconciler Tests**: State comparison, decision making, safety checks
- **Provider Tests**: AWS integration, resource filtering

---

*Ovi: Infrastructure reconciliation done right.*