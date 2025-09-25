package types

import "time"

// EnforcementEvent records a policy enforcement action
type EnforcementEvent struct {
	Timestamp    time.Time         `json:"timestamp"`
	ResourceID   string            `json:"resource_id"`
	ResourceType string            `json:"resource_type"`
	Provider     string            `json:"provider"`
	Action       string            `json:"action"` // notify, flag, deny, ignore
	Decision     string            `json:"decision"`
	Reason       string            `json:"reason"`
	Tags         map[string]string `json:"tags,omitempty"` // Tags applied if action=flag
	Success      bool              `json:"success"`
	Error        string            `json:"error,omitempty"`
}
