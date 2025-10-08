# Engine Day 2 Integration

## Goal

Replace IaC state enforcement with Day 2 operations: observe, detect changes, enforce policies.

## What We're Replacing

### OLD (IaC Approach)
```go
type Engine struct {
    comparator    Comparator          // Compare current vs desired
    decisionMaker DecisionMaker       // Decide create/update/delete
}

func (e *Engine) Reconcile() {
    current := observe()
    desired := buildFromConfig()      // ← Read config files
    diffs := comparator.Compare(current, desired)
    decisions := decisionMaker.Decide(diffs)
}
```

**Problems:**
- Requires declaring "desired state" in config
- Focused on create/delete operations
- No change detection over time
- No policy enforcement
- No baseline handling

### NEW (Day 2 Approach)
```go
type Engine struct {
    changeDetector      ChangeDetector       // Detect changes over time
    policyDecisionMaker PolicyDecisionMaker  // Enforce policies
}

func (e *Engine) Reconcile() {
    current := observe()
    storeObservations(current)          // ← Store in MVCC
    changes := changeDetector.DetectChanges(current)  // ← Compare with history
    decisions := policyDecisionMaker.Decide(changes)  // ← Apply policies
}
```

**Benefits:**
- No desired state files needed
- Temporal change detection (what changed since last scan?)
- OPA policy enforcement
- Baseline scan on first run
- Infrastructure summary reports

## Components Being Replaced

| Old Component | New Component | Status |
|--------------|---------------|---------|
| `Comparator` (comparator.go) | `TemporalChangeDetector` (change_detector.go) | ✅ Built |
| `SimpleDecisionMaker` (decision_maker.go) | `PolicyEnforcingDecisionMaker` (policy_decision_maker.go) | ✅ Built |
| `buildDesiredState()` | *(removed)* | 🗑️ Delete |

## New Flow

### First Scan (Baseline)
```
1. Observe current infrastructure
2. Store in MVCC (empty before)
3. Detect: len(previousIDs) == 0 → First scan!
4. Mark all as ChangeBaseline
5. Generate BaselineSummary
6. Display: "247 resources discovered, oldest 3 years ago..."
7. Return ActionAudit decisions (silent)
```

### Subsequent Scans
```
1. Observe current infrastructure
2. Store in MVCC
3. Detect changes:
   - appeared (new resources)
   - disappeared (deleted)
   - tag_drift (tags changed)
   - status_changed (running → stopped)
   - modified (config changed)
4. Evaluate OPA policies
5. Return decisions:
   - notify (FYI)
   - alert (blessed resource disappeared!)
   - auto_tag (fix tags)
   - enforce_tags (restore tags)
   - protect (mark as blessed)
```

## Files to Modify

### reconciler/reconciler.go
- Remove `comparator` and `decisionMaker` fields
- Add `changeDetector` and `policyDecisionMaker` fields
- Update `NewEngine()` constructor
- Replace `compareAndDecide()` with `detectAndDecide()`
- Add `handleBaselineScan()` for first scan
- Remove `buildDesiredState()` entirely

### reconciler/types.go
- Remove `Diff` struct (IaC concept)
- Keep `Change` struct (Day 2 concept)
- Remove `Config.Resources` (no desired state)

### cmd/elava/cmd_reconcile.go
- Create `TemporalChangeDetector`
- Create `PolicyEnforcingDecisionMaker`
- Pass to `NewEngine()`
- Display baseline summary on first scan

### Delete (Safe to Remove)
- reconciler/comparator.go (replaced by change_detector.go)
- reconciler/comparator_test.go
- reconciler/decision_maker.go (replaced by policy_decision_maker.go)
- reconciler/decision_maker_test.go

## Implementation Steps

1. **Update Engine struct** - Replace old fields with new
2. **Update NewEngine()** - Accept new components
3. **Replace compareAndDecide()** - Use detectAndDecide()
4. **Add baseline handling** - Detect first scan, show summary
5. **Update tests** - Use new components
6. **Wire in CLI** - Create and pass new components
7. **Delete old files** - Remove comparator and old decision_maker
8. **Test E2E** - Full Day 2 flow

## Breaking Changes

None - we have no users yet!

## Rollout

Immediate replacement. No migration needed.
