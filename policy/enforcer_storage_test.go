package policy

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/yairfalse/elava/storage"
	"github.com/yairfalse/elava/types"
)

func TestEnforcer_StoresEnforcementEvents(t *testing.T) {
	// Create temp storage
	tmpDir := t.TempDir()
	store, err := storage.NewMVCCStorage(tmpDir)
	require.NoError(t, err)
	defer func() { _ = store.Close() }()

	// Create enforcer with storage
	enforcer := NewEnforcerWithStorage(store)

	decision := PolicyResult{
		Decision: "deny",
		Action:   "flag",
		Reason:   "Missing tags",
	}

	resource := types.Resource{
		ID:       "i-123",
		Type:     "ec2",
		Provider: "aws",
	}

	// Execute enforcement
	err = enforcer.Execute(context.Background(), decision, resource)
	assert.NoError(t, err)

	// Wait for async storage
	time.Sleep(100 * time.Millisecond)

	// Query stored events
	events, err := store.QueryEnforcements(context.Background(), types.ResourceFilter{
		IDs: []string{"i-123"},
	})
	require.NoError(t, err)
	assert.Len(t, events, 1)

	// Verify event details
	event := events[0]
	assert.Equal(t, "i-123", event.ResourceID)
	assert.Equal(t, "flag", event.Action)
	assert.Equal(t, "deny", event.Decision)
	assert.Equal(t, "Missing tags", event.Reason)
	assert.True(t, event.Success)
}

func TestEnforcer_StoresFailedEnforcement(t *testing.T) {
	// Create temp storage
	tmpDir := t.TempDir()
	store, err := storage.NewMVCCStorage(tmpDir)
	require.NoError(t, err)
	defer func() { _ = store.Close() }()

	// Mock provider that fails
	mockProvider := &FailingMockProvider{}

	// Create enforcer with storage and failing provider
	enforcer := NewEnforcerWithStorageAndProvider(store, mockProvider)

	decision := PolicyResult{
		Decision: "deny",
		Action:   "flag",
		Reason:   "Policy violation",
	}

	resource := types.Resource{
		ID:   "i-456",
		Type: "ec2",
	}

	// Execute enforcement (will fail)
	err = enforcer.Execute(context.Background(), decision, resource)
	assert.Error(t, err)

	// Wait for async storage
	time.Sleep(100 * time.Millisecond)

	// Query stored events
	events, err := store.QueryEnforcements(context.Background(), types.ResourceFilter{
		IDs: []string{"i-456"},
	})
	require.NoError(t, err)
	assert.Len(t, events, 1)

	// Verify failure is recorded
	event := events[0]
	assert.False(t, event.Success)
	assert.Contains(t, event.Error, "mock failure")
}

// FailingMockProvider for testing
type FailingMockProvider struct {
	MockProvider
}

func (f *FailingMockProvider) TagResource(ctx context.Context, id string, tags map[string]string) error {
	return fmt.Errorf("mock failure: unable to tag resource")
}
