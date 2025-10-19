package types

import (
	"fmt"
	"time"
)

// Action types - these are RECOMMENDATIONS only, not executed by Elava
// Elava is observability-only and does not modify infrastructure
const (
	// Observability actions - currently used
	ActionNotify = "notify" // Send notification about resource state
	ActionAlert  = "alert"  // High-priority alert (e.g., resource disappeared)
	ActionIgnore = "ignore" // Acknowledged, no action needed
	ActionAudit  = "audit"  // Log for audit trail only
	ActionNoop   = "noop"   // No action recommended

	// Legacy constants - NOT USED, kept for compatibility
	// These do NOT trigger any infrastructure modifications
	ActionCreate        = "create"         // (unused - Elava doesn't create resources)
	ActionUpdate        = "update"         // (unused - Elava doesn't update resources)
	ActionDelete        = "delete"         // (unused - Elava doesn't delete resources)
	ActionTerminate     = "terminate"      // (unused - Elava doesn't terminate resources)
	ActionProtect       = "protect"        // (unused - no protection mechanism)
	ActionEnforceTags   = "enforce_tags"   // (unused - no tag enforcement)
	ActionEnforcePolicy = "enforce_policy" // (unused - no policy enforcement)
	ActionAutoTag       = "auto_tag"       // (unused - no auto-tagging)
	ActionTag           = "tag"            // (unused - no tagging operations)
)

// Decision represents an action to take on a resource
type Decision struct {
	Action       string         `json:"action"`
	ResourceID   string         `json:"resource_id"`
	ResourceType string         `json:"resource_type,omitempty"`
	Reason       string         `json:"reason"`
	IsBlessed    bool           `json:"is_blessed,omitempty"`
	CreatedAt    time.Time      `json:"created_at"`
	ExecutedAt   time.Time      `json:"executed_at,omitempty"`
	Metadata     map[string]any `json:"metadata,omitempty"` // Day 2: Additional context (change type, policy info, etc.)
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
