# Change Detector Design

**Status:** Implementation
**Date:** 2025-10-19

## Problem Statement

After each scan, we need to detect infrastructure changes by comparing scan N vs scan N-1:
- **New resources** (appeared since last scan)
- **Modified resources** (tags, status, metadata changed)
- **Disappeared resources** (no longer seen)

## Storage-First Design

### Read Operations
```go
// Get previous scan resources (revision N-1)
previousResources := storage.GetAllCurrentResources()

// Current scan resources (revision N) - passed in
currentResources := []types.Resource{...}
```

### Write Operations
```go
// Generate ChangeEvents
events := []storage.ChangeEvent{
    {ResourceID: "i-abc123", ChangeType: "created", ...},
    {ResourceID: "sg-xyz789", ChangeType: "modified", ...},
}

// Caller stores events
storage.StoreChangeEventBatch(ctx, events)
```

## Interface

```go
type ChangeDetector interface {
    // DetectChanges compares current scan with previous revision
    // Returns ChangeEvents but does NOT store them
    DetectChanges(ctx context.Context, currentScan []types.Resource) ([]storage.ChangeEvent, error)
}
```

## Implementation Steps (TDD)

### RED Phase
1. Write test `TestChangeDetector_DetectNewResources`
2. Write test `TestChangeDetector_DetectModified`
3. Write test `TestChangeDetector_DetectDisappeared`
4. Verify compilation fails (undefined: NewChangeDetector)

### GREEN Phase
1. Create minimal `ChangeDetectorImpl` struct
2. Implement `DetectChanges()` - simple comparison logic
3. Verify all tests pass

### REFACTOR Phase
1. Extract `compareResources()` helper
2. Extract `buildChangeEvent()` helper
3. Add edge cases (first scan, no changes, etc.)
4. Keep functions < 50 lines

## Edge Cases

1. **First scan** (no previous revision) → return empty events
2. **No changes** → return empty events
3. **Only metadata changed** → still count as "modified"
4. **Tombstones** → mark as "disappeared"

## Integration

Called by scan pipeline after `RecordObservationBatch`:

```go
// In scan.go
revision, _ := storage.RecordObservationBatch(resources)

// Detect changes
detector := analyzer.NewChangeDetector(storage)
changes, _ := detector.DetectChanges(ctx, resources)

// Store change events
storage.StoreChangeEventBatch(ctx, changes)
```

## Success Criteria

- [ ] All tests pass (RED → GREEN → REFACTOR)
- [ ] Functions < 50 lines
- [ ] No map[string]interface{}
- [ ] Error context included
- [ ] 80%+ test coverage
