package types

import (
	"time"
)

// Resource represents a cloud resource with Day 2 operations data
type Resource struct {
	ID         string                 `json:"id"`
	Type       string                 `json:"type"`
	Provider   string                 `json:"provider"`
	Region     string                 `json:"region"`
	AccountID  string                 `json:"account_id"`
	Name       string                 `json:"name"`
	Status     string                 `json:"status"`
	Tags       Tags                   `json:"tags"`
	CreatedAt  time.Time              `json:"created_at"`
	LastSeenAt time.Time              `json:"last_seen_at"`
	IsOrphaned bool                   `json:"is_orphaned"`
	Metadata   map[string]interface{} `json:"metadata,omitempty"`
}

// ResourceSpec defines desired resource configuration
type ResourceSpec struct {
	Type   string `yaml:"type" json:"type"`
	Count  int    `yaml:"count,omitempty" json:"count,omitempty"`
	Size   string `yaml:"size,omitempty" json:"size,omitempty"`
	Region string `yaml:"region,omitempty" json:"region,omitempty"`
	Tags   Tags   `yaml:"tags,omitempty" json:"tags,omitempty"`
}

// ResourceFilter for querying resources
type ResourceFilter struct {
	Type     string   `json:"type,omitempty"`
	Region   string   `json:"region,omitempty"`
	Provider string   `json:"provider,omitempty"`
	Owner    string   `json:"owner,omitempty"`   // Filter by owner
	Managed  bool     `json:"managed,omitempty"` // Filter managed resources
	IDs      []string `json:"ids,omitempty"`
}

// IsManaged checks if resource is managed by Elava
func (r *Resource) IsManaged() bool {
	return r.Tags.IsManaged()
}

// IsBlessed checks if resource should be protected
func (r *Resource) IsBlessed() bool {
	return r.Tags.IsBlessed()
}

// Matches checks if resource matches filter criteria
func (r *Resource) Matches(filter ResourceFilter) bool {
	return r.matchesBasicFields(filter) && r.matchesIDs(filter) && r.matchesTags(filter)
}

// matchesBasicFields checks type, region, provider
func (r *Resource) matchesBasicFields(filter ResourceFilter) bool {
	if filter.Type != "" && r.Type != filter.Type {
		return false
	}
	if filter.Region != "" && r.Region != filter.Region {
		return false
	}
	if filter.Provider != "" && r.Provider != filter.Provider {
		return false
	}
	return true
}

// matchesIDs checks if resource ID is in filter list
func (r *Resource) matchesIDs(filter ResourceFilter) bool {
	if len(filter.IDs) == 0 {
		return true
	}
	for _, id := range filter.IDs {
		if r.ID == id {
			return true
		}
	}
	return false
}

// matchesTags checks if filter criteria match resource tags
func (r *Resource) matchesTags(filter ResourceFilter) bool {
	// Check owner filter
	if filter.Owner != "" && r.Tags.GetOwner() != filter.Owner {
		return false
	}

	// Check managed filter
	if filter.Managed && !r.Tags.IsManaged() {
		return false
	}

	return true
}
