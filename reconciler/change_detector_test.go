package reconciler

import (
	"context"
	"testing"

	"github.com/yairfalse/elava/storage"
	"github.com/yairfalse/elava/types"
)

func TestTemporalChangeDetector_DetectChanges_NewResources(t *testing.T) {
	// Create storage with previous observation
	tmpDir := t.TempDir()
	mvccStorage, err := storage.NewMVCCStorage(tmpDir)
	if err != nil {
		t.Fatalf("Failed to create MVCC storage: %v", err)
	}
	defer func() {
		if err := mvccStorage.Close(); err != nil {
			t.Errorf("Failed to close storage: %v", err)
		}
	}()

	// Record a previous observation so it's not first scan
	previous := types.Resource{
		ID:       "i-existing",
		Type:     "ec2",
		Provider: "aws",
		Status:   "running",
		Tags:     types.Tags{ElavaManaged: true},
	}
	_, err = mvccStorage.RecordObservation(previous)
	if err != nil {
		t.Fatalf("Failed to record previous observation: %v", err)
	}

	detector := NewTemporalChangeDetector(mvccStorage)

	// Current observation with new resources
	current := []types.Resource{
		previous, // Keep existing
		{
			ID:       "i-new1",
			Type:     "ec2",
			Provider: "aws",
			Status:   "running",
			Tags:     types.Tags{ElavaManaged: true, ElavaOwner: "team1"},
		},
		{
			ID:       "i-new2",
			Type:     "ec2",
			Provider: "aws",
			Status:   "running",
			Tags:     types.Tags{}, // Unmanaged
		},
	}

	changes, err := detector.DetectChanges(context.Background(), current)
	if err != nil {
		t.Fatalf("DetectChanges failed: %v", err)
	}

	if len(changes) != 2 {
		t.Errorf("Expected 2 changes, got %d", len(changes))
	}

	// Check managed resource appeared
	if changes[0].Type != ChangeAppeared {
		t.Errorf("Expected ChangeAppeared, got %v", changes[0].Type)
	}

	// Check unmanaged resource detected
	if changes[1].Type != ChangeUnmanaged {
		t.Errorf("Expected ChangeUnmanaged, got %v", changes[1].Type)
	}
}

func TestTemporalChangeDetector_DetectChanges_DisappearedResources(t *testing.T) {
	tmpDir := t.TempDir()
	mvccStorage, err := storage.NewMVCCStorage(tmpDir)
	if err != nil {
		t.Fatalf("Failed to create MVCC storage: %v", err)
	}
	defer func() {
		if err := mvccStorage.Close(); err != nil {
			t.Errorf("Failed to close storage: %v", err)
		}
	}()

	// Record previous observation
	previous := types.Resource{
		ID:       "i-old",
		Type:     "ec2",
		Provider: "aws",
		Status:   "running",
		Tags:     types.Tags{ElavaManaged: true},
	}
	_, err = mvccStorage.RecordObservation(previous)
	if err != nil {
		t.Fatalf("Failed to record previous observation: %v", err)
	}

	detector := NewTemporalChangeDetector(mvccStorage)

	// Current observation - resource is gone
	current := []types.Resource{}

	changes, err := detector.DetectChanges(context.Background(), current)
	if err != nil {
		t.Fatalf("DetectChanges failed: %v", err)
	}

	if len(changes) != 1 {
		t.Errorf("Expected 1 change, got %d", len(changes))
	}

	if changes[0].Type != ChangeDisappeared {
		t.Errorf("Expected ChangeDisappeared, got %v", changes[0].Type)
	}

	if changes[0].ResourceID != "i-old" {
		t.Errorf("Expected resource i-old, got %s", changes[0].ResourceID)
	}
}

func TestTemporalChangeDetector_DetectChanges_StatusChanged(t *testing.T) {
	tmpDir := t.TempDir()
	mvccStorage, err := storage.NewMVCCStorage(tmpDir)
	if err != nil {
		t.Fatalf("Failed to create MVCC storage: %v", err)
	}
	defer func() {
		if err := mvccStorage.Close(); err != nil {
			t.Errorf("Failed to close storage: %v", err)
		}
	}()

	// Record previous observation
	previous := types.Resource{
		ID:       "i-123",
		Type:     "ec2",
		Provider: "aws",
		Status:   "running",
		Tags:     types.Tags{ElavaManaged: true},
	}
	_, err = mvccStorage.RecordObservation(previous)
	if err != nil {
		t.Fatalf("Failed to record previous observation: %v", err)
	}

	detector := NewTemporalChangeDetector(mvccStorage)

	// Current observation - status changed
	current := []types.Resource{
		{
			ID:       "i-123",
			Type:     "ec2",
			Provider: "aws",
			Status:   "stopped", // Changed!
			Tags:     types.Tags{ElavaManaged: true},
		},
	}

	changes, err := detector.DetectChanges(context.Background(), current)
	if err != nil {
		t.Fatalf("DetectChanges failed: %v", err)
	}

	if len(changes) != 1 {
		t.Errorf("Expected 1 change, got %d", len(changes))
	}

	if changes[0].Type != ChangeStatusChanged {
		t.Errorf("Expected ChangeStatusChanged, got %v", changes[0].Type)
	}

	if changes[0].Metadata["previous_status"] != "running" {
		t.Errorf("Expected previous_status=running")
	}
	if changes[0].Metadata["current_status"] != "stopped" {
		t.Errorf("Expected current_status=stopped")
	}
}

func TestTemporalChangeDetector_DetectChanges_TagDrift(t *testing.T) {
	tmpDir := t.TempDir()
	mvccStorage, err := storage.NewMVCCStorage(tmpDir)
	if err != nil {
		t.Fatalf("Failed to create MVCC storage: %v", err)
	}
	defer func() {
		if err := mvccStorage.Close(); err != nil {
			t.Errorf("Failed to close storage: %v", err)
		}
	}()

	// Record previous observation
	previous := types.Resource{
		ID:       "i-123",
		Type:     "ec2",
		Provider: "aws",
		Status:   "running",
		Tags: types.Tags{
			ElavaManaged: true,
			ElavaOwner:   "team1",
			Environment:  "prod",
		},
	}
	_, err = mvccStorage.RecordObservation(previous)
	if err != nil {
		t.Fatalf("Failed to record previous observation: %v", err)
	}

	detector := NewTemporalChangeDetector(mvccStorage)

	// Current observation - tags changed
	current := []types.Resource{
		{
			ID:       "i-123",
			Type:     "ec2",
			Provider: "aws",
			Status:   "running",
			Tags: types.Tags{
				ElavaManaged: true,
				ElavaOwner:   "team2", // Changed!
				Environment:  "prod",
			},
		},
	}

	changes, err := detector.DetectChanges(context.Background(), current)
	if err != nil {
		t.Fatalf("DetectChanges failed: %v", err)
	}

	if len(changes) != 1 {
		t.Errorf("Expected 1 change, got %d", len(changes))
	}

	if changes[0].Type != ChangeTagDrift {
		t.Errorf("Expected ChangeTagDrift, got %v", changes[0].Type)
	}
}

func TestTemporalChangeDetector_DetectChanges_NoChanges(t *testing.T) {
	tmpDir := t.TempDir()
	mvccStorage, err := storage.NewMVCCStorage(tmpDir)
	if err != nil {
		t.Fatalf("Failed to create MVCC storage: %v", err)
	}
	defer func() {
		if err := mvccStorage.Close(); err != nil {
			t.Errorf("Failed to close storage: %v", err)
		}
	}()

	// Record previous observation
	resource := types.Resource{
		ID:       "i-123",
		Type:     "ec2",
		Provider: "aws",
		Status:   "running",
		Tags:     types.Tags{ElavaManaged: true},
	}
	_, err = mvccStorage.RecordObservation(resource)
	if err != nil {
		t.Fatalf("Failed to record previous observation: %v", err)
	}

	detector := NewTemporalChangeDetector(mvccStorage)

	// Current observation - identical
	current := []types.Resource{resource}

	changes, err := detector.DetectChanges(context.Background(), current)
	if err != nil {
		t.Fatalf("DetectChanges failed: %v", err)
	}

	if len(changes) != 0 {
		t.Errorf("Expected 0 changes (no drift), got %d", len(changes))
	}
}

func TestTemporalChangeDetector_DetectChanges_Mixed(t *testing.T) {
	tmpDir := t.TempDir()
	mvccStorage, err := storage.NewMVCCStorage(tmpDir)
	if err != nil {
		t.Fatalf("Failed to create MVCC storage: %v", err)
	}
	defer func() {
		if err := mvccStorage.Close(); err != nil {
			t.Errorf("Failed to close storage: %v", err)
		}
	}()

	// Record previous observations
	resources := []types.Resource{
		{
			ID:       "i-existing",
			Type:     "ec2",
			Provider: "aws",
			Status:   "running",
			Tags:     types.Tags{ElavaManaged: true},
		},
		{
			ID:       "i-disappeared",
			Type:     "ec2",
			Provider: "aws",
			Status:   "running",
			Tags:     types.Tags{ElavaManaged: true},
		},
	}

	for _, r := range resources {
		_, err = mvccStorage.RecordObservation(r)
		if err != nil {
			t.Fatalf("Failed to record observation: %v", err)
		}
	}

	detector := NewTemporalChangeDetector(mvccStorage)

	// Current observation
	current := []types.Resource{
		{
			ID:       "i-existing",
			Type:     "ec2",
			Provider: "aws",
			Status:   "running",
			Tags:     types.Tags{ElavaManaged: true},
		}, // No change
		{
			ID:       "i-new",
			Type:     "ec2",
			Provider: "aws",
			Status:   "running",
			Tags:     types.Tags{ElavaManaged: true},
		}, // Appeared
		// i-disappeared is missing
	}

	changes, err := detector.DetectChanges(context.Background(), current)
	if err != nil {
		t.Fatalf("DetectChanges failed: %v", err)
	}

	// Should detect: 1 appeared, 1 disappeared (i-existing has no changes)
	if len(changes) != 2 {
		t.Errorf("Expected 2 changes, got %d", len(changes))
	}

	// Check change types
	hasAppeared := false
	hasDisappeared := false
	for _, change := range changes {
		if change.Type == ChangeAppeared {
			hasAppeared = true
		}
		if change.Type == ChangeDisappeared {
			hasDisappeared = true
		}
	}

	if !hasAppeared {
		t.Error("Expected ChangeAppeared not found")
	}
	if !hasDisappeared {
		t.Error("Expected ChangeDisappeared not found")
	}
}

func TestHasTagDrift(t *testing.T) {
	tests := []struct {
		name     string
		current  types.Tags
		previous types.Tags
		want     bool
	}{
		{
			name: "no drift",
			current: types.Tags{
				ElavaManaged: true,
				ElavaOwner:   "team1",
				Environment:  "prod",
			},
			previous: types.Tags{
				ElavaManaged: true,
				ElavaOwner:   "team1",
				Environment:  "prod",
			},
			want: false,
		},
		{
			name: "owner changed",
			current: types.Tags{
				ElavaManaged: true,
				ElavaOwner:   "team2",
			},
			previous: types.Tags{
				ElavaManaged: true,
				ElavaOwner:   "team1",
			},
			want: true,
		},
		{
			name: "environment changed",
			current: types.Tags{
				Environment: "staging",
			},
			previous: types.Tags{
				Environment: "prod",
			},
			want: true,
		},
		{
			name: "managed flag changed",
			current: types.Tags{
				ElavaManaged: false,
			},
			previous: types.Tags{
				ElavaManaged: true,
			},
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := hasTagDrift(tt.current, tt.previous)
			if got != tt.want {
				t.Errorf("hasTagDrift() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestHasModifications(t *testing.T) {
	tests := []struct {
		name     string
		current  types.Resource
		previous types.Resource
		want     bool
	}{
		{
			name: "no modifications",
			current: types.Resource{
				Type:     "ec2",
				Region:   "us-east-1",
				Provider: "aws",
				Name:     "instance-1",
			},
			previous: types.Resource{
				Type:     "ec2",
				Region:   "us-east-1",
				Provider: "aws",
				Name:     "instance-1",
			},
			want: false,
		},
		{
			name: "type changed",
			current: types.Resource{
				Type: "rds",
			},
			previous: types.Resource{
				Type: "ec2",
			},
			want: true,
		},
		{
			name: "region changed",
			current: types.Resource{
				Region: "us-west-2",
			},
			previous: types.Resource{
				Region: "us-east-1",
			},
			want: true,
		},
		{
			name: "name changed",
			current: types.Resource{
				Name: "new-name",
			},
			previous: types.Resource{
				Name: "old-name",
			},
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := hasModifications(tt.current, tt.previous)
			if got != tt.want {
				t.Errorf("hasModifications() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestNewTemporalChangeDetector(t *testing.T) {
	tmpDir := t.TempDir()
	mvccStorage, err := storage.NewMVCCStorage(tmpDir)
	if err != nil {
		t.Fatalf("Failed to create MVCC storage: %v", err)
	}
	defer func() {
		if err := mvccStorage.Close(); err != nil {
			t.Errorf("Failed to close storage: %v", err)
		}
	}()
	detector := NewTemporalChangeDetector(mvccStorage)

	if detector == nil {
		t.Fatal("NewTemporalChangeDetector returned nil")
	}

	if detector.storage != mvccStorage {
		t.Error("ChangeDetector storage not set correctly")
	}
}

func TestTemporalChangeDetector_DetectChanges_FirstScanBaseline(t *testing.T) {
	// Create empty storage (first scan scenario)
	tmpDir := t.TempDir()
	mvccStorage, err := storage.NewMVCCStorage(tmpDir)
	if err != nil {
		t.Fatalf("Failed to create MVCC storage: %v", err)
	}
	defer func() {
		if err := mvccStorage.Close(); err != nil {
			t.Errorf("Failed to close storage: %v", err)
		}
	}()

	detector := NewTemporalChangeDetector(mvccStorage)

	// First scan with resources
	current := []types.Resource{
		{
			ID:       "i-123",
			Type:     "ec2",
			Provider: "aws",
			Status:   "running",
			Tags:     types.Tags{ElavaManaged: true},
		},
		{
			ID:       "db-456",
			Type:     "rds",
			Provider: "aws",
			Status:   "available",
			Tags:     types.Tags{ElavaManaged: true},
		},
	}

	changes, err := detector.DetectChanges(context.Background(), current)
	if err != nil {
		t.Fatalf("DetectChanges failed: %v", err)
	}

	// All resources should be marked as baseline
	if len(changes) != 2 {
		t.Errorf("Expected 2 baseline changes, got %d", len(changes))
	}

	for i, change := range changes {
		if change.Type != ChangeBaseline {
			t.Errorf("Change %d: expected ChangeBaseline, got %v", i, change.Type)
		}
		if change.Details != "Baseline observation" {
			t.Errorf("Change %d: expected 'Baseline observation', got %q", i, change.Details)
		}
		if change.Current == nil {
			t.Errorf("Change %d: Current should not be nil", i)
		}
	}
}

func TestTemporalChangeDetector_DetectChanges_SecondScanAfterBaseline(t *testing.T) {
	// Create storage and record baseline
	tmpDir := t.TempDir()
	mvccStorage, err := storage.NewMVCCStorage(tmpDir)
	if err != nil {
		t.Fatalf("Failed to create MVCC storage: %v", err)
	}
	defer func() {
		if err := mvccStorage.Close(); err != nil {
			t.Errorf("Failed to close storage: %v", err)
		}
	}()

	// Record baseline observations
	baseline := types.Resource{
		ID:       "i-123",
		Type:     "ec2",
		Provider: "aws",
		Status:   "running",
		Tags:     types.Tags{ElavaManaged: true},
	}
	_, err = mvccStorage.RecordObservation(baseline)
	if err != nil {
		t.Fatalf("Failed to record baseline: %v", err)
	}

	detector := NewTemporalChangeDetector(mvccStorage)

	// Second scan - one existing, one new
	current := []types.Resource{
		baseline, // No change
		{
			ID:       "i-new",
			Type:     "ec2",
			Provider: "aws",
			Status:   "running",
			Tags:     types.Tags{ElavaManaged: true},
		}, // New resource
	}

	changes, err := detector.DetectChanges(context.Background(), current)
	if err != nil {
		t.Fatalf("DetectChanges failed: %v", err)
	}

	// Should detect only the new resource, not baseline
	if len(changes) != 1 {
		t.Errorf("Expected 1 change (appeared), got %d", len(changes))
	}

	if changes[0].Type != ChangeAppeared {
		t.Errorf("Expected ChangeAppeared, got %v", changes[0].Type)
	}
	if changes[0].ResourceID != "i-new" {
		t.Errorf("Expected resource i-new, got %s", changes[0].ResourceID)
	}
}
