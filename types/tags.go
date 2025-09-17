package types

// Tags represents resource tags as a structured type
// No maps! Everything is explicit
type Tags struct {
	// Elava management tags
	ElavaOwner      string `json:"elava_owner,omitempty"`
	ElavaManaged    bool   `json:"elava_managed,omitempty"`
	ElavaBlessed    bool   `json:"elava_blessed,omitempty"`
	ElavaGeneration string `json:"elava_generation,omitempty"`
	ElavaClaimedAt  string `json:"elava_claimed_at,omitempty"`

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

// IsManaged checks if resource is managed by Elava
func (t Tags) IsManaged() bool {
	return t.ElavaOwner != "" || t.ElavaManaged
}

// IsBlessed checks if resource should be protected
func (t Tags) IsBlessed() bool {
	return t.ElavaBlessed
}

// GetOwner returns the owner of the resource
func (t Tags) GetOwner() string {
	if t.ElavaOwner != "" {
		return t.ElavaOwner
	}
	// Fallback to Team if no Elava owner
	return t.Team
}

// Get returns the value of a tag by key name
func (t Tags) Get(key string) string {
	switch key {
	case "elava:owner", "elava_owner":
		return t.ElavaOwner
	case "elava:generation", "elava_generation":
		return t.ElavaGeneration
	case "elava:claimed_at", "elava_claimed_at":
		return t.ElavaClaimedAt
	case "Name", "name":
		return t.Name
	case "Environment", "environment":
		return t.Environment
	case "Team", "team":
		return t.Team
	case "Project", "project":
		return t.Project
	case "CostCenter", "cost_center":
		return t.CostCenter
	case "Application", "application":
		return t.Application
	case "Owner", "owner":
		return t.Owner
	case "Contact", "contact":
		return t.Contact
	case "CreatedBy", "created_by":
		return t.CreatedBy
	case "CreatedDate", "created_date":
		return t.CreatedDate
	default:
		return ""
	}
}

// ToMap converts structured tags to map for AWS API compatibility
//
//nolint:gocyclo // Simple field mapping, complexity is acceptable
func (t Tags) ToMap() map[string]string {
	tags := make(map[string]string)

	if t.ElavaOwner != "" {
		tags["elava:owner"] = t.ElavaOwner
	}
	if t.ElavaManaged {
		tags["elava:managed"] = "true"
	}
	if t.ElavaBlessed {
		tags["elava:blessed"] = "true"
	}
	if t.ElavaGeneration != "" {
		tags["elava:generation"] = t.ElavaGeneration
	}
	if t.ElavaClaimedAt != "" {
		tags["elava:claimed_at"] = t.ElavaClaimedAt
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

	if val, ok := tagMap["elava:owner"]; ok {
		tags.ElavaOwner = val
	}
	if val, ok := tagMap["elava:managed"]; ok && val == "true" {
		tags.ElavaManaged = true
	}
	if val, ok := tagMap["elava:blessed"]; ok && val == "true" {
		tags.ElavaBlessed = true
	}
	if val, ok := tagMap["elava:generation"]; ok {
		tags.ElavaGeneration = val
	}
	if val, ok := tagMap["elava:claimed_at"]; ok {
		tags.ElavaClaimedAt = val
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
