package types

import (
	"fmt"
	"time"
)

// Action types
const (
	ActionCreate    = "create"
	ActionUpdate    = "update"
	ActionDelete    = "delete"
	ActionTerminate = "terminate"
	ActionNotify    = "notify"
	ActionTag       = "tag"
	ActionNoop      = "noop"
)

// Decision represents an action to take on a resource
type Decision struct {
	Action       string    `json:"action"`
	ResourceID   string    `json:"resource_id"`
	ResourceType string    `json:"resource_type,omitempty"`
	Reason       string    `json:"reason"`
	IsBlessed    bool      `json:"is_blessed,omitempty"`
	CreatedAt    time.Time `json:"created_at"`
	ExecutedAt   time.Time `json:"executed_at,omitempty"`
}

// Validate ensures the decision has required fields
func (d *Decision) Validate() error {
	if d.Action == "" {
		return fmt.Errorf("decision action cannot be empty")
	}
	// For create actions, ResourceID is not required (will be generated)
	if d.Action != ActionCreate && d.ResourceID == "" {
		return fmt.Errorf("decision resource ID cannot be empty")
	}
	if d.Reason == "" {
		return fmt.Errorf("decision reason cannot be empty")
	}
	return nil
}

// IsDestructive checks if action removes resources
func (d *Decision) IsDestructive() bool {
	return d.Action == ActionDelete || d.Action == ActionTerminate
}

// RequiresConfirmation checks if user confirmation needed
func (d *Decision) RequiresConfirmation() bool {
	// Destructive actions require confirmation
	if d.IsDestructive() {
		return true
	}
	// Blessed resources require confirmation
	if d.IsBlessed {
		return true
	}
	return false
}
