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

// Coordinator ensures multiple Ovi instances don't conflict
type Coordinator interface {
	ClaimResources(ctx context.Context, resourceIDs []string, ttl time.Duration) error
	ReleaseResources(ctx context.Context, resourceIDs []string) error
	IsResourceClaimed(ctx context.Context, resourceID string) (bool, error)
}

// Config defines what infrastructure should exist
type Config struct {
	Version   string               `yaml:"version"`
	Provider  string               `yaml:"provider"`
	Region    string               `yaml:"region"`
	Resources []types.ResourceSpec `yaml:"resources"`
}

// Diff represents a difference between current and desired state
type Diff struct {
	Type       DiffType        `json:"type"`
	ResourceID string          `json:"resource_id"`
	Current    *types.Resource `json:"current,omitempty"`
	Desired    *types.Resource `json:"desired,omitempty"`
	Reason     string          `json:"reason"`
}

// DiffType categorizes the type of difference found
type DiffType string

const (
	DiffMissing   DiffType = "missing"   // Resource should exist but doesn't
	DiffUnwanted  DiffType = "unwanted"  // Resource exists but shouldn't
	DiffDrifted   DiffType = "drifted"   // Resource exists but has wrong configuration
	DiffUnmanaged DiffType = "unmanaged" // Resource exists but isn't managed by Ovi
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
