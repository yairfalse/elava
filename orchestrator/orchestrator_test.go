package orchestrator

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/yairfalse/elava/storage"
	"github.com/yairfalse/elava/types"
)

func TestOrchestrator_RunCycle(t *testing.T) {
	// Create temp storage
	tmpDir := t.TempDir()
	store, err := storage.NewMVCCStorage(tmpDir)
	require.NoError(t, err)
	defer func() { _ = store.Close() }()

	// Create orchestrator with mocks
	orch := NewOrchestrator(store)

	// Mock scanner that returns test resources
	mockScanner := &MockScanner{
		resources: []types.Resource{
			{
				ID:       "i-123",
				Type:     "ec2",
				Provider: "aws",
				Tags: types.Tags{
					ElavaOwner: "", // Orphan
				},
			},
			{
				ID:       "i-456",
				Type:     "ec2",
				Provider: "aws",
				Tags: types.Tags{
					ElavaOwner: "team-web",
				},
			},
		},
	}
	orch.scanner = mockScanner

	// Run a cycle
	ctx := context.Background()
	result, err := orch.RunCycle(ctx)
	require.NoError(t, err)

	// Verify results
	assert.Equal(t, 2, result.ResourcesScanned)
	assert.Equal(t, 2, result.PoliciesEvaluated)
	assert.Greater(t, result.Duration.Nanoseconds(), int64(0))
}

func TestOrchestrator_HandlesPolicyViolations(t *testing.T) {
	tmpDir := t.TempDir()
	store, err := storage.NewMVCCStorage(tmpDir)
	require.NoError(t, err)
	defer func() { _ = store.Close() }()

	orch := NewOrchestrator(store)

	// Load orphan policy - simplified for testing
	orphanPolicy := `package elava

import rego.v1

# For testing - flag all EC2 instances as orphans
decision := "deny" if {
	input.resource.type == "ec2"
}

action := "flag" if {
	decision == "deny"
}

reason := "Resource has no owner" if {
	decision == "deny"
}

confidence := 1.0 if {
	decision == "deny"
}

risk := "medium" if {
	decision == "deny"
}`

	err = orch.policyEngine.LoadPolicy(context.Background(), "orphan", orphanPolicy)
	require.NoError(t, err)

	// Resource that violates policy
	mockScanner := &MockScanner{
		resources: []types.Resource{
			{
				ID:       "i-orphan",
				Type:     "ec2",
				Provider: "aws",
				Tags: types.Tags{
					ElavaOwner: "", // Explicit empty owner = policy violation
				},
			},
		},
	}
	orch.scanner = mockScanner

	// Run cycle
	ctx := context.Background()
	result, err := orch.RunCycle(ctx)
	require.NoError(t, err)

	// Should have enforcement action
	assert.Equal(t, 1, result.EnforcementActions)
}

// MockScanner for testing
type MockScanner struct {
	resources []types.Resource
	err       error
}

func (m *MockScanner) Scan(ctx context.Context) ([]types.Resource, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.resources, nil
}
