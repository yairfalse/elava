package types

// BuildResourceMap converts a slice of resources to a map for efficient lookup by ID
func BuildResourceMap(resources []Resource) map[string]Resource {
	resourceMap := make(map[string]Resource)
	for _, resource := range resources {
		resourceMap[resource.ID] = resource
	}
	return resourceMap
}
