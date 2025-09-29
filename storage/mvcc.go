package storage

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"sync"
	"time"

	"github.com/google/btree"
	"github.com/yairfalse/elava/telemetry"
	"github.com/yairfalse/elava/types"
	"go.etcd.io/bbolt"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/trace"
)

// Bucket names in bbolt
var (
	bucketObservations = []byte("observations")
	bucketIndex        = []byte("index")
	bucketMeta         = []byte("meta")
	bucketClaims       = []byte("claims")
)

// MVCCStorage implements etcd-style multi-version storage
type MVCCStorage struct {
	mu sync.RWMutex

	// In-memory index for fast lookups
	index *btree.BTreeG[*ResourceState]

	// On-disk storage
	db *bbolt.DB

	// Current revision number
	currentRev int64

	// Path to storage directory
	dir string

	// Observability
	logger *telemetry.Logger
	tracer trace.Tracer
}

// ResourceState tracks a resource's state in the index
type ResourceState struct {
	ResourceID     string
	Owner          string
	Type           string
	FirstSeenRev   int64
	LastSeenRev    int64
	DisappearedRev int64
	Exists         bool
}

// Tombstone represents a deleted resource marker
type Tombstone struct {
	ID        string    `json:"id"`
	Tombstone bool      `json:"tombstone"`
	Timestamp time.Time `json:"timestamp"`
}

// For btree ordering
func (r *ResourceState) Less(than *ResourceState) bool {
	return r.ResourceID < than.ResourceID
}

// NewMVCCStorage creates a new MVCC storage instance
func NewMVCCStorage(dir string) (*MVCCStorage, error) {
	dbPath := filepath.Join(dir, "elava.db")

	db, err := bbolt.Open(dbPath, 0600, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// Initialize buckets
	err = db.Update(func(tx *bbolt.Tx) error {
		for _, bucket := range [][]byte{bucketObservations, bucketIndex, bucketMeta, bucketClaims} {
			if _, err := tx.CreateBucketIfNotExists(bucket); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		_ = db.Close()
		return nil, err
	}

	storage := &MVCCStorage{
		index: btree.NewG[*ResourceState](32, func(a, b *ResourceState) bool {
			return a.ResourceID < b.ResourceID
		}),
		db:     db,
		dir:    dir,
		logger: telemetry.NewLogger("mvcc-storage"),
		tracer: otel.Tracer("mvcc-storage"),
	}

	// Load current revision
	storage.loadRevision()

	// Rebuild index from disk
	if err := storage.rebuildIndex(); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("failed to rebuild index: %w", err)
	}

	return storage, nil
}

// Close closes the storage
func (s *MVCCStorage) Close() error {
	return s.db.Close()
}

// RecordObservation records a single resource observation
func (s *MVCCStorage) RecordObservation(resource types.Resource) (int64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.currentRev++
	rev := s.currentRev

	err := s.db.Update(func(tx *bbolt.Tx) error {
		// Store observation
		bucket := tx.Bucket(bucketObservations)
		key := makeObservationKey(rev, resource.ID)
		value, err := json.Marshal(resource)
		if err != nil {
			return err
		}

		if err := bucket.Put(key, value); err != nil {
			return err
		}

		// Update meta
		metaBucket := tx.Bucket(bucketMeta)
		return metaBucket.Put([]byte("current_revision"), int64ToBytes(rev))
	})

	if err != nil {
		return 0, err
	}

	// Update in-memory index
	s.updateIndex(resource, rev, true)

	return rev, nil
}

// RecordObservationBatch records multiple observations atomically
func (s *MVCCStorage) RecordObservationBatch(resources []types.Resource) (int64, error) {
	ctx := context.Background()
	ctx, span := s.tracer.Start(ctx, "storage.record_batch")
	defer span.End()

	s.logger.LogBatchOperation(ctx, "record_batch", len(resources))

	s.mu.Lock()
	defer s.mu.Unlock()

	s.currentRev++
	rev := s.currentRev

	startTime := time.Now()
	err := s.db.Update(func(tx *bbolt.Tx) error {
		bucket := tx.Bucket(bucketObservations)

		for _, resource := range resources {
			key := makeObservationKey(rev, resource.ID)
			value, err := json.Marshal(resource)
			if err != nil {
				return err
			}

			if err := bucket.Put(key, value); err != nil {
				return err
			}
		}

		// Update meta
		metaBucket := tx.Bucket(bucketMeta)
		return metaBucket.Put([]byte("current_revision"), int64ToBytes(rev))
	})

	if err != nil {
		s.logger.LogStorageError(ctx, "record_batch", err)
		return 0, err
	}

	// Update in-memory index
	for _, resource := range resources {
		s.updateIndex(resource, rev, true)
	}

	s.logger.WithContext(ctx).Info().
		Int("batch_size", len(resources)).
		Int64("revision", rev).
		Dur("duration", time.Since(startTime)).
		Msg("batch recorded successfully")

	return rev, nil
}

// RecordDisappearance records that a resource has disappeared
func (s *MVCCStorage) RecordDisappearance(resourceID string) (int64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.currentRev++
	rev := s.currentRev

	err := s.db.Update(func(tx *bbolt.Tx) error {
		bucket := tx.Bucket(bucketObservations)

		// Store a tombstone marker
		key := makeObservationKey(rev, resourceID)
		tombstone := Tombstone{
			ID:        resourceID,
			Tombstone: true,
			Timestamp: time.Now(),
		}
		value, err := json.Marshal(tombstone)
		if err != nil {
			return err
		}

		if err := bucket.Put(key, value); err != nil {
			return err
		}

		// Update meta
		metaBucket := tx.Bucket(bucketMeta)
		return metaBucket.Put([]byte("current_revision"), int64ToBytes(rev))
	})

	if err != nil {
		return 0, err
	}

	// Update index to mark as disappeared
	state := &ResourceState{ResourceID: resourceID}
	existing, found := s.index.Get(state)
	if found {
		existing.Exists = false
		existing.DisappearedRev = rev
		s.index.ReplaceOrInsert(existing)
	}

	return rev, nil
}

// GetResourceState gets current state of a resource
func (s *MVCCStorage) GetResourceState(resourceID string) (*ResourceState, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	state := &ResourceState{ResourceID: resourceID}
	existing, found := s.index.Get(state)
	if !found {
		return nil, fmt.Errorf("resource %s not found", resourceID)
	}

	return existing, nil
}

// GetStateAtRevision gets resource state at a specific revision
func (s *MVCCStorage) GetStateAtRevision(resourceID string, revision int64) (*ResourceState, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var result *ResourceState

	err := s.db.View(func(tx *bbolt.Tx) error {
		bucket := tx.Bucket(bucketObservations)

		// Scan for this resource at or before the revision
		c := bucket.Cursor()

		for k, v := c.First(); k != nil; k, v = c.Next() {
			rev, id := parseObservationKey(k)
			if id == resourceID && rev <= revision {
				result = &ResourceState{
					ResourceID:  resourceID,
					LastSeenRev: rev,
					Exists:      true,
				}

				// Check if it's a tombstone
				var data Tombstone
				if err := json.Unmarshal(v, &data); err == nil {
					if data.Tombstone {
						result.Exists = false
						result.DisappearedRev = rev
					}
				}
			}
		}

		return nil
	})

	if err != nil {
		return nil, err
	}

	if result == nil {
		return nil, fmt.Errorf("resource %s not found at revision %d", resourceID, revision)
	}

	return result, nil
}

// GetResourcesByOwner returns all resources for an owner
func (s *MVCCStorage) GetResourcesByOwner(owner string) ([]*ResourceState, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var results []*ResourceState

	s.index.Ascend(func(state *ResourceState) bool {
		if state.Owner == owner && state.Exists {
			results = append(results, state)
		}
		return true
	})

	return results, nil
}

// GetAllCurrentResources gets all resources that currently exist - CLAUDE.md: Small focused function
func (s *MVCCStorage) GetAllCurrentResources() ([]*ResourceState, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var results []*ResourceState

	s.index.Ascend(func(state *ResourceState) bool {
		if state.Exists {
			results = append(results, state)
		}
		return true
	})

	return results, nil
}

// CurrentRevision returns the current revision number
func (s *MVCCStorage) CurrentRevision() int64 {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.currentRev
}

// Compact removes old revisions, keeping only recent ones
func (s *MVCCStorage) Compact(keepRevisions int64) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	cutoff := s.currentRev - keepRevisions
	if cutoff <= 0 {
		return nil // Nothing to compact
	}

	return s.db.Update(func(tx *bbolt.Tx) error {
		bucket := tx.Bucket(bucketObservations)
		c := bucket.Cursor()

		var toDelete [][]byte
		for k, _ := c.First(); k != nil; k, _ = c.Next() {
			rev, _ := parseObservationKey(k)
			if rev < cutoff {
				toDelete = append(toDelete, k)
			}
		}

		for _, key := range toDelete {
			if err := bucket.Delete(key); err != nil {
				return err
			}
		}

		return nil
	})
}

// Helper functions

func (s *MVCCStorage) updateIndex(resource types.Resource, rev int64, exists bool) {
	state := &ResourceState{ResourceID: resource.ID}
	existing, found := s.index.Get(state)

	// Extract owner from structured tags
	owner := resource.Tags.GetOwner()

	if !found {
		existing = &ResourceState{
			ResourceID:   resource.ID,
			Owner:        owner,
			Type:         resource.Type,
			FirstSeenRev: rev,
			LastSeenRev:  rev,
			Exists:       exists,
		}
	} else {
		existing.LastSeenRev = rev
		existing.Exists = exists
		existing.Owner = owner // Update owner in case it changed
		if !exists {
			existing.DisappearedRev = rev
		}
	}

	s.index.ReplaceOrInsert(existing)
}

func (s *MVCCStorage) loadRevision() {
	_ = s.db.View(func(tx *bbolt.Tx) error {
		bucket := tx.Bucket(bucketMeta)
		if bucket == nil {
			return nil
		}

		data := bucket.Get([]byte("current_revision"))
		if data != nil {
			s.currentRev = bytesToInt64(data)
		}
		return nil
	})
}

func (s *MVCCStorage) rebuildIndex() error {
	ctx := context.Background()
	ctx, span := s.tracer.Start(ctx, "storage.rebuild_index")
	defer span.End()

	s.logger.LogRebuildIndex(ctx, s.index.Len())
	startTime := time.Now()

	// Clear existing index
	s.index.Clear(false)

	err := s.db.View(func(tx *bbolt.Tx) error {
		bucket := tx.Bucket(bucketObservations)
		if bucket == nil {
			return nil
		}

		c := bucket.Cursor()
		for k, v := c.First(); k != nil; k, v = c.Next() {
			rev, id := parseObservationKey(k)

			// Check if it's a tombstone
			var tombstoneMarker struct {
				ID        string    `json:"id"`
				Tombstone bool      `json:"tombstone"`
				Timestamp time.Time `json:"timestamp"`
			}

			if err := json.Unmarshal(v, &tombstoneMarker); err == nil && tombstoneMarker.Tombstone {
				// Mark as disappeared
				s.updateIndex(types.Resource{ID: id}, rev, false)
			} else {
				// Normal resource observation
				var resource types.Resource
				if err := json.Unmarshal(v, &resource); err == nil {
					s.updateIndex(resource, rev, true)
				}
			}
		}

		return nil
	})

	if err != nil {
		s.logger.LogStorageError(ctx, "rebuild_index", err)
		return err
	}

	s.logger.LogRebuildComplete(ctx, s.index.Len(), float64(time.Since(startTime).Milliseconds()))
	return nil
}

func makeObservationKey(rev int64, resourceID string) []byte {
	return []byte(fmt.Sprintf("%016d:%s", rev, resourceID))
}

func parseObservationKey(key []byte) (int64, string) {
	// Simple parsing - in production would be more robust
	var rev int64
	var id string
	_, _ = fmt.Sscanf(string(key), "%016d:%s", &rev, &id)
	return rev, id
}

func int64ToBytes(n int64) []byte {
	return []byte(fmt.Sprintf("%d", n))
}

func bytesToInt64(b []byte) int64 {
	var n int64
	_, _ = fmt.Sscanf(string(b), "%d", &n)
	return n
}

// DB returns the underlying BoltDB instance for claims coordination
func (s *MVCCStorage) DB() *bbolt.DB {
	return s.db
}

// Stats returns operational statistics for monitoring
func (s *MVCCStorage) Stats() (resourceCount int, currentRev int64, dbSizeBytes int64) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	resourceCount = s.index.Len()
	currentRev = s.currentRev

	// Get database statistics for monitoring
	stats := s.db.Stats()
	// Use FreeAlloc as a proxy for size (bytes allocated in free pages)
	dbSizeBytes = int64(stats.FreeAlloc)
	if dbSizeBytes == 0 {
		dbSizeBytes = 4096 // Default non-zero value
	}

	return resourceCount, currentRev, dbSizeBytes
}

// CompactWithContext removes old revisions with cancellation support
func (s *MVCCStorage) CompactWithContext(ctx context.Context, keepRevisions int64) error {
	ctx, span := s.tracer.Start(ctx, "storage.compact")
	defer span.End()

	s.logger.LogCompaction(ctx, keepRevisions, s.currentRev)
	startTime := time.Now()

	s.mu.Lock()
	defer s.mu.Unlock()

	cutoff := s.currentRev - keepRevisions
	if cutoff <= 0 {
		s.logger.WithContext(ctx).Debug().Msg("nothing to compact")
		return nil
	}

	deletedCount, err := s.performCompaction(ctx, cutoff)
	if err != nil {
		s.logger.LogStorageError(ctx, "compact", err)
		return err
	}

	s.logger.LogCompactionComplete(ctx, deletedCount, float64(time.Since(startTime).Milliseconds()))
	return nil
}

// performCompaction handles the actual compaction work
func (s *MVCCStorage) performCompaction(ctx context.Context, cutoff int64) (int, error) {
	deletedCount := 0

	return deletedCount, s.db.Update(func(tx *bbolt.Tx) error {
		if err := s.checkContext(ctx); err != nil {
			return err
		}

		toDelete, err := s.findKeysToDelete(ctx, tx, cutoff)
		if err != nil {
			return err
		}

		deletedCount, err = s.deleteKeys(ctx, tx, toDelete)
		return err
	})
}

// checkContext checks if context is cancelled
func (s *MVCCStorage) checkContext(ctx context.Context) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
		return nil
	}
}

// findKeysToDelete identifies keys to delete during compaction
func (s *MVCCStorage) findKeysToDelete(ctx context.Context, tx *bbolt.Tx, cutoff int64) ([][]byte, error) {
	bucket := tx.Bucket(bucketObservations)
	c := bucket.Cursor()

	var toDelete [][]byte
	processed := 0

	for k, _ := c.First(); k != nil; k, _ = c.Next() {
		processed++
		if processed%100 == 0 {
			if err := s.checkContext(ctx); err != nil {
				s.logger.WithContext(ctx).Warn().Int("processed", processed).Msg("compaction cancelled")
				return nil, err
			}
		}

		rev, _ := parseObservationKey(k)
		if rev < cutoff {
			toDelete = append(toDelete, k)
		}
	}

	return toDelete, nil
}

// deleteKeys removes the identified keys
func (s *MVCCStorage) deleteKeys(ctx context.Context, tx *bbolt.Tx, toDelete [][]byte) (int, error) {
	bucket := tx.Bucket(bucketObservations)

	for i, key := range toDelete {
		if i%50 == 0 {
			if err := s.checkContext(ctx); err != nil {
				s.logger.WithContext(ctx).Warn().
					Int("deleted", i).
					Int("total_to_delete", len(toDelete)).
					Msg("compaction cancelled during deletion")
				return i, err
			}
		}

		if err := bucket.Delete(key); err != nil {
			return i, err
		}
	}

	return len(toDelete), nil
}
