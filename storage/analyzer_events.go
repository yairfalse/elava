package storage

import (
	"context"
	"encoding/binary"
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

// StoreChangeEvent stores a change event
func (s *MVCCStorage) StoreChangeEvent(ctx context.Context, event ChangeEvent) error {
	return storeAnalyzerEvent(s, ctx, bucketChanges, event)
}

// StoreDriftEvent stores a drift event
func (s *MVCCStorage) StoreDriftEvent(ctx context.Context, event DriftEvent) error {
	return storeAnalyzerEvent(s, ctx, bucketDrift, event)
}

// StoreWastePattern stores a waste pattern
func (s *MVCCStorage) StoreWastePattern(ctx context.Context, pattern WastePattern) error {
	return storeAnalyzerEvent(s, ctx, bucketWaste, pattern)
}

// StoreChangeEventBatch stores multiple change events atomically
func (s *MVCCStorage) StoreChangeEventBatch(ctx context.Context, events []ChangeEvent) error {
	return storeAnalyzerEventBatch(s, ctx, bucketChanges, events)
}

// StoreDriftEventBatch stores multiple drift events atomically
func (s *MVCCStorage) StoreDriftEventBatch(ctx context.Context, events []DriftEvent) error {
	return storeAnalyzerEventBatch(s, ctx, bucketDrift, events)
}

// StoreWastePatternBatch stores multiple waste patterns atomically
func (s *MVCCStorage) StoreWastePatternBatch(ctx context.Context, patterns []WastePattern) error {
	return storeAnalyzerEventBatch(s, ctx, bucketWaste, patterns)
}

// storeAnalyzerEventBatch stores multiple events in a single transaction using generics
func storeAnalyzerEventBatch[T any](s *MVCCStorage, ctx context.Context, bucketName []byte, events []T) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Get base timestamp for all events in batch
	baseTimestamp := s.getCurrentTimestamp()

	err := s.db.Update(func(tx *bbolt.Tx) error {
		bucket, err := tx.CreateBucketIfNotExists(bucketName)
		if err != nil {
			return fmt.Errorf("failed to create bucket %s: %w", bucketName, err)
		}

		// Store each event in the batch
		for i, event := range events {
			key := makeAnalyzerEventKey(baseTimestamp, s.currentRev+int64(i+1))
			value, err := json.Marshal(event)
			if err != nil {
				return fmt.Errorf("failed to marshal event at index %d: %w", i, err)
			}
			if err := bucket.Put(key, value); err != nil {
				return fmt.Errorf("failed to put event at index %d: %w", i, err)
			}
		}

		// Only increment revision on successful transaction
		s.currentRev += int64(len(events))
		return nil
	})

	if err != nil {
		return fmt.Errorf("failed to store event batch in bucket %s: %w", bucketName, err)
	}

	return nil
}

// storeAnalyzerEvent stores any analyzer event using generics
func storeAnalyzerEvent[T any](s *MVCCStorage, ctx context.Context, bucketName []byte, data T) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Use current time in nanoseconds for key
	timestamp := s.getCurrentTimestamp()

	value, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("failed to marshal event for bucket %s: %w", bucketName, err)
	}

	err = s.db.Update(func(tx *bbolt.Tx) error {
		bucket, err := tx.CreateBucketIfNotExists(bucketName)
		if err != nil {
			return fmt.Errorf("failed to create bucket %s: %w", bucketName, err)
		}

		// Create key inside transaction with next revision
		key := makeAnalyzerEventKey(timestamp, s.currentRev+1)
		if err := bucket.Put(key, value); err != nil {
			return fmt.Errorf("failed to put event: %w", err)
		}

		// Only increment revision on successful transaction
		s.currentRev++
		return nil
	})

	if err != nil {
		return fmt.Errorf("failed to store event in bucket %s: %w", bucketName, err)
	}

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

// makeAnalyzerEventKey creates timestamp-ordered key for analyzer events
// Uses timestamp (nanoseconds) + revision for uniqueness and ordering
func makeAnalyzerEventKey(timestamp, revision int64) []byte {
	key := make([]byte, 16)
	binary.BigEndian.PutUint64(key[0:8], uint64(timestamp)) //nolint:gosec // timestamp is always positive
	binary.BigEndian.PutUint64(key[8:16], uint64(revision)) //nolint:gosec // revision is always positive
	return key
}
