package main

import (
	"testing"

	"github.com/yairfalse/ovi/storage"
	"github.com/yairfalse/ovi/types"
)

// Test storeObservations function - CLAUDE.md: Small focused tests
func TestStoreObservations_EmptyResources(t *testing.T) {
	tmpDir := t.TempDir()
	storage, err := storage.NewMVCCStorage(tmpDir)
	if err != nil {
		t.Fatalf("Failed to create storage: %v", err)
	}
	defer func() { _ = storage.Close() }()

	var resources []types.Resource

	revision, err := storeObservations(storage, resources)

	if err != nil {
		t.Errorf("Expected no error for empty resources, got %v", err)
	}
	if revision != 1 {
		t.Errorf("Expected revision 1, got %d", revision)
	}
}

func TestStoreObservations_SingleResource(t *testing.T) {
	tmpDir := t.TempDir()
	storage, err := storage.NewMVCCStorage(tmpDir)
	if err != nil {
		t.Fatalf("Failed to create storage: %v", err)
	}
	defer func() { _ = storage.Close() }()

	resources := []types.Resource{
		{
			ID:       "i-abc123",
			Type:     "ec2",
			Provider: "aws",
			Region:   "us-east-1",
			Status:   "running",
		},
	}

	revision, err := storeObservations(storage, resources)

	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}
	if revision != 1 {
		t.Errorf("Expected revision 1, got %d", revision)
	}
}

func TestStoreObservations_BatchResources(t *testing.T) {
	tmpDir := t.TempDir()
	storage, err := storage.NewMVCCStorage(tmpDir)
	if err != nil {
		t.Fatalf("Failed to create storage: %v", err)
	}
	defer func() { _ = storage.Close() }()

	resources := []types.Resource{
		{ID: "i-abc123", Type: "ec2", Status: "running"},
		{ID: "vol-def456", Type: "ebs", Status: "available"},
		{ID: "rds-ghi789", Type: "rds", Status: "available"},
	}

	revision, err := storeObservations(storage, resources)

	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}
	if revision != 1 {
		t.Errorf("Expected revision 1, got %d", revision)
	}

	// Verify all resources were stored
	currentState, err := storage.GetAllCurrentResources()
	if err != nil {
		t.Fatalf("Failed to get current state: %v", err)
	}
	if len(currentState) != 3 {
		t.Errorf("Expected 3 resources in storage, got %d", len(currentState))
	}
}

// Test getPreviousState function
func TestGetPreviousState_EmptyStorage(t *testing.T) {
	tmpDir := t.TempDir()
	storage, err := storage.NewMVCCStorage(tmpDir)
	if err != nil {
		t.Fatalf("Failed to create storage: %v", err)
	}
	defer func() { _ = storage.Close() }()

	previous, err := getPreviousState(storage)

	if err != nil {
		t.Errorf("Expected no error for empty storage, got %v", err)
	}
	if len(previous) != 0 {
		t.Errorf("Expected empty previous state, got %d resources", len(previous))
	}
}

func TestGetPreviousState_WithData(t *testing.T) {
	tmpDir := t.TempDir()
	storage, err := storage.NewMVCCStorage(tmpDir)
	if err != nil {
		t.Fatalf("Failed to create storage: %v", err)
	}
	defer func() { _ = storage.Close() }()

	// Store some resources first
	resources := []types.Resource{
		{ID: "i-abc123", Type: "ec2", Status: "running"},
	}
	_, err = storeObservations(storage, resources)
	if err != nil {
		t.Fatalf("Failed to store initial resources: %v", err)
	}

	previous, err := getPreviousState(storage)

	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}
	if len(previous) != 1 {
		t.Errorf("Expected 1 resource, got %d", len(previous))
	}
	if previous[0].ID != "i-abc123" {
		t.Errorf("Expected resource i-abc123, got %s", previous[0].ID)
	}
}

// Test detectChanges function
func TestDetectChanges_NoChanges(t *testing.T) {
	current := []types.Resource{
		{ID: "i-abc123", Type: "ec2", Status: "running"},
	}
	previous := []types.Resource{
		{ID: "i-abc123", Type: "ec2", Status: "running"},
	}

	changes := detectChanges(current, previous)

	if len(changes.New) != 0 {
		t.Errorf("Expected no new resources, got %d", len(changes.New))
	}
	if len(changes.Modified) != 0 {
		t.Errorf("Expected no modified resources, got %d", len(changes.Modified))
	}
	if len(changes.Disappeared) != 0 {
		t.Errorf("Expected no disappeared resources, got %d", len(changes.Disappeared))
	}
}

func TestDetectChanges_NewResource(t *testing.T) {
	current := []types.Resource{
		{ID: "i-abc123", Type: "ec2", Status: "running"},
		{ID: "i-def456", Type: "ec2", Status: "running"}, // New
	}
	previous := []types.Resource{
		{ID: "i-abc123", Type: "ec2", Status: "running"},
	}

	changes := detectChanges(current, previous)

	if len(changes.New) != 1 {
		t.Errorf("Expected 1 new resource, got %d", len(changes.New))
	}
	if changes.New[0].ID != "i-def456" {
		t.Errorf("Expected new resource i-def456, got %s", changes.New[0].ID)
	}
}

func TestDetectChanges_DisappearedResource(t *testing.T) {
	current := []types.Resource{
		{ID: "i-abc123", Type: "ec2", Status: "running"},
	}
	previous := []types.Resource{
		{ID: "i-abc123", Type: "ec2", Status: "running"},
		{ID: "i-def456", Type: "ec2", Status: "running"}, // Disappeared
	}

	changes := detectChanges(current, previous)

	if len(changes.Disappeared) != 1 {
		t.Errorf("Expected 1 disappeared resource, got %d", len(changes.Disappeared))
	}
	if changes.Disappeared[0] != "i-def456" {
		t.Errorf("Expected disappeared resource i-def456, got %s", changes.Disappeared[0])
	}
}

func TestDetectChanges_ModifiedResource(t *testing.T) {
	current := []types.Resource{
		{ID: "i-abc123", Type: "ec2", Status: "stopped"}, // Changed status
	}
	previous := []types.Resource{
		{ID: "i-abc123", Type: "ec2", Status: "running"},
	}

	changes := detectChanges(current, previous)

	if len(changes.Modified) != 1 {
		t.Errorf("Expected 1 modified resource, got %d", len(changes.Modified))
	}
	if changes.Modified[0].Current.ID != "i-abc123" {
		t.Errorf("Expected modified resource i-abc123, got %s", changes.Modified[0].Current.ID)
	}
	if changes.Modified[0].Previous.Status != "running" {
		t.Errorf("Expected previous status running, got %s", changes.Modified[0].Previous.Status)
	}
	if changes.Modified[0].Current.Status != "stopped" {
		t.Errorf("Expected current status stopped, got %s", changes.Modified[0].Current.Status)
	}
}

// Table-driven test for complex change scenarios
func TestDetectChanges_ComplexScenarios(t *testing.T) {
	tests := []struct {
		name     string
		current  []types.Resource
		previous []types.Resource
		wantNew  int
		wantMod  int
		wantDis  int
	}{
		{
			name:     "empty to empty",
			current:  []types.Resource{},
			previous: []types.Resource{},
			wantNew:  0,
			wantMod:  0,
			wantDis:  0,
		},
		{
			name: "first scan",
			current: []types.Resource{
				{ID: "i-abc123", Status: "running"},
			},
			previous: []types.Resource{},
			wantNew:  1,
			wantMod:  0,
			wantDis:  0,
		},
		{
			name:    "everything disappeared",
			current: []types.Resource{},
			previous: []types.Resource{
				{ID: "i-abc123", Status: "running"},
			},
			wantNew: 0,
			wantMod: 0,
			wantDis: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			changes := detectChanges(tt.current, tt.previous)

			if len(changes.New) != tt.wantNew {
				t.Errorf("New: got %d, want %d", len(changes.New), tt.wantNew)
			}
			if len(changes.Modified) != tt.wantMod {
				t.Errorf("Modified: got %d, want %d", len(changes.Modified), tt.wantMod)
			}
			if len(changes.Disappeared) != tt.wantDis {
				t.Errorf("Disappeared: got %d, want %d", len(changes.Disappeared), tt.wantDis)
			}
		})
	}
}
