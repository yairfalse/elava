package analyzer

import (
	"context"
	"fmt"
	"time"

	"github.com/yairfalse/elava/storage"
	"github.com/yairfalse/elava/types"
)

// ChangeDetectorImpl detects changes between scans
type ChangeDetectorImpl struct {
	storage *storage.MVCCStorage
}

// NewChangeDetector creates a new change detector
func NewChangeDetector(store *storage.MVCCStorage) *ChangeDetectorImpl {
	return &ChangeDetectorImpl{
		storage: store,
	}
}

// DetectChanges compares current scan with previous revision
func (c *ChangeDetectorImpl) DetectChanges(ctx context.Context, currentScan []types.Resource) ([]storage.ChangeEvent, error) {
	// Get current revision before comparison
	revision := c.storage.CurrentRevision()

	// Get previous resources from storage
	previousStates, err := c.storage.GetAllCurrentResources()
	if err != nil {
		// Check if this is truly a first scan (no resources stored)
		resourceCount, _, _ := c.storage.Stats()
		if resourceCount == 0 {
			// First scan - all resources are new
			return c.allCreated(currentScan, revision), nil
		}
		// Real storage error - return it
		return nil, fmt.Errorf("failed to get previous resources: %w", err)
	}

	// Build maps for comparison
	previousMap, err := c.buildResourceMapFromStates(previousStates)
	if err != nil {
		return nil, fmt.Errorf("failed to build resource map: %w", err)
	}
	currentMap := types.BuildResourceMap(currentScan)

	var events []storage.ChangeEvent

	// Check for new and modified resources
	for id, current := range currentMap {
		if previous, exists := previousMap[id]; exists {
			// Resource existed - check if modified
			if c.resourceChanged(previous, current) {
				events = append(events, c.buildModifiedEvent(previous, current, revision))
			}
		} else {
			// New resource
			events = append(events, c.buildCreatedEvent(current, revision))
		}
	}

	// Check for disappeared resources
	for id, previous := range previousMap {
		if _, exists := currentMap[id]; !exists {
			events = append(events, c.buildDisappearedEvent(previous, revision))
		}
	}

	return events, nil
}

// allCreated generates created events for all resources (first scan)
func (c *ChangeDetectorImpl) allCreated(resources []types.Resource, revision int64) []storage.ChangeEvent {
	events := make([]storage.ChangeEvent, 0, len(resources))
	for _, resource := range resources {
		events = append(events, c.buildCreatedEvent(resource, revision))
	}
	return events
}

// resourceChanged checks if resource has meaningful changes
func (c *ChangeDetectorImpl) resourceChanged(previous, current types.Resource) bool {
	// Check status change
	if previous.Status != current.Status {
		return true
	}

	// Check tag changes
	if previous.Tags != current.Tags {
		return true
	}

	// Check name change
	if previous.Name != current.Name {
		return true
	}

	// Check region/account changes (rare but important)
	if previous.Region != current.Region || previous.AccountID != current.AccountID {
		return true
	}

	// Check metadata changes
	if c.metadataChanged(previous.Metadata, current.Metadata) {
		return true
	}

	return false
}

// metadataChanged checks for significant metadata changes
func (c *ChangeDetectorImpl) metadataChanged(previous, current types.ResourceMetadata) bool {
	// Check critical infrastructure changes
	if previous.InstanceType != current.InstanceType {
		return true
	}

	// Check security changes
	if previous.Encrypted != current.Encrypted {
		return true
	}

	if previous.PublicIP != current.PublicIP {
		return true
	}

	// Check operational changes
	if previous.State != current.State {
		return true
	}

	// No significant metadata changes
	return false
}

// buildCreatedEvent creates a "created" event
func (c *ChangeDetectorImpl) buildCreatedEvent(resource types.Resource, revision int64) storage.ChangeEvent {
	return storage.ChangeEvent{
		ResourceID: resource.ID,
		ChangeType: "created",
		Timestamp:  time.Now(),
		Revision:   revision,
		Current:    &resource,
		Previous:   nil,
	}
}

// buildModifiedEvent creates a "modified" event
func (c *ChangeDetectorImpl) buildModifiedEvent(previous, current types.Resource, revision int64) storage.ChangeEvent {
	return storage.ChangeEvent{
		ResourceID: previous.ID,
		ChangeType: "modified",
		Timestamp:  time.Now(),
		Revision:   revision,
		Current:    &current,
		Previous:   &previous,
	}
}

// buildDisappearedEvent creates a "disappeared" event
func (c *ChangeDetectorImpl) buildDisappearedEvent(resource types.Resource, revision int64) storage.ChangeEvent {
	return storage.ChangeEvent{
		ResourceID: resource.ID,
		ChangeType: "disappeared",
		Timestamp:  time.Now(),
		Revision:   revision,
		Current:    nil,
		Previous:   &resource,
	}
}

// buildResourceMapFromStates converts ResourceStates to Resource map
func (c *ChangeDetectorImpl) buildResourceMapFromStates(states []*storage.ResourceState) (map[string]types.Resource, error) {
	resourceMap := make(map[string]types.Resource)

	for _, state := range states {
		if !state.Exists {
			continue // Skip disappeared resources
		}

		// Fetch full resource data from storage
		resource, err := c.storage.GetLatestResource(state.ResourceID)
		if err != nil {
			// Check if it's a "not found" error vs real storage error
			// For now, we'll skip not-found but that's a data inconsistency
			// In production, consider logging this
			continue
		}

		resourceMap[state.ResourceID] = *resource
	}

	return resourceMap, nil
}
