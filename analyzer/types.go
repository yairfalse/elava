package analyzer

import (
	"context"
	"time"

	"github.com/yairfalse/elava/types"
)

// Analyzer is the brain that queries patterns from storage
type Analyzer interface {
	// Temporal queries
	QueryEngine

	// Pattern detection
	DriftAnalyzer
	WasteAnalyzer
	PatternDetector
}

// QueryEngine provides temporal query capabilities
type QueryEngine interface {
	// Query resources by time range
	QueryByTimeRange(ctx context.Context, start, end time.Time) ([]types.Resource, error)

	// Get all changes since a revision
	QueryChangesSince(ctx context.Context, revision int64) ([]ChangeEvent, error)

	// Get resource history
	QueryResourceHistory(ctx context.Context, resourceID string) ([]ResourceRevision, error)

	// Aggregate metrics by tag
	AggregateByTag(ctx context.Context, tag string, period time.Duration) (map[string]Metrics, error)
}

// DriftAnalyzer detects configuration drift
type DriftAnalyzer interface {
	// Detect drift between two time points
	AnalyzeDrift(ctx context.Context, from, to time.Time) ([]DriftEvent, error)

	// Get drift for specific resource
	GetResourceDrift(ctx context.Context, resourceID string, period time.Duration) ([]DriftEvent, error)
}

// WasteAnalyzer identifies unused resources
type WasteAnalyzer interface {
	// Find potentially wasted resources
	AnalyzeWaste(ctx context.Context) ([]WastePattern, error)

	// Get orphaned resources
	FindOrphans(ctx context.Context, since time.Time) ([]types.Resource, error)

	// Identify idle resources
	FindIdleResources(ctx context.Context, idleThreshold time.Duration) ([]types.Resource, error)
}

// PatternDetector identifies recurring behaviors
type PatternDetector interface {
	// Detect lifecycle patterns
	DetectLifecyclePatterns(ctx context.Context) ([]Pattern, error)

	// Find resources with similar behavior
	FindSimilarResources(ctx context.Context, resourceID string) ([]types.Resource, error)

	// Predict future state based on patterns
	PredictResourceState(ctx context.Context, resourceID string, future time.Duration) (*Prediction, error)
}

// ChangeEvent represents a resource change
type ChangeEvent struct {
	Revision   int64           `json:"revision"`
	Timestamp  time.Time       `json:"timestamp"`
	ResourceID string          `json:"resource_id"`
	Type       ChangeType      `json:"type"`
	Before     *types.Resource `json:"before,omitempty"`
	After      *types.Resource `json:"after,omitempty"`
}

// ChangeType categorizes changes
type ChangeType string

const (
	ChangeCreated      ChangeType = "created"
	ChangeModified     ChangeType = "modified"
	ChangeDisappeared  ChangeType = "disappeared"
	ChangeTagsChanged  ChangeType = "tags_changed"
	ChangeStateChanged ChangeType = "state_changed"
)

// DriftEvent represents configuration drift
type DriftEvent struct {
	ResourceID string                 `json:"resource_id"`
	Timestamp  time.Time              `json:"timestamp"`
	Type       string                 `json:"type"`
	Field      string                 `json:"field"`
	OldValue   interface{}            `json:"old_value"`
	NewValue   interface{}            `json:"new_value"`
	Severity   DriftSeverity          `json:"severity"`
	Metadata   map[string]interface{} `json:"metadata,omitempty"`
}

// DriftSeverity levels
type DriftSeverity string

const (
	DriftLow      DriftSeverity = "low"
	DriftMedium   DriftSeverity = "medium"
	DriftHigh     DriftSeverity = "high"
	DriftCritical DriftSeverity = "critical"
)

// WastePattern identifies waste patterns
type WastePattern struct {
	Type        WasteType `json:"type"`
	ResourceIDs []string  `json:"resource_ids"`
	Reason      string    `json:"reason"`
	// Impact field removed - let FinOps tools calculate costs
	Confidence float64                `json:"confidence"`
	FirstSeen  time.Time              `json:"first_seen"`
	Metadata   map[string]interface{} `json:"metadata,omitempty"`
}

// WasteType categories
type WasteType string

const (
	WasteOrphaned   WasteType = "orphaned"
	WasteIdle       WasteType = "idle"
	WasteOversized  WasteType = "oversized"
	WasteDuplicate  WasteType = "duplicate"
	WasteUnattached WasteType = "unattached"
	WasteObsolete   WasteType = "obsolete"
)

// Pattern represents recurring behavior
type Pattern struct {
	ID          string                 `json:"id"`
	Type        PatternType            `json:"type"`
	Description string                 `json:"description"`
	Resources   []string               `json:"resources"`
	Frequency   time.Duration          `json:"frequency"`
	Confidence  float64                `json:"confidence"`
	LastSeen    time.Time              `json:"last_seen"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
}

// PatternType categories
type PatternType string

const (
	PatternDaily  PatternType = "daily"
	PatternWeekly PatternType = "weekly"
	PatternCyclic PatternType = "cyclic"
	PatternGrowth PatternType = "growth"
	PatternDecay  PatternType = "decay"
	PatternBurst  PatternType = "burst"
)

// Prediction of future state
type Prediction struct {
	ResourceID string                 `json:"resource_id"`
	Timestamp  time.Time              `json:"timestamp"`
	State      string                 `json:"state"`
	Confidence float64                `json:"confidence"`
	Reasoning  string                 `json:"reasoning"`
	Metadata   map[string]interface{} `json:"metadata,omitempty"`
}

// ResourceRevision tracks resource history
type ResourceRevision struct {
	Revision  int64          `json:"revision"`
	Timestamp time.Time      `json:"timestamp"`
	Resource  types.Resource `json:"resource"`
	Change    ChangeType     `json:"change"`
}

// Metrics for aggregation
type Metrics struct {
	Count         int                    `json:"count"`
	ResourceTypes map[string]int         `json:"resource_types"`
	Tags          map[string]string      `json:"tags"`
	Metadata      map[string]interface{} `json:"metadata,omitempty"`
}
