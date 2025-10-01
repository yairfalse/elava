package storage

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"go.etcd.io/bbolt"
)

// Bucket names for analyzer events
var (
	bucketChanges = []byte("changes")
	bucketDrift   = []byte("drift")
	bucketWaste   = []byte("waste")
)

// StoreChangeEvent stores a change event (generic interface{})
func (s *MVCCStorage) StoreChangeEvent(ctx context.Context, event interface{}) error {
	return s.storeAnalyzerEvent(ctx, bucketChanges, event)
}

// StoreDriftEvent stores a drift event (generic interface{})
func (s *MVCCStorage) StoreDriftEvent(ctx context.Context, event interface{}) error {
	return s.storeAnalyzerEvent(ctx, bucketDrift, event)
}

// StoreWastePattern stores a waste pattern (generic interface{})
func (s *MVCCStorage) StoreWastePattern(ctx context.Context, pattern interface{}) error {
	return s.storeAnalyzerEvent(ctx, bucketWaste, pattern)
}

// storeAnalyzerEvent stores any analyzer event
func (s *MVCCStorage) storeAnalyzerEvent(ctx context.Context, bucketName []byte, data interface{}) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Use current time in nanoseconds for key
	timestamp := s.getCurrentTimestamp()
	key := makeAnalyzerEventKey(timestamp, s.currentRev)

	value, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("failed to marshal event: %w", err)
	}

	err = s.db.Update(func(tx *bbolt.Tx) error {
		bucket, err := tx.CreateBucketIfNotExists(bucketName)
		if err != nil {
			return err
		}
		return bucket.Put(key, value)
	})

	if err != nil {
		return fmt.Errorf("failed to store event: %w", err)
	}

	s.currentRev++
	return nil
}

// getCurrentTimestamp returns current time in nanoseconds
func (s *MVCCStorage) getCurrentTimestamp() int64 {
	return s.getTime().UnixNano()
}

// getTime returns current time (mockable for testing)
func (s *MVCCStorage) getTime() time.Time {
	return time.Now()
}

// makeAnalyzerEventKey creates timestamp-ordered key
func makeAnalyzerEventKey(timestamp, revision int64) []byte {
	return []byte(fmt.Sprintf("%020d:%020d", timestamp, revision))
}

// makeEventKey creates timestamp-ordered key
func makeEventKey(timestamp int64, id string) []byte {
	return []byte(fmt.Sprintf("%020d:%s", timestamp, id))
}
