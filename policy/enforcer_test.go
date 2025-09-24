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

func TestEnforcer_ExecuteFlagWithProvider(t *testing.T) {
	// Mock provider that tracks tag calls
	mockProvider := &MockProvider{
		taggedResources: make(map[string]map[string]string),
	}

	enforcer := NewEnforcerWithProvider(mockProvider)

	decision := PolicyResult{
		Decision: "deny",
		Action:   "flag",
		Reason:   "Resource violates policy",
	}

	resource := types.Resource{
		ID:       "i-789",
		Type:     "ec2",
		Provider: "aws",
	}

	err := enforcer.Execute(context.Background(), decision, resource)
	require.NoError(t, err)

	// Verify tags were applied
	tags, exists := mockProvider.taggedResources["i-789"]
	assert.True(t, exists)
	assert.Equal(t, "deny", tags["elava:policy-flag"])
	assert.Equal(t, "Resource violates policy", tags["elava:policy-reason"])
}

// MockProvider for testing
type MockProvider struct {
	taggedResources map[string]map[string]string
}

func (m *MockProvider) TagResource(ctx context.Context, id string, tags map[string]string) error {
	m.taggedResources[id] = tags
	return nil
}

func (m *MockProvider) ListResources(ctx context.Context, filter types.ResourceFilter) ([]types.Resource, error) {
	return nil, nil
}

func (m *MockProvider) CreateResource(ctx context.Context, spec types.ResourceSpec) (*types.Resource, error) {
	return nil, nil
}

func (m *MockProvider) DeleteResource(ctx context.Context, id string) error {
	return nil
}

func (m *MockProvider) Name() string {
	return "mock"
}

func (m *MockProvider) Region() string {
	return "us-mock-1"
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
