package policy

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/yairfalse/elava/types"
)

func TestEngine_LoadOPAPolicy(t *testing.T) {
	// Test loading a real .rego file
	engine := NewEngine()

	// Load a simple policy
	err := engine.LoadPolicy("testdata/orphan.rego")
	assert.NoError(t, err)

	// Test evaluation with OPA
	resource := types.Resource{
		ID:   "i-456",
		Type: "ec2",
		Tags: types.Tags{}, // No owner
	}

	decision, err := engine.EvaluateWithOPA(context.Background(), resource)
	assert.NoError(t, err)
	assert.NotNil(t, decision)
	assert.Equal(t, ResultDeny, decision.Result)
}
