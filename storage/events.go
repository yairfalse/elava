package storage

import (
	"time"

	"github.com/yairfalse/elava/types"
)

// ChangeEvent represents a detected infrastructure change
type ChangeEvent struct {
	ResourceID string            `json:"resource_id"`
	ChangeType string            `json:"change_type"` // created, modified, disappeared
	Timestamp  time.Time         `json:"timestamp"`
	Revision   int64             `json:"revision"`
	Previous   *types.Resource   `json:"previous,omitempty"`
	Current    *types.Resource   `json:"current,omitempty"`
	Metadata   map[string]string `json:"metadata,omitempty"`
}

// DriftEvent represents detected drift from desired state
type DriftEvent struct {
	ResourceID string            `json:"resource_id"`
	DriftType  string            `json:"drift_type"` // tag_drift, config_drift, state_drift
	Timestamp  time.Time         `json:"timestamp"`
	Field      string            `json:"field"`
	Expected   string            `json:"expected"`
	Actual     string            `json:"actual"`
	Severity   string            `json:"severity"` // low, medium, high, critical
	Metadata   map[string]string `json:"metadata,omitempty"`
}

// WastePattern represents detected resource waste
type WastePattern struct {
	PatternType   string            `json:"pattern_type"` // idle, oversized, orphaned, unattached
	ResourceIDs   []string          `json:"resource_ids"`
	Timestamp     time.Time         `json:"timestamp"`
	EstimatedCost float64           `json:"estimated_cost,omitempty"`
	Confidence    float64           `json:"confidence"` // 0.0 to 1.0
	Reason        string            `json:"reason"`
	Metadata      map[string]string `json:"metadata,omitempty"`
}
