package reconciler

import (
	"github.com/yairfalse/ovi/types"
)

// SimpleComparator implements basic state comparison logic
type SimpleComparator struct{}

// NewSimpleComparator creates a new simple comparator
func NewSimpleComparator() *SimpleComparator {
	return &SimpleComparator{}
}

// Compare identifies differences between current and desired state
func (c *SimpleComparator) Compare(current, desired []types.Resource) ([]Diff, error) {
	var diffs []Diff

	// Build maps for efficient lookup
	currentMap := buildResourceMap(current)
	desiredMap := buildResourceMap(desired)

	// Find missing resources (desired but not current)
	for id, desiredResource := range desiredMap {
		if _, exists := currentMap[id]; !exists {
			diffs = append(diffs, Diff{
				Type:       DiffMissing,
				ResourceID: id,
				Desired:    &desiredResource,
				Reason:     "Resource specified in config but not found in cloud",
			})
		}
	}

	// Find unwanted resources (current but not desired)
	for id, currentResource := range currentMap {
		if _, exists := desiredMap[id]; !exists {
			// Check if it's managed by Ovi
			if currentResource.IsManaged() {
				diffs = append(diffs, Diff{
					Type:       DiffUnwanted,
					ResourceID: id,
					Current:    &currentResource,
					Reason:     "Resource managed by Ovi but not in current config",
				})
			} else {
				diffs = append(diffs, Diff{
					Type:       DiffUnmanaged,
					ResourceID: id,
					Current:    &currentResource,
					Reason:     "Resource exists but not managed by Ovi",
				})
			}
		}
	}

	// Find drifted resources (exist in both but differ)
	for id, desiredResource := range desiredMap {
		if currentResource, exists := currentMap[id]; exists {
			if isDrifted(currentResource, desiredResource) {
				diffs = append(diffs, Diff{
					Type:       DiffDrifted,
					ResourceID: id,
					Current:    &currentResource,
					Desired:    &desiredResource,
					Reason:     "Resource configuration differs from desired state",
				})
			}
		}
	}

	return diffs, nil
}

// buildResourceMap creates a map of resources keyed by ID
func buildResourceMap(resources []types.Resource) map[string]types.Resource {
	resourceMap := make(map[string]types.Resource)
	for _, resource := range resources {
		resourceMap[resource.ID] = resource
	}
	return resourceMap
}

// isDrifted checks if a resource has drifted from desired state
func isDrifted(current, desired types.Resource) bool {
	// Compare basic fields
	if current.Type != desired.Type {
		return true
	}
	if current.Provider != desired.Provider {
		return true
	}
	if current.Region != desired.Region {
		return true
	}

	// Compare tags
	return isTagsDrifted(current.Tags, desired.Tags)
}

// isTagsDrifted checks if tags have drifted
func isTagsDrifted(current, desired types.Tags) bool {
	// Check Ovi management tags
	if current.OviOwner != desired.OviOwner {
		return true
	}
	if current.OviManaged != desired.OviManaged {
		return true
	}

	// Check standard infrastructure tags
	if current.Environment != desired.Environment {
		return true
	}
	if current.Team != desired.Team {
		return true
	}
	if current.Project != desired.Project {
		return true
	}

	return false
}
