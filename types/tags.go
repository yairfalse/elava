package types

// Tags represents resource tags as a structured type
// No maps! Everything is explicit
type Tags struct {
	// Ovi management tags
	OviOwner      string `json:"ovi_owner,omitempty"`
	OviManaged    bool   `json:"ovi_managed,omitempty"`
	OviBlessed    bool   `json:"ovi_blessed,omitempty"`
	OviGeneration string `json:"ovi_generation,omitempty"`
	OviClaimedAt  string `json:"ovi_claimed_at,omitempty"`

	// Standard infrastructure tags
	Name        string `json:"name,omitempty"`
	Environment string `json:"environment,omitempty"`
	Team        string `json:"team,omitempty"`
	Project     string `json:"project,omitempty"`
	CostCenter  string `json:"cost_center,omitempty"`

	// AWS common tags
	Application string `json:"application,omitempty"`
	Owner       string `json:"owner,omitempty"`
	Contact     string `json:"contact,omitempty"`
	CreatedBy   string `json:"created_by,omitempty"`
	CreatedDate string `json:"created_date,omitempty"`
}

// IsManaged checks if resource is managed by Ovi
func (t Tags) IsManaged() bool {
	return t.OviOwner != "" || t.OviManaged
}

// IsBlessed checks if resource should be protected
func (t Tags) IsBlessed() bool {
	return t.OviBlessed
}

// GetOwner returns the owner of the resource
func (t Tags) GetOwner() string {
	if t.OviOwner != "" {
		return t.OviOwner
	}
	// Fallback to Team if no Ovi owner
	return t.Team
}

// ToMap converts structured tags to map for AWS API compatibility
//
//nolint:gocyclo // Simple field mapping, complexity is acceptable
func (t Tags) ToMap() map[string]string {
	tags := make(map[string]string)

	if t.OviOwner != "" {
		tags["ovi:owner"] = t.OviOwner
	}
	if t.OviManaged {
		tags["ovi:managed"] = "true"
	}
	if t.OviBlessed {
		tags["ovi:blessed"] = "true"
	}
	if t.OviGeneration != "" {
		tags["ovi:generation"] = t.OviGeneration
	}
	if t.OviClaimedAt != "" {
		tags["ovi:claimed_at"] = t.OviClaimedAt
	}
	if t.Name != "" {
		tags["Name"] = t.Name
	}
	if t.Environment != "" {
		tags["Environment"] = t.Environment
	}
	if t.Team != "" {
		tags["Team"] = t.Team
	}
	if t.Project != "" {
		tags["Project"] = t.Project
	}
	if t.CostCenter != "" {
		tags["CostCenter"] = t.CostCenter
	}
	if t.Application != "" {
		tags["Application"] = t.Application
	}
	if t.Owner != "" {
		tags["Owner"] = t.Owner
	}
	if t.Contact != "" {
		tags["Contact"] = t.Contact
	}
	if t.CreatedBy != "" {
		tags["CreatedBy"] = t.CreatedBy
	}
	if t.CreatedDate != "" {
		tags["CreatedDate"] = t.CreatedDate
	}

	return tags
}

// TagsFromMap creates structured tags from a map (for AWS API compatibility)
//
//nolint:gocyclo // Simple field mapping, complexity is acceptable
func TagsFromMap(tagMap map[string]string) Tags {
	tags := Tags{}

	if val, ok := tagMap["ovi:owner"]; ok {
		tags.OviOwner = val
	}
	if val, ok := tagMap["ovi:managed"]; ok && val == "true" {
		tags.OviManaged = true
	}
	if val, ok := tagMap["ovi:blessed"]; ok && val == "true" {
		tags.OviBlessed = true
	}
	if val, ok := tagMap["ovi:generation"]; ok {
		tags.OviGeneration = val
	}
	if val, ok := tagMap["ovi:claimed_at"]; ok {
		tags.OviClaimedAt = val
	}
	if val, ok := tagMap["Name"]; ok {
		tags.Name = val
	}
	if val, ok := tagMap["Environment"]; ok {
		tags.Environment = val
	}
	if val, ok := tagMap["Team"]; ok {
		tags.Team = val
	}
	if val, ok := tagMap["Project"]; ok {
		tags.Project = val
	}
	if val, ok := tagMap["CostCenter"]; ok {
		tags.CostCenter = val
	}
	if val, ok := tagMap["Application"]; ok {
		tags.Application = val
	}
	if val, ok := tagMap["Owner"]; ok {
		tags.Owner = val
	}
	if val, ok := tagMap["Contact"]; ok {
		tags.Contact = val
	}
	if val, ok := tagMap["CreatedBy"]; ok {
		tags.CreatedBy = val
	}
	if val, ok := tagMap["CreatedDate"]; ok {
		tags.CreatedDate = val
	}

	return tags
}
