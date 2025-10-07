package reconciler

import (
	"context"
	"time"

	"github.com/yairfalse/elava/storage"
	"github.com/yairfalse/elava/types"
)

// ChangeDetector detects changes between current and previous observations
// Uses MVCC storage to query temporal history
type ChangeDetector interface {
	DetectChanges(ctx context.Context, current []types.Resource) ([]Change, error)
}

// Change represents a detected change between observations over time
type Change struct {
	Type       ChangeType      `json:"type"`
	ResourceID string          `json:"resource_id"`
	Current    *types.Resource `json:"current,omitempty"`  // Current observation
	Previous   *types.Resource `json:"previous,omitempty"` // Previous observation from MVCC
	Timestamp  time.Time       `json:"timestamp"`
	Details    string          `json:"details"`
	Metadata   map[string]any  `json:"metadata,omitempty"`
}

// ChangeType categorizes detected changes
type ChangeType string

const (
	// ChangeAppeared - new resource observed
	ChangeAppeared ChangeType = "appeared"

	// ChangeDisappeared - resource no longer observed
	ChangeDisappeared ChangeType = "disappeared"

	// ChangeModified - resource configuration changed
	ChangeModified ChangeType = "modified"

	// ChangeTagDrift - resource tags changed
	ChangeTagDrift ChangeType = "tag_drift"

	// ChangeStatusChanged - resource status changed
	ChangeStatusChanged ChangeType = "status_changed"

	// ChangeUnmanaged - resource exists but not Elava-managed
	ChangeUnmanaged ChangeType = "unmanaged"
)

// TemporalChangeDetector implements change detection using MVCC storage
type TemporalChangeDetector struct {
	storage *storage.MVCCStorage
}

// NewTemporalChangeDetector creates a change detector with MVCC storage
func NewTemporalChangeDetector(storage *storage.MVCCStorage) *TemporalChangeDetector {
	return &TemporalChangeDetector{
		storage: storage,
	}
}

// DetectChanges compares current observations with previous state
func (d *TemporalChangeDetector) DetectChanges(ctx context.Context, current []types.Resource) ([]Change, error) {
	var changes []Change

	// Build current resource map
	currentMap := buildResourceMap(current)

	// Get all resource IDs we've seen before
	previousIDs, err := d.getPreviousResourceIDs(ctx)
	if err != nil {
		return nil, err
	}

	// Detect appeared and modified resources
	for _, resource := range current {
		change := d.detectResourceChange(ctx, resource, previousIDs)
		if change != nil {
			changes = append(changes, *change)
		}
	}

	// Detect disappeared resources
	disappeared := d.detectDisappeared(ctx, currentMap, previousIDs)
	changes = append(changes, disappeared...)

	return changes, nil
}

// detectResourceChange detects changes for a single resource
func (d *TemporalChangeDetector) detectResourceChange(ctx context.Context, resource types.Resource, previousIDs map[string]bool) *Change {
	// Check if this is a new resource
	if !previousIDs[resource.ID] {
		return d.createAppearedChange(resource)
	}

	// Get previous observation
	previous, err := d.storage.GetLatestResource(resource.ID)
	if err != nil {
		// Can't get previous - treat as appeared
		return d.createAppearedChange(resource)
	}

	// Detect modifications
	return d.compareResources(resource, *previous)
}

// createAppearedChange creates a change for newly appeared resources
func (d *TemporalChangeDetector) createAppearedChange(resource types.Resource) *Change {
	changeType := ChangeAppeared
	details := "New resource observed"

	// Check if unmanaged
	if !resource.IsManaged() {
		changeType = ChangeUnmanaged
		details = "Unmanaged resource detected"
	}

	return &Change{
		Type:       changeType,
		ResourceID: resource.ID,
		Current:    &resource,
		Timestamp:  time.Now(),
		Details:    details,
	}
}

// compareResources compares current and previous resource states
func (d *TemporalChangeDetector) compareResources(current types.Resource, previous types.Resource) *Change {
	// Check status change
	if current.Status != previous.Status {
		return &Change{
			Type:       ChangeStatusChanged,
			ResourceID: current.ID,
			Current:    &current,
			Previous:   &previous,
			Timestamp:  time.Now(),
			Details:    "Resource status changed",
			Metadata: map[string]any{
				"previous_status": previous.Status,
				"current_status":  current.Status,
			},
		}
	}

	// Check tag drift
	if hasTagDrift(current.Tags, previous.Tags) {
		return &Change{
			Type:       ChangeTagDrift,
			ResourceID: current.ID,
			Current:    &current,
			Previous:   &previous,
			Timestamp:  time.Now(),
			Details:    "Resource tags changed",
			Metadata: map[string]any{
				"previous_tags": previous.Tags,
				"current_tags":  current.Tags,
			},
		}
	}

	// Check other modifications
	if hasModifications(current, previous) {
		return &Change{
			Type:       ChangeModified,
			ResourceID: current.ID,
			Current:    &current,
			Previous:   &previous,
			Timestamp:  time.Now(),
			Details:    "Resource configuration changed",
		}
	}

	// No changes detected
	return nil
}

// detectDisappeared finds resources that were seen before but not now
func (d *TemporalChangeDetector) detectDisappeared(ctx context.Context, currentMap map[string]types.Resource, previousIDs map[string]bool) []Change {
	var changes []Change

	for id := range previousIDs {
		if _, exists := currentMap[id]; !exists {
			// Resource disappeared
			previous, err := d.storage.GetLatestResource(id)
			if err != nil {
				continue // Skip if we can't get previous state
			}

			changes = append(changes, Change{
				Type:       ChangeDisappeared,
				ResourceID: id,
				Previous:   previous,
				Timestamp:  time.Now(),
				Details:    "Resource no longer observed",
			})
		}
	}

	return changes
}

// getPreviousResourceIDs gets all resource IDs from MVCC storage
func (d *TemporalChangeDetector) getPreviousResourceIDs(ctx context.Context) (map[string]bool, error) {
	ids := make(map[string]bool)

	// Get all current resource states
	resourceStates, err := d.storage.GetAllCurrentResources()
	if err != nil {
		return nil, err
	}

	for _, state := range resourceStates {
		if state.Exists {
			ids[state.ResourceID] = true
		}
	}

	return ids, nil
}

// hasTagDrift checks if tags have changed
func hasTagDrift(current, previous types.Tags) bool {
	// Check Elava management tags
	if current.ElavaOwner != previous.ElavaOwner {
		return true
	}
	if current.ElavaManaged != previous.ElavaManaged {
		return true
	}

	// Check standard tags
	if current.Environment != previous.Environment {
		return true
	}
	if current.Team != previous.Team {
		return true
	}
	if current.Project != previous.Project {
		return true
	}

	return false
}

// hasModifications checks for other configuration changes
func hasModifications(current, previous types.Resource) bool {
	// Check basic fields
	if current.Type != previous.Type {
		return true
	}
	if current.Region != previous.Region {
		return true
	}
	if current.Provider != previous.Provider {
		return true
	}
	if current.Name != previous.Name {
		return true
	}

	return false
}
