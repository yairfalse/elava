package observer

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/yairfalse/elava/storage"
	"github.com/yairfalse/elava/types"
)

// Test extractResourceType helper function
func TestExtractResourceType_BothCurrentAndPrevious(t *testing.T) {
	event := storage.ChangeEvent{
		Current:  &types.Resource{Type: "ec2"},
		Previous: &types.Resource{Type: "ec2"},
	}

	result := extractResourceType(event)
	assert.Equal(t, "ec2", result)
}

func TestExtractResourceType_OnlyCurrent(t *testing.T) {
	event := storage.ChangeEvent{
		Current: &types.Resource{Type: "ec2"},
	}

	result := extractResourceType(event)
	assert.Equal(t, "ec2", result)
}

func TestExtractResourceType_OnlyPrevious(t *testing.T) {
	event := storage.ChangeEvent{
		Previous: &types.Resource{Type: "rds"},
	}

	result := extractResourceType(event)
	assert.Equal(t, "rds", result)
}

func TestExtractResourceType_Missing(t *testing.T) {
	event := storage.ChangeEvent{}

	result := extractResourceType(event)
	assert.Equal(t, "unknown", result)
}

// Test extractRegion helper function
func TestExtractRegion_BothCurrentAndPrevious(t *testing.T) {
	event := storage.ChangeEvent{
		Current:  &types.Resource{Region: "us-east-1"},
		Previous: &types.Resource{Region: "us-east-1"},
	}

	result := extractRegion(event)
	assert.Equal(t, "us-east-1", result)
}

func TestExtractRegion_OnlyCurrent(t *testing.T) {
	event := storage.ChangeEvent{
		Current: &types.Resource{Region: "us-west-2"},
	}

	result := extractRegion(event)
	assert.Equal(t, "us-west-2", result)
}

func TestExtractRegion_OnlyPrevious(t *testing.T) {
	event := storage.ChangeEvent{
		Previous: &types.Resource{Region: "eu-west-1"},
	}

	result := extractRegion(event)
	assert.Equal(t, "eu-west-1", result)
}

func TestExtractRegion_Missing(t *testing.T) {
	event := storage.ChangeEvent{}

	result := extractRegion(event)
	assert.Equal(t, "unknown", result)
}

// Test ChangeEventMetrics creation
func TestNewChangeEventMetrics_Success(t *testing.T) {
	metrics, err := NewChangeEventMetrics()

	require.NoError(t, err)
	assert.NotNil(t, metrics)
	assert.NotNil(t, metrics.meter)
	assert.NotNil(t, metrics.resourcesCreated)
	assert.NotNil(t, metrics.resourcesModified)
	assert.NotNil(t, metrics.resourcesDisappeared)
	assert.NotNil(t, metrics.changeEventsTotal)
}

// Test recording created events
func TestRecordChangeEvents_Created(t *testing.T) {
	metrics, err := NewChangeEventMetrics()
	require.NoError(t, err)

	event := storage.ChangeEvent{
		ChangeType: "created",
		Current: &types.Resource{
			ID:     "i-abc123",
			Type:   "ec2",
			Region: "us-east-1",
		},
	}

	ctx := context.Background()

	// Should not panic or error
	metrics.RecordChangeEvents(ctx, []storage.ChangeEvent{event})
}

// Test recording modified events
func TestRecordChangeEvents_Modified(t *testing.T) {
	metrics, err := NewChangeEventMetrics()
	require.NoError(t, err)

	event := storage.ChangeEvent{
		ChangeType: "modified",
		Current: &types.Resource{
			ID:     "i-abc123",
			Type:   "ec2",
			Region: "us-east-1",
			Status: "stopped",
		},
		Previous: &types.Resource{
			ID:     "i-abc123",
			Type:   "ec2",
			Region: "us-east-1",
			Status: "running",
		},
	}

	ctx := context.Background()

	// Should not panic or error
	metrics.RecordChangeEvents(ctx, []storage.ChangeEvent{event})
}

// Test recording disappeared events
func TestRecordChangeEvents_Disappeared(t *testing.T) {
	metrics, err := NewChangeEventMetrics()
	require.NoError(t, err)

	event := storage.ChangeEvent{
		ChangeType: "disappeared",
		Previous: &types.Resource{
			ID:     "i-abc123",
			Type:   "ec2",
			Region: "us-east-1",
		},
	}

	ctx := context.Background()

	// Should not panic or error
	metrics.RecordChangeEvents(ctx, []storage.ChangeEvent{event})
}

// Test recording batch of mixed events
func TestRecordChangeEvents_Batch(t *testing.T) {
	metrics, err := NewChangeEventMetrics()
	require.NoError(t, err)

	events := []storage.ChangeEvent{
		{
			ChangeType: "created",
			Current:    &types.Resource{Type: "ec2", Region: "us-east-1"},
		},
		{
			ChangeType: "modified",
			Current:    &types.Resource{Type: "rds", Region: "us-west-2"},
			Previous:   &types.Resource{Type: "rds", Region: "us-west-2"},
		},
		{
			ChangeType: "disappeared",
			Previous:   &types.Resource{Type: "s3_bucket", Region: "eu-west-1"},
		},
	}

	ctx := context.Background()

	// Should not panic or error
	metrics.RecordChangeEvents(ctx, events)
}

// Test empty batch
func TestRecordChangeEvents_EmptyBatch(t *testing.T) {
	metrics, err := NewChangeEventMetrics()
	require.NoError(t, err)

	ctx := context.Background()

	// Should handle gracefully
	metrics.RecordChangeEvents(ctx, []storage.ChangeEvent{})
}

// Test unknown change type
func TestRecordChangeEvents_UnknownType(t *testing.T) {
	metrics, err := NewChangeEventMetrics()
	require.NoError(t, err)

	event := storage.ChangeEvent{
		ChangeType: "unknown-type",
		Current:    &types.Resource{Type: "ec2", Region: "us-east-1"},
	}

	ctx := context.Background()

	// Should handle gracefully (skip unknown types)
	metrics.RecordChangeEvents(ctx, []storage.ChangeEvent{event})
}

// Test multiple events of same type
func TestRecordChangeEvents_MultipleCreated(t *testing.T) {
	metrics, err := NewChangeEventMetrics()
	require.NoError(t, err)

	events := []storage.ChangeEvent{
		{
			ChangeType: "created",
			Current:    &types.Resource{Type: "ec2", Region: "us-east-1"},
		},
		{
			ChangeType: "created",
			Current:    &types.Resource{Type: "ec2", Region: "us-east-1"},
		},
		{
			ChangeType: "created",
			Current:    &types.Resource{Type: "rds", Region: "us-west-2"},
		},
	}

	ctx := context.Background()

	// Should record all 3 events
	metrics.RecordChangeEvents(ctx, events)
}

// Test context cancellation
func TestRecordChangeEvents_ContextCancelled(t *testing.T) {
	metrics, err := NewChangeEventMetrics()
	require.NoError(t, err)

	event := storage.ChangeEvent{
		ChangeType: "created",
		Current:    &types.Resource{Type: "ec2", Region: "us-east-1"},
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	// Should handle gracefully
	metrics.RecordChangeEvents(ctx, []storage.ChangeEvent{event})
}
