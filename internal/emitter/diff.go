package emitter

import (
	"encoding/json"
	"maps"
	"sync"

	"github.com/yairfalse/elava/pkg/resource"
)

// DiffTracker tracks resource state between scans and detects changes.
type DiffTracker struct {
	mu          sync.RWMutex
	previous    map[string]resource.Resource
	initialized bool
}

// NewDiffTracker creates a new diff tracker.
func NewDiffTracker() *DiffTracker {
	return &DiffTracker{
		previous: make(map[string]resource.Resource),
	}
}

// ComputeDiff compares current resources against previous state.
// Returns nil on first scan (baseline establishment).
// Returns empty slice if no changes detected.
func (d *DiffTracker) ComputeDiff(current []resource.Resource) []resource.ResourceDiff {
	d.mu.RLock()
	defer d.mu.RUnlock()

	if !d.initialized {
		return nil
	}

	currentMap := indexResources(current)
	diffs := make([]resource.ResourceDiff, 0)
	diffs = append(diffs, d.findDeletedAndModified(currentMap)...)
	diffs = append(diffs, d.findAdded(currentMap)...)

	return diffs
}

// indexResources creates a map of resources keyed by their unique identifier.
func indexResources(resources []resource.Resource) map[string]resource.Resource {
	m := make(map[string]resource.Resource)
	for _, r := range resources {
		m[resource.ResourceKey(r)] = r
	}
	return m
}

// findDeletedAndModified checks previous resources for deletions and modifications.
func (d *DiffTracker) findDeletedAndModified(currentMap map[string]resource.Resource) []resource.ResourceDiff {
	var diffs []resource.ResourceDiff
	for key, prev := range d.previous {
		if curr, exists := currentMap[key]; exists {
			if changes := detectChanges(prev, curr); len(changes) > 0 {
				prevCopy := prev
				diffs = append(diffs, resource.ResourceDiff{
					Type:     resource.DiffModified,
					Resource: curr,
					Previous: &prevCopy,
					Changes:  changes,
				})
			}
		} else {
			prevCopy := prev
			diffs = append(diffs, resource.ResourceDiff{
				Type:     resource.DiffDeleted,
				Resource: prev,
				Previous: &prevCopy,
			})
		}
	}
	return diffs
}

// findAdded checks for new resources not in previous state.
func (d *DiffTracker) findAdded(currentMap map[string]resource.Resource) []resource.ResourceDiff {
	var diffs []resource.ResourceDiff
	for key, curr := range currentMap {
		if _, exists := d.previous[key]; !exists {
			diffs = append(diffs, resource.ResourceDiff{
				Type:     resource.DiffAdded,
				Resource: curr,
				Previous: nil,
			})
		}
	}
	return diffs
}

// Update stores the current resources as the new baseline for future comparisons.
func (d *DiffTracker) Update(current []resource.Resource) {
	d.mu.Lock()
	defer d.mu.Unlock()

	d.previous = make(map[string]resource.Resource)
	for _, r := range current {
		d.previous[resource.ResourceKey(r)] = r
	}
	d.initialized = true
}

// detectChanges compares two resources and returns detected field changes.
// Note: ScannedAt is intentionally excluded as it changes on every scan.
func detectChanges(prev, curr resource.Resource) map[string]resource.Change {
	changes := make(map[string]resource.Change)

	if prev.Name != curr.Name {
		changes["name"] = resource.Change{
			Previous: prev.Name,
			Current:  curr.Name,
		}
	}

	if prev.Status != curr.Status {
		changes["status"] = resource.Change{
			Previous: prev.Status,
			Current:  curr.Status,
		}
	}

	if !maps.Equal(prev.Labels, curr.Labels) {
		changes["labels"] = resource.Change{
			Previous: mapToJSON(prev.Labels),
			Current:  mapToJSON(curr.Labels),
		}
	}

	if !maps.Equal(prev.Attrs, curr.Attrs) {
		changes["attrs"] = resource.Change{
			Previous: mapToJSON(prev.Attrs),
			Current:  mapToJSON(curr.Attrs),
		}
	}

	return changes
}

// mapToJSON converts a map to a deterministic JSON string for comparison.
// JSON marshaling sorts keys alphabetically, ensuring consistent output.
func mapToJSON(m map[string]string) string {
	if m == nil {
		return "{}"
	}
	b, _ := json.Marshal(m)
	return string(b)
}
