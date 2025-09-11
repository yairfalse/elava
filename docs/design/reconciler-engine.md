# Reconciler Engine Design

## Overview

The Reconciler Engine is the heart of Ovi that orchestrates the complete reconciliation process: observing current state, comparing with desired configuration, making intelligent decisions, and coordinating execution.

## Core Components

```go
type Engine struct {
    observer      Observer      // Polls cloud providers
    comparator    Comparator    // Finds state differences
    decisionMaker DecisionMaker // Generates safe actions
    coordinator   Coordinator   // Prevents conflicts
    storage       MVCCStorage   // Persistent observations
    wal           WAL           // Audit logging
    instanceID    string        // Unique instance identifier
    options       ReconcilerOptions
}
```

## Reconciliation Flow

### 1. Observation Phase
```
Cloud Provider → Observer → MVCC Storage → WAL
```

**Purpose**: Capture current reality from cloud providers

```go
func (e *Engine) observeCurrentState(ctx context.Context, config Config) ([]Resource, error) {
    filter := ResourceFilter{
        Provider: config.Provider,
        Region:   config.Region,
    }
    
    // Poll cloud provider
    resources, err := e.observer.Observe(ctx, filter)
    if err != nil {
        return nil, fmt.Errorf("observation failed: %w", err)
    }
    
    // Log what we observed
    observationData := map[string]interface{}{
        "provider":        config.Provider,
        "region":          config.Region,
        "resources_found": len(resources),
    }
    
    e.wal.Append(EntryObserved, "", observationData)
    
    return resources, nil
}
```

### 2. Storage Phase
```
Observed Resources → MVCC Storage (with revision)
```

**Purpose**: Persistently store observations with revision tracking

```go
// Store observations atomically
_, err = e.storage.RecordObservationBatch(current)
if err != nil {
    return nil, fmt.Errorf("failed to store observations: %w", err)
}
```

### 3. Comparison Phase
```
Current State + Desired Config → Comparator → Diff List
```

**Purpose**: Identify what needs to change

```go
// Build desired state from config
desired := e.buildDesiredState(config)

// Find differences
diffs, err := e.comparator.Compare(current, desired)
if err != nil {
    return nil, fmt.Errorf("failed to compare states: %w", err)
}
```

### 4. Decision Phase
```
Diff List → Decision Maker → Decision List → WAL
```

**Purpose**: Generate safe, intelligent actions

```go
// Make decisions based on diffs
decisions, err := e.decisionMaker.Decide(diffs)
if err != nil {
    return nil, fmt.Errorf("failed to make decisions: %w", err)
}

// Log all decisions for audit
for _, decision := range decisions {
    e.wal.Append(EntryDecided, decision.ResourceID, decision)
}
```

## State Comparison Logic

### Diff Types

The comparator identifies four types of differences:

```go
type DiffType string

const (
    DiffMissing    DiffType = "missing"    // Should exist but doesn't
    DiffUnwanted   DiffType = "unwanted"   // Exists but shouldn't (Ovi-managed)
    DiffDrifted    DiffType = "drifted"    // Exists but wrong configuration
    DiffUnmanaged  DiffType = "unmanaged"  // Exists but not Ovi-managed
)
```

### Comparison Algorithm

```go
func (c *SimpleComparator) Compare(current, desired []Resource) ([]Diff, error) {
    currentMap := buildResourceMap(current)
    desiredMap := buildResourceMap(desired)
    
    var diffs []Diff
    
    // Find missing resources (desired but not current)
    for id, desiredResource := range desiredMap {
        if _, exists := currentMap[id]; !exists {
            diffs = append(diffs, Diff{
                Type:       DiffMissing,
                ResourceID: id,
                Desired:    &desiredResource,
                Reason:     "Resource specified in config but not found in cloud",
            })
        }
    }
    
    // Find unwanted resources (current but not desired)
    for id, currentResource := range currentMap {
        if _, exists := desiredMap[id]; !exists {
            if currentResource.IsManaged() {
                diffs = append(diffs, Diff{
                    Type:       DiffUnwanted,
                    ResourceID: id,
                    Current:    &currentResource,
                    Reason:     "Resource managed by Ovi but not in current config",
                })
            } else {
                diffs = append(diffs, Diff{
                    Type:       DiffUnmanaged,
                    ResourceID: id,
                    Current:    &currentResource,
                    Reason:     "Resource exists but not managed by Ovi",
                })
            }
        }
    }
    
    // Find drifted resources (exist in both but differ)
    for id, desiredResource := range desiredMap {
        if currentResource, exists := currentMap[id]; exists {
            if isDrifted(currentResource, desiredResource) {
                diffs = append(diffs, Diff{
                    Type:       DiffDrifted,
                    ResourceID: id,
                    Current:    &currentResource,
                    Desired:    &desiredResource,
                    Reason:     "Resource configuration differs from desired state",
                })
            }
        }
    }
    
    return diffs, nil
}
```

### Drift Detection

```go
func isDrifted(current, desired Resource) bool {
    // Compare basic fields
    if current.Type != desired.Type ||
       current.Provider != desired.Provider ||
       current.Region != desired.Region {
        return true
    }
    
    // Compare tags
    return isTagsDrifted(current.Tags, desired.Tags)
}

func isTagsDrifted(current, desired Tags) bool {
    return current.OviOwner != desired.OviOwner ||
           current.OviManaged != desired.OviManaged ||
           current.Environment != desired.Environment ||
           current.Team != desired.Team ||
           current.Project != desired.Project
}
```

## Decision Making Logic

### Decision Rules

The decision maker follows these rules:

1. **Missing resources** → `create`
2. **Unwanted managed resources** → `delete` (unless blessed or skip-destructive)
3. **Drifted resources** → `update`
4. **Unmanaged resources** → `notify` (don't touch)
5. **Blessed resources** → `notify` (protected)

### Safety Mechanisms

#### Blessed Resource Protection
```go
func (dm *SimpleDecisionMaker) decideUnwanted(diff Diff) (*Decision, error) {
    // Check if resource is blessed (protected)
    if diff.Current.IsBlessed() {
        return &Decision{
            Action:     "notify",
            ResourceID: diff.ResourceID,
            Reason:     "Resource is blessed and cannot be deleted automatically",
        }, nil
    }
    
    // Skip destructive actions if configured
    if dm.skipDestructive {
        return &Decision{
            Action:     "notify",
            ResourceID: diff.ResourceID,
            Reason:     "Skipping destructive action due to configuration",
        }, nil
    }
    
    return &Decision{
        Action:     "delete",
        ResourceID: diff.ResourceID,
        Reason:     diff.Reason,
    }, nil
}
```

#### Configuration Options
```go
type ReconcilerOptions struct {
    DryRun          bool          // Preview without executing
    MaxConcurrency  int           // Parallel execution limit
    ClaimTTL        time.Duration // Resource claim duration
    SkipDestructive bool          // Avoid delete operations
}
```

## Desired State Generation

### From Configuration
```go
func (e *Engine) buildDesiredState(config Config) []Resource {
    var desired []Resource
    
    for i, spec := range config.Resources {
        for j := 0; j < max(spec.Count, 1); j++ {
            resource := Resource{
                ID:       fmt.Sprintf("%s-%d-%d", spec.Type, i, j),
                Type:     spec.Type,
                Provider: config.Provider,
                Region:   spec.Region,
                Tags:     spec.Tags,
            }
            
            // Mark as Ovi-managed
            resource.Tags.OviManaged = true
            if resource.Tags.OviOwner == "" {
                resource.Tags.OviOwner = "ovi"
            }
            
            desired = append(desired, resource)
        }
    }
    
    return desired
}
```

### Configuration Format
```yaml
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
      project: web-app
  - type: rds
    count: 1
    size: db.t3.micro
    tags:
      environment: prod
      team: platform
      project: web-app
```

## Coordination and Conflict Prevention

### Resource Claims
```go
type Claim struct {
    ResourceID string    `json:"resource_id"`
    InstanceID string    `json:"instance_id"`
    ClaimedAt  time.Time `json:"claimed_at"`
    ExpiresAt  time.Time `json:"expires_at"`
}
```

### Claim Lifecycle
```go
// Before acting on resources
resourceIDs := extractResourceIDs(decisions)
err := e.coordinator.ClaimResources(ctx, resourceIDs, e.options.ClaimTTL)
if err != nil {
    return nil, fmt.Errorf("failed to claim resources: %w", err)
}

// Execute decisions (in separate component)

// Release claims when done
defer e.coordinator.ReleaseResources(ctx, resourceIDs)
```

## Error Handling

### Graceful Degradation
```go
func (e *Engine) Reconcile(ctx context.Context, config Config) ([]Decision, error) {
    // Continue processing even if individual steps fail
    
    current, err := e.observeCurrentState(ctx, config)
    if err != nil {
        // Log error but could continue with cached state
        e.wal.AppendError(EntryFailed, "", "observation", err)
        return nil, fmt.Errorf("failed to observe current state: %w", err)
    }
    
    // Each phase has its own error handling
    // Failures are logged to WAL for debugging
}
```

### Recovery Scenarios
1. **Observation failure**: Use cached state or fail-safe
2. **Storage failure**: Cannot proceed, log and exit
3. **Comparison failure**: Likely config error, log and skip
4. **Decision failure**: Log individual failures, continue with others

## Performance Optimizations

### Parallel Processing
```go
// Future enhancement: parallel resource processing
func (e *Engine) processResourcesConcurrently(resources []Resource) {
    semaphore := make(chan struct{}, e.options.MaxConcurrency)
    
    for _, resource := range resources {
        go func(r Resource) {
            semaphore <- struct{}{}
            defer func() { <-semaphore }()
            
            // Process individual resource
            e.processResource(r)
        }(resource)
    }
}
```

### Caching
```go
// Cache provider responses to avoid redundant API calls
type CachedObserver struct {
    observer Observer
    cache    map[string][]Resource
    ttl      time.Duration
}
```

## Testing Strategy

### Component Testing
Each component is tested in isolation:

```go
func TestEngine_Reconcile(t *testing.T) {
    // Mock all dependencies
    observer := &MockObserver{resources: []Resource{...}}
    comparator := &MockComparator{diffs: []Diff{...}}
    decisionMaker := &MockDecisionMaker{decisions: []Decision{...}}
    
    engine := NewEngine(observer, comparator, decisionMaker, ...)
    
    decisions, err := engine.Reconcile(ctx, config)
    
    // Verify behavior
    assert.NoError(t, err)
    assert.Len(t, decisions, expectedCount)
}
```

### Integration Testing
```go
func TestFullReconciliation(t *testing.T) {
    // Real storage, real WAL, mock cloud provider
    storage := createTestStorage(t)
    wal := createTestWAL(t)
    mockProvider := &MockProvider{...}
    
    // Run full reconciliation cycle
    // Verify end-to-end behavior
}
```

## Monitoring and Observability

### Key Metrics
- **Reconciliation duration**: Time per reconcile cycle
- **Resources processed**: Count per cycle
- **Decisions made**: By type (create/update/delete/notify)
- **Success rate**: Decisions executed successfully
- **Error rate**: Failed operations per minute

### Structured Logging
```go
type ReconcileResult struct {
    Timestamp       time.Time     `json:"timestamp"`
    ResourcesFound  int           `json:"resources_found"`
    DiffsDetected   int           `json:"diffs_detected"`
    DecisionsMade   int           `json:"decisions_made"`
    ExecutionErrors []string      `json:"execution_errors,omitempty"`
    Duration        time.Duration `json:"duration"`
    Decisions       []Decision    `json:"decisions"`
}
```

## Extension Points

### Custom Comparators
```go
type AdvancedComparator struct {
    // Custom comparison logic
    configRules []ComparisonRule
}

func (c *AdvancedComparator) Compare(current, desired []Resource) ([]Diff, error) {
    // Implement custom comparison logic
    // Example: ignore certain tag differences
    // Example: custom drift detection rules
}
```

### Custom Decision Makers
```go
type PolicyBasedDecisionMaker struct {
    policies []Policy
}

func (dm *PolicyBasedDecisionMaker) Decide(diffs []Diff) ([]Decision, error) {
    // Apply policies to generate decisions
    // Example: require approval for expensive resources
    // Example: schedule destructive actions for maintenance windows
}
```

---

The Reconciler Engine provides a flexible, safe, and auditable foundation for infrastructure reconciliation. Its modular design allows for customization while maintaining strong safety guarantees and complete operational visibility.