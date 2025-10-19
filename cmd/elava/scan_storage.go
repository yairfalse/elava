package main

import (
	"context"
	"fmt"

	"github.com/yairfalse/elava/analyzer"
	"github.com/yairfalse/elava/storage"
	"github.com/yairfalse/elava/types"
)

// ChangeSet represents detected changes between scans
type ChangeSet struct {
	New         []types.Resource `json:"new"`
	Modified    []ResourceChange `json:"modified"`
	Disappeared []string         `json:"disappeared"`
}

// ResourceChange represents a modification to a resource
type ResourceChange struct {
	Current  types.Resource `json:"current"`
	Previous types.Resource `json:"previous"`
}

// storeObservations records all resources in storage at a new revision - CLAUDE.md: Small focused function
func storeObservations(storage *storage.MVCCStorage, resources []types.Resource) (int64, error) {
	if len(resources) == 0 {
		// Still increment revision for empty batches
		return storage.CurrentRevision() + 1, nil
	}

	return storage.RecordObservationBatch(resources)
}

// detectChanges uses ChangeDetector to find differences and stores events
func detectChanges(ctx context.Context, store *storage.MVCCStorage, current []types.Resource) ChangeSet {
	// Use ChangeDetector analyzer
	detector := analyzer.NewChangeDetector(store)
	changeEvents, err := detector.DetectChanges(ctx, current)
	if err != nil {
		// Storage error during change detection
		fmt.Printf("Warning: failed to detect changes: %v\n", err)
		return ChangeSet{}
	}

	// Store change events in MVCC storage
	if len(changeEvents) > 0 {
		if err := store.StoreChangeEventBatch(ctx, changeEvents); err != nil {
			// Log but continue - events are in memory and will be displayed
			fmt.Printf("Warning: failed to store change events: %v\n", err)
		}
	}

	// Convert ChangeEvents to ChangeSet for display
	return convertToChangeSet(changeEvents)
}

// convertToChangeSet converts storage.ChangeEvents to display ChangeSet
func convertToChangeSet(events []storage.ChangeEvent) ChangeSet {
	changes := ChangeSet{}

	for _, event := range events {
		switch event.ChangeType {
		case "created":
			if event.Current != nil {
				changes.New = append(changes.New, *event.Current)
			}
		case "modified":
			if event.Current != nil && event.Previous != nil {
				changes.Modified = append(changes.Modified, ResourceChange{
					Current:  *event.Current,
					Previous: *event.Previous,
				})
			}
		case "disappeared":
			changes.Disappeared = append(changes.Disappeared, event.ResourceID)
		}
	}

	return changes
}
