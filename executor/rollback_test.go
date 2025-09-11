package executor

import (
	"context"
	"testing"
	"time"

	"github.com/yairfalse/ovi/storage"
	"github.com/yairfalse/ovi/types"
	"github.com/yairfalse/ovi/wal"
)

func TestDefaultRollbackManager_RecordExecution(t *testing.T) {
	tmpDir := t.TempDir()

	// Setup
	storage, _ := storage.NewMVCCStorage(tmpDir)
	defer func() { _ = storage.Close() }()

	walInstance, _ := wal.Open(tmpDir)
	defer func() { _ = walInstance.Close() }()

	manager := NewDefaultRollbackManager(storage, walInstance)

	// Create rollback entry
	entry := RollbackEntry{
		Decision: types.Decision{
			Action:       types.ActionCreate,
			ResourceID:   "new-resource",
			ResourceType: "ec2",
			Reason:       "Test creation",
		},
		OriginalState: nil,
		ReverseAction: types.ActionDelete,
		ExecutedAt:    time.Now(),
		CanRollback:   true,
	}

	// Record execution
	ctx := context.Background()
	err := manager.RecordExecution(ctx, entry)

	// Verify
	if err != nil {
		t.Fatalf("RecordExecution failed: %v", err)
	}

	// Check history
	history := manager.GetRollbackHistory()
	if len(history) != 1 {
		t.Errorf("History length = %d, want 1", len(history))
	}

	if history[0].Decision.ResourceID != "new-resource" {
		t.Errorf("Recorded resource = %v, want new-resource", history[0].Decision.ResourceID)
	}
}

func TestDefaultRollbackManager_CanRollback(t *testing.T) {
	tmpDir := t.TempDir()

	// Setup
	storage, _ := storage.NewMVCCStorage(tmpDir)
	defer func() { _ = storage.Close() }()

	walInstance, _ := wal.Open(tmpDir)
	defer func() { _ = walInstance.Close() }()

	manager := NewDefaultRollbackManager(storage, walInstance)
	ctx := context.Background()

	tests := []struct {
		name       string
		decision   types.Decision
		wantCan    bool
		wantReason string
	}{
		{
			name: "create action can be rolled back",
			decision: types.Decision{
				Action: types.ActionCreate,
			},
			wantCan:    true,
			wantReason: "can delete created resource",
		},
		{
			name: "tag action can be rolled back",
			decision: types.Decision{
				Action: types.ActionTag,
			},
			wantCan:    true,
			wantReason: "can remove applied tags",
		},
		{
			name: "update action cannot be rolled back",
			decision: types.Decision{
				Action: types.ActionUpdate,
			},
			wantCan:    false,
			wantReason: "updates are difficult to rollback safely",
		},
		{
			name: "delete action cannot be rolled back",
			decision: types.Decision{
				Action: types.ActionDelete,
			},
			wantCan:    false,
			wantReason: "cannot restore deleted resources",
		},
		{
			name: "terminate action cannot be rolled back",
			decision: types.Decision{
				Action: types.ActionTerminate,
			},
			wantCan:    false,
			wantReason: "cannot restore deleted resources",
		},
		{
			name: "notify action cannot be rolled back",
			decision: types.Decision{
				Action: types.ActionNotify,
			},
			wantCan:    false,
			wantReason: "notifications cannot be unsent",
		},
		{
			name: "noop action can be rolled back",
			decision: types.Decision{
				Action: types.ActionNoop,
			},
			wantCan:    true,
			wantReason: "no-op operations have no effect to rollback",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			can, reason := manager.CanRollback(ctx, tt.decision)

			if can != tt.wantCan {
				t.Errorf("CanRollback() can = %v, want %v", can, tt.wantCan)
			}

			if reason != tt.wantReason {
				t.Errorf("CanRollback() reason = %v, want %v", reason, tt.wantReason)
			}
		})
	}
}

func TestDefaultRollbackManager_Rollback(t *testing.T) {
	tmpDir := t.TempDir()

	// Setup
	storage, _ := storage.NewMVCCStorage(tmpDir)
	defer func() { _ = storage.Close() }()

	walInstance, _ := wal.Open(tmpDir)
	defer func() { _ = walInstance.Close() }()

	manager := NewDefaultRollbackManager(storage, walInstance)

	// Create entries to rollback
	entries := []RollbackEntry{
		{
			Decision: types.Decision{
				Action:       types.ActionCreate,
				ResourceID:   "resource-1",
				ResourceType: "ec2",
			},
			ReverseAction: types.ActionDelete,
			ExecutedAt:    time.Now().Add(-2 * time.Minute),
			CanRollback:   true,
		},
		{
			Decision: types.Decision{
				Action:       types.ActionTag,
				ResourceID:   "resource-2",
				ResourceType: "ec2",
			},
			ReverseAction: "untag",
			ExecutedAt:    time.Now().Add(-1 * time.Minute),
			CanRollback:   true,
		},
		{
			Decision: types.Decision{
				Action:       types.ActionDelete,
				ResourceID:   "resource-3",
				ResourceType: "ec2",
			},
			ReverseAction:  types.ActionNoop,
			ExecutedAt:     time.Now(),
			CanRollback:    false,
			RollbackReason: "cannot restore deleted resources",
		},
	}

	// Execute rollback
	ctx := context.Background()
	err := manager.Rollback(ctx, entries)

	// Should complete with errors (one entry cannot be rolled back)
	if err == nil {
		t.Error("Expected error for non-rollbackable entry")
	}

	// Verify error message contains info about failed rollback
	if err != nil {
		errStr := err.Error()
		if errStr == "" {
			t.Error("Error message should not be empty")
		}
		// Should mention the resource that couldn't be rolled back
		if !contains(errStr, "resource-3") {
			t.Errorf("Error should mention resource-3, got: %v", errStr)
		}
	}
}

func TestDefaultRollbackManager_ValidateRollbackSequence(t *testing.T) {
	tmpDir := t.TempDir()

	// Setup
	storage, _ := storage.NewMVCCStorage(tmpDir)
	defer func() { _ = storage.Close() }()

	walInstance, _ := wal.Open(tmpDir)
	defer func() { _ = walInstance.Close() }()

	manager := NewDefaultRollbackManager(storage, walInstance)

	// Create entries with dependencies
	entries := []RollbackEntry{
		{
			Decision: types.Decision{
				Action:       types.ActionCreate,
				ResourceID:   "resource-1",
				ResourceType: "ec2",
			},
			CanRollback: true,
		},
		{
			Decision: types.Decision{
				Action:       types.ActionUpdate,
				ResourceID:   "resource-1", // Same resource - dependency!
				ResourceType: "ec2",
			},
			CanRollback:    false,
			RollbackReason: "updates are difficult to rollback",
		},
		{
			Decision: types.Decision{
				Action:       types.ActionDelete,
				ResourceID:   "resource-2",
				ResourceType: "ec2",
			},
			CanRollback:    false,
			RollbackReason: "cannot restore deleted resources",
		},
	}

	// Validate sequence
	warnings := manager.ValidateRollbackSequence(entries)

	// Should have warnings for non-rollbackable entries and dependencies
	if len(warnings) < 2 {
		t.Errorf("Expected at least 2 warnings, got %d", len(warnings))
	}

	// Check for specific warnings
	hasNonRollbackableWarning := false
	hasDependencyWarning := false

	for _, warning := range warnings {
		if contains(warning, "cannot be rolled back") {
			hasNonRollbackableWarning = true
		}
		if contains(warning, "depends on") {
			hasDependencyWarning = true
		}
	}

	if !hasNonRollbackableWarning {
		t.Error("Should have warning for non-rollbackable entries")
	}

	if !hasDependencyWarning {
		t.Error("Should have warning for dependencies")
	}
}

func TestDefaultRollbackManager_ClearHistory(t *testing.T) {
	tmpDir := t.TempDir()

	// Setup
	storage, _ := storage.NewMVCCStorage(tmpDir)
	defer func() { _ = storage.Close() }()

	walInstance, _ := wal.Open(tmpDir)
	defer func() { _ = walInstance.Close() }()

	manager := NewDefaultRollbackManager(storage, walInstance)

	// Add some entries
	ctx := context.Background()
	for i := 0; i < 3; i++ {
		entry := RollbackEntry{
			Decision: types.Decision{
				Action:     types.ActionCreate,
				ResourceID: string(rune(i)),
			},
			CanRollback: true,
		}
		_ = manager.RecordExecution(ctx, entry)
	}

	// Verify history exists
	history := manager.GetRollbackHistory()
	if len(history) != 3 {
		t.Errorf("History length = %d, want 3", len(history))
	}

	// Clear history
	manager.ClearHistory()

	// Verify history is empty
	history = manager.GetRollbackHistory()
	if len(history) != 0 {
		t.Errorf("History length after clear = %d, want 0", len(history))
	}
}

func TestDefaultRollbackManager_HasDependency(t *testing.T) {
	tmpDir := t.TempDir()

	// Setup
	storage, _ := storage.NewMVCCStorage(tmpDir)
	defer func() { _ = storage.Close() }()

	walInstance, _ := wal.Open(tmpDir)
	defer func() { _ = walInstance.Close() }()

	manager := NewDefaultRollbackManager(storage, walInstance)

	tests := []struct {
		name          string
		entry1        RollbackEntry
		entry2        RollbackEntry
		hasDependency bool
	}{
		{
			name: "same resource has dependency",
			entry1: RollbackEntry{
				Decision: types.Decision{
					ResourceID: "resource-1",
				},
			},
			entry2: RollbackEntry{
				Decision: types.Decision{
					ResourceID: "resource-1",
				},
			},
			hasDependency: true,
		},
		{
			name: "different resources no dependency",
			entry1: RollbackEntry{
				Decision: types.Decision{
					ResourceID: "resource-1",
				},
			},
			entry2: RollbackEntry{
				Decision: types.Decision{
					ResourceID: "resource-2",
				},
			},
			hasDependency: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := manager.hasDependency(tt.entry1, tt.entry2)
			if result != tt.hasDependency {
				t.Errorf("hasDependency() = %v, want %v", result, tt.hasDependency)
			}
		})
	}
}

// Helper function
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && len(substr) > 0 &&
		(s[:len(substr)] == substr || contains(s[1:], substr)))
}
