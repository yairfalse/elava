package reconciler

import (
	"context"
	"fmt"

	"github.com/yairfalse/elava/policy"
	"github.com/yairfalse/elava/types"
)

// PolicyDecisionMaker makes decisions based on detected changes and policies
type PolicyDecisionMaker interface {
	Decide(ctx context.Context, changes []Change) ([]types.Decision, error)
}

// PolicyEnforcingDecisionMaker makes decisions based on detected changes and OPA policies
// This is the Day 2 operations decision maker that focuses on:
// - Protecting blessed resources
// - Enforcing policies on drift
// - Notifying about unmanaged resources
// - Alerting on unexpected changes
type PolicyEnforcingDecisionMaker struct {
	policyEngine *policy.PolicyEngine
}

// NewPolicyEnforcingDecisionMaker creates a new policy-enforcing decision maker
func NewPolicyEnforcingDecisionMaker(policyEngine *policy.PolicyEngine) *PolicyEnforcingDecisionMaker {
	return &PolicyEnforcingDecisionMaker{
		policyEngine: policyEngine,
	}
}

// Decide generates decisions from detected changes using policy evaluation
func (dm *PolicyEnforcingDecisionMaker) Decide(ctx context.Context, changes []Change) ([]types.Decision, error) {
	var decisions []types.Decision

	for _, change := range changes {
		decision, err := dm.decideFromChange(ctx, change)
		if err != nil {
			return nil, fmt.Errorf("failed to decide for change %s: %w", change.ResourceID, err)
		}

		if decision != nil {
			decisions = append(decisions, *decision)
		}
	}

	return decisions, nil
}

// decideFromChange makes a decision for a single detected change
func (dm *PolicyEnforcingDecisionMaker) decideFromChange(ctx context.Context, change Change) (*types.Decision, error) {
	switch change.Type {
	case ChangeBaseline:
		return dm.decideBaseline(ctx, change)
	case ChangeAppeared:
		return dm.decideAppeared(ctx, change)
	case ChangeDisappeared:
		return dm.decideDisappeared(ctx, change)
	case ChangeStatusChanged:
		return dm.decideStatusChanged(ctx, change)
	case ChangeTagDrift:
		return dm.decideTagDrift(ctx, change)
	case ChangeModified:
		return dm.decideModified(ctx, change)
	case ChangeUnmanaged:
		return dm.decideUnmanaged(ctx, change)
	default:
		return nil, fmt.Errorf("unknown change type: %s", change.Type)
	}
}

// decideAppeared handles newly appeared resources
func (dm *PolicyEnforcingDecisionMaker) decideAppeared(ctx context.Context, change Change) (*types.Decision, error) {
	if change.Current == nil {
		return nil, fmt.Errorf("appeared change without current state")
	}

	// Evaluate policy for new resource
	policyResult, err := dm.evaluatePolicy(ctx, *change.Current)
	if err != nil {
		// On policy error, default to notify
		return &types.Decision{
			Action:     types.ActionNotify,
			ResourceID: change.ResourceID,
			Reason:     fmt.Sprintf("New resource appeared: %s (policy evaluation failed: %v)", change.Details, err),
		}, nil
	}

	// Map policy decision to action
	return &types.Decision{
		Action:     dm.mapPolicyAction(policyResult.Action, types.ActionNotify),
		ResourceID: change.ResourceID,
		Reason:     fmt.Sprintf("New resource appeared: %s (policy: %s)", change.Details, policyResult.Reason),
		Metadata: map[string]any{
			"change_type":       string(change.Type),
			"policy_decision":   policyResult.Decision,
			"policy_risk":       policyResult.Risk,
			"policy_confidence": policyResult.Confidence,
		},
	}, nil
}

// decideDisappeared handles disappeared resources
func (dm *PolicyEnforcingDecisionMaker) decideDisappeared(ctx context.Context, change Change) (*types.Decision, error) {
	if change.Previous == nil {
		return nil, fmt.Errorf("disappeared change without previous state")
	}

	// Check if resource was blessed (protected)
	if change.Previous.IsBlessed() {
		return &types.Decision{
			Action:     types.ActionAlert,
			ResourceID: change.ResourceID,
			Reason:     "ALERT: Blessed (protected) resource has disappeared unexpectedly!",
			Metadata: map[string]any{
				"change_type": string(change.Type),
				"severity":    "high",
				"blessed":     true,
			},
		}, nil
	}

	// For non-blessed resources, notify about disappearance
	return &types.Decision{
		Action:     types.ActionNotify,
		ResourceID: change.ResourceID,
		Reason:     fmt.Sprintf("Resource disappeared: %s", change.Details),
		Metadata: map[string]any{
			"change_type": string(change.Type),
		},
	}, nil
}

// decideStatusChanged handles status changes
func (dm *PolicyEnforcingDecisionMaker) decideStatusChanged(ctx context.Context, change Change) (*types.Decision, error) {
	if change.Current == nil {
		return nil, fmt.Errorf("status change without current state")
	}

	prevStatus := ""
	currStatus := ""
	if v, ok := change.Metadata["previous_status"].(string); ok {
		prevStatus = v
	}
	if v, ok := change.Metadata["current_status"].(string); ok {
		currStatus = v
	}

	// Evaluate policy
	policyResult, err := dm.evaluatePolicy(ctx, *change.Current)
	if err != nil {
		return &types.Decision{
			Action:     types.ActionNotify,
			ResourceID: change.ResourceID,
			Reason:     fmt.Sprintf("Status changed from %s to %s", prevStatus, currStatus),
		}, nil
	}

	return &types.Decision{
		Action:     dm.mapPolicyAction(policyResult.Action, types.ActionNotify),
		ResourceID: change.ResourceID,
		Reason:     fmt.Sprintf("Status changed from %s to %s (policy: %s)", prevStatus, currStatus, policyResult.Reason),
		Metadata: map[string]any{
			"change_type":     string(change.Type),
			"previous_status": prevStatus,
			"current_status":  currStatus,
			"policy_decision": policyResult.Decision,
		},
	}, nil
}

// decideTagDrift handles tag drift
func (dm *PolicyEnforcingDecisionMaker) decideTagDrift(ctx context.Context, change Change) (*types.Decision, error) {
	if change.Current == nil {
		return nil, fmt.Errorf("tag drift without current state")
	}

	// Tag drift is important - evaluate policy
	policyResult, err := dm.evaluatePolicy(ctx, *change.Current)
	if err != nil {
		return &types.Decision{
			Action:     types.ActionNotify,
			ResourceID: change.ResourceID,
			Reason:     "Tag drift detected: " + change.Details,
		}, nil
	}

	// Check if we should auto-fix tags based on policy
	action := dm.mapPolicyAction(policyResult.Action, types.ActionNotify)
	if policyResult.Decision == "deny" && policyResult.Action == "tag" {
		action = types.ActionEnforceTags
	}

	return &types.Decision{
		Action:     action,
		ResourceID: change.ResourceID,
		Reason:     fmt.Sprintf("Tag drift detected (policy: %s): %s", policyResult.Reason, change.Details),
		Metadata: map[string]any{
			"change_type":     string(change.Type),
			"policy_decision": policyResult.Decision,
			"policy_action":   policyResult.Action,
			"previous_tags":   change.Metadata["previous_tags"],
			"current_tags":    change.Metadata["current_tags"],
		},
	}, nil
}

// decideModified handles configuration modifications
func (dm *PolicyEnforcingDecisionMaker) decideModified(ctx context.Context, change Change) (*types.Decision, error) {
	if change.Current == nil {
		return nil, fmt.Errorf("modified change without current state")
	}

	// Evaluate policy
	policyResult, err := dm.evaluatePolicy(ctx, *change.Current)
	if err != nil {
		return &types.Decision{
			Action:     types.ActionNotify,
			ResourceID: change.ResourceID,
			Reason:     "Resource configuration changed: " + change.Details,
		}, nil
	}

	return &types.Decision{
		Action:     dm.mapPolicyAction(policyResult.Action, types.ActionNotify),
		ResourceID: change.ResourceID,
		Reason:     fmt.Sprintf("Configuration changed (policy: %s): %s", policyResult.Reason, change.Details),
		Metadata: map[string]any{
			"change_type":     string(change.Type),
			"policy_decision": policyResult.Decision,
		},
	}, nil
}

// decideUnmanaged handles unmanaged resources
func (dm *PolicyEnforcingDecisionMaker) decideUnmanaged(ctx context.Context, change Change) (*types.Decision, error) {
	if change.Current == nil {
		return nil, fmt.Errorf("unmanaged change without current state")
	}

	// For unmanaged resources, notify but don't take action
	return &types.Decision{
		Action:     types.ActionNotify,
		ResourceID: change.ResourceID,
		Reason:     "Unmanaged resource detected: " + change.Details,
		Metadata: map[string]any{
			"change_type": string(change.Type),
			"managed":     false,
		},
	}, nil
}

// decideBaseline handles baseline observations (first scan)
func (dm *PolicyEnforcingDecisionMaker) decideBaseline(ctx context.Context, change Change) (*types.Decision, error) {
	if change.Current == nil {
		return nil, fmt.Errorf("baseline change without current state")
	}

	// For baseline, just audit - don't alert or notify
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

// evaluatePolicy evaluates OPA policy for a resource
func (dm *PolicyEnforcingDecisionMaker) evaluatePolicy(ctx context.Context, resource types.Resource) (policy.PolicyResult, error) {
	if dm.policyEngine == nil {
		// No policy engine configured - allow with default
		return policy.PolicyResult{
			Decision:   "allow",
			Action:     types.ActionNotify,
			Risk:       "low",
			Confidence: 0.5,
			Reason:     "no policy engine configured",
		}, nil
	}

	// Build policy input
	input, err := dm.policyEngine.BuildPolicyInput(ctx, resource)
	if err != nil {
		return policy.PolicyResult{}, fmt.Errorf("failed to build policy input: %w", err)
	}

	// Evaluate
	return dm.policyEngine.Evaluate(ctx, input)
}

// mapPolicyAction maps OPA policy actions to Day 2 decision actions
func (dm *PolicyEnforcingDecisionMaker) mapPolicyAction(policyAction, defaultAction string) string {
	// Map policy actions to reconciler actions
	mapping := map[string]string{
		"delete":  types.ActionNotify, // Day 2: never auto-delete, only notify
		"tag":     types.ActionAutoTag,
		"notify":  types.ActionNotify,
		"ignore":  types.ActionIgnore,
		"protect": types.ActionProtect,
		"alert":   types.ActionAlert,
	}

	if action, ok := mapping[policyAction]; ok {
		return action
	}

	return defaultAction
}
