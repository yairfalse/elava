# First Scan Baseline Design

## Problem Statement

On Elava's first-ever scan (or any baseline scan), the MVCC storage is empty. The ChangeDetector marks ALL resources as `ChangeAppeared` even if they've existed for months/years. This creates noise and misses the opportunity to establish an intelligent baseline.

## The Core Insight

**Resources carry their own temporal history even when Elava first observes them:**
- `CreatedAt` timestamp → Resource age
- Tags → Management status, environment, ownership
- Status → Current operational state
- Configuration → Current setup

**We should use this metadata to establish an intelligent baseline, not just mark everything as "appeared".**

## Minimal Solution (In Scope)

### 1. Add ChangeBaseline Type

Just one new constant - that's it.

```go
const (
    ChangeAppeared      ChangeType = "appeared"       // New resource (normal scans)
    ChangeBaseline      ChangeType = "baseline"       // First observation (first scan only)
    ChangeDisappeared   ChangeType = "disappeared"
    ChangeModified      ChangeType = "modified"
    ChangeTagDrift      ChangeType = "tag_drift"
    ChangeStatusChanged ChangeType = "status_changed"
    ChangeUnmanaged     ChangeType = "unmanaged"
)
```

### 2. Detect First Scan

Simple check in ChangeDetector:

```go
func (d *TemporalChangeDetector) DetectChanges(ctx context.Context, current []types.Resource) ([]Change, error) {
    var changes []Change

    currentMap := buildResourceMap(current)

    // Get all resource IDs we've seen before
    previousIDs, err := d.getPreviousResourceIDs(ctx)
    if err != nil {
        return nil, err
    }

    // FIRST SCAN: If no previous resources, this is baseline
    isFirstScan := len(previousIDs) == 0

    for _, resource := range current {
        if isFirstScan {
            // Mark as baseline, not appeared
            changes = append(changes, Change{
                Type:       ChangeBaseline,
                ResourceID: resource.ID,
                Current:    &resource,
                Timestamp:  time.Now(),
                Details:    "Baseline observation",
            })
        } else {
            // Normal change detection
            change := d.detectResourceChange(ctx, resource, previousIDs)
            if change != nil {
                changes = append(changes, *change)
            }
        }
    }

    // No disappeared resources on first scan
    if !isFirstScan {
        disappeared := d.detectDisappeared(ctx, currentMap, previousIDs)
        changes = append(changes, disappeared...)
    }

    return changes, nil
}
```

### 3. Handle Baseline in DecisionMaker

Just add one case in the switch:

```go
func (dm *PolicyEnforcingDecisionMaker) decideFromChange(ctx context.Context, change Change) (*types.Decision, error) {
    switch change.Type {
    case ChangeBaseline:
        return dm.decideBaseline(ctx, change)  // New case
    case ChangeAppeared:
        return dm.decideAppeared(ctx, change)
    case ChangeDisappeared:
        return dm.decideDisappeared(ctx, change)
    // ... rest
    }
}

func (dm *PolicyEnforcingDecisionMaker) decideBaseline(ctx context.Context, change Change) (*types.Decision, error) {
    // For baseline, just audit - don't alert
    return &types.Decision{
        Action:     types.ActionAudit,
        ResourceID: change.ResourceID,
        Reason:     "Baseline observation recorded",
        Metadata: map[string]any{
            "change_type": string(change.Type),
            "is_baseline": true,
        },
    }, nil
}
```

### 4. Store Observations in MVCC

Baseline changes still get stored - this IS the historical starting point:

```go
// After detecting changes (including baseline)
for _, change := range changes {
    if change.Current != nil {
        // Store in MVCC - whether baseline or appeared
        err := storage.StoreObservation(ctx, *change.Current)
        if err != nil {
            return err
        }
    }
}
```

That's it. Simple.

## What This Achieves

1. **First scan** → All resources marked as `ChangeBaseline` → Decision: `ActionAudit` (silent)
2. **Second scan** → Now we have MVCC history → Normal change detection works
3. **Historical foundation** → All baseline observations stored, ready for temporal queries

## Out of Scope (Nice to Have Later)

The fancy stuff we're NOT doing now (but could add later):

- Resource age detection (using CreatedAt to distinguish old vs new)
- Health scoring (tagging %, compliance %, etc)
- Detailed issue detection (orphans, security, cost)
- Pretty formatted reports
- Actionable recommendations

We can add those incrementally. For now, just baseline detection.

## Implementation Checklist

### Core Baseline (30 min)
1. [ ] Add `ChangeBaseline` constant to reconciler/change_detector.go
2. [ ] Update `DetectChanges()` to check if `len(previousIDs) == 0`
3. [ ] If first scan, mark all as `ChangeBaseline` instead of `ChangeAppeared`
4. [ ] Add `ChangeBaseline` case to PolicyEnforcingDecisionMaker switch
5. [ ] Implement `decideBaseline()` → return `ActionAudit`

### Simple Summary (30 min)
6. [ ] Add `BaselineSummary` struct (Total, ByType, ByEnvironment, Untagged, NoOwner)
7. [ ] Implement `summarizeBaseline()` - just counting and checking tags
8. [ ] Print summary to console after first scan
9. [ ] Show: what you have, potential issues, quick wins

### Tests (30 min)
10. [ ] Test: Empty MVCC → scan → verify all marked baseline
11. [ ] Test: Second scan → verify normal change detection works
12. [ ] Test: Summary counts resources correctly
13. [ ] Test: Summary flags untagged/no-owner correctly

**Total: ~1.5 hours for baseline + useful summary**

---

## Appendix: Fancy Features (Future)

<details>
<summary>Click to see future enhancements we're skipping for now</summary>

```go
// Future: Health reporting
type BaselineReport struct {
    // Overview
    Summary BaselineSummary

    // Health Indicators
    Health BaselineHealth

    // Actionable Issues
    Issues []BaselineIssue

    // Recommendations
    Recommendations []string
}

type BaselineSummary struct {
    TotalResources    int
    PreExisting       int  // Age > 24h
    RecentlyCreated   int  // Age < 24h
    Unmanaged         int
    OldestResource    time.Time
    NewestResource    time.Time
    ByProvider        map[string]int
    ByType            map[string]int
    ByEnvironment     map[string]int
}

type BaselineHealth struct {
    // Tagging health (percentage of resources with proper tags)
    TaggingScore      float64  // 0.0 - 1.0
    UntaggedCount     int
    PartiallyTagged   int
    FullyTagged       int

    // Management health
    ManagedCount      int
    UnmanagedCount    int
    OrphanCount       int      // No owner/team tags

    // Policy compliance (from OPA evaluation)
    CompliantCount    int
    ViolationCount    int
    ViolationSeverity map[string]int  // "high": 5, "medium": 12, "low": 3

    // Resource age distribution
    Ancient           int  // > 1 year
    Old               int  // > 6 months
    Mature            int  // > 1 month
    Recent            int  // < 1 month

    // Cost indicators (if available)
    IdleResources     int  // Stopped instances, unused volumes
    OversizedResources int // Based on utilization
}

type BaselineIssue struct {
    Severity    string   // "critical", "high", "medium", "low"
    Category    string   // "tagging", "security", "cost", "compliance"
    Title       string
    Description string
    Count       int      // How many resources affected
    Examples    []string // Resource IDs (max 5)
    Remediation string   // How to fix
}
```

### 6. Baseline Analysis Logic

Analyze baseline to detect common issues:

```go
func (r *Reconciler) analyzeBaseline(resources []types.Resource) BaselineReport {
    report := BaselineReport{
        Summary: r.calculateSummary(resources),
        Health:  r.calculateHealth(resources),
    }

    // Detect issues
    report.Issues = append(report.Issues, r.detectTaggingIssues(resources)...)
    report.Issues = append(report.Issues, r.detectOrphans(resources)...)
    report.Issues = append(report.Issues, r.detectSecurityIssues(resources)...)
    report.Issues = append(report.Issues, r.detectCostIssues(resources)...)
    report.Issues = append(report.Issues, r.detectComplianceIssues(resources)...)

    // Generate recommendations
    report.Recommendations = r.generateRecommendations(report)

    return report
}

func (r *Reconciler) detectTaggingIssues(resources []types.Resource) []BaselineIssue {
    var issues []BaselineIssue
    var untagged []string
    var missingOwner []string

    for _, res := range resources {
        if len(res.Tags.ToMap()) == 0 {
            untagged = append(untagged, res.ID)
        } else if res.Tags.ElavaOwner == "" && res.Tags.Team == "" {
            missingOwner = append(missingOwner, res.ID)
        }
    }

    if len(untagged) > 0 {
        issues = append(issues, BaselineIssue{
            Severity:    "medium",
            Category:    "tagging",
            Title:       "Untagged Resources Detected",
            Description: "Resources without any tags make cost tracking and ownership difficult",
            Count:       len(untagged),
            Examples:    untagged[:min(5, len(untagged))],
            Remediation: "Add tags: elava:owner, elava:environment, elava:team",
        })
    }

    if len(missingOwner) > 0 {
        issues = append(issues, BaselineIssue{
            Severity:    "high",
            Category:    "tagging",
            Title:       "Orphaned Resources (No Owner)",
            Description: "Resources without owner or team tags are difficult to manage",
            Count:       len(missingOwner),
            Examples:    missingOwner[:min(5, len(missingOwner))],
            Remediation: "Add elava:owner or team tag to establish ownership",
        })
    }

    return issues
}

func (r *Reconciler) detectCostIssues(resources []types.Resource) []BaselineIssue {
    var issues []BaselineIssue
    var stopped []string
    var ancient []string

    for _, res := range resources {
        // Stopped instances still costing money
        if res.Type == "ec2" && res.Status == "stopped" {
            stopped = append(stopped, res.ID)
        }

        // Very old resources might be forgotten
        age := time.Since(res.CreatedAt)
        if age > 2*365*24*time.Hour { // > 2 years
            ancient = append(ancient, res.ID)
        }
    }

    if len(stopped) > 0 {
        issues = append(issues, BaselineIssue{
            Severity:    "low",
            Category:    "cost",
            Title:       "Stopped EC2 Instances",
            Description: "Stopped instances still incur EBS volume costs",
            Count:       len(stopped),
            Examples:    stopped[:min(5, len(stopped))],
            Remediation: "Terminate if no longer needed, or document reason for keeping",
        })
    }

    if len(ancient) > 0 {
        issues = append(issues, BaselineIssue{
            Severity:    "low",
            Category:    "cost",
            Title:       "Ancient Resources (>2 years old)",
            Description: "Very old resources might be forgotten or unused",
            Count:       len(ancient),
            Examples:    ancient[:min(5, len(ancient))],
            Remediation: "Review if still needed, consider modernization",
        })
    }

    return issues
}

func (r *Reconciler) generateRecommendations(report BaselineReport) []string {
    var recs []string

    // Tagging recommendations
    if report.Health.TaggingScore < 0.5 {
        recs = append(recs, "🏷️  Improve tagging: Only %.0f%% of resources are properly tagged. Consider running 'elava tag --baseline' to auto-tag resources",
            report.Health.TaggingScore*100)
    }

    // Orphan recommendations
    if report.Health.OrphanCount > 0 {
        recs = append(recs, "👤 Establish ownership: %d resources lack owner/team tags. This makes accountability difficult",
            report.Health.OrphanCount)
    }

    // Policy recommendations
    if report.Health.ViolationCount > 0 {
        if report.Health.ViolationSeverity["critical"] > 0 {
            recs = append(recs, "🚨 CRITICAL: %d resources have critical policy violations. Address immediately",
                report.Health.ViolationSeverity["critical"])
        }
        recs = append(recs, "📋 Policy compliance: %d/%d resources violate policies. Run 'elava policy --fix' to auto-remediate",
            report.Health.ViolationCount, report.Summary.TotalResources)
    }

    // Management recommendations
    if report.Health.UnmanagedCount > report.Summary.TotalResources/2 {
        recs = append(recs, "⚙️  Consider adopting more resources: %d/%d are unmanaged. Tag with elava:managed=true to enable drift detection",
            report.Health.UnmanagedCount, report.Summary.TotalResources)
    }

    return recs
}
```

### 7. User-Friendly Output

Generate human-readable baseline report:

```
╔══════════════════════════════════════════════════════════════╗
║           ELAVA BASELINE SCAN COMPLETE                       ║
╚══════════════════════════════════════════════════════════════╝

📊 INFRASTRUCTURE SUMMARY
────────────────────────────────────────────────────────────────
Total Resources:        247
  ├─ Pre-existing:      231 (93%)
  ├─ Recent (<24h):     11 (4%)
  └─ Unmanaged:         5 (2%)

Oldest Resource:        3 years, 2 months ago
Newest Resource:        2 hours ago

By Provider:
  ├─ AWS:              234
  └─ GCP:              13

By Type:
  ├─ EC2:              156
  ├─ RDS:              34
  ├─ S3:               28
  └─ Other:            29

By Environment:
  ├─ production:       89
  ├─ staging:          67
  ├─ development:      45
  └─ unknown:          46

🏥 INFRASTRUCTURE HEALTH
────────────────────────────────────────────────────────────────
Tagging:              ⚠️  62% (Needs Improvement)
  ├─ Fully tagged:     154
  ├─ Partial tags:     58
  └─ Untagged:         35

Management:           ✓  98% Managed
  ├─ Managed:          242
  ├─ Unmanaged:        5
  └─ Orphaned:         46 (no owner)

Policy Compliance:    ⚠️  78% Compliant
  ├─ Compliant:        193
  └─ Violations:       54
      ├─ Critical:     3  🚨
      ├─ High:         12 ⚠️
      └─ Medium:       39

Resource Age:
  ├─ Ancient (>1yr):   89
  ├─ Old (>6mo):       67
  ├─ Mature (>1mo):    45
  └─ Recent (<1mo):    46

Cost Indicators:
  ├─ Idle resources:   12 (stopped instances)
  └─ Ancient (>2yr):   23 (review needed)

⚠️  DETECTED ISSUES (5)
────────────────────────────────────────────────────────────────

[CRITICAL] Security Group Too Permissive
  Category:     security
  Affected:     3 resources
  Examples:     sg-abc123, sg-def456, sg-ghi789
  Issue:        Security groups allow 0.0.0.0/0 on sensitive ports
  Remediation:  Restrict ingress rules to specific IP ranges

[HIGH] Orphaned Resources
  Category:     tagging
  Affected:     46 resources
  Examples:     i-abc123, vol-def456, db-ghi789, ...
  Issue:        Resources without owner or team tags
  Remediation:  Add elava:owner or team tag to establish ownership

[MEDIUM] Untagged Resources
  Category:     tagging
  Affected:     35 resources
  Examples:     i-xyz789, vol-uvw456, s3-rst123, ...
  Issue:        Resources without any tags
  Remediation:  Add tags: elava:owner, elava:environment, elava:team

[MEDIUM] Unknown Environments
  Category:     compliance
  Affected:     46 resources
  Examples:     i-lmn456, db-opq789, s3-tuv012, ...
  Issue:        Resources without environment tag
  Remediation:  Tag with environment: production, staging, or development

[LOW] Stopped EC2 Instances
  Category:     cost
  Affected:     12 resources
  Examples:     i-stopped1, i-stopped2, i-stopped3, ...
  Issue:        Stopped instances still incur EBS volume costs
  Remediation:  Terminate if no longer needed, or document reason

💡 RECOMMENDATIONS
────────────────────────────────────────────────────────────────
🏷️  Improve tagging: Only 62% of resources are properly tagged
    → Run: elava tag --baseline --dry-run

👤 Establish ownership: 46 resources lack owner/team tags
    → Run: elava tag --owner <team-name> --filter unowned

🚨 CRITICAL: 3 resources have critical security violations
    → Run: elava policy --show-critical

📋 Policy compliance: 54/247 resources violate policies
    → Run: elava policy --fix --dry-run

💰 Cost optimization: 12 idle resources detected
    → Run: elava cost --analyze

✅ NEXT STEPS
────────────────────────────────────────────────────────────────
1. Review critical security issues (3 resources)
2. Establish ownership for orphaned resources (46 resources)
3. Improve tagging coverage to >80%
4. Enable drift detection on all managed resources
5. Schedule next scan: elava scan (recommended: hourly)

Baseline established successfully!
MVCC storage now contains 247 resource observations.
Future scans will detect drift, changes, and anomalies.

// ... (all the fancy stuff collapsed in <details>)
```

</details>

---

## First Scan Simple Summary (Easy Win!)

After baseline scan, show users **quick observations** - just counting:

```go
type BaselineSummary struct {
    Total         int
    ByType        map[string]int    // "ec2": 45, "rds": 12
    ByEnvironment map[string]int    // "prod": 30, "staging": 15, "unknown": 10
    Untagged      []string          // Resources with NO tags at all
    NoOwner       []string          // Resources missing owner/team tags
    OldestResource time.Time        // When the oldest resource was created
    NewestResource time.Time        // When the newest resource was created
}

func summarizeBaseline(resources []types.Resource) BaselineSummary {
    summary := BaselineSummary{
        Total:         len(resources),
        ByType:        make(map[string]int),
        ByEnvironment: make(map[string]int),
    }

    for _, r := range resources {
        // Count by type
        summary.ByType[r.Type]++

        // Count by environment (or "unknown")
        env := r.Tags.Environment
        if env == "" {
            env = "unknown"
            summary.ByEnvironment["unknown"]++
        } else {
            summary.ByEnvironment[env]++
        }

        // Flag completely untagged
        if len(r.Tags.ToMap()) == 0 {
            summary.Untagged = append(summary.Untagged, r.ID)
        }

        // Flag missing owner
        if r.Tags.ElavaOwner == "" && r.Tags.Team == "" {
            summary.NoOwner = append(summary.NoOwner, r.ID)
        }

        // Track oldest/newest
        if summary.OldestResource.IsZero() || r.CreatedAt.Before(summary.OldestResource) {
            summary.OldestResource = r.CreatedAt
        }
        if summary.NewestResource.IsZero() || r.CreatedAt.After(summary.NewestResource) {
            summary.NewestResource = r.CreatedAt
        }
    }

    return summary
}
```

**Print to console:**
```
════════════════════════════════════════════════════
  BASELINE SCAN COMPLETE - 247 resources discovered
════════════════════════════════════════════════════

INFRASTRUCTURE AGE
  Oldest resource created 3 years, 2 months ago
  Newest resource created 2 hours ago

RESOURCE BREAKDOWN
  156  EC2 Instances
   34  RDS Databases
   28  S3 Buckets
   18  VPC Networks
   11  Security Groups

ENVIRONMENT DISTRIBUTION
   89  production
   67  staging
   45  development
   46  (no environment tag)

TAGGING STATUS
   35  resources have no tags
   46  resources missing owner

────────────────────────────────────────────────────
Baseline saved. Next scan will detect changes.
════════════════════════════════════════════════════
```

## Simple Example Flow

**First Scan:**
```
MVCC storage: empty
Scan finds: 100 EC2 instances

ChangeDetector:
  len(previousIDs) == 0  → isFirstScan = true
  → Mark all 100 as ChangeBaseline

PolicyEnforcingDecisionMaker:
  ChangeBaseline → ActionAudit (silent)

Summary Generator:
  Count by type, environment
  Flag untagged, no owner
  Print simple report

Storage:
  Store all 100 observations in MVCC

Result: Baseline established + quick summary printed
```

**Second Scan (1 hour later):**
```
MVCC storage: 100 resources
Scan finds: 101 EC2 instances

ChangeDetector:
  len(previousIDs) == 100  → isFirstScan = false
  → Normal change detection
  → 100 unchanged (no Change objects)
  → 1 new → ChangeAppeared

PolicyEnforcingDecisionMaker:
  ChangeAppeared → Policy evaluation → Decision

Result: Normal Day 2 operations, drift detection active
```

## Conclusion

Simple, minimal, in scope:
1. One new constant: `ChangeBaseline`
2. One check: `len(previousIDs) == 0`
3. One new function: `decideBaseline()` returns `ActionAudit`
4. Store everything in MVCC regardless

The first scan establishes the baseline silently. The second scan and beyond can detect real changes.
