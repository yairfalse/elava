package storage

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/yairfalse/elava/types"
	"go.etcd.io/bbolt"
)

// StoreEnforcement stores an enforcement event
func (s *MVCCStorage) StoreEnforcement(ctx context.Context, event types.EnforcementEvent) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Generate key with timestamp for ordering
	key := fmt.Sprintf("enforcement:%d:%s", event.Timestamp.UnixNano(), event.ResourceID)

	// Serialize event
	data, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("failed to marshal enforcement event: %w", err)
	}

	// Store in database
	err = s.db.Update(func(tx *bbolt.Tx) error {
		bucket, err := tx.CreateBucketIfNotExists([]byte("enforcements"))
		if err != nil {
			return err
		}
		return bucket.Put([]byte(key), data)
	})

	if err != nil {
		return fmt.Errorf("failed to store enforcement event: %w", err)
	}

	s.currentRev++
	return nil
}

// QueryEnforcements retrieves enforcement events by filter
func (s *MVCCStorage) QueryEnforcements(ctx context.Context, filter types.ResourceFilter) ([]types.EnforcementEvent, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var events []types.EnforcementEvent
	prefix := []byte("enforcement:")

	err := s.db.View(func(tx *bbolt.Tx) error {
		bucket := tx.Bucket([]byte("enforcements"))
		if bucket == nil {
			return nil // No events yet
		}

		// Scan all keys with prefix
		c := bucket.Cursor()
		for k, v := c.Seek(prefix); k != nil && len(k) >= len(prefix); k, v = c.Next() {
			// Check prefix match
			if string(k[:len(prefix)]) != string(prefix) {
				break
			}

			// Deserialize event
			var event types.EnforcementEvent
			if err := json.Unmarshal(v, &event); err != nil {
				continue // Skip malformed events
			}

			// Apply filters
			if !matchesFilter(event, filter) {
				continue
			}

			events = append(events, event)
		}
		return nil
	})

	return events, err
}

func matchesFilter(event types.EnforcementEvent, filter types.ResourceFilter) bool {
	// Filter by resource IDs
	if len(filter.IDs) > 0 {
		found := false
		for _, id := range filter.IDs {
			if event.ResourceID == id {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}

	// Filter by type
	if filter.Type != "" && event.ResourceType != filter.Type {
		return false
	}

	// Filter by provider
	if filter.Provider != "" && event.Provider != filter.Provider {
		return false
	}

	// Note: ResourceFilter doesn't have Since field, so we don't filter by time
	// Could add a separate EnforcementFilter type if needed

	return true
}
