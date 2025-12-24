// Package resource provides resource types and diffing.
package resource

// DiffType represents the type of change detected.
type DiffType string

const (
	// DiffAdded indicates a new resource was discovered.
	DiffAdded DiffType = "added"
	// DiffDeleted indicates a resource no longer exists.
	DiffDeleted DiffType = "deleted"
	// DiffModified indicates a resource's properties changed.
	DiffModified DiffType = "modified"
)

// Change represents a single field change.
// The field name is the map key in ResourceDiff.Changes.
type Change struct {
	Previous string
	Current  string
}

// ResourceDiff represents a detected change in a resource.
type ResourceDiff struct {
	Type     DiffType
	Resource Resource
	Previous *Resource         // nil for added resources
	Changes  map[string]Change // field name → change details
}

// ResourceKey returns a unique key for identifying a resource across scans.
func ResourceKey(r Resource) string {
	return r.ID + "|" + r.Provider + "|" + r.Region + "|" + r.Account
}
