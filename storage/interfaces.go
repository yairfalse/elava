package storage

import (
	"context"
	"time"

	"github.com/yairfalse/elava/types"
)

// ObservationWriter records resource observations
type ObservationWriter interface {
	RecordObservation(resource types.Resource) (revision int64, err error)
	RecordObservationBatch(resources []types.Resource) (revision int64, err error)
	RecordDisappearance(resourceID string) (revision int64, err error)
}

// ObservationReader queries resource observations
type ObservationReader interface {
	GetResourceState(resourceID string) (*ResourceState, error)
	GetStateAtRevision(resourceID string, revision int64) (*ResourceState, error)
	GetResourcesByOwner(owner string) ([]*ResourceState, error)
	GetAllCurrentResources() ([]*ResourceState, error)
}

// ObservationStorage combines read and write for observations
type ObservationStorage interface {
	ObservationWriter
	ObservationReader
	CurrentRevision() int64
}

// AnalyzerEventWriter stores analyzer-generated events
type AnalyzerEventWriter interface {
	// Single event operations
	StoreChangeEvent(ctx context.Context, event ChangeEvent) error
	StoreDriftEvent(ctx context.Context, event DriftEvent) error
	StoreWastePattern(ctx context.Context, pattern WastePattern) error

	// Batch operations for performance
	StoreChangeEventBatch(ctx context.Context, events []ChangeEvent) error
	StoreDriftEventBatch(ctx context.Context, events []DriftEvent) error
	StoreWastePatternBatch(ctx context.Context, patterns []WastePattern) error
}

// AnalyzerEventReader queries analyzer events
type AnalyzerEventReader interface {
	QueryChangesSince(ctx context.Context, since time.Time) ([]ChangeEvent, error)
	QueryDriftEvents(ctx context.Context, since time.Time) ([]DriftEvent, error)
	QueryWastePatterns(ctx context.Context, since time.Time) ([]WastePattern, error)
}

// AnalyzerEventStorage combines analyzer event operations
type AnalyzerEventStorage interface {
	AnalyzerEventWriter
	AnalyzerEventReader
}

// Compactor handles storage compaction
type Compactor interface {
	Compact(keepRevisions int64) error
	CompactWithContext(ctx context.Context, keepRevisions int64) error
}

// StorageStats provides operational metrics
type StorageStats interface {
	Stats() (resourceCount int, currentRev int64, dbSizeBytes int64)
}

// Lifecycle manages storage lifecycle
type Lifecycle interface {
	Close() error
}

// Storage is the complete storage interface combining all capabilities
type Storage interface {
	ObservationStorage
	AnalyzerEventStorage
	Compactor
	StorageStats
	Lifecycle
}
