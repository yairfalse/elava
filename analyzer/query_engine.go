package analyzer

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/yairfalse/elava/storage"
	"github.com/yairfalse/elava/types"
	"go.etcd.io/bbolt"
)

// QueryEngineImpl implements temporal queries on MVCC storage
type QueryEngineImpl struct {
	storage *storage.MVCCStorage
}

// NewQueryEngine creates a new query engine
func NewQueryEngine(s *storage.MVCCStorage) *QueryEngineImpl {
	return &QueryEngineImpl{
		storage: s,
	}
}

// QueryByTimeRange returns resources within a time range
func (q *QueryEngineImpl) QueryByTimeRange(ctx context.Context, start, end time.Time) ([]types.Resource, error) {
	var resources []types.Resource
	seen := make(map[string]bool)

	err := q.storage.DB().View(func(tx *bbolt.Tx) error {
		bucket := tx.Bucket([]byte("observations"))
		if bucket == nil {
			return fmt.Errorf("observations bucket not found")
		}

		return q.scanTimeRange(bucket, start, end, &resources, seen)
	})

	return resources, err
}

// scanTimeRange scans observations within time range
func (q *QueryEngineImpl) scanTimeRange(bucket *bbolt.Bucket, start, end time.Time,
	resources *[]types.Resource, seen map[string]bool) error {

	return bucket.ForEach(func(k, v []byte) error {
		var resource types.Resource
		if err := json.Unmarshal(v, &resource); err != nil {
			return nil // Skip malformed entries
		}

		if q.isInTimeRange(resource, start, end) && !seen[resource.ID] {
			*resources = append(*resources, resource)
			seen[resource.ID] = true
		}
		return nil
	})
}

// isInTimeRange checks if resource is within time range
func (q *QueryEngineImpl) isInTimeRange(resource types.Resource, start, end time.Time) bool {
	return resource.LastSeenAt.After(start) && resource.LastSeenAt.Before(end)
}

// QueryChangesSince returns all changes since a revision
func (q *QueryEngineImpl) QueryChangesSince(ctx context.Context, revision int64) ([]ChangeEvent, error) {
	var changes []ChangeEvent

	err := q.storage.DB().View(func(tx *bbolt.Tx) error {
		bucket := tx.Bucket([]byte("observations"))
		if bucket == nil {
			return fmt.Errorf("observations bucket not found")
		}

		return q.scanChangesSince(bucket, revision, &changes)
	})

	return changes, err
}

// scanChangesSince scans for changes after revision
func (q *QueryEngineImpl) scanChangesSince(bucket *bbolt.Bucket, revision int64,
	changes *[]ChangeEvent) error {

	resourceStates := make(map[string]*types.Resource)

	return bucket.ForEach(func(k, v []byte) error {
		rev := q.extractRevision(k)
		if rev <= revision {
			return nil
		}

		var resource types.Resource
		if err := json.Unmarshal(v, &resource); err != nil {
			return nil
		}

		change := q.detectChange(resource, resourceStates[resource.ID])
		if change != nil {
			change.Revision = rev
			*changes = append(*changes, *change)
		}

		resourceStates[resource.ID] = &resource
		return nil
	})
}

// detectChange identifies what changed between states
func (q *QueryEngineImpl) detectChange(current types.Resource, previous *types.Resource) *ChangeEvent {
	if previous == nil {
		return &ChangeEvent{
			Timestamp:  current.CreatedAt,
			ResourceID: current.ID,
			Type:       ChangeCreated,
			After:      &current,
		}
	}

	if current.Status != previous.Status {
		return &ChangeEvent{
			Timestamp:  current.LastSeenAt,
			ResourceID: current.ID,
			Type:       ChangeStateChanged,
			Before:     previous,
			After:      &current,
		}
	}

	if q.tagsChanged(current.Tags, previous.Tags) {
		return &ChangeEvent{
			Timestamp:  current.LastSeenAt,
			ResourceID: current.ID,
			Type:       ChangeTagsChanged,
			Before:     previous,
			After:      &current,
		}
	}

	return nil
}

// tagsChanged checks if tags have changed
func (q *QueryEngineImpl) tagsChanged(current, previous types.Tags) bool {
	return current.ElavaOwner != previous.ElavaOwner ||
		current.Team != previous.Team ||
		current.Project != previous.Project ||
		current.Environment != previous.Environment
}

// QueryResourceHistory returns history of a resource
func (q *QueryEngineImpl) QueryResourceHistory(ctx context.Context, resourceID string) ([]ResourceRevision, error) {
	var revisions []ResourceRevision

	err := q.storage.DB().View(func(tx *bbolt.Tx) error {
		bucket := tx.Bucket([]byte("observations"))
		if bucket == nil {
			return fmt.Errorf("observations bucket not found")
		}

		return q.scanResourceHistory(bucket, resourceID, &revisions)
	})

	return revisions, err
}

// scanResourceHistory scans all revisions of a resource
func (q *QueryEngineImpl) scanResourceHistory(bucket *bbolt.Bucket, resourceID string,
	revisions *[]ResourceRevision) error {

	var lastResource *types.Resource

	return bucket.ForEach(func(k, v []byte) error {
		var resource types.Resource
		if err := json.Unmarshal(v, &resource); err != nil {
			return nil
		}

		if resource.ID != resourceID {
			return nil
		}

		revision := ResourceRevision{
			Revision:  q.extractRevision(k),
			Timestamp: resource.LastSeenAt,
			Resource:  resource,
			Change:    q.determineChangeType(&resource, lastResource),
		}

		*revisions = append(*revisions, revision)
		lastResource = &resource
		return nil
	})
}

// determineChangeType identifies the type of change
func (q *QueryEngineImpl) determineChangeType(current, previous *types.Resource) ChangeType {
	if previous == nil {
		return ChangeCreated
	}
	if current.Status != previous.Status {
		return ChangeStateChanged
	}
	if q.tagsChanged(current.Tags, previous.Tags) {
		return ChangeTagsChanged
	}
	return ChangeModified
}

// AggregateByTag aggregates metrics by tag value
func (q *QueryEngineImpl) AggregateByTag(ctx context.Context, tag string, period time.Duration) (map[string]Metrics, error) {
	metrics := make(map[string]Metrics)
	cutoff := time.Now().Add(-period)

	err := q.storage.DB().View(func(tx *bbolt.Tx) error {
		bucket := tx.Bucket([]byte("observations"))
		if bucket == nil {
			return fmt.Errorf("observations bucket not found")
		}

		return q.aggregateResources(bucket, tag, cutoff, metrics)
	})

	// Calculate averages
	for k, m := range metrics {
		if m.Count > 0 {
			m.AverageCost = m.TotalCost / float64(m.Count)
			metrics[k] = m
		}
	}

	return metrics, err
}

// aggregateResources aggregates resources by tag
func (q *QueryEngineImpl) aggregateResources(bucket *bbolt.Bucket, tag string,
	cutoff time.Time, metrics map[string]Metrics) error {

	return bucket.ForEach(func(k, v []byte) error {
		var resource types.Resource
		if err := json.Unmarshal(v, &resource); err != nil {
			return nil
		}

		if resource.LastSeenAt.Before(cutoff) {
			return nil
		}

		tagValue := q.getTagValue(resource.Tags, tag)
		if tagValue == "" {
			tagValue = "untagged"
		}

		m := metrics[tagValue]
		q.updateMetrics(&m, resource)
		metrics[tagValue] = m
		return nil
	})
}

// getTagValue extracts tag value by name
func (q *QueryEngineImpl) getTagValue(tags types.Tags, tagName string) string {
	switch tagName {
	case "owner":
		return tags.ElavaOwner
	case "team":
		return tags.Team
	case "project":
		return tags.Project
	case "environment":
		return tags.Environment
	case "cost_center":
		return tags.CostCenter
	default:
		return ""
	}
}

// updateMetrics updates aggregated metrics
func (q *QueryEngineImpl) updateMetrics(m *Metrics, resource types.Resource) {
	if m.ResourceTypes == nil {
		m.ResourceTypes = make(map[string]int)
	}

	m.Count++
	m.ResourceTypes[resource.Type]++

	// Extract cost from metadata if available
	if cost, ok := resource.Metadata["monthly_cost_estimate"].(float64); ok {
		m.TotalCost += cost
	}
}

// extractRevision gets revision from key
func (q *QueryEngineImpl) extractRevision(key []byte) int64 {
	// Key format: revision_resourceID
	// Simple extraction - improve if needed
	var rev int64
	_, _ = fmt.Sscanf(string(key), "%d_", &rev)
	return rev
}
