package executor

import (
	"context"
	"fmt"
	"time"

	"github.com/yairfalse/ovi/storage"
	"github.com/yairfalse/ovi/types"
	"github.com/yairfalse/ovi/wal"
)

// DefaultRollbackManager implements rollback functionality
type DefaultRollbackManager struct {
	storage *storage.MVCCStorage
	wal     *wal.WAL
	entries []RollbackEntry
}

// NewDefaultRollbackManager creates a new rollback manager
func NewDefaultRollbackManager(storage *storage.MVCCStorage, walInstance *wal.WAL) *DefaultRollbackManager {
	return &DefaultRollbackManager{
		storage: storage,
		wal:     walInstance,
		entries: make([]RollbackEntry, 0),
	}
}

// RecordExecution records an execution for potential rollback
func (rm *DefaultRollbackManager) RecordExecution(ctx context.Context, entry RollbackEntry) error {
	// Log the rollback entry to WAL
	if err := rm.wal.Append(wal.EntryExecuted, entry.Decision.ResourceID, entry); err != nil {
		return fmt.Errorf("failed to log rollback entry: %w", err)
	}

	// Store in memory for immediate rollback capability
	rm.entries = append(rm.entries, entry)

	return nil
}

// Rollback attempts to rollback a set of operations
func (rm *DefaultRollbackManager) Rollback(ctx context.Context, entries []RollbackEntry) error {
	var rollbackErrors []string

	// Process rollbacks in reverse order (LIFO)
	for i := len(entries) - 1; i >= 0; i-- {
		entry := entries[i]

		if !entry.CanRollback {
			rollbackErrors = append(rollbackErrors,
				fmt.Sprintf("Cannot rollback %s on %s: %s",
					entry.Decision.Action, entry.Decision.ResourceID, entry.RollbackReason))
			continue
		}

		if err := rm.rollbackSingle(ctx, entry); err != nil {
			rollbackErrors = append(rollbackErrors,
				fmt.Sprintf("Failed to rollback %s on %s: %v",
					entry.Decision.Action, entry.Decision.ResourceID, err))
		}
	}

	if len(rollbackErrors) > 0 {
		return fmt.Errorf("rollback completed with errors: %v", rollbackErrors)
	}

	return nil
}

// CanRollback determines if a decision can be rolled back
func (rm *DefaultRollbackManager) CanRollback(ctx context.Context, decision types.Decision) (bool, string) {
	switch decision.Action {
	case types.ActionCreate:
		return true, "can delete created resource"
	case types.ActionTag:
		return true, "can remove applied tags"
	case types.ActionUpdate:
		return false, "updates are difficult to rollback safely"
	case types.ActionDelete, types.ActionTerminate:
		return false, "cannot restore deleted resources"
	case types.ActionNotify:
		return false, "notifications cannot be unsent"
	case types.ActionNoop:
		return true, "no-op operations have no effect to rollback"
	default:
		return false, fmt.Sprintf("unknown action: %s", decision.Action)
	}
}

// rollbackSingle performs rollback for a single entry
func (rm *DefaultRollbackManager) rollbackSingle(ctx context.Context, entry RollbackEntry) error {
	decision := entry.Decision

	// Log rollback start
	if err := rm.wal.Append("rollback_start", decision.ResourceID, entry); err != nil {
		return fmt.Errorf("failed to log rollback start: %w", err)
	}

	var err error
	switch entry.ReverseAction {
	case types.ActionDelete:
		err = rm.rollbackCreate(ctx, entry)
	case "untag":
		err = rm.rollbackTag(ctx, entry)
	case types.ActionNoop:
		// No rollback needed
		err = nil
	default:
		err = fmt.Errorf("unsupported reverse action: %s", entry.ReverseAction)
	}

	// Log rollback result
	if err != nil {
		if walErr := rm.wal.AppendError("rollback_failed", decision.ResourceID, entry, err); walErr != nil {
			return fmt.Errorf("rollback failed and WAL error: rollback: %w, wal: %w", err, walErr)
		}
		return err
	}

	if err := rm.wal.Append("rollback_success", decision.ResourceID, entry); err != nil {
		// Rollback succeeded but WAL failed - log warning
		fmt.Printf("Warning: rollback succeeded but WAL logging failed: %v\n", err)
	}

	return nil
}

// rollbackCreate rolls back a create operation by deleting the resource
func (rm *DefaultRollbackManager) rollbackCreate(ctx context.Context, entry RollbackEntry) error {
	// This would need access to the provider to actually delete the resource
	// For now, we'll just record the rollback intention

	rollbackDecision := types.Decision{
		Action:       types.ActionDelete,
		ResourceID:   entry.Decision.ResourceID,
		ResourceType: entry.Decision.ResourceType,
		Reason:       fmt.Sprintf("Rollback of create operation executed at %s", entry.ExecutedAt.Format(time.RFC3339)),
		IsBlessed:    false, // Rollback operations bypass blessed protection
		CreatedAt:    time.Now(),
	}

	// Record the rollback decision
	if err := rm.wal.Append(wal.EntryDecided, rollbackDecision.ResourceID, rollbackDecision); err != nil {
		return fmt.Errorf("failed to log rollback decision: %w", err)
	}

	// In a full implementation, this would:
	// 1. Get the appropriate provider
	// 2. Call provider.DeleteResource(ctx, entry.Decision.ResourceID)
	// 3. Update storage with the deletion

	return nil
}

// rollbackTag rolls back a tag operation by removing the tags
func (rm *DefaultRollbackManager) rollbackTag(ctx context.Context, entry RollbackEntry) error {
	// This would need access to the provider to actually remove tags
	// For now, we'll just record the rollback intention

	rollbackDecision := types.Decision{
		Action:       "untag",
		ResourceID:   entry.Decision.ResourceID,
		ResourceType: entry.Decision.ResourceType,
		Reason:       fmt.Sprintf("Rollback of tag operation executed at %s", entry.ExecutedAt.Format(time.RFC3339)),
		IsBlessed:    false, // Rollback operations bypass blessed protection
		CreatedAt:    time.Now(),
	}

	// Record the rollback decision
	if err := rm.wal.Append(wal.EntryDecided, rollbackDecision.ResourceID, rollbackDecision); err != nil {
		return fmt.Errorf("failed to log rollback decision: %w", err)
	}

	// In a full implementation, this would:
	// 1. Get the appropriate provider
	// 2. Call provider.TagResource with empty tags to remove them
	// 3. Update storage with the tag changes

	return nil
}

// GetRollbackHistory returns recent rollback entries
func (rm *DefaultRollbackManager) GetRollbackHistory() []RollbackEntry {
	// Return a copy to prevent external modification
	history := make([]RollbackEntry, len(rm.entries))
	copy(history, rm.entries)
	return history
}

// ClearHistory clears the in-memory rollback history
func (rm *DefaultRollbackManager) ClearHistory() {
	rm.entries = make([]RollbackEntry, 0)
}

// ValidateRollbackSequence checks if a sequence of operations can be safely rolled back
func (rm *DefaultRollbackManager) ValidateRollbackSequence(entries []RollbackEntry) []string {
	var warnings []string

	for i, entry := range entries {
		if !entry.CanRollback {
			warnings = append(warnings,
				fmt.Sprintf("Entry %d (%s on %s) cannot be rolled back: %s",
					i, entry.Decision.Action, entry.Decision.ResourceID, entry.RollbackReason))
		}

		// Check for dependencies
		for j := i + 1; j < len(entries); j++ {
			if rm.hasDependency(entry, entries[j]) {
				warnings = append(warnings,
					fmt.Sprintf("Entry %d depends on entry %d, rollback order may cause issues", i, j))
			}
		}
	}

	return warnings
}

// hasDependency checks if one rollback entry depends on another
func (rm *DefaultRollbackManager) hasDependency(entry1, entry2 RollbackEntry) bool {
	// Simple dependency check - in a full implementation this would be more sophisticated

	// If both entries affect the same resource, there's a dependency
	if entry1.Decision.ResourceID == entry2.Decision.ResourceID {
		return true
	}

	// Check for resource relationship dependencies (e.g., VPC and subnets)
	// This would require more complex logic in a full implementation

	return false
}
