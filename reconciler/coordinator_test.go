package reconciler

import (
	"context"
	"testing"
	"time"

	"github.com/yairfalse/elava/storage"
)

func TestSimpleCoordinator_ClaimResources(t *testing.T) {
	tmpDir := t.TempDir()
	storageInstance, err := storage.NewMVCCStorage(tmpDir)
	if err != nil {
		t.Fatalf("Failed to create storage: %v", err)
	}
	defer func() { _ = storageInstance.Close() }()

	coordinator := NewSimpleCoordinator(storageInstance, "test-instance")

	ctx := context.Background()
	resourceIDs := []string{"i-123", "i-456"}
	ttl := time.Minute

	err = coordinator.ClaimResources(ctx, resourceIDs, ttl)
	if err != nil {
		t.Errorf("ClaimResources() error = %v", err)
	}

	// Verify resources are claimed
	for _, resourceID := range resourceIDs {
		claimed, err := coordinator.IsResourceClaimed(ctx, resourceID)
		if err != nil {
			t.Fatalf("IsResourceClaimed() error = %v", err)
		}
		if claimed {
			t.Errorf("Resource %s should not be claimed by same instance", resourceID)
		}
	}
}

func TestSimpleCoordinator_ClaimAlreadyClaimed(t *testing.T) {
	tmpDir := t.TempDir()
	storageInstance, err := storage.NewMVCCStorage(tmpDir)
	if err != nil {
		t.Fatalf("Failed to create storage: %v", err)
	}
	defer func() { _ = storageInstance.Close() }()

	coordinator1 := NewSimpleCoordinator(storageInstance, "instance-1")
	coordinator2 := NewSimpleCoordinator(storageInstance, "instance-2")

	ctx := context.Background()
	resourceIDs := []string{"i-123"}
	ttl := time.Minute

	// First instance claims resource
	err = coordinator1.ClaimResources(ctx, resourceIDs, ttl)
	if err != nil {
		t.Fatalf("First ClaimResources() error = %v", err)
	}

	// Second instance tries to claim same resource - should fail
	err = coordinator2.ClaimResources(ctx, resourceIDs, ttl)
	if err == nil {
		t.Error("Second ClaimResources() should have failed")
	}
}

func TestSimpleCoordinator_ReleaseResources(t *testing.T) {
	tmpDir := t.TempDir()
	storageInstance, err := storage.NewMVCCStorage(tmpDir)
	if err != nil {
		t.Fatalf("Failed to create storage: %v", err)
	}
	defer func() { _ = storageInstance.Close() }()

	coordinator := NewSimpleCoordinator(storageInstance, "test-instance")

	ctx := context.Background()
	resourceIDs := []string{"i-123", "i-456"}

	// First claim the resources
	err = coordinator.ClaimResources(ctx, resourceIDs, time.Minute)
	if err != nil {
		t.Fatalf("ClaimResources() error = %v", err)
	}

	// Then release them
	err = coordinator.ReleaseResources(ctx, resourceIDs)
	if err != nil {
		t.Errorf("ReleaseResources() error = %v", err)
	}

	// Verify resources can be claimed again
	err = coordinator.ClaimResources(ctx, resourceIDs, time.Minute)
	if err != nil {
		t.Errorf("ClaimResources() after release error = %v", err)
	}
}

func TestSimpleCoordinator_IsResourceClaimed(t *testing.T) {
	tmpDir := t.TempDir()
	storageInstance, err := storage.NewMVCCStorage(tmpDir)
	if err != nil {
		t.Fatalf("Failed to create storage: %v", err)
	}
	defer func() { _ = storageInstance.Close() }()

	coordinator1 := NewSimpleCoordinator(storageInstance, "instance-1")
	coordinator2 := NewSimpleCoordinator(storageInstance, "instance-2")

	ctx := context.Background()
	resourceID := "i-123"

	// Check unclaimed resource
	claimed, err := coordinator1.IsResourceClaimed(ctx, resourceID)
	if err != nil {
		t.Fatalf("IsResourceClaimed() error = %v", err)
	}
	if claimed {
		t.Errorf("IsResourceClaimed() should return false for unclaimed resource")
	}

	// Claim resource with instance 1
	err = coordinator1.ClaimResources(ctx, []string{resourceID}, time.Minute)
	if err != nil {
		t.Fatalf("ClaimResources() error = %v", err)
	}

	// Check from instance 1 perspective (should be false)
	claimed, err = coordinator1.IsResourceClaimed(ctx, resourceID)
	if err != nil {
		t.Fatalf("IsResourceClaimed() error = %v", err)
	}
	if claimed {
		t.Errorf("IsResourceClaimed() should return false for own claim")
	}

	// Check from instance 2 perspective (should be true)
	claimed, err = coordinator2.IsResourceClaimed(ctx, resourceID)
	if err != nil {
		t.Fatalf("IsResourceClaimed() error = %v", err)
	}
	if !claimed {
		t.Errorf("IsResourceClaimed() should return true for other instance claim")
	}
}

func TestSimpleCoordinator_TTLExpiry(t *testing.T) {
	tmpDir := t.TempDir()
	storageInstance, err := storage.NewMVCCStorage(tmpDir)
	if err != nil {
		t.Fatalf("Failed to create storage: %v", err)
	}
	defer func() { _ = storageInstance.Close() }()

	coordinator1 := NewSimpleCoordinator(storageInstance, "instance-1")
	coordinator2 := NewSimpleCoordinator(storageInstance, "instance-2")

	ctx := context.Background()
	resourceID := "i-123"
	shortTTL := 100 * time.Millisecond

	// Claim resource with short TTL
	err = coordinator1.ClaimResources(ctx, []string{resourceID}, shortTTL)
	if err != nil {
		t.Fatalf("ClaimResources() error = %v", err)
	}

	// Verify it's claimed
	claimed, err := coordinator2.IsResourceClaimed(ctx, resourceID)
	if err != nil {
		t.Fatalf("IsResourceClaimed() error = %v", err)
	}
	if !claimed {
		t.Errorf("Resource should be claimed initially")
	}

	// Wait for TTL to expire
	time.Sleep(150 * time.Millisecond)

	// Check that claim has expired
	claimed, err = coordinator2.IsResourceClaimed(ctx, resourceID)
	if err != nil {
		t.Fatalf("IsResourceClaimed() error = %v", err)
	}
	if claimed {
		t.Errorf("Resource should not be claimed after TTL expiry")
	}

	// Should be able to claim again
	err = coordinator2.ClaimResources(ctx, []string{resourceID}, time.Minute)
	if err != nil {
		t.Errorf("ClaimResources() after TTL expiry error = %v", err)
	}
}

func TestSimpleCoordinator_CleanupExpiredClaims(t *testing.T) {
	tmpDir := t.TempDir()
	storageInstance, err := storage.NewMVCCStorage(tmpDir)
	if err != nil {
		t.Fatalf("Failed to create storage: %v", err)
	}
	defer func() { _ = storageInstance.Close() }()

	coordinator := NewSimpleCoordinator(storageInstance, "test-instance")

	ctx := context.Background()
	resourceIDs := []string{"i-123", "i-456"}
	shortTTL := 50 * time.Millisecond

	// Claim resources with short TTL
	err = coordinator.ClaimResources(ctx, resourceIDs, shortTTL)
	if err != nil {
		t.Fatalf("ClaimResources() error = %v", err)
	}

	// Wait for TTL to expire
	time.Sleep(100 * time.Millisecond)

	// Run cleanup
	err = coordinator.CleanupExpiredClaims(ctx)
	if err != nil {
		t.Errorf("CleanupExpiredClaims() error = %v", err)
	}

	// Resources should be available for claiming again
	err = coordinator.ClaimResources(ctx, resourceIDs, time.Minute)
	if err != nil {
		t.Errorf("ClaimResources() after cleanup error = %v", err)
	}
}

func TestSimpleCoordinator_AtomicClaiming(t *testing.T) {
	tmpDir := t.TempDir()
	storageInstance, err := storage.NewMVCCStorage(tmpDir)
	if err != nil {
		t.Fatalf("Failed to create storage: %v", err)
	}
	defer func() { _ = storageInstance.Close() }()

	coordinator1 := NewSimpleCoordinator(storageInstance, "instance-1")
	coordinator2 := NewSimpleCoordinator(storageInstance, "instance-2")

	ctx := context.Background()
	resourceIDs := []string{"i-123", "i-456", "i-789"}
	ttl := time.Minute

	// Instance 1 claims first resource
	err = coordinator1.ClaimResources(ctx, []string{resourceIDs[0]}, ttl)
	if err != nil {
		t.Fatalf("ClaimResources() error = %v", err)
	}

	// Instance 2 tries to claim all three resources (should fail atomically)
	err = coordinator2.ClaimResources(ctx, resourceIDs, ttl)
	if err == nil {
		t.Error("ClaimResources() should fail when one resource is already claimed")
	}

	// Verify none of the resources were claimed by instance 2
	for i, resourceID := range resourceIDs {
		claimed, err := coordinator1.IsResourceClaimed(ctx, resourceID)
		if err != nil {
			t.Fatalf("IsResourceClaimed() error = %v", err)
		}
		if i == 0 {
			// First resource should not be claimed by coordinator1 (it's their own claim)
			if claimed {
				t.Errorf("Resource %s should not show as claimed by owning instance", resourceID)
			}
		} else {
			// Other resources should be available
			if claimed {
				t.Errorf("Resource %s should not be claimed after failed atomic operation", resourceID)
			}
		}
	}
}

func TestNewSimpleCoordinator(t *testing.T) {
	tmpDir := t.TempDir()
	storageInstance, err := storage.NewMVCCStorage(tmpDir)
	if err != nil {
		t.Fatalf("Failed to create storage: %v", err)
	}
	defer func() { _ = storageInstance.Close() }()

	coordinator := NewSimpleCoordinator(storageInstance, "test-instance")

	if coordinator == nil {
		t.Error("NewSimpleCoordinator() returned nil")
		return
	}

	if coordinator.instanceID != "test-instance" {
		t.Errorf("NewSimpleCoordinator().instanceID = %v, want test-instance", coordinator.instanceID)
	}

	if coordinator.storage != storageInstance {
		t.Error("NewSimpleCoordinator() storage not set correctly")
	}
}

func TestClaim_Struct(t *testing.T) {
	now := time.Now()
	claim := Claim{
		ResourceID: "i-123",
		InstanceID: "test-instance",
		ClaimedAt:  now,
		ExpiresAt:  now.Add(time.Minute),
	}

	if claim.ResourceID != "i-123" {
		t.Errorf("Claim.ResourceID = %v, want i-123", claim.ResourceID)
	}

	if claim.InstanceID != "test-instance" {
		t.Errorf("Claim.InstanceID = %v, want test-instance", claim.InstanceID)
	}

	if claim.ClaimedAt != now {
		t.Errorf("Claim.ClaimedAt = %v, want %v", claim.ClaimedAt, now)
	}

	expectedExpiry := now.Add(time.Minute)
	if claim.ExpiresAt != expectedExpiry {
		t.Errorf("Claim.ExpiresAt = %v, want %v", claim.ExpiresAt, expectedExpiry)
	}
}
