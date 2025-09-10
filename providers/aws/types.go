package aws

import "time"

// EC2Instance represents an AWS EC2 instance
type EC2Instance struct {
	InstanceID   string
	InstanceType string
	State        string
	LaunchTime   time.Time
	Tags         map[string]string // Tags are allowed per CLAUDE.md line 303
}

// InstanceFilter for EC2 queries
type InstanceFilter struct {
	States []string
	Tags   map[string]string // Tags are allowed per CLAUDE.md line 303
}

// InstanceSpec for creating EC2 instances
type InstanceSpec struct {
	InstanceType string
	Tags         map[string]string // Tags are allowed per CLAUDE.md line 303
}

// TagOperation represents a tagging operation
type TagOperation struct {
	InstanceID string
	Tags       map[string]string // Tags are allowed per CLAUDE.md line 303
}
