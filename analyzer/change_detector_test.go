package analyzer

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/yairfalse/elava/storage"
	"github.com/yairfalse/elava/types"
)

func TestChangeDetector_DetectNewResources(t *testing.T) {
	// Setup storage
	store := setupTestStorage(t)
	defer func() { _ = store.Close() }()

	detector := NewChangeDetector(store)
	require.NotNil(t, detector)

	// First scan - new resource appears
	newResource := types.Resource{
		ID:         "i-new123",
		Type:       "ec2_instance",
		Provider:   "aws",
		Region:     "us-east-1",
		AccountID:  "123456789",
		Name:       "web-server-1",
		Status:     "running",
		CreatedAt:  time.Now(),
		LastSeenAt: time.Now(),
		Tags: types.Tags{
			Name:        "web-server-1",
			Environment: "production",
		},
	}

	// Detect changes (no previous scan)
	changes, err := detector.DetectChanges(context.Background(), []types.Resource{newResource})
	require.NoError(t, err)

	// First scan should report the resource as "created"
	require.Len(t, changes, 1)
	assert.Equal(t, "created", changes[0].ChangeType)
	assert.Equal(t, "i-new123", changes[0].ResourceID)
	assert.NotNil(t, changes[0].Current)
	assert.Nil(t, changes[0].Previous)
}

func TestChangeDetector_DetectModifiedResources(t *testing.T) {
	// Setup storage
	store := setupTestStorage(t)
	defer func() { _ = store.Close() }()

	detector := NewChangeDetector(store)

	// First scan - initial state
	original := types.Resource{
		ID:         "i-abc123",
		Type:       "ec2_instance",
		Status:     "running",
		LastSeenAt: time.Now(),
		Tags: types.Tags{
			Environment: "development",
		},
	}

	// Store original
	_, err := store.RecordObservation(original)
	require.NoError(t, err)

	// Second scan - modified tags
	modified := original
	modified.Tags.Environment = "production" // Changed!
	modified.LastSeenAt = time.Now()

	// Detect changes
	changes, err := detector.DetectChanges(context.Background(), []types.Resource{modified})
	require.NoError(t, err)

	// Should detect modification
	require.Len(t, changes, 1)
	assert.Equal(t, "modified", changes[0].ChangeType)
	assert.Equal(t, "i-abc123", changes[0].ResourceID)
	assert.NotNil(t, changes[0].Current)
	assert.NotNil(t, changes[0].Previous)
	assert.Equal(t, "production", changes[0].Current.Tags.Environment)
	assert.Equal(t, "development", changes[0].Previous.Tags.Environment)
}

func TestChangeDetector_DetectDisappearedResources(t *testing.T) {
	// Setup storage
	store := setupTestStorage(t)
	defer func() { _ = store.Close() }()

	detector := NewChangeDetector(store)

	// First scan - resource exists
	existing := types.Resource{
		ID:         "sg-old123",
		Type:       "security_group",
		Status:     "active",
		LastSeenAt: time.Now(),
	}

	_, err := store.RecordObservation(existing)
	require.NoError(t, err)

	// Second scan - resource is gone (not in current scan)
	currentScan := []types.Resource{} // Empty - resource disappeared

	// Detect changes
	changes, err := detector.DetectChanges(context.Background(), currentScan)
	require.NoError(t, err)

	// Should detect disappearance
	require.Len(t, changes, 1)
	assert.Equal(t, "disappeared", changes[0].ChangeType)
	assert.Equal(t, "sg-old123", changes[0].ResourceID)
	assert.Nil(t, changes[0].Current)
	assert.NotNil(t, changes[0].Previous)
}

func TestChangeDetector_NoChanges(t *testing.T) {
	// Setup storage
	store := setupTestStorage(t)
	defer func() { _ = store.Close() }()

	detector := NewChangeDetector(store)

	// First scan
	resource := types.Resource{
		ID:         "i-same123",
		Type:       "ec2_instance",
		Status:     "running",
		LastSeenAt: time.Now(),
	}

	_, err := store.RecordObservation(resource)
	require.NoError(t, err)

	// Second scan - identical resource
	resource.LastSeenAt = time.Now() // Only timestamp changed

	changes, err := detector.DetectChanges(context.Background(), []types.Resource{resource})
	require.NoError(t, err)

	// No meaningful changes (timestamp doesn't count)
	assert.Empty(t, changes)
}

func TestChangeDetector_MetadataChange(t *testing.T) {
	// Setup storage
	store := setupTestStorage(t)
	defer func() { _ = store.Close() }()

	detector := NewChangeDetector(store)

	// First scan - original metadata
	original := types.Resource{
		ID:         "i-metadata",
		Type:       "ec2_instance",
		Status:     "running",
		LastSeenAt: time.Now(),
		Metadata: types.ResourceMetadata{
			InstanceType: "t2.micro",
			Encrypted:    false,
		},
	}

	_, err := store.RecordObservation(original)
	require.NoError(t, err)

	// Second scan - instance type changed
	modified := original
	modified.Metadata.InstanceType = "t3.medium" // Changed!
	modified.LastSeenAt = time.Now()

	changes, err := detector.DetectChanges(context.Background(), []types.Resource{modified})
	require.NoError(t, err)

	// Should detect metadata modification
	require.Len(t, changes, 1)
	assert.Equal(t, "modified", changes[0].ChangeType)
	assert.Equal(t, "t3.medium", changes[0].Current.Metadata.InstanceType)
	assert.Equal(t, "t2.micro", changes[0].Previous.Metadata.InstanceType)
}

func TestChangeDetector_FirstScan(t *testing.T) {
	// Setup storage
	store := setupTestStorage(t)
	defer func() { _ = store.Close() }()

	detector := NewChangeDetector(store)

	// First ever scan - 3 new resources
	resources := []types.Resource{
		{ID: "i-1", Type: "ec2_instance", Status: "running", LastSeenAt: time.Now()},
		{ID: "i-2", Type: "ec2_instance", Status: "running", LastSeenAt: time.Now()},
		{ID: "i-3", Type: "ec2_instance", Status: "stopped", LastSeenAt: time.Now()},
	}

	changes, err := detector.DetectChanges(context.Background(), resources)
	require.NoError(t, err)

	// All should be "created"
	require.Len(t, changes, 3)
	for _, change := range changes {
		assert.Equal(t, "created", change.ChangeType)
	}
}

// setupTestStorage creates a temporary storage for testing
func setupTestStorage(t *testing.T) *storage.MVCCStorage {
	dir := t.TempDir()
	store, err := storage.NewMVCCStorage(dir)
	require.NoError(t, err)
	return store
}
