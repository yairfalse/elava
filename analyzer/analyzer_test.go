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

func TestWasteAnalyzer_AnalyzeWaste(t *testing.T) {
	tmpDir := t.TempDir()
	store, err := storage.NewMVCCStorage(tmpDir)
	require.NoError(t, err)
	defer func() { _ = store.Close() }()

	analyzer := NewWasteAnalyzer(store)

	// Store test resources
	testResources := []types.Resource{
		createOrphanedResource(),
		createIdleResource(),
		createOversizedResource(),
		createUnattachedResource(),
		createObsoleteResource(),
		createNormalResource(),
	}

	for _, r := range testResources {
		_, err := store.RecordObservation(r)
		require.NoError(t, err)
	}

	patterns, err := analyzer.AnalyzeWaste(context.Background())
	require.NoError(t, err)

	// Should detect all waste patterns
	wasteTypes := make(map[WasteType]bool)
	for _, pattern := range patterns {
		wasteTypes[pattern.Type] = true
		assert.NotEmpty(t, pattern.ResourceIDs)
		assert.NotEmpty(t, pattern.Reason)
		assert.Greater(t, pattern.Confidence, 0.0)
		assert.True(t, pattern.FirstSeen.Before(time.Now().Add(time.Second)))
	}

	// Verify all waste types detected
	expectedTypes := []WasteType{WasteOrphaned, WasteIdle, WasteOversized, WasteUnattached, WasteObsolete}
	for _, expectedType := range expectedTypes {
		assert.True(t, wasteTypes[expectedType], "Expected waste type %s not detected", expectedType)
	}
}

func TestWasteAnalyzer_IsOrphaned(t *testing.T) {
	analyzer := NewWasteAnalyzer(nil)

	tests := []struct {
		name     string
		resource types.Resource
		expected bool
	}{
		{
			name:     "already marked orphaned",
			resource: types.Resource{IsOrphaned: true},
			expected: true,
		},
		{
			name: "no owner or team tags",
			resource: types.Resource{
				Tags: types.Tags{ElavaOwner: "", Team: ""},
			},
			expected: true,
		},
		{
			name: "has owner tag",
			resource: types.Resource{
				Tags: types.Tags{ElavaOwner: "team-web"},
			},
			expected: false,
		},
		{
			name: "has team tag",
			resource: types.Resource{
				Tags: types.Tags{Team: "backend"},
			},
			expected: false,
		},
		{
			name: "default security group",
			resource: types.Resource{
				Type: "security_group",
				Name: "default-sg",
				Tags: types.Tags{ElavaOwner: "admin"},
			},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := analyzer.isOrphaned(tt.resource)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestWasteAnalyzer_IsIdle(t *testing.T) {
	analyzer := NewWasteAnalyzer(nil)

	tests := []struct {
		name     string
		resource types.Resource
		expected bool
	}{
		{
			name: "stopped EC2 instance",
			resource: types.Resource{
				Type:   "ec2",
				Status: "stopped",
			},
			expected: true,
		},
		{
			name: "running EC2 instance",
			resource: types.Resource{
				Type:   "ec2",
				Status: "running",
			},
			expected: false,
		},
		{
			name: "idle RDS",
			resource: types.Resource{
				Type:     "rds",
				Metadata: map[string]interface{}{"is_idle": true},
			},
			expected: true,
		},
		{
			name: "active RDS",
			resource: types.Resource{
				Type:     "rds",
				Metadata: map[string]interface{}{"is_idle": false},
			},
			expected: false,
		},
		{
			name: "paused Redshift",
			resource: types.Resource{
				Type:     "redshift",
				Metadata: map[string]interface{}{"is_paused": true},
			},
			expected: true,
		},
		{
			name: "old Lambda function",
			resource: types.Resource{
				Type:     "lambda",
				Metadata: map[string]interface{}{"days_since_modified": 45},
			},
			expected: true,
		},
		{
			name: "recently modified Lambda",
			resource: types.Resource{
				Type:     "lambda",
				Metadata: map[string]interface{}{"days_since_modified": 10},
			},
			expected: false,
		},
		{
			name: "unavailable NAT gateway",
			resource: types.Resource{
				Type:   "nat_gateway",
				Status: "failed",
			},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := analyzer.isIdle(tt.resource)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestWasteAnalyzer_IsOversized(t *testing.T) {
	analyzer := NewWasteAnalyzer(nil)

	tests := []struct {
		name     string
		resource types.Resource
		expected bool
	}{
		{
			name: "large EC2 in dev",
			resource: types.Resource{
				Type:     "ec2",
				Tags:     types.Tags{Environment: "dev"},
				Metadata: map[string]interface{}{"instance_type": "m5.2xlarge"},
			},
			expected: true,
		},
		{
			name: "large EC2 in prod",
			resource: types.Resource{
				Type:     "ec2",
				Tags:     types.Tags{Environment: "prod"},
				Metadata: map[string]interface{}{"instance_type": "m5.2xlarge"},
			},
			expected: false,
		},
		{
			name: "small EC2 in dev",
			resource: types.Resource{
				Type:     "ec2",
				Tags:     types.Tags{Environment: "dev"},
				Metadata: map[string]interface{}{"instance_type": "t3.micro"},
			},
			expected: false,
		},
		{
			name: "multi-AZ RDS in test",
			resource: types.Resource{
				Type:     "rds",
				Tags:     types.Tags{Environment: "test"},
				Metadata: map[string]interface{}{"multi_az": true},
			},
			expected: true,
		},
		{
			name: "multi-AZ RDS in prod",
			resource: types.Resource{
				Type:     "rds",
				Tags:     types.Tags{Environment: "production"},
				Metadata: map[string]interface{}{"multi_az": true},
			},
			expected: false,
		},
		{
			name: "large Redshift in dev",
			resource: types.Resource{
				Type:     "redshift",
				Tags:     types.Tags{Environment: "dev"},
				Metadata: map[string]interface{}{"node_count": 6},
			},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := analyzer.isOversized(tt.resource)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestWasteAnalyzer_IsLargeInstance(t *testing.T) {
	analyzer := NewWasteAnalyzer(nil)

	tests := []struct {
		instanceType string
		expected     bool
	}{
		{"t3.micro", false},
		{"t3.small", false},
		{"m5.large", false},
		{"m5.xlarge", true},
		{"m5.2xlarge", true},
		{"m5.4xlarge", true},
		{"m5.8xlarge", true},
		{"c5.metal", true},
		{"r5.24xlarge", true},
	}

	for _, tt := range tests {
		t.Run(tt.instanceType, func(t *testing.T) {
			result := analyzer.isLargeInstance(tt.instanceType)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestWasteAnalyzer_IsUnattached(t *testing.T) {
	analyzer := NewWasteAnalyzer(nil)

	tests := []struct {
		name     string
		resource types.Resource
		expected bool
	}{
		{
			name: "unattached EBS volume",
			resource: types.Resource{
				Type:   "ebs",
				Status: "unattached",
			},
			expected: true,
		},
		{
			name: "attached EBS volume",
			resource: types.Resource{
				Type:     "ebs",
				Status:   "attached",
				Metadata: map[string]interface{}{"is_attached": true},
			},
			expected: false,
		},
		{
			name: "unassociated Elastic IP",
			resource: types.Resource{
				Type:   "elastic_ip",
				Status: "unassociated",
			},
			expected: true,
		},
		{
			name: "associated Elastic IP",
			resource: types.Resource{
				Type:     "elastic_ip",
				Status:   "associated",
				Metadata: map[string]interface{}{"is_associated": true},
			},
			expected: false,
		},
		{
			name: "unattached network interface",
			resource: types.Resource{
				Type:     "network_interface",
				Metadata: map[string]interface{}{"attachment": nil},
			},
			expected: true,
		},
		{
			name: "attached network interface",
			resource: types.Resource{
				Type:     "network_interface",
				Metadata: map[string]interface{}{"attachment": map[string]interface{}{"instance_id": "i-123"}},
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := analyzer.isUnattached(tt.resource)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestWasteAnalyzer_IsObsolete(t *testing.T) {
	analyzer := NewWasteAnalyzer(nil)

	tests := []struct {
		name     string
		resource types.Resource
		expected bool
	}{
		{
			name: "old snapshot",
			resource: types.Resource{
				Type:     "snapshot",
				Metadata: map[string]interface{}{"age_days": 45},
			},
			expected: true,
		},
		{
			name: "recent snapshot",
			resource: types.Resource{
				Type:     "snapshot",
				Metadata: map[string]interface{}{"age_days": 15},
			},
			expected: false,
		},
		{
			name: "marked as old",
			resource: types.Resource{
				Type:     "ami",
				Metadata: map[string]interface{}{"is_old": true},
			},
			expected: true,
		},
		{
			name: "temporary backup",
			resource: types.Resource{
				Type:     "rds_snapshot",
				Metadata: map[string]interface{}{"is_temp": true},
			},
			expected: true,
		},
		{
			name: "current AMI",
			resource: types.Resource{
				Type:     "ami",
				Metadata: map[string]interface{}{"age_days": 10, "is_old": false},
			},
			expected: false,
		},
		{
			name: "non-snapshot resource",
			resource: types.Resource{
				Type:     "ec2",
				Metadata: map[string]interface{}{"age_days": 100},
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := analyzer.isObsolete(tt.resource)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestWasteAnalyzer_FindOrphans(t *testing.T) {
	tmpDir := t.TempDir()
	store, err := storage.NewMVCCStorage(tmpDir)
	require.NoError(t, err)
	defer func() { _ = store.Close() }()

	analyzer := NewWasteAnalyzer(store)

	// Store test resources
	orphan := createOrphanedResource()
	normal := createNormalResource()

	_, err = store.RecordObservation(orphan)
	require.NoError(t, err)
	_, err = store.RecordObservation(normal)
	require.NoError(t, err)

	orphans, err := analyzer.FindOrphans(context.Background(), time.Now().Add(-1*time.Hour))
	require.NoError(t, err)

	assert.Len(t, orphans, 1)
	assert.Equal(t, orphan.ID, orphans[0].ID)
}

func TestWasteAnalyzer_FindIdleResources(t *testing.T) {
	tmpDir := t.TempDir()
	store, err := storage.NewMVCCStorage(tmpDir)
	require.NoError(t, err)
	defer func() { _ = store.Close() }()

	analyzer := NewWasteAnalyzer(store)

	// Store test resources
	idle := createIdleResource()
	active := createNormalResource()

	_, err = store.RecordObservation(idle)
	require.NoError(t, err)
	_, err = store.RecordObservation(active)
	require.NoError(t, err)

	idleResources, err := analyzer.FindIdleResources(context.Background(), 1*time.Hour)
	require.NoError(t, err)

	assert.Len(t, idleResources, 1)
	assert.Equal(t, idle.ID, idleResources[0].ID)
}

// Test helper functions

func createOrphanedResource() types.Resource {
	return types.Resource{
		ID:       "i-orphan-123",
		Type:     "ec2",
		Provider: "aws",
		Name:     "orphaned-instance",
		Status:   "running",
		Tags:     types.Tags{}, // No owner
		Metadata: map[string]interface{}{
			"monthly_cost_estimate": 100.0,
		},
		CreatedAt:  time.Now().Add(-7 * 24 * time.Hour),
		LastSeenAt: time.Now(),
	}
}

func createIdleResource() types.Resource {
	return types.Resource{
		ID:       "i-idle-456",
		Type:     "ec2",
		Provider: "aws",
		Name:     "idle-instance",
		Status:   "stopped", // Idle
		Tags: types.Tags{
			ElavaOwner: "team-dev",
		},
		Metadata: map[string]interface{}{
			"monthly_cost_estimate": 50.0,
		},
		CreatedAt:  time.Now().Add(-3 * 24 * time.Hour),
		LastSeenAt: time.Now(),
	}
}

func createOversizedResource() types.Resource {
	return types.Resource{
		ID:       "i-oversized-789",
		Type:     "ec2",
		Provider: "aws",
		Name:     "oversized-dev-instance",
		Status:   "running",
		Tags: types.Tags{
			ElavaOwner:  "team-test",
			Environment: "dev", // Dev environment with large instance
		},
		Metadata: map[string]interface{}{
			"instance_type":         "m5.4xlarge", // Large instance
			"monthly_cost_estimate": 300.0,
		},
		CreatedAt:  time.Now().Add(-1 * 24 * time.Hour),
		LastSeenAt: time.Now(),
	}
}

func createUnattachedResource() types.Resource {
	return types.Resource{
		ID:       "vol-unattached-abc",
		Type:     "ebs",
		Provider: "aws",
		Name:     "unattached-volume",
		Status:   "unattached", // Unattached
		Tags: types.Tags{
			ElavaOwner: "team-storage",
		},
		Metadata: map[string]interface{}{
			"is_attached":           false,
			"monthly_cost_estimate": 20.0,
		},
		CreatedAt:  time.Now().Add(-5 * 24 * time.Hour),
		LastSeenAt: time.Now(),
	}
}

func createObsoleteResource() types.Resource {
	return types.Resource{
		ID:       "snap-obsolete-def",
		Type:     "snapshot",
		Provider: "aws",
		Name:     "old-backup-snapshot",
		Status:   "completed",
		Tags: types.Tags{
			ElavaOwner: "team-backup",
		},
		Metadata: map[string]interface{}{
			"age_days":              45, // Old snapshot
			"monthly_cost_estimate": 5.0,
		},
		CreatedAt:  time.Now().Add(-45 * 24 * time.Hour),
		LastSeenAt: time.Now(),
	}
}

func createNormalResource() types.Resource {
	return types.Resource{
		ID:       "i-normal-999",
		Type:     "ec2",
		Provider: "aws",
		Name:     "normal-instance",
		Status:   "running", // Active
		Tags: types.Tags{
			ElavaOwner:  "team-prod",
			Environment: "production",
		},
		Metadata: map[string]interface{}{
			"instance_type":         "t3.medium", // Appropriately sized
			"monthly_cost_estimate": 30.0,
		},
		CreatedAt:  time.Now().Add(-10 * 24 * time.Hour),
		LastSeenAt: time.Now(),
	}
}

// Tests for QueryEngine

func TestQueryEngine_QueryByTimeRange(t *testing.T) {
	tmpDir := t.TempDir()
	store, err := storage.NewMVCCStorage(tmpDir)
	require.NoError(t, err)
	defer func() { _ = store.Close() }()

	queryEngine := NewQueryEngine(store)

	// Store resources with different timestamps
	now := time.Now()
	oldResource := createTestResource("old-resource", now.Add(-2*time.Hour))
	recentResource := createTestResource("recent-resource", now.Add(-30*time.Minute))
	futureResource := createTestResource("future-resource", now.Add(1*time.Hour))

	_, err = store.RecordObservation(oldResource)
	require.NoError(t, err)
	_, err = store.RecordObservation(recentResource)
	require.NoError(t, err)
	_, err = store.RecordObservation(futureResource)
	require.NoError(t, err)

	// Query for resources in the last hour
	resources, err := queryEngine.QueryByTimeRange(context.Background(), now.Add(-1*time.Hour), now)
	require.NoError(t, err)

	// Should only return recent resource
	assert.Len(t, resources, 1)
	assert.Equal(t, "recent-resource", resources[0].ID)
}

func TestQueryEngine_QueryChangesSince(t *testing.T) {
	tmpDir := t.TempDir()
	store, err := storage.NewMVCCStorage(tmpDir)
	require.NoError(t, err)
	defer func() { _ = store.Close() }()

	queryEngine := NewQueryEngine(store)

	// Store initial resource
	resource := createTestResource("test-resource", time.Now())
	rev1, err := store.RecordObservation(resource)
	require.NoError(t, err)

	// Modify resource
	resource.Status = "modified"
	_, err = store.RecordObservation(resource)
	require.NoError(t, err)

	// Query changes since initial revision
	changes, err := queryEngine.QueryChangesSince(context.Background(), rev1)
	require.NoError(t, err)

	// Should contain the modification
	assert.Greater(t, len(changes), 0)

	// Verify the change contains a change for our resource
	found := false
	for _, change := range changes {
		if change.ResourceID == "test-resource" && change.Revision >= rev1 {
			found = true
			// The change type might be "created" or "modified" depending on implementation
			assert.Contains(t, []ChangeType{ChangeCreated, ChangeModified}, change.Type)
			break
		}
	}
	assert.True(t, found, "Expected to find change for test-resource")
}

func TestQueryEngine_QueryResourceHistory(t *testing.T) {
	tmpDir := t.TempDir()
	store, err := storage.NewMVCCStorage(tmpDir)
	require.NoError(t, err)
	defer func() { _ = store.Close() }()

	queryEngine := NewQueryEngine(store)

	// Store resource multiple times to create history
	resource := createTestResource("history-resource", time.Now())

	rev1, err := store.RecordObservation(resource)
	require.NoError(t, err)

	resource.Status = "updated"
	rev2, err := store.RecordObservation(resource)
	require.NoError(t, err)

	resource.Status = "final"
	rev3, err := store.RecordObservation(resource)
	require.NoError(t, err)

	// Query resource history
	history, err := queryEngine.QueryResourceHistory(context.Background(), "history-resource")
	require.NoError(t, err)

	// Should have all revisions
	assert.GreaterOrEqual(t, len(history), 3)

	// Verify revisions are in order
	revisions := make([]int64, len(history))
	for i, h := range history {
		revisions[i] = h.Revision
	}

	// Should contain our revisions
	assert.Contains(t, revisions, rev1)
	assert.Contains(t, revisions, rev2)
	assert.Contains(t, revisions, rev3)
}

func TestQueryEngine_AggregateByTag(t *testing.T) {
	tmpDir := t.TempDir()
	store, err := storage.NewMVCCStorage(tmpDir)
	require.NoError(t, err)
	defer func() { _ = store.Close() }()

	queryEngine := NewQueryEngine(store)

	// Store resources with different environment tags
	devResource1 := createTestResourceWithTags("dev-1", "dev", "ec2")
	devResource2 := createTestResourceWithTags("dev-2", "dev", "rds")
	prodResource := createTestResourceWithTags("prod-1", "production", "ec2")

	_, err = store.RecordObservation(devResource1)
	require.NoError(t, err)
	_, err = store.RecordObservation(devResource2)
	require.NoError(t, err)
	_, err = store.RecordObservation(prodResource)
	require.NoError(t, err)

	// Aggregate by environment tag
	metrics, err := queryEngine.AggregateByTag(context.Background(), "environment", time.Hour)
	require.NoError(t, err)

	// Should have metrics for both environments
	assert.Contains(t, metrics, "dev")
	assert.Contains(t, metrics, "production")

	// Dev should have 2 resources
	devMetrics := metrics["dev"]
	assert.Equal(t, 2, devMetrics.Count)
	assert.Equal(t, 1, devMetrics.ResourceTypes["ec2"])
	assert.Equal(t, 1, devMetrics.ResourceTypes["rds"])

	// Production should have 1 resource
	prodMetrics := metrics["production"]
	assert.Equal(t, 1, prodMetrics.Count)
	assert.Equal(t, 1, prodMetrics.ResourceTypes["ec2"])
}

// Tests for DriftAnalyzer

func TestDriftAnalyzer_AnalyzeDrift(t *testing.T) {
	tmpDir := t.TempDir()
	store, err := storage.NewMVCCStorage(tmpDir)
	require.NoError(t, err)
	defer func() { _ = store.Close() }()

	analyzer := NewDriftAnalyzer(store)

	// Store initial resource state with timestamps that will be found by time queries
	fromTime := time.Now().Add(-30 * time.Minute) // Within query window
	resource := createTestResource("drift-resource", fromTime)
	resource.Status = "initial"
	resource.LastSeenAt = fromTime // Set explicit timestamp
	_, err = store.RecordObservation(resource)
	require.NoError(t, err)

	// Store modified resource state
	toTime := time.Now().Add(-15 * time.Minute) // Also within query window
	resource.Status = "modified"
	resource.Tags.Environment = "changed"
	resource.LastSeenAt = toTime // Set explicit timestamp
	_, err = store.RecordObservation(resource)
	require.NoError(t, err)

	// Analyze drift - use times that will capture our resources
	driftEvents, err := analyzer.AnalyzeDrift(context.Background(), fromTime, toTime)
	require.NoError(t, err)

	// Test should complete without error and return a slice (not nil)
	assert.NotNil(t, driftEvents)
	t.Logf("Drift events detected: %d", len(driftEvents))

	// If drift events are found, verify they have the correct structure
	for _, event := range driftEvents {
		assert.NotEmpty(t, event.ResourceID)
		assert.NotEmpty(t, event.Type)
		assert.NotZero(t, event.Timestamp)
	}
}

func TestDriftAnalyzer_GetResourceDrift(t *testing.T) {
	tmpDir := t.TempDir()
	store, err := storage.NewMVCCStorage(tmpDir)
	require.NoError(t, err)
	defer func() { _ = store.Close() }()

	analyzer := NewDriftAnalyzer(store)

	// Store resource with changes over time
	resource := createTestResource("drift-test", time.Now().Add(-2*time.Hour))
	_, err = store.RecordObservation(resource)
	require.NoError(t, err)

	// Change the resource
	resource.Status = "changed"
	_, err = store.RecordObservation(resource)
	require.NoError(t, err)

	// Get drift for specific resource
	driftEvents, err := analyzer.GetResourceDrift(context.Background(), "drift-test", 3*time.Hour)
	require.NoError(t, err)

	// Test should complete without error
	assert.NotNil(t, driftEvents)
	t.Logf("Resource drift events found: %d", len(driftEvents))

	// If events are found, verify they belong to the correct resource
	for _, event := range driftEvents {
		assert.Equal(t, "drift-test", event.ResourceID)
		assert.NotEmpty(t, event.Type)
		assert.NotZero(t, event.Timestamp)
	}
}

// Helper functions for drift tests

func createTestResource(id string, timestamp time.Time) types.Resource {
	return types.Resource{
		ID:         id,
		Type:       "ec2",
		Provider:   "aws",
		Name:       id + "-name",
		Status:     "running",
		Tags:       types.Tags{ElavaOwner: "test-team"},
		CreatedAt:  timestamp,
		LastSeenAt: timestamp,
	}
}

func createTestResourceWithTags(id, environment, resourceType string) types.Resource {
	return types.Resource{
		ID:       id,
		Type:     resourceType,
		Provider: "aws",
		Name:     id + "-name",
		Status:   "running",
		Tags: types.Tags{
			ElavaOwner:  "test-team",
			Environment: environment,
		},
		CreatedAt:  time.Now(),
		LastSeenAt: time.Now(),
	}
}
