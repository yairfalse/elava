package orchestrator

import (
	"context"
	"time"

	"github.com/yairfalse/elava/types"
)

// CycleResult contains the results of a reconciliation cycle
type CycleResult struct {
	StartTime          time.Time     `json:"start_time"`
	EndTime            time.Time     `json:"end_time"`
	Duration           time.Duration `json:"duration"`
	ResourcesScanned   int           `json:"resources_scanned"`
	PoliciesEvaluated  int           `json:"policies_evaluated"`
	EnforcementActions int           `json:"enforcement_actions"`
	Errors             []string      `json:"errors,omitempty"`
	Success            bool          `json:"success"`
}

// Scanner interface for resource discovery
type Scanner interface {
	Scan(ctx context.Context) ([]types.Resource, error)
}
