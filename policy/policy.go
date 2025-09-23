package policy

import (
	"context"
	"os"

	"github.com/open-policy-agent/opa/v1/rego"
	"github.com/yairfalse/elava/types"
)

// Engine evaluates OPA policies
type Engine struct {
	policyContent string
}

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

// LoadPolicy loads a .rego policy file
func (e *Engine) LoadPolicy(path string) error {
	content, err := os.ReadFile(path) // #nosec G304 - path is controlled
	if err != nil {
		return err
	}

	e.policyContent = string(content)
	return nil
}

// EvaluateWithOPA evaluates using OPA
func (e *Engine) EvaluateWithOPA(ctx context.Context, resource types.Resource) (*Decision, error) {
	if e.policyContent == "" {
		return e.Evaluate(ctx, resource)
	}

	input := map[string]interface{}{
		"resource": map[string]interface{}{
			"id":   resource.ID,
			"type": resource.Type,
			"tags": map[string]interface{}{
				"owner": resource.Tags.ElavaOwner,
			},
		},
	}

	results, err := rego.New(
		rego.Query("data.elava.orphan.deny"),
		rego.Module("policy.rego", e.policyContent),
		rego.Input(input),
	).Eval(ctx)
	if err != nil {
		return nil, err
	}

	if len(results) > 0 && len(results[0].Expressions) > 0 {
		if denials, ok := results[0].Expressions[0].Value.([]interface{}); ok && len(denials) > 0 {
			return &Decision{
				ResourceID: resource.ID,
				PolicyID:   "orphan",
				Result:     ResultDeny,
				Reason:     "OPA policy violation",
			}, nil
		}
	}

	return &Decision{
		ResourceID: resource.ID,
		PolicyID:   "orphan",
		Result:     ResultAllow,
		Reason:     "OPA policy passed",
	}, nil
}
