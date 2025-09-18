# OPA Integration Design Session - Policy-Driven Elava

## Problem
Current Elava uses hardcoded decision logic. Need flexible, policy-driven decisions that teams can customize for their environments.

## Solution
Integrate Open Policy Agent (OPA) as the decision engine, with MVCC storage providing rich historical context.

## Architecture Flow
```
┌─────────────┐    ┌─────────────┐    ┌─────────────┐    ┌─────────────┐
│   Scanner   │───▶│ MVCC Brain  │───▶│ OPA Engine  │───▶│ Reconciler  │
│             │    │             │    │             │    │             │
│ - AWS APIs  │    │ - History   │    │ - Policies  │    │ - Actions   │
│ - Resources │    │ - Context   │    │ - Rules     │    │ - Execution │
│ - State     │    │ - Metadata  │    │ - Logic     │    │ - Safety    │
└─────────────┘    └─────────────┘    └─────────────┘    └─────────────┘
```

## Core Components

### 1. Policy Engine (`policy/engine.go`)
```go
type PolicyEngine struct {
    opa     *opa.OPA
    storage *storage.MVCCStorage
    logger  *telemetry.Logger
    tracer  trace.Tracer
}

type PolicyInput struct {
    Resource     types.Resource      `json:"resource"`
    History      []types.Resource    `json:"history"`
    Context      PolicyContext       `json:"context"`
    Environment  string              `json:"environment"`
}

type PolicyResult struct {
    Decision    string              `json:"decision"`     // "allow", "deny", "require_approval"
    Reason      string              `json:"reason"`
    Confidence  float64             `json:"confidence"`
    Policies    []string            `json:"policies"`     // Which policies matched
    Metadata    map[string]any      `json:"metadata"`
}
```

### 2. Policy Types

#### Waste Detection Policies (Observable Behaviors, Not Cost Guessing)
```rego
package elava.waste

# Detect stopped instances (observable state, not cost)
decision := "flag" if {
    input.resource.type == "ec2"
    input.resource.status == "stopped"
    input.context.last_seen_days > 7
    not input.resource.tags.elava_blessed
}

# Unattached volumes are pure waste (observable, not cost)
decision := "flag" if {
    input.resource.type == "ebs"
    input.resource.status == "available"  # Not attached
    input.context.resource_age_days > 3
}

# Old snapshots without access (observable pattern)
decision := "flag" if {
    input.resource.type == "snapshot"
    input.context.resource_age_days > 90
    input.context.last_seen_days > 30  # Not accessed
}
```

#### Security Policies
```rego
package elava.security

# High-risk public resources
flag_high_risk {
    input.resource.type == "s3"
    input.resource.public_read == true
    not input.resource.tags.approved_public
}

flag_security_group_violation {
    input.resource.type == "ec2"
    some rule in input.resource.security_groups
    rule.from_port == 22
    rule.source == "0.0.0.0/0"
    input.environment == "prod"
}
```

#### Ownership Policies
```rego
package elava.ownership

# Require ownership tags
require_owner {
    input.resource.tags.elava_owner == ""
    input.resource.type in ["ec2", "rds", "elb", "s3"]
}

# Orphan detection
flag_orphan {
    input.resource.tags.elava_owner == ""
    resource_age_days > 30
    not input.resource.tags.temporary
}

resource_age_days := time.diff_days(time.now_ns(), input.resource.created_at)
```

### 3. Policy Context Builder
```go
type PolicyContext struct {
    Account         string              `json:"account"`
    Region          string              `json:"region"`
    Environment     string              `json:"environment"`
    TeamPolicies    []string            `json:"team_policies"`
    CostBudget      *CostBudget         `json:"cost_budget,omitempty"`
    ComplianceRules []ComplianceRule    `json:"compliance_rules"`
}

func (pe *PolicyEngine) BuildContext(ctx context.Context, resource types.Resource) (PolicyContext, error) {
    // Query MVCC storage for historical context
    history, err := pe.storage.GetResourceHistory(resource.ID, 30) // 30 days
    if err != nil {
        return PolicyContext{}, err
    }

    // Build rich context from storage
    return PolicyContext{
        Account:      resource.Account,
        Region:       resource.Region,
        Environment:  detectEnvironment(resource),
        TeamPolicies: getTeamPolicies(resource.Tags.ElavaOwner),
        CostBudget:   getCostBudget(resource.Tags.Team),
    }, nil
}
```

### 4. Integration with Reconciler
```go
// reconciler/decision_maker.go
func (dm *DecisionMaker) MakeDecision(ctx context.Context, resource types.Resource) (types.Decision, error) {
    ctx, span := dm.tracer.Start(ctx, "decision_maker.make_decision")
    defer span.End()

    // Build policy input from MVCC storage
    policyInput, err := dm.policyEngine.BuildPolicyInput(ctx, resource)
    if err != nil {
        return types.Decision{}, fmt.Errorf("failed to build policy input: %w", err)
    }

    // Evaluate all relevant policies
    results, err := dm.policyEngine.Evaluate(ctx, policyInput)
    if err != nil {
        return types.Decision{}, fmt.Errorf("policy evaluation failed: %w", err)
    }

    // Convert policy results to Elava decision
    decision := dm.convertPolicyToDecision(resource, results)

    // Store decision in MVCC for audit trail
    if err := dm.storage.RecordDecision(ctx, decision); err != nil {
        dm.logger.LogStorageError(ctx, "record_decision", err)
    }

    return decision, nil
}
```

## Policy Bundle Management

### 1. Policy Loading
```go
type PolicyLoader struct {
    bundlePath  string
    opa         *opa.OPA
    watcher     *fsnotify.Watcher
}

func (pl *PolicyLoader) LoadPolicies(ctx context.Context) error {
    // Load from filesystem or ConfigMap
    policies, err := pl.loadFromPath(pl.bundlePath)
    if err != nil {
        return err
    }

    // Compile and load into OPA
    for _, policy := range policies {
        if err := pl.opa.SetPolicy(policy.Name, policy.Content); err != nil {
            return fmt.Errorf("failed to load policy %s: %w", policy.Name, err)
        }
    }

    return nil
}
```

### 2. Policy Hot-Reload
```go
func (pl *PolicyLoader) WatchPolicies(ctx context.Context) error {
    for {
        select {
        case event := <-pl.watcher.Events:
            if event.Op&fsnotify.Write == fsnotify.Write {
                if err := pl.reloadPolicy(event.Name); err != nil {
                    log.Printf("Failed to reload policy %s: %v", event.Name, err)
                }
            }
        case <-ctx.Done():
            return ctx.Err()
        }
    }
}
```

## Storage Schema for Policy Data

### Policy Evaluation Results
```go
type PolicyEvaluation struct {
    ID             string              `json:"id"`
    ResourceID     string              `json:"resource_id"`
    PolicyName     string              `json:"policy_name"`
    Result         PolicyResult        `json:"result"`
    Input          PolicyInput         `json:"input"`
    EvaluatedAt    time.Time           `json:"evaluated_at"`
    Revision       int64               `json:"revision"`
}
```

### Policy Decision History
```go
func (s *MVCCStorage) RecordPolicyDecision(ctx context.Context, eval PolicyEvaluation) (int64, error) {
    s.mu.Lock()
    defer s.mu.Unlock()

    s.currentRev++
    rev := s.currentRev

    return rev, s.db.Update(func(tx *bbolt.Tx) error {
        bucket := tx.Bucket(bucketPolicyEvaluations)
        key := makePolicyKey(rev, eval.ResourceID, eval.PolicyName)

        data, err := json.Marshal(eval)
        if err != nil {
            return err
        }

        return bucket.Put(key, data)
    })
}
```

## Configuration

### Elava Config with OPA
```yaml
# elava.yaml
policy:
  enabled: true
  bundle_path: "./policies"
  hot_reload: true
  default_decision: "require_approval"

environments:
  prod:
    policies: ["security", "cost", "compliance"]
    strict_mode: true
  staging:
    policies: ["cost", "ownership"]
    strict_mode: false
  dev:
    policies: ["ownership"]
    strict_mode: false
```

## Benefits

1. **Flexible Governance** - Teams define policies in Rego
2. **Audit Trail** - All policy decisions stored with history
3. **Contextual Decisions** - Rich context from MVCC storage
4. **Hot-Reload** - Update policies without restart
5. **Environment-Specific** - Different policies per environment
6. **Compliance** - Enforce security and cost policies as code

## Next Steps

1. Implement basic `PolicyEngine` with OPA integration
2. Create sample policy bundles for common use cases
3. Add policy evaluation to reconciler decision flow
4. Implement policy decision storage in MVCC
5. Add policy management CLI commands
6. Create policy testing framework

This turns Elava into a true policy-driven infrastructure engine! 🚀