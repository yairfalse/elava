# Design Session: Core Types

## What problem are we solving?
Need to represent cloud resources in a provider-agnostic way that allows Ovi to reconcile desired state with actual state.

## What's the simplest solution?
- Resource: Represents any cloud resource (EC2, RDS, S3)
- Decision: Represents what action to take on a resource
- State: Represents desired vs actual comparison

## Can we break it into smaller functions?
Yes:
- Resource validation (max 30 lines)
- Resource matching against filters (max 30 lines)
- Tag checking for Ovi management (max 30 lines)

## What interfaces do we need?
```go
// Resource methods
- IsManaged() bool - Check if Ovi manages this
- IsBlessed() bool - Check if protected
- Matches(filter) bool - Check if matches filter

// Decision methods
- Execute() error - Perform the action
- Validate() error - Ensure decision is valid
```

## What can go wrong?
- Missing required fields (ID, Type, Provider)
- Invalid resource types
- Nil maps causing panics
- Tag conflicts

## Flow diagram
```
Cloud API → Resource struct → Reconciler
                                   ↓
                              Decision struct
                                   ↓
                              Execute action
```

## Smallest testable units:
1. Resource.IsManaged() - Check tag presence
2. Resource.IsBlessed() - Check blessed tag
3. Resource.Matches() - Filter matching logic
4. Decision.Validate() - Ensure valid action