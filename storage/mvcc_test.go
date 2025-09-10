package storage

import (
	"testing"
	"time"

	"github.com/yairfalse/ovi/types"
)

func TestMVCCStorage_RecordObservation(t *testing.T) {
	// Create temp storage
	tmpDir := t.TempDir()
	storage, err := NewMVCCStorage(tmpDir)
	if err != nil {
		t.Fatalf("Failed to create storage: %v", err)
	}
	defer func() { _ = storage.Close() }()

	// Create test resource
	resource := types.Resource{
		ID:       "i-123456",
		Type:     "ec2",
		Provider: "aws",
		Status:   "running",
		Tags: types.Tags{
			OviOwner: "team-web",
		},
	}

	// Record observation
	rev, err := storage.RecordObservation(resource)
	if err != nil {
		t.Fatalf("RecordObservation failed: %v", err)
	}

	if rev != 1 {
		t.Errorf("Expected first revision to be 1, got %d", rev)
	}

	// Verify we can retrieve it
	state, err := storage.GetResourceState(resource.ID)
	if err != nil {
		t.Fatalf("GetResourceState failed: %v", err)
	}

	if state.ResourceID != resource.ID {
		t.Errorf("ResourceID = %v, want %v", state.ResourceID, resource.ID)
	}
	if state.LastSeenRev != 1 {
		t.Errorf("LastSeenRev = %v, want 1", state.LastSeenRev)
	}
	if !state.Exists {
		t.Error("Resource should exist")
	}
}

func TestMVCCStorage_MultipleObservations(t *testing.T) {
	tmpDir := t.TempDir()
	storage, err := NewMVCCStorage(tmpDir)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = storage.Close() }()

	// Observe multiple resources
	resources := []types.Resource{
		{ID: "i-001", Type: "ec2", Tags: types.Tags{OviOwner: "team-web"}},
		{ID: "i-002", Type: "ec2", Tags: types.Tags{OviOwner: "team-web"}},
		{ID: "i-003", Type: "ec2", Tags: types.Tags{OviOwner: "team-api"}},
	}

	// Record batch observation
	rev, err := storage.RecordObservationBatch(resources)
	if err != nil {
		t.Fatalf("RecordObservationBatch failed: %v", err)
	}

	// All should have same revision
	for _, r := range resources {
		state, err := storage.GetResourceState(r.ID)
		if err != nil {
			t.Fatalf("GetResourceState(%s) failed: %v", r.ID, err)
		}
		if state.LastSeenRev != rev {
			t.Errorf("Resource %s has rev %d, want %d", r.ID, state.LastSeenRev, rev)
		}
	}
}

func TestMVCCStorage_ResourceDisappears(t *testing.T) {
	tmpDir := t.TempDir()
	storage, err := NewMVCCStorage(tmpDir)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = storage.Close() }()

	resource := types.Resource{ID: "i-123", Type: "ec2", Tags: types.Tags{OviOwner: "team-web"}}

	// First observation - resource exists
	rev1, _ := storage.RecordObservation(resource)

	// Second observation - resource gone
	rev2, _ := storage.RecordDisappearance(resource.ID)

	if rev2 <= rev1 {
		t.Errorf("Revision should increase: rev1=%d, rev2=%d", rev1, rev2)
	}

	// Check state shows it's gone
	state, _ := storage.GetResourceState(resource.ID)
	if state.Exists {
		t.Error("Resource should not exist")
	}
	if state.DisappearedRev != rev2 {
		t.Errorf("DisappearedRev = %d, want %d", state.DisappearedRev, rev2)
	}
}

func TestMVCCStorage_GetStateAtRevision(t *testing.T) {
	tmpDir := t.TempDir()
	storage, err := NewMVCCStorage(tmpDir)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = storage.Close() }()

	// Simulate resource lifecycle
	resource := types.Resource{ID: "i-123", Type: "ec2", Tags: types.Tags{OviOwner: "team-web"}}

	// Rev 1: Resource appears
	rev1, _ := storage.RecordObservation(resource)

	// Rev 2: Still there
	resource.Status = "stopping"
	rev2, _ := storage.RecordObservation(resource)

	// Rev 3: Gone
	rev3, _ := storage.RecordDisappearance(resource.ID)

	// Time travel! Check state at each revision
	stateAt1, _ := storage.GetStateAtRevision(resource.ID, rev1)
	if !stateAt1.Exists {
		t.Error("Resource should exist at rev1")
	}

	stateAt2, _ := storage.GetStateAtRevision(resource.ID, rev2)
	if !stateAt2.Exists {
		t.Error("Resource should exist at rev2")
	}

	stateAt3, _ := storage.GetStateAtRevision(resource.ID, rev3)
	if stateAt3.Exists {
		t.Error("Resource should not exist at rev3")
	}
}

func TestMVCCStorage_QueryByOwner(t *testing.T) {
	tmpDir := t.TempDir()
	storage, err := NewMVCCStorage(tmpDir)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = storage.Close() }()

	// Mix of resources from different owners
	resources := []types.Resource{
		{ID: "i-001", Type: "ec2", Tags: types.Tags{OviOwner: "team-web"}},
		{ID: "i-002", Type: "ec2", Tags: types.Tags{OviOwner: "team-web"}},
		{ID: "i-003", Type: "ec2", Tags: types.Tags{OviOwner: "team-api"}},
		{ID: "i-004", Type: "rds", Tags: types.Tags{OviOwner: "team-web"}},
	}

	_, _ = storage.RecordObservationBatch(resources)

	// Query team-web resources
	webResources, err := storage.GetResourcesByOwner("team-web")
	if err != nil {
		t.Fatalf("GetResourcesByOwner failed: %v", err)
	}

	if len(webResources) != 3 {
		t.Errorf("Expected 3 resources for team-web, got %d", len(webResources))
	}

	// Query team-api resources
	apiResources, _ := storage.GetResourcesByOwner("team-api")
	if len(apiResources) != 1 {
		t.Errorf("Expected 1 resource for team-api, got %d", len(apiResources))
	}
}

func TestMVCCStorage_Compaction(t *testing.T) {
	tmpDir := t.TempDir()
	storage, err := NewMVCCStorage(tmpDir)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = storage.Close() }()

	resource := types.Resource{ID: "i-123", Type: "ec2", Tags: types.Tags{OviOwner: "team-web"}}

	// Create many revisions
	for i := 0; i < 100; i++ {
		_, _ = storage.RecordObservation(resource)
	}

	// Get current revision
	currentRev := storage.CurrentRevision()
	if currentRev < 100 {
		t.Errorf("Should have at least 100 revisions, got %d", currentRev)
	}

	// Compact, keeping only last 10 revisions
	err = storage.Compact(10)
	if err != nil {
		t.Fatalf("Compact failed: %v", err)
	}

	// Should still be able to query recent revisions
	state, err := storage.GetStateAtRevision(resource.ID, currentRev)
	if err != nil {
		t.Errorf("Should be able to query recent revision: %v", err)
	}
	if !state.Exists {
		t.Error("Resource should exist in recent revision")
	}

	// But not old revisions
	_, err = storage.GetStateAtRevision(resource.ID, 1)
	if err == nil {
		t.Error("Should not be able to query compacted revision")
	}
}

func TestMVCCStorage_ConcurrentAccess(t *testing.T) {
	tmpDir := t.TempDir()
	storage, err := NewMVCCStorage(tmpDir)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = storage.Close() }()

	// Simulate multiple Ovi instances writing concurrently
	done := make(chan bool, 3)

	// Writer 1
	go func() {
		for i := 0; i < 10; i++ {
			r := types.Resource{ID: "web-" + string(rune(i)), Tags: types.Tags{OviOwner: "team-web"}}
			_, _ = storage.RecordObservation(r)
			time.Sleep(10 * time.Millisecond)
		}
		done <- true
	}()

	// Writer 2
	go func() {
		for i := 0; i < 10; i++ {
			r := types.Resource{ID: "api-" + string(rune(i)), Tags: types.Tags{OviOwner: "team-api"}}
			_, _ = storage.RecordObservation(r)
			time.Sleep(10 * time.Millisecond)
		}
		done <- true
	}()

	// Reader
	go func() {
		for i := 0; i < 20; i++ {
			_, _ = storage.GetResourcesByOwner("team-web")
			time.Sleep(5 * time.Millisecond)
		}
		done <- true
	}()

	// Wait for all to complete
	for i := 0; i < 3; i++ {
		<-done
	}

	// Verify we have all resources
	webResources, _ := storage.GetResourcesByOwner("team-web")
	apiResources, _ := storage.GetResourcesByOwner("team-api")

	if len(webResources) == 0 {
		t.Error("Should have web resources")
	}
	if len(apiResources) == 0 {
		t.Error("Should have api resources")
	}
}
