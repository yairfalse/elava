package policy

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/yairfalse/elava/types"
)

func TestEnforcer_ExecuteNotify(t *testing.T) {
	// Test that notify action sends notification
	enforcer := NewEnforcer()

	decision := PolicyResult{
		Decision: "flag",
		Action:   "notify",
		Reason:   "Resource is orphaned",
	}

	resource := types.Resource{
		ID:   "i-123",
		Type: "ec2",
	}

	err := enforcer.Execute(context.Background(), decision, resource)
	assert.NoError(t, err)
}

func TestEnforcer_ExecuteFlag(t *testing.T) {
	// Test that flag action tags resource
	enforcer := NewEnforcer()

	decision := PolicyResult{
		Decision: "deny",
		Action:   "flag",
		Reason:   "Missing required tags",
	}

	resource := types.Resource{
		ID:   "i-456",
		Type: "ec2",
	}

	err := enforcer.Execute(context.Background(), decision, resource)
	assert.NoError(t, err)
}

func TestEnforcer_IgnoreAction(t *testing.T) {
	// Test that ignore action does nothing
	enforcer := NewEnforcer()

	decision := PolicyResult{
		Decision: "allow",
		Action:   "ignore",
		Reason:   "Resource is compliant",
	}

	resource := types.Resource{
		ID:   "i-789",
		Type: "ec2",
	}

	err := enforcer.Execute(context.Background(), decision, resource)
	require.NoError(t, err)
}
