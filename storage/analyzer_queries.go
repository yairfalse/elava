package storage

import (
	"context"
	"fmt"
	"time"

	"go.etcd.io/bbolt"
)

// QueryChangesSince retrieves change events since a timestamp
func (s *MVCCStorage) QueryChangesSince(ctx context.Context, since time.Time) ([][]byte, error) {
	return s.queryEventsSince(bucketChanges, since)
}

// QueryDriftEvents retrieves drift events since a timestamp
func (s *MVCCStorage) QueryDriftEvents(ctx context.Context, since time.Time) ([][]byte, error) {
	return s.queryEventsSince(bucketDrift, since)
}

// QueryWastePatterns retrieves waste patterns since a timestamp
func (s *MVCCStorage) QueryWastePatterns(ctx context.Context, since time.Time) ([][]byte, error) {
	return s.queryEventsSince(bucketWaste, since)
}

// queryEventsSince is a generic query helper that returns raw JSON
func (s *MVCCStorage) queryEventsSince(bucketName []byte, since time.Time) ([][]byte, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var results [][]byte
	sinceKey := makeEventKey(since.UnixNano(), "")

	err := s.db.View(func(tx *bbolt.Tx) error {
		bucket := tx.Bucket(bucketName)
		if bucket == nil {
			return nil
		}

		c := bucket.Cursor()
		for k, v := c.Seek(sinceKey); k != nil; k, v = c.Next() {
			// Copy value since it's only valid during transaction
			valueCopy := make([]byte, len(v))
			copy(valueCopy, v)
			results = append(results, valueCopy)
		}

		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("query failed: %w", err)
	}

	return results, nil
}
