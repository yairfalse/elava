package reconciler

import (
	"github.com/yairfalse/elava/types"
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

	// Find missing resources
	missing := c.findMissingResources(currentMap, desiredMap)
	diffs = append(diffs, missing...)

	// Find unwanted resources
	unwanted := c.findUnwantedResources(currentMap, desiredMap)
	diffs = append(diffs, unwanted...)

	// Find drifted resources
	drifted := c.findDriftedResources(currentMap, desiredMap)
	diffs = append(diffs, drifted...)

	return diffs, nil
}

// findMissingResources finds resources that are desired but not current
func (c *SimpleComparator) findMissingResources(currentMap, desiredMap map[string]types.Resource) []Diff {
	var diffs []Diff
	for id, desiredResource := range desiredMap {
		if _, exists := currentMap[id]; !exists {
			desiredCopy := desiredResource
			diffs = append(diffs, Diff{
				Type:       DiffMissing,
				ResourceID: id,
				Desired:    &desiredCopy,
				Reason:     "Resource specified in config but not found in cloud",
			})
		}
	}
	return diffs
}

// findUnwantedResources finds resources that exist but are not desired
func (c *SimpleComparator) findUnwantedResources(currentMap, desiredMap map[string]types.Resource) []Diff {
	var diffs []Diff
	for id, currentResource := range currentMap {
		if _, exists := desiredMap[id]; !exists {
			currentCopy := currentResource
			if currentResource.IsManaged() {
				diffs = append(diffs, Diff{
					Type:       DiffUnwanted,
					ResourceID: id,
					Current:    &currentCopy,
					Reason:     "Resource managed by Elava but not in current config",
				})
			} else {
				diffs = append(diffs, Diff{
					Type:       DiffUnmanaged,
					ResourceID: id,
					Current:    &currentCopy,
					Reason:     "Resource exists but not managed by Elava",
				})
			}
		}
	}
	return diffs
}

// findDriftedResources finds resources that exist in both but differ
func (c *SimpleComparator) findDriftedResources(currentMap, desiredMap map[string]types.Resource) []Diff {
	var diffs []Diff
	for id, desiredResource := range desiredMap {
		if currentResource, exists := currentMap[id]; exists {
			if isDrifted(currentResource, desiredResource) {
				currentCopy := currentResource
				desiredCopy := desiredResource
				diffs = append(diffs, Diff{
					Type:       DiffDrifted,
					ResourceID: id,
					Current:    &currentCopy,
					Desired:    &desiredCopy,
					Reason:     "Resource configuration differs from desired state",
				})
			}
		}
	}
	return diffs
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
	// Check Elava management tags
	if current.ElavaOwner != desired.ElavaOwner {
		return true
	}
	if current.ElavaManaged != desired.ElavaManaged {
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
