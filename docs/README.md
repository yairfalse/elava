# Elava Documentation

**Elava** is an infrastructure observability platform with temporal memory - think "git log for your cloud infrastructure".

## Overview

Elava continuously observes your cloud infrastructure, tracks changes over time, and provides temporal queries to understand what changed, when, and why. Unlike traditional monitoring tools, Elava has memory through an MVCC storage system that enables time-travel queries.

## Key Principles

- **Observability-Only**: Read-only scanning, no infrastructure modifications
- **Temporal Awareness**: MVCC storage provides complete historical context
- **Time-Travel Queries**: Query infrastructure state at any point in time
- **Drift Detection**: Compare current reality with historical snapshots
- **Complete Audit Trail**: WAL logging of all observations and changes
- **Multi-Account Support**: Scan across multiple AWS accounts

## Documentation Structure

### Architecture
- [**Overall Architecture**](architecture/overview.md) - High-level system design
- [**MVCC Storage**](architecture/mvcc-storage.md) - Multi-version storage system
- [**WAL System**](architecture/wal-system.md) - Write-ahead logging for audit

### Design
- [**Analyzer Engine**](design/analyzer-engine.md) - Core analysis and drift detection
- [**Provider Interface**](design/provider-interface.md) - Cloud provider abstraction
- [**Type System**](design/type-system.md) - Structured types (no maps!)

### Integration
- [**Elava → Ahti Integration**](elava-ahti-integration.md) - Enterprise correlation with Tapio

### API Reference
- [**Storage API**](api/storage.md) - MVCC storage operations
- [**WAL API**](api/wal.md) - Write-ahead log operations
- [**Analyzer API**](api/analyzer.md) - Analysis and drift detection

## Quick Start

```bash
# Clone and build
git clone https://github.com/yairfalse/elava
cd elava
go build ./cmd/elava

# Scan AWS infrastructure
./elava scan --region us-east-1

# View inventory
./elava show ec2

# Time-travel queries
./elava history changes --since "24h ago"
./elava history resource i-abc123 --at "2025-10-01"

# Find missing tags
./elava inventory tags missing --required "Environment,Team,Owner"
```

## What Makes Elava Different

| Feature | CloudWatch | Elava |
|---------|-----------|-------|
| State Management | Current state only | MVCC temporal storage |
| History | Limited metrics | Complete observation history |
| Time-Travel | No | Yes - query any point in time |
| Drift Detection | No | Yes - compare historical states |
| Tag Compliance | Manual | Automated scanning |
| Change Tracking | CloudTrail (events) | Resource-level changes with context |

## Core Components

1. **Scanner** - Polls cloud providers for current state (read-only)
2. **Storage** - MVCC system for tracking observations over time
3. **Analyzer** - Identifies drift, missing tags, orphaned resources
4. **Query Engine** - Time-travel queries and historical analysis
5. **WAL** - Immutable log of all observations for debugging/recovery

## Enterprise: Elava + Ahti Integration

For enterprise deployments, Elava integrates with **Ahti** (unified observability platform):

- Correlates infrastructure events (Elava) with runtime events (Tapio)
- Root cause analysis across K8s and AWS layers
- Graph-based relationship modeling
- Cross-system visibility

See [Integration Design](elava-ahti-integration.md) for details.

## Test Coverage

Elava has comprehensive test coverage with 67+ passing tests across all components:

```bash
go test ./... -v
```

- **Storage Tests**: Time-travel queries, concurrent access, compaction
- **WAL Tests**: Audit trails, replay functionality, data integrity
- **Analyzer Tests**: Drift detection, pattern recognition
- **Provider Tests**: AWS integration, resource filtering

---

*Elava: Infrastructure observability with memory.*
