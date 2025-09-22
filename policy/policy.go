package policy

import (
	"context"

	"github.com/yairfalse/elava/types"
)

// Engine evaluates OPA policies
type Engine struct{}

// NewEngine creates a new policy engine
func NewEngine() *Engine {
	return &Engine{}
}

// Evaluate checks resource against policies
func (e *Engine) Evaluate(ctx context.Context, resource types.Resource) (*Decision, error) {
	// Simplest implementation - check for orphan
	if resource.Tags.ElavaOwner == "" {
		return &Decision{
			ResourceID: resource.ID,
			PolicyID:   "orphan",
			Result:     ResultDeny,
			Reason:     "Resource has no owner",
		}, nil
	}

	return &Decision{
		ResourceID: resource.ID,
		PolicyID:   "default",
		Result:     ResultAllow,
		Reason:     "Resource has owner",
	}, nil
}
