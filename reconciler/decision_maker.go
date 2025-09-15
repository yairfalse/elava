package reconciler

import (
	"fmt"

	"github.com/yairfalse/elava/types"
)

// SimpleDecisionMaker implements basic decision-making logic
type SimpleDecisionMaker struct {
	skipDestructive bool
}

// NewSimpleDecisionMaker creates a new simple decision maker
func NewSimpleDecisionMaker(skipDestructive bool) *SimpleDecisionMaker {
	return &SimpleDecisionMaker{
		skipDestructive: skipDestructive,
	}
}

// Decide generates decisions based on state differences
func (dm *SimpleDecisionMaker) Decide(diffs []Diff) ([]types.Decision, error) {
	var decisions []types.Decision

	for _, diff := range diffs {
		decision, err := dm.decideSingle(diff)
		if err != nil {
			return nil, fmt.Errorf("failed to decide for diff %s: %w", diff.ResourceID, err)
		}

		if decision != nil {
			decisions = append(decisions, *decision)
		}
	}

	return decisions, nil
}

// decideSingle makes a decision for a single diff
func (dm *SimpleDecisionMaker) decideSingle(diff Diff) (*types.Decision, error) {
	switch diff.Type {
	case DiffMissing:
		return dm.decideMissing(diff)
	case DiffUnwanted:
		return dm.decideUnwanted(diff)
	case DiffDrifted:
		return dm.decideDrifted(diff)
	case DiffUnmanaged:
		return dm.decideUnmanaged(diff)
	default:
		return nil, fmt.Errorf("unknown diff type: %s", diff.Type)
	}
}

// decideMissing handles missing resources
func (dm *SimpleDecisionMaker) decideMissing(diff Diff) (*types.Decision, error) {
	if diff.Desired == nil {
		return nil, fmt.Errorf("missing diff without desired state")
	}

	return &types.Decision{
		Action:     "create",
		ResourceID: diff.ResourceID,
		Reason:     diff.Reason,
	}, nil
}

// decideUnwanted handles unwanted resources
func (dm *SimpleDecisionMaker) decideUnwanted(diff Diff) (*types.Decision, error) {
	if diff.Current == nil {
		return nil, fmt.Errorf("unwanted diff without current state")
	}

	// Check if resource is blessed (protected)
	if diff.Current.IsBlessed() {
		return &types.Decision{
			Action:     "notify",
			ResourceID: diff.ResourceID,
			Reason:     "Resource is blessed and cannot be deleted automatically",
		}, nil
	}

	// Skip destructive actions if configured
	if dm.skipDestructive {
		return &types.Decision{
			Action:     "notify",
			ResourceID: diff.ResourceID,
			Reason:     "Skipping destructive action due to configuration",
		}, nil
	}

	return &types.Decision{
		Action:     "delete",
		ResourceID: diff.ResourceID,
		Reason:     diff.Reason,
	}, nil
}

// decideDrifted handles configuration drift
func (dm *SimpleDecisionMaker) decideDrifted(diff Diff) (*types.Decision, error) {
	if diff.Current == nil || diff.Desired == nil {
		return nil, fmt.Errorf("drifted diff without both current and desired state")
	}

	return &types.Decision{
		Action:     "update",
		ResourceID: diff.ResourceID,
		Reason:     diff.Reason,
	}, nil
}

// decideUnmanaged handles unmanaged resources
func (dm *SimpleDecisionMaker) decideUnmanaged(diff Diff) (*types.Decision, error) {
	if diff.Current == nil {
		return nil, fmt.Errorf("unmanaged diff without current state")
	}

	// For unmanaged resources, we just notify (don't take action)
	return &types.Decision{
		Action:     "notify",
		ResourceID: diff.ResourceID,
		Reason:     "Resource exists but is not managed by Elava",
	}, nil
}
