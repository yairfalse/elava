// Package filter provides resource filtering for Elava scanners.
package filter

import (
	"github.com/yairfalse/elava/pkg/resource"
)

// Filter controls which resource types to scan and which resources to include.
type Filter struct {
	excludeTypes map[string]bool
	includeTags  map[string]string
	excludeTags  map[string]string
}

// New creates a new Filter from the provided configuration.
func New(excludeTypes []string, includeTags, excludeTags map[string]string) *Filter {
	excludeMap := make(map[string]bool)
	for _, t := range excludeTypes {
		excludeMap[t] = true
	}

	return &Filter{
		excludeTypes: excludeMap,
		includeTags:  includeTags,
		excludeTags:  excludeTags,
	}
}

// ShouldScanType returns true if the given resource type should be scanned.
func (f *Filter) ShouldScanType(typ string) bool {
	return !f.excludeTypes[typ]
}

// ShouldIncludeResource returns true if the resource passes tag filters.
func (f *Filter) ShouldIncludeResource(r resource.Resource) bool {
	// Check include tags (whitelist) - ALL must match
	if len(f.includeTags) > 0 {
		for k, v := range f.includeTags {
			if r.Labels == nil || r.Labels[k] != v {
				return false
			}
		}
	}

	// Check exclude tags (blacklist) - ANY match excludes
	if len(f.excludeTags) > 0 {
		for k, v := range f.excludeTags {
			if r.Labels != nil && r.Labels[k] == v {
				return false
			}
		}
	}

	return true
}

// FilterResources returns only resources that pass the filter.
func (f *Filter) FilterResources(resources []resource.Resource) []resource.Resource {
	if f.IsEmpty() {
		return resources
	}

	filtered := make([]resource.Resource, 0, len(resources))
	for _, r := range resources {
		if f.ShouldIncludeResource(r) {
			filtered = append(filtered, r)
		}
	}
	return filtered
}

// IsEmpty returns true if no filters are configured.
func (f *Filter) IsEmpty() bool {
	return len(f.excludeTypes) == 0 && len(f.includeTags) == 0 && len(f.excludeTags) == 0
}
