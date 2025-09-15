package reconciler

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/yairfalse/elava/storage"
	"go.etcd.io/bbolt"
)

// SimpleCoordinator implements resource claiming with MVCC storage brain
type SimpleCoordinator struct {
	storage    *storage.MVCCStorage
	instanceID string
}

// NewSimpleCoordinator creates a new simple coordinator
func NewSimpleCoordinator(storage *storage.MVCCStorage, instanceID string) *SimpleCoordinator {
	return &SimpleCoordinator{
		storage:    storage,
		instanceID: instanceID,
	}
}

// ClaimResources atomically claims resources using MVCC storage brain
func (c *SimpleCoordinator) ClaimResources(ctx context.Context, resourceIDs []string, ttl time.Duration) error {
	now := time.Now()
	expiresAt := now.Add(ttl)

	return c.storage.DB().Update(func(tx *bbolt.Tx) error {
		bucket := tx.Bucket([]byte("claims"))
		if bucket == nil {
			return fmt.Errorf("claims bucket not found")
		}

		// Check all resources before claiming any
		for _, resourceID := range resourceIDs {
			if err := c.checkClaimAvailable(bucket, resourceID, now); err != nil {
				return err
			}
		}

		// Claim all resources atomically
		for _, resourceID := range resourceIDs {
			claim := Claim{
				ResourceID: resourceID,
				InstanceID: c.instanceID,
				ClaimedAt:  now,
				ExpiresAt:  expiresAt,
			}

			data, err := json.Marshal(claim)
			if err != nil {
				return fmt.Errorf("failed to marshal claim for %s: %w", resourceID, err)
			}

			if err := bucket.Put([]byte(resourceID), data); err != nil {
				return fmt.Errorf("failed to store claim for %s: %w", resourceID, err)
			}
		}

		return nil
	})
}

// ReleaseResources atomically releases claimed resources
func (c *SimpleCoordinator) ReleaseResources(ctx context.Context, resourceIDs []string) error {
	return c.storage.DB().Update(func(tx *bbolt.Tx) error {
		bucket := tx.Bucket([]byte("claims"))
		if bucket == nil {
			return nil
		}

		for _, resourceID := range resourceIDs {
			if err := bucket.Delete([]byte(resourceID)); err != nil {
				return fmt.Errorf("failed to release claim for %s: %w", resourceID, err)
			}
		}

		return nil
	})
}

// IsResourceClaimed checks if resource is claimed with TTL enforcement
func (c *SimpleCoordinator) IsResourceClaimed(ctx context.Context, resourceID string) (bool, error) {
	var claimed bool
	now := time.Now()

	err := c.storage.DB().View(func(tx *bbolt.Tx) error {
		bucket := tx.Bucket([]byte("claims"))
		if bucket == nil {
			claimed = false
			return nil
		}

		val := bucket.Get([]byte(resourceID))
		if val == nil {
			claimed = false
			return nil
		}

		var existing Claim
		if err := json.Unmarshal(val, &existing); err != nil {
			claimed = false
			return nil
		}

		if existing.ExpiresAt.After(now) {
			claimed = existing.InstanceID != c.instanceID
		} else {
			claimed = false
		}

		return nil
	})

	return claimed, err
}

// checkClaimAvailable verifies if a resource can be claimed
func (c *SimpleCoordinator) checkClaimAvailable(bucket *bbolt.Bucket, resourceID string, now time.Time) error {
	val := bucket.Get([]byte(resourceID))
	if val == nil {
		return nil
	}

	var existing Claim
	if err := json.Unmarshal(val, &existing); err != nil {
		return nil
	}

	if existing.ExpiresAt.After(now) {
		return fmt.Errorf("resource %s is already claimed by %s", resourceID, existing.InstanceID)
	}

	return nil
}

// CleanupExpiredClaims removes expired claims from storage
func (c *SimpleCoordinator) CleanupExpiredClaims(ctx context.Context) error {
	now := time.Now()

	return c.storage.DB().Update(func(tx *bbolt.Tx) error {
		bucket := tx.Bucket([]byte("claims"))
		if bucket == nil {
			return nil
		}

		c := bucket.Cursor()
		var toDelete [][]byte

		for k, v := c.First(); k != nil; k, v = c.Next() {
			var claim Claim
			if err := json.Unmarshal(v, &claim); err != nil {
				toDelete = append(toDelete, k)
				continue
			}

			if claim.ExpiresAt.Before(now) {
				toDelete = append(toDelete, k)
			}
		}

		for _, key := range toDelete {
			if err := bucket.Delete(key); err != nil {
				return fmt.Errorf("failed to delete expired claim: %w", err)
			}
		}

		return nil
	})
}
