package reconciler

import (
	"context"
	"time"

	"github.com/yairfalse/elava/types"
)

// Reconciler orchestrates the observation, comparison, and decision-making process
type Reconciler interface {
	Reconcile(ctx context.Context, config Config) ([]types.Decision, error)
}

// Observer polls cloud providers for current infrastructure state
type Observer interface {
	Observe(ctx context.Context, filter types.ResourceFilter) ([]types.Resource, error)
}

// Comparator identifies differences between current and desired state
type Comparator interface {
	Compare(current, desired []types.Resource) ([]Diff, error)
}

// DecisionMaker generates decisions based on state differences
type DecisionMaker interface {
	Decide(diffs []Diff) ([]types.Decision, error)
}

// Coordinator ensures multiple Elava instances don't conflict
type Coordinator interface {
	ClaimResources(ctx context.Context, resourceIDs []string, ttl time.Duration) error
	ReleaseResources(ctx context.Context, resourceIDs []string) error
	IsResourceClaimed(ctx context.Context, resourceID string) (bool, error)
}

// Config defines reconciliation configuration for Day 2 operations
type Config struct {
	Version  string `yaml:"version"`
	Provider string `yaml:"provider"`
	Region   string `yaml:"region"`

	// Deprecated: Resources field is deprecated and will be removed in v2.0.
	// Elava no longer manages infrastructure via config declarations.
	// This field is ignored. Use tag-based resource tracking instead.
	Resources []types.ResourceSpec `yaml:"resources,omitempty"`
}

// Diff represents a difference or change detected between observations
//
// Note: "Diff" name is legacy from IaC approach. In Day 2 operations, this represents
// a detected change over time, not a diff from desired state.
// TODO(v2.0): Rename to "Change" to better reflect Day 2 operations.
type Diff struct {
	Type       DiffType        `json:"type"`
	ResourceID string          `json:"resource_id"`
	Current    *types.Resource `json:"current,omitempty"`  // Current state
	Desired    *types.Resource `json:"desired,omitempty"`  // Deprecated: use Previous instead for Day 2 operations
	Previous   *types.Resource `json:"previous,omitempty"` // Preferred: previous state in Day 2 operations
	Reason     string          `json:"reason"`
}

// DiffType categorizes the type of change detected
//
// Note: Some types are deprecated as part of Day 2 pivot
type DiffType string

const (
	// Deprecated: DiffMissing represents IaC mindset (state enforcement)
	// In Day 2 ops, we don't declare what "should" exist
	DiffMissing DiffType = "missing"

	// Deprecated: DiffUnwanted represents IaC mindset (state enforcement)
	// In Day 2 ops, we observe and notify, not enforce deletions
	DiffUnwanted DiffType = "unwanted"

	// DiffDrifted detects configuration or tag changes (valid for Day 2)
	DiffDrifted DiffType = "drifted"

	// DiffUnmanaged detects resources not tagged with elava:managed (valid for Day 2)
	DiffUnmanaged DiffType = "unmanaged"
)

// Claim represents a resource claim for coordination
type Claim struct {
	ResourceID string    `json:"resource_id"`
	InstanceID string    `json:"instance_id"`
	ClaimedAt  time.Time `json:"claimed_at"`
	ExpiresAt  time.Time `json:"expires_at"`
}

// ReconcileResult contains the outcome of a reconciliation cycle
type ReconcileResult struct {
	Timestamp       time.Time        `json:"timestamp"`
	ResourcesFound  int              `json:"resources_found"`
	DiffsDetected   int              `json:"diffs_detected"`
	DecisionsMade   int              `json:"decisions_made"`
	ExecutionErrors []string         `json:"execution_errors,omitempty"`
	Duration        time.Duration    `json:"duration"`
	Decisions       []types.Decision `json:"decisions"`
}

// ReconcilerOptions configure reconciler behavior
type ReconcilerOptions struct {
	DryRun          bool          `json:"dry_run"`
	MaxConcurrency  int           `json:"max_concurrency"`
	ClaimTTL        time.Duration `json:"claim_ttl"`
	SkipDestructive bool          `json:"skip_destructive"`
}
