package storage

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"sync"
	"time"

	"github.com/google/btree"
	"github.com/yairfalse/ovi/types"
	"go.etcd.io/bbolt"
)

// Bucket names in bbolt
var (
	bucketObservations = []byte("observations")
	bucketIndex        = []byte("index")
	bucketMeta         = []byte("meta")
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

// For btree ordering
func (r *ResourceState) Less(than *ResourceState) bool {
	return r.ResourceID < than.ResourceID
}

// NewMVCCStorage creates a new MVCC storage instance
func NewMVCCStorage(dir string) (*MVCCStorage, error) {
	dbPath := filepath.Join(dir, "ovi.db")

	db, err := bbolt.Open(dbPath, 0600, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// Initialize buckets
	err = db.Update(func(tx *bbolt.Tx) error {
		for _, bucket := range [][]byte{bucketObservations, bucketIndex, bucketMeta} {
			if _, err := tx.CreateBucketIfNotExists(bucket); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		db.Close()
		return nil, err
	}

	storage := &MVCCStorage{
		index: btree.NewG[*ResourceState](32, func(a, b *ResourceState) bool {
			return a.ResourceID < b.ResourceID
		}),
		db:  db,
		dir: dir,
	}

	// Load current revision
	storage.loadRevision()

	// Rebuild index from disk
	storage.rebuildIndex()

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
	s.mu.Lock()
	defer s.mu.Unlock()

	s.currentRev++
	rev := s.currentRev

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
		return 0, err
	}

	// Update in-memory index
	for _, resource := range resources {
		s.updateIndex(resource, rev, true)
	}

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
		tombstone := map[string]interface{}{
			"id":        resourceID,
			"tombstone": true,
			"timestamp": time.Now(),
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
				var data map[string]interface{}
				if err := json.Unmarshal(v, &data); err == nil {
					if tombstone, ok := data["tombstone"].(bool); ok && tombstone {
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
	s.db.View(func(tx *bbolt.Tx) error {
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

func (s *MVCCStorage) rebuildIndex() {
	// This would scan all observations and rebuild the index
	// For now, keeping it simple
}

func makeObservationKey(rev int64, resourceID string) []byte {
	return []byte(fmt.Sprintf("%016d:%s", rev, resourceID))
}

func parseObservationKey(key []byte) (int64, string) {
	// Simple parsing - in production would be more robust
	var rev int64
	var id string
	fmt.Sscanf(string(key), "%016d:%s", &rev, &id)
	return rev, id
}

func int64ToBytes(n int64) []byte {
	return []byte(fmt.Sprintf("%d", n))
}

func bytesToInt64(b []byte) int64 {
	var n int64
	fmt.Sscanf(string(b), "%d", &n)
	return n
}
