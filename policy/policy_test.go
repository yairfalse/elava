package policy

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/yairfalse/elava/types"
)

func TestEngine_EvaluateBasicPolicy(t *testing.T) {
	// Simplest test - orphan resource should be flagged
	resource := types.Resource{
		ID:   "i-123",
		Type: "ec2",
		Tags: types.Tags{}, // No owner = orphan
	}

	engine := NewEngine()

	decision, err := engine.Evaluate(context.Background(), resource)

	assert.NoError(t, err)
	assert.NotNil(t, decision)
	assert.Equal(t, "orphan", decision.PolicyID)
	assert.Equal(t, ResultDeny, decision.Result)
}
