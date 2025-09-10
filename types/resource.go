package types

import "time"

// Resource represents a cloud resource (EC2, RDS, S3, etc)
type Resource struct {
	ID        string            `json:"id"`
	Type      string            `json:"type"`
	Provider  string            `json:"provider"`
	Region    string            `json:"region"`
	Name      string            `json:"name"`
	Status    string            `json:"status"`
	Tags      map[string]string `json:"tags"`
	CreatedAt time.Time         `json:"created_at"`
	UpdatedAt time.Time         `json:"updated_at"`
}

// ResourceSpec defines desired resource configuration
type ResourceSpec struct {
	Type   string            `yaml:"type" json:"type"`
	Count  int               `yaml:"count,omitempty" json:"count,omitempty"`
	Size   string            `yaml:"size,omitempty" json:"size,omitempty"`
	Region string            `yaml:"region,omitempty" json:"region,omitempty"`
	Tags   map[string]string `yaml:"tags,omitempty" json:"tags,omitempty"`
}

// ResourceFilter for querying resources
type ResourceFilter struct {
	Type     string            `json:"type,omitempty"`
	Region   string            `json:"region,omitempty"`
	Provider string            `json:"provider,omitempty"`
	Tags     map[string]string `json:"tags,omitempty"`
	IDs      []string          `json:"ids,omitempty"`
}

// IsManaged checks if resource is managed by Ovi
func (r *Resource) IsManaged() bool {
	if r.Tags == nil {
		return false
	}
	_, hasOwner := r.Tags["ovi:owner"]
	_, hasManaged := r.Tags["ovi:managed"]
	return hasOwner || hasManaged
}

// IsBlessed checks if resource should be protected
func (r *Resource) IsBlessed() bool {
	if r.Tags == nil {
		return false
	}
	return r.Tags["ovi:blessed"] == "true"
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

// matchesTags checks if all filter tags match resource tags
func (r *Resource) matchesTags(filter ResourceFilter) bool {
	for key, value := range filter.Tags {
		if r.Tags[key] != value {
			return false
		}
	}
	return true
}
