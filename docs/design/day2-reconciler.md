# Day 2 Operations Reconciler Design

## 🎯 Philosophy Shift

**From:** Infrastructure-as-Code state enforcer
**To:** Living infrastructure observer & protector

```
OLD: "Make reality match my config file"
NEW: "Observe reality, detect changes, enforce policies"
```

## Core Principles

### 1. Cloud IS the Source of Truth
- No state files declaring "what should exist"
- Observe what actually exists in AWS/GCP
- Track changes over time in MVCC storage
- Tag-based ownership and management

### 2. Continuous Observation
- Tiered scanning based on resource criticality
- Store all observations with full history
- Detect changes between observations
- Temporal queries: "What changed? When? Why?"

### 3. Policy Enforcement, Not State Enforcement
- OPA policies define acceptable states
- Detect violations, don't dictate infrastructure
- Protect blessed resources
- Notify on policy violations

### 4. Friendly, Not Aggressive
- Never auto-delete (unless explicitly configured)
- Notify before taking action
- Respect "blessed" tags
- Track orphans without touching them

## Architecture

### The New Reconciliation Loop

```
┌─────────────────────────────────────────────────────────┐
│                    RECONCILE CYCLE                       │
└─────────────────────────────────────────────────────────┘
                           │
                           ▼
              ┌────────────────────────┐
              │   1. OBSERVE & STORE   │
              │  ─────────────────────  │
              │  • Scan cloud APIs     │
              │  • Record in MVCC      │
              │  • Detect new/gone     │
              └────────┬───────────────┘
                       │
                       ▼
              ┌────────────────────────┐
              │  2. DETECT CHANGES     │
              │  ─────────────────────  │
              │  • Compare with prev   │
              │  • Tag changes         │
              │  • Status changes      │
              │  • Disappeared         │
              │  • New resources       │
              └────────┬───────────────┘
                       │
                       ▼
              ┌────────────────────────┐
              │  3. CHECK POLICIES     │
              │  ─────────────────────  │
              │  • OPA evaluation      │
              │  • Tag compliance      │
              │  • Security rules      │
              │  • Cost policies       │
              └────────┬───────────────┘
                       │
                       ▼
              ┌────────────────────────┐
              │   4. MAKE DECISIONS    │
              │  ─────────────────────  │
              │  • notify              │
              │  • protect             │
              │  • enforce_policy      │
              │  • alert               │
              │  • auto_tag (opt)      │
              └────────┬───────────────┘
                       │
                       ▼
              ┌────────────────────────┐
              │   5. LOG TO WAL        │
              │  ─────────────────────  │
              │  • Full audit trail    │
              │  • All observations    │
              │  • All decisions       │
              └────────────────────────┘
```

## New Component Responsibilities

### Observer
```go
// Scans cloud providers and returns current state
// NO concept of "desired" state
type Observer interface {
    Observe(ctx context.Context, filter ResourceFilter) ([]Resource, error)
}
```

**What it does:**
- Call AWS/GCP APIs
- Retrieve current resources
- No comparison, just observation
- Return raw reality

### ChangeDetector (replaces Comparator)
```go
// Detects changes between observations over time
// Uses MVCC storage to query history
type ChangeDetector interface {
    DetectChanges(ctx context.Context, current []Resource) ([]Change, error)
}

type Change struct {
    Type       ChangeType  // appeared, disappeared, modified, tag_drift
    ResourceID string
    Previous   *Resource   // From MVCC history
    Current    *Resource   // From latest observation
    Timestamp  time.Time
    Details    string
}

type ChangeType string
const (
    ChangeAppeared     ChangeType = "appeared"      // New resource seen
    ChangeDisappeared  ChangeType = "disappeared"   // Resource gone
    ChangeModified     ChangeType = "modified"      // Status/config changed
    ChangeTagDrift     ChangeType = "tag_drift"     // Tags changed
    ChangeCostIncrease ChangeType = "cost_increase" // Cost anomaly
)
```

**What it does:**
- Query MVCC for previous observations
- Compare current vs previous
- Identify what changed
- Track temporal patterns

### PolicyEnforcer
```go
// Evaluates resources against OPA policies
type PolicyEnforcer interface {
    Evaluate(ctx context.Context, resource Resource) (PolicyResult, error)
    BatchEvaluate(ctx context.Context, resources []Resource) ([]PolicyResult, error)
}

type PolicyResult struct {
    ResourceID string
    Violations []PolicyViolation
    Compliant  bool
}

type PolicyViolation struct {
    Policy     string   // Which policy failed
    Severity   string   // critical, warning, info
    Message    string   // Human-readable description
    Suggestion string   // How to fix
}
```

**What it does:**
- Run OPA policies on resources
- Check tag compliance
- Security validation
- Cost policy checks

### DecisionMaker (updated)
```go
// Makes Day 2 operation decisions based on changes + policies
type DecisionMaker interface {
    Decide(changes []Change, policyResults []PolicyResult) ([]Decision, error)
}

type Decision struct {
    Action     ActionType
    ResourceID string
    Reason     string
    Severity   string
    Metadata   map[string]interface{}
}

type ActionType string
const (
    ActionNotify        ActionType = "notify"         // Send alert
    ActionProtect       ActionType = "protect"        // Mark as protected
    ActionEnforcePolicy ActionType = "enforce_policy" // Fix policy violation
    ActionAutoTag       ActionType = "auto_tag"       // Add missing tags
    ActionAlert         ActionType = "alert"          // High-priority notification
    ActionAudit         ActionType = "audit"          // Just log, no action
)
```

**Decision logic:**
- Resource disappeared + blessed → `protect` (too late!) + `alert`
- Tag drifted → `enforce_policy` or `notify`
- Policy violation → `alert` or `notify` based on severity
- Orphan found → `notify` (never auto-delete)
- Untagged resource → `auto_tag` (if enabled) or `notify`

### Coordinator (unchanged)
- Multi-instance coordination
- Claim resources for exclusive processing
- TTL-based claims
- Already well-designed for Day 2

## Data Flow

### 1. Storage Integration

**MVCC Storage Queries:**
```go
// Get resource history
history := storage.GetResourceHistory(resourceID)

// Get all changes in time range
changes := storage.GetChangesSince(time.Now().Add(-24*time.Hour))

// Query: "What resources disappeared?"
disappeared := storage.QueryDisappearedResources(since)

// Query: "What had tag changes?"
tagChanges := storage.QueryTagChanges(since, []string{"Owner", "Environment"})
```

**New MVCC Methods Needed:**
```go
type MVCCStorage interface {
    // Existing methods...

    // New for change detection
    GetPreviousObservation(resourceID string) (*Resource, error)
    GetChangesSince(since time.Time) ([]ResourceChange, error)
    QueryDisappearedResources(since time.Time) ([]Resource, error)
    QueryNewResources(since time.Time) ([]Resource, error)
    QueryModifiedResources(since time.Time) ([]ResourceChange, error)
}
```

### 2. Policy Integration

**OPA Integration:**
```go
// policies/day2/required_tags.rego
package day2.tags

deny[msg] {
    not input.tags.elava_managed
    msg = "Resource missing elava:managed tag"
}

deny[msg] {
    input.tags.elava_managed == true
    not input.tags.elava_owner
    msg = "Managed resource missing elava:owner tag"
}

warn[msg] {
    not input.tags.environment
    msg = "Resource missing environment tag"
}
```

**Policy Enforcer:**
```go
func (pe *OPAEnforcer) Evaluate(resource Resource) (PolicyResult, error) {
    input := map[string]interface{}{
        "resource": resource,
        "tags":     resource.Tags,
        "type":     resource.Type,
    }

    result, err := pe.opa.Eval(ctx, "data.day2", input)
    // Check deny[], warn[], info[] arrays
    // Return violations with severity
}
```

### 3. WAL Integration

**Enhanced WAL Entries:**
```go
// Log change detection
wal.Append(EntryObserved, "", ChangeDetectionResult{
    ChangesFound:     len(changes),
    Appeared:         5,
    Disappeared:      2,
    Modified:         3,
    PolicyViolations: 1,
})

// Log policy evaluation
wal.Append(EntryDecided, resourceID, PolicyEvaluation{
    ResourceID: resourceID,
    Violations: violations,
    Action:     "notify",
})

// Log decision execution
wal.Append(EntryExecuted, resourceID, DecisionExecution{
    Action:  "auto_tag",
    Success: true,
    Tags:    map[string]string{"elava:managed": "true"},
})
```

## Configuration

### New Config Structure

```yaml
version: "2.0"
provider: aws
region: us-east-1

# Observation settings (keep existing tiered scanning)
scanning:
  enabled: true
  adaptive_hours: true
  tiers:
    critical:
      scan_interval: 15m
      patterns:
        - type: rds
          tags:
            environment: production
    # ... other tiers

# Change detection (enhanced)
change_detection:
  enabled: true
  check_interval: 5m
  lookback_window: 24h

  track:
    - resource_lifecycle  # appeared/disappeared
    - tag_changes
    - status_changes
    - cost_changes
    - configuration_drift

  alert_on:
    disappeared_blessed: critical
    disappeared_managed: warning
    new_untagged: info
    tag_drift: warning
    policy_violation: severity_based

# Policy enforcement
policies:
  path: ./policies/day2
  enforcement_mode: notify  # notify, enforce, or dry_run

  auto_remediate:
    tag_violations: false
    blessed_protection: true

  severity_actions:
    critical: alert
    warning: notify
    info: audit

# Actions
actions:
  notifications:
    slack:
      enabled: true
      webhook: ${SLACK_WEBHOOK}
      channels:
        critical: "#alerts-critical"
        warning: "#alerts-warn"

  protection:
    respect_blessed: true
    auto_protect_prod: true

  tagging:
    auto_tag_untagged: false
    required_tags:
      - elava:managed
      - elava:owner
      - environment

# Behavioral rules
behavior:
  grace_period: 24h        # Wait before alerting on disappeared
  auto_delete: false       # Never auto-delete
  require_approval: true   # All destructive actions need approval
```

## Implementation Plan

### Phase 1: Core Refactoring
- [ ] Create `ChangeDetector` interface and implementation
- [ ] Update `DecisionMaker` with new action types
- [ ] Deprecate `buildDesiredState()`
- [ ] Remove `Config.Resources` field
- [ ] Update tests to match Day 2 behavior

### Phase 2: MVCC Integration
- [ ] Add temporal query methods to MVCC storage
- [ ] Implement `GetPreviousObservation()`
- [ ] Implement `GetChangesSince()`
- [ ] Add change detection queries

### Phase 3: Policy Integration
- [ ] Create `PolicyEnforcer` interface
- [ ] Integrate OPA evaluation
- [ ] Add policy result handling to DecisionMaker
- [ ] Create example Day 2 policies

### Phase 4: Enhanced Decision Making
- [ ] Implement change-based decisions
- [ ] Add policy-based decisions
- [ ] Integrate blessed resource protection
- [ ] Add notification/alerting logic

### Phase 5: Testing & Documentation
- [ ] Update all tests
- [ ] Add integration tests
- [ ] Update documentation
- [ ] Create migration guide

## Migration Path

### For Existing Code

**What to Keep:**
- ✅ Observer interface (perfect as-is)
- ✅ Coordinator (multi-instance coordination)
- ✅ MVCC Storage (enhance with queries)
- ✅ WAL (enhance with new entry types)
- ✅ Tiered scanning config

**What to Change:**
- 🔄 Comparator → ChangeDetector (different purpose)
- 🔄 DecisionMaker → New action types
- 🔄 Engine.Reconcile() → Focus on changes, not state

**What to Remove:**
- ❌ `buildDesiredState()` function
- ❌ `Config.Resources` field
- ❌ IaC-style "create/delete" mindset
- ❌ Comparison to "desired state"

### Backward Compatibility

For users who might have started using the IaC approach:

```go
// Deprecation notice
// Deprecated: buildDesiredState is deprecated and will be removed in v2.0.
// Elava has pivoted to Day 2 operations and no longer manages infrastructure state.
// Use tag-based resource tracking instead.
func (e *Engine) buildDesiredState(config Config) []types.Resource {
    // Keep minimal implementation but log warning
    log.Warn("buildDesiredState is deprecated - Elava now focuses on Day 2 operations")
    return nil
}
```

## Examples

### Example 1: Detect Disappeared Resource

```go
// Current observation
current := observer.Observe(ctx, filter)

// Detect changes (queries MVCC)
changes := changeDetector.DetectChanges(ctx, current)

// Find disappeared resources
for _, change := range changes {
    if change.Type == ChangeDisappeared {
        if change.Previous.IsBlessed() {
            // Critical alert!
            decisions = append(decisions, Decision{
                Action:     ActionAlert,
                ResourceID: change.ResourceID,
                Reason:     "Blessed resource disappeared",
                Severity:   "critical",
            })
        } else {
            // Just notify
            decisions = append(decisions, Decision{
                Action:     ActionNotify,
                ResourceID: change.ResourceID,
                Reason:     "Resource disappeared",
                Severity:   "warning",
            })
        }
    }
}
```

### Example 2: Policy Violation Detection

```go
// Check policies on all current resources
policyResults := policyEnforcer.BatchEvaluate(ctx, current)

for _, result := range policyResults {
    if !result.Compliant {
        for _, violation := range result.Violations {
            decision := Decision{
                ResourceID: result.ResourceID,
                Reason:     violation.Message,
            }

            // Decide action based on severity
            switch violation.Severity {
            case "critical":
                decision.Action = ActionAlert
                decision.Severity = "critical"
            case "warning":
                decision.Action = ActionNotify
                decision.Severity = "warning"
            default:
                decision.Action = ActionAudit
                decision.Severity = "info"
            }

            decisions = append(decisions, decision)
        }
    }
}
```

### Example 3: Tag Drift Auto-Remediation

```go
// Detect tag changes
for _, change := range changes {
    if change.Type == ChangeTagDrift {
        // Check if auto-remediation enabled
        if config.AutoRemediateTagDrift {
            decisions = append(decisions, Decision{
                Action:     ActionEnforcePolicy,
                ResourceID: change.ResourceID,
                Reason:     "Tag drift detected - auto-remediating",
                Metadata: map[string]interface{}{
                    "previous_tags": change.Previous.Tags,
                    "current_tags":  change.Current.Tags,
                    "action":        "restore_tags",
                },
            })
        } else {
            decisions = append(decisions, Decision{
                Action:     ActionNotify,
                ResourceID: change.ResourceID,
                Reason:     "Tag drift detected",
                Severity:   "warning",
            })
        }
    }
}
```

## Success Metrics

### Before (IaC approach):
- Resources created: N
- Resources deleted: N
- State matches config: Yes/No

### After (Day 2 approach):
- Resources observed: N
- Changes detected: N (appeared/disappeared/modified)
- Policy violations found: N
- Alerts sent: N
- Protected resources: N
- Audit trail completeness: 100%
- Time to detect change: <5min (critical tier)

## Conclusion

This design transforms Elava from an IaC tool to a true Day 2 operations platform:

- **Observes** reality instead of enforcing declarations
- **Detects** changes instead of calculating diffs from config
- **Protects** infrastructure instead of managing it
- **Notifies** operators instead of auto-deleting
- **Enforces policies** instead of enforcing state

The cloud IS the source of truth. Elava watches, learns, and protects.
