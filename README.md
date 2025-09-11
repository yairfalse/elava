# Ovi

**A living infrastructure reconciliation engine with complete auditability and time-travel debugging.**

Your cloud is the state. Every decision is explained. Nothing is hidden.

## What Makes Ovi Different

Unlike traditional infrastructure tools, Ovi provides:

- **No state files** - MVCC storage with time-travel capabilities instead
- **Complete audit trail** - WAL logs every observation, decision, and execution
- **Intelligent reconciliation** - Observes reality first, then makes safe decisions
- **Multi-instance coordination** - Multiple Ovi instances work together harmoniously
- **Blessed resource protection** - Critical resources can't be accidentally deleted

## Architecture

```
┌─────────────────┐    ┌─────────────────┐    ┌─────────────────┐
│   Ovi Instance  │    │   Ovi Instance  │    │   Ovi Instance  │
│                 │    │                 │    │                 │
│   Reconciler    │    │   Reconciler    │    │   Reconciler    │
│   Engine        │    │   Engine        │    │   Engine        │
└────────┬────────┘    └────────┬────────┘    └────────┬────────┘
         │                      │                      │
         └──────────────────────┼──────────────────────┘
                               │
                    ┌──────────┴──────────┐
                    │                     │
                    │   Shared Storage    │
                    │                     │
                    │  ┌───────────────┐  │
                    │  │ MVCC Storage  │  │ ← Time-travel queries
                    │  │   (bbolt)     │  │
                    │  └───────────────┘  │
                    │                     │
                    │  ┌───────────────┐  │
                    │  │ WAL Audit Log │  │ ← Complete transparency
                    │  │  (append-only)│  │
                    │  └───────────────┘  │
                    └─────────────────────┘
```

## The Reconciliation Flow

```
Observe → Store → Compare → Decide → Log → Execute
   ↑                                           │
   └───────────────────────────────────────────┘
              (Continuous Loop)
```

## How It Works

### 1. Define Your Infrastructure

```yaml
# infrastructure.yaml
version: v1
provider: aws
region: us-east-1

resources:
  - type: ec2
    count: 3
    size: t3.micro
    tags:
      environment: prod
      team: platform
      ovi:blessed: true  # Protected from deletion
```

### 2. Run Reconciliation

```bash
# See what Ovi will do (dry-run)
ovi reconcile --dry-run

# Apply changes
ovi reconcile

# Run continuously
ovi daemon --interval 30s
```

### 3. Understand Every Decision

```bash
# Why did Ovi delete that resource?
ovi wal replay --resource-id "i-123456" --since "1 hour ago"

# Output:
# 10:30:00 [observed]  Resource i-123456 found in cloud
# 10:30:01 [decided]   Delete: Not in desired configuration
# 10:30:02 [executing] Starting deletion
# 10:30:03 [executed]  Successfully deleted
```

## Key Features

### 🔍 Complete Observability
- Every action logged to Write-Ahead Log (WAL)
- Time-travel debugging with MVCC storage
- Query infrastructure state at any point in history

### 🛡️ Safety First
- Blessed resources protected from deletion
- Configurable destructive action controls
- Dry-run mode for previewing changes
- Multi-instance coordination prevents conflicts

### 📊 Intelligent Reconciliation
- Detects 4 types of drift: missing, unwanted, drifted, unmanaged
- Makes smart decisions based on resource state
- Handles partial failures gracefully

### 🏗️ Production Ready
- 86+ comprehensive tests
- Strictly typed (no `map[string]interface{}` anywhere)
- Complete error handling
- Extensive documentation

## Installation

```bash
# Clone and build
git clone https://github.com/yairfalse/ovi
cd ovi
go build ./cmd/ovi

# Run tests
go test ./...
```

## Documentation

Comprehensive documentation is available in the [`docs/`](docs/) directory:

- [Architecture Overview](docs/architecture/overview.md)
- [MVCC Storage Design](docs/architecture/mvcc-storage.md)
- [WAL Audit System](docs/architecture/wal-system.md)
- [Reconciler Engine](docs/design/reconciler-engine.md)
- [Type System Philosophy](docs/design/type-system.md)

## Development Status

Core engine complete with:
- ✅ MVCC storage with time-travel
- ✅ WAL audit logging
- ✅ Reconciler engine
- ✅ AWS provider interface
- ✅ Comprehensive test coverage
- ⏳ CLI interface (in progress)
- ⏳ Executor component (in progress)

## Example: Debugging with WAL

When something unexpected happens, Ovi's WAL provides complete transparency:

```bash
# Show all decisions in the last hour
ovi wal replay --since "1 hour ago" --type decided

# Track a specific resource lifecycle
ovi wal analyze --resource-id "prod-database" --timeline

# Find failed operations
ovi wal replay --type failed --since "1 day ago"
```

## Philosophy

1. **Stateless with Memory**: No state files, but MVCC provides perfect coordination
2. **Reality First**: Always observe actual cloud state before making decisions
3. **Complete Transparency**: Every decision is logged and explainable
4. **Safety Over Speed**: Better to notify than accidentally delete
5. **Strictly Typed**: Compile-time safety through structured types

## Testing

```bash
# Run all tests
go test ./... -v

# Run specific package tests
go test ./reconciler/... -v
go test ./storage/... -v
go test ./wal/... -v
```

## Contributing

Read [CLAUDE.md](CLAUDE.md) for our strict development standards:
- Test-Driven Development required
- No `map[string]interface{}` allowed
- Functions must be <50 lines
- All changes must have tests

## License

MIT

---

**Ovi**: Infrastructure reconciliation with unprecedented transparency and safety.