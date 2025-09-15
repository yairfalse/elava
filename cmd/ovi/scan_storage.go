package main

import (
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

// getPreviousState retrieves the last known state from storage - CLAUDE.md: Small focused function
func getPreviousState(storage *storage.MVCCStorage) ([]types.Resource, error) {
	states, err := storage.GetAllCurrentResources()
	if err != nil {
		return nil, err
	}

	// Convert ResourceState back to Resource
	// Note: This is simplified - in real implementation we'd need to store full Resource data
	var resources []types.Resource
	for _, state := range states {
		resource := types.Resource{
			ID:   state.ResourceID,
			Type: state.Type,
		}
		resources = append(resources, resource)
	}

	return resources, nil
}

// detectChanges compares current and previous resources to find differences - CLAUDE.md: Small focused function
func detectChanges(current, previous []types.Resource) ChangeSet {
	// Build lookup maps for efficient comparison
	currentMap := buildResourceMap(current)
	previousMap := buildResourceMap(previous)

	changes := ChangeSet{}

	// Find new resources
	for id, resource := range currentMap {
		if _, existed := previousMap[id]; !existed {
			changes.New = append(changes.New, resource)
		}
	}

	// Find disappeared resources
	for id := range previousMap {
		if _, exists := currentMap[id]; !exists {
			changes.Disappeared = append(changes.Disappeared, id)
		}
	}

	// Find modified resources
	for id, currentResource := range currentMap {
		if previousResource, existed := previousMap[id]; existed {
			if resourceChanged(currentResource, previousResource) {
				changes.Modified = append(changes.Modified, ResourceChange{
					Current:  currentResource,
					Previous: previousResource,
				})
			}
		}
	}

	return changes
}

// buildResourceMap creates ID->Resource lookup map - CLAUDE.md: Small helper function
func buildResourceMap(resources []types.Resource) map[string]types.Resource {
	resourceMap := make(map[string]types.Resource)
	for _, resource := range resources {
		resourceMap[resource.ID] = resource
	}
	return resourceMap
}

// resourceChanged checks if resource has meaningful changes - CLAUDE.md: Small helper function
func resourceChanged(current, previous types.Resource) bool {
	// Compare key fields that indicate meaningful changes
	return current.Status != previous.Status ||
		current.Tags.OviOwner != previous.Tags.OviOwner ||
		current.Tags.Environment != previous.Tags.Environment ||
		current.Region != previous.Region
}
