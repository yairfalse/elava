package storage

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"go.etcd.io/bbolt"
)

// QueryChangesSince retrieves change events since a timestamp
func (s *MVCCStorage) QueryChangesSince(ctx context.Context, since time.Time) ([]ChangeEvent, error) {
	rawEvents, err := s.queryEventsSince(ctx, bucketChanges, since)
	if err != nil {
		return nil, err
	}

	events := make([]ChangeEvent, 0, len(rawEvents))
	for _, data := range rawEvents {
		var event ChangeEvent
		if err := json.Unmarshal(data, &event); err != nil {
			continue // Skip malformed events
		}
		events = append(events, event)
	}
	return events, nil
}

// QueryDriftEvents retrieves drift events since a timestamp
func (s *MVCCStorage) QueryDriftEvents(ctx context.Context, since time.Time) ([]DriftEvent, error) {
	rawEvents, err := s.queryEventsSince(ctx, bucketDrift, since)
	if err != nil {
		return nil, err
	}

	events := make([]DriftEvent, 0, len(rawEvents))
	for _, data := range rawEvents {
		var event DriftEvent
		if err := json.Unmarshal(data, &event); err != nil {
			continue // Skip malformed events
		}
		events = append(events, event)
	}
	return events, nil
}

// QueryWastePatterns retrieves waste patterns since a timestamp
func (s *MVCCStorage) QueryWastePatterns(ctx context.Context, since time.Time) ([]WastePattern, error) {
	rawEvents, err := s.queryEventsSince(ctx, bucketWaste, since)
	if err != nil {
		return nil, err
	}

	patterns := make([]WastePattern, 0, len(rawEvents))
	for _, data := range rawEvents {
		var pattern WastePattern
		if err := json.Unmarshal(data, &pattern); err != nil {
			continue // Skip malformed events
		}
		patterns = append(patterns, pattern)
	}
	return patterns, nil
}

// queryEventsSince is a generic query helper that returns raw JSON
func (s *MVCCStorage) queryEventsSince(ctx context.Context, bucketName []byte, since time.Time) ([][]byte, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var results [][]byte
	// Increment timestamp to exclude events at the exact 'since' time with any revision
	sinceKey := makeAnalyzerEventKey(since.UnixNano()+1, 0)

	err := s.db.View(func(tx *bbolt.Tx) error {
		// Check context cancellation
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		bucket := tx.Bucket(bucketName)
		if bucket == nil {
			return nil
		}

		c := bucket.Cursor()
		for k, v := c.Seek(sinceKey); k != nil; k, v = c.Next() {
			// Check context periodically during iteration
			select {
			case <-ctx.Done():
				return ctx.Err()
			default:
			}

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
