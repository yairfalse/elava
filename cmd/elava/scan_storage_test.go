package main

import (
	"context"
	"testing"

	"github.com/yairfalse/elava/storage"
	"github.com/yairfalse/elava/types"
)

// Test storeObservations function - CLAUDE.md: Small focused tests
func TestStoreObservations_EmptyResources(t *testing.T) {
	tmpDir := t.TempDir()
	store, err := storage.NewMVCCStorage(tmpDir)
	if err != nil {
		t.Fatalf("Failed to create storage: %v", err)
	}
	defer func() { _ = store.Close() }()

	var resources []types.Resource

	revision, err := storeObservations(store, resources)

	if err != nil {
		t.Errorf("Expected no error for empty resources, got %v", err)
	}
	if revision != 1 {
		t.Errorf("Expected revision 1, got %d", revision)
	}
}

func TestStoreObservations_SingleResource(t *testing.T) {
	tmpDir := t.TempDir()
	store, err := storage.NewMVCCStorage(tmpDir)
	if err != nil {
		t.Fatalf("Failed to create storage: %v", err)
	}
	defer func() { _ = store.Close() }()

	resources := []types.Resource{
		{
			ID:       "i-abc123",
			Type:     "ec2",
			Provider: "aws",
			Region:   "us-east-1",
			Status:   "running",
		},
	}

	revision, err := storeObservations(store, resources)

	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}
	if revision != 1 {
		t.Errorf("Expected revision 1, got %d", revision)
	}
}

func TestStoreObservations_BatchResources(t *testing.T) {
	tmpDir := t.TempDir()
	store, err := storage.NewMVCCStorage(tmpDir)
	if err != nil {
		t.Fatalf("Failed to create storage: %v", err)
	}
	defer func() { _ = store.Close() }()

	resources := []types.Resource{
		{ID: "i-abc123", Type: "ec2", Status: "running"},
		{ID: "vol-def456", Type: "ebs", Status: "available"},
		{ID: "rds-ghi789", Type: "rds", Status: "available"},
	}

	revision, err := storeObservations(store, resources)

	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}
	if revision != 1 {
		t.Errorf("Expected revision 1, got %d", revision)
	}

	// Verify all resources were stored
	currentState, err := store.GetAllCurrentResources()
	if err != nil {
		t.Fatalf("Failed to get current state: %v", err)
	}
	if len(currentState) != 3 {
		t.Errorf("Expected 3 resources in storage, got %d", len(currentState))
	}
}

// Test detectChanges function with ChangeDetector integration
func TestDetectChanges_FirstScan(t *testing.T) {
	tmpDir := t.TempDir()
	store, err := storage.NewMVCCStorage(tmpDir)
	if err != nil {
		t.Fatalf("Failed to create storage: %v", err)
	}
	defer func() { _ = store.Close() }()

	ctx := context.Background()
	resources := []types.Resource{
		{ID: "i-new1", Type: "ec2", Status: "running"},
		{ID: "i-new2", Type: "ec2", Status: "running"},
	}

	changes := detectChanges(ctx, store, resources)

	if len(changes.New) != 2 {
		t.Errorf("Expected 2 new resources, got %d", len(changes.New))
	}
	if len(changes.Modified) != 0 {
		t.Errorf("Expected 0 modified resources, got %d", len(changes.Modified))
	}
	if len(changes.Disappeared) != 0 {
		t.Errorf("Expected 0 disappeared resources, got %d", len(changes.Disappeared))
	}
}

func TestDetectChanges_WithModifications(t *testing.T) {
	tmpDir := t.TempDir()
	store, err := storage.NewMVCCStorage(tmpDir)
	if err != nil {
		t.Fatalf("Failed to create storage: %v", err)
	}
	defer func() { _ = store.Close() }()

	ctx := context.Background()

	// First scan
	original := types.Resource{
		ID:     "i-abc123",
		Type:   "ec2",
		Status: "running",
		Tags:   types.Tags{Environment: "dev"},
	}
	_, _ = store.RecordObservation(original)

	// Second scan with modification
	modified := original
	modified.Tags.Environment = "prod" // Changed!

	changes := detectChanges(ctx, store, []types.Resource{modified})

	if len(changes.Modified) != 1 {
		t.Errorf("Expected 1 modified resource, got %d", len(changes.Modified))
	}
}

func TestDetectChanges_WithDisappearances(t *testing.T) {
	tmpDir := t.TempDir()
	store, err := storage.NewMVCCStorage(tmpDir)
	if err != nil {
		t.Fatalf("Failed to create storage: %v", err)
	}
	defer func() { _ = store.Close() }()

	ctx := context.Background()

	// First scan - 2 resources
	resources := []types.Resource{
		{ID: "i-keep", Type: "ec2", Status: "running"},
		{ID: "i-gone", Type: "ec2", Status: "running"},
	}
	_, _ = store.RecordObservationBatch(resources)

	// Second scan - only 1 resource (i-gone disappeared)
	changes := detectChanges(ctx, store, []types.Resource{
		{ID: "i-keep", Type: "ec2", Status: "running"},
	})

	if len(changes.Disappeared) != 1 {
		t.Errorf("Expected 1 disappeared resource, got %d", len(changes.Disappeared))
	}
	if changes.Disappeared[0] != "i-gone" {
		t.Errorf("Expected i-gone to disappear, got %s", changes.Disappeared[0])
	}
}

func TestConvertToChangeSet(t *testing.T) {
	created := types.Resource{ID: "i-new", Type: "ec2"}
	previous := types.Resource{ID: "i-mod", Type: "ec2", Status: "running"}
	current := types.Resource{ID: "i-mod", Type: "ec2", Status: "stopped"}

	events := []storage.ChangeEvent{
		{ChangeType: "created", ResourceID: "i-new", Current: &created},
		{ChangeType: "modified", ResourceID: "i-mod", Previous: &previous, Current: &current},
		{ChangeType: "disappeared", ResourceID: "i-gone"},
	}

	changes := convertToChangeSet(events)

	if len(changes.New) != 1 {
		t.Errorf("Expected 1 new resource, got %d", len(changes.New))
	}
	if len(changes.Modified) != 1 {
		t.Errorf("Expected 1 modified resource, got %d", len(changes.Modified))
	}
	if len(changes.Disappeared) != 1 {
		t.Errorf("Expected 1 disappeared resource, got %d", len(changes.Disappeared))
	}
}
