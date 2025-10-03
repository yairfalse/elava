package storage

import (
	"context"
	"sync"
	"testing"
	"time"
)

// TestConcurrentAnalyzerEventWrites tests concurrent writes to analyzer events
func TestConcurrentAnalyzerEventWrites(t *testing.T) {
	tmpDir := t.TempDir()
	storage, err := NewMVCCStorage(tmpDir)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = storage.Close() }()

	ctx := context.Background()
	const numGoroutines = 10
	const eventsPerGoroutine = 20

	var wg sync.WaitGroup
	wg.Add(numGoroutines)

	// Launch concurrent writers
	for i := 0; i < numGoroutines; i++ {
		go func(id int) {
			defer wg.Done()
			for j := 0; j < eventsPerGoroutine; j++ {
				event := ChangeEvent{
					ResourceID: "concurrent-test",
					ChangeType: "created",
					Timestamp:  time.Now(),
				}
				if err := storage.StoreChangeEvent(ctx, event); err != nil {
					t.Errorf("goroutine %d: StoreChangeEvent failed: %v", id, err)
				}
			}
		}(i)
	}

	wg.Wait()

	// Verify all events were stored
	events, err := storage.QueryChangesSince(ctx, time.Now().Add(-1*time.Hour))
	if err != nil {
		t.Fatalf("QueryChangesSince failed: %v", err)
	}

	expectedCount := numGoroutines * eventsPerGoroutine
	if len(events) != expectedCount {
		t.Errorf("Expected %d events, got %d", expectedCount, len(events))
	}

	// Verify revision incremented correctly
	finalRev := storage.CurrentRevision()
	if finalRev != int64(expectedCount) {
		t.Errorf("Revision = %d, want %d", finalRev, expectedCount)
	}
}

// TestConcurrentBatchWrites tests concurrent batch writes
func TestConcurrentBatchWrites(t *testing.T) {
	tmpDir := t.TempDir()
	storage, err := NewMVCCStorage(tmpDir)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = storage.Close() }()

	ctx := context.Background()
	const numGoroutines = 5
	const batchSize = 10

	var wg sync.WaitGroup
	wg.Add(numGoroutines)

	for i := 0; i < numGoroutines; i++ {
		go func(id int) {
			defer wg.Done()
			events := make([]DriftEvent, batchSize)
			for j := 0; j < batchSize; j++ {
				events[j] = DriftEvent{
					ResourceID: "batch-concurrent",
					DriftType:  "test",
					Field:      "test",
					Expected:   "a",
					Actual:     "b",
					Severity:   "low",
					Timestamp:  time.Now(),
				}
			}
			if err := storage.StoreDriftEventBatch(ctx, events); err != nil {
				t.Errorf("goroutine %d: StoreDriftEventBatch failed: %v", id, err)
			}
		}(i)
	}

	wg.Wait()

	// Verify correct count
	events, err := storage.QueryDriftEvents(ctx, time.Now().Add(-1*time.Hour))
	if err != nil {
		t.Fatalf("QueryDriftEvents failed: %v", err)
	}

	expectedCount := numGoroutines * batchSize
	if len(events) != expectedCount {
		t.Errorf("Expected %d events, got %d", expectedCount, len(events))
	}
}

// TestRevisionNumberConsistency verifies revision numbers are sequential and unique
func TestRevisionNumberConsistency(t *testing.T) {
	tmpDir := t.TempDir()
	storage, err := NewMVCCStorage(tmpDir)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = storage.Close() }()

	ctx := context.Background()

	// Store events and track expected revisions
	initialRev := storage.CurrentRevision()

	// Single event
	if err := storage.StoreChangeEvent(ctx, ChangeEvent{
		ResourceID: "r1",
		ChangeType: "created",
		Timestamp:  time.Now(),
	}); err != nil {
		t.Fatal(err)
	}

	rev1 := storage.CurrentRevision()
	if rev1 != initialRev+1 {
		t.Errorf("After 1 event: revision = %d, want %d", rev1, initialRev+1)
	}

	// Batch of 3
	if err := storage.StoreChangeEventBatch(ctx, []ChangeEvent{
		{ResourceID: "r2", ChangeType: "created", Timestamp: time.Now()},
		{ResourceID: "r3", ChangeType: "created", Timestamp: time.Now()},
		{ResourceID: "r4", ChangeType: "created", Timestamp: time.Now()},
	}); err != nil {
		t.Fatal(err)
	}

	rev2 := storage.CurrentRevision()
	if rev2 != rev1+3 {
		t.Errorf("After batch of 3: revision = %d, want %d", rev2, rev1+3)
	}

	// Another single
	if err := storage.StoreDriftEvent(ctx, DriftEvent{
		ResourceID: "r5",
		DriftType:  "test",
		Field:      "f",
		Expected:   "e",
		Actual:     "a",
		Severity:   "low",
		Timestamp:  time.Now(),
	}); err != nil {
		t.Fatal(err)
	}

	finalRev := storage.CurrentRevision()
	if finalRev != rev2+1 {
		t.Errorf("After final event: revision = %d, want %d", finalRev, rev2+1)
	}
}

// TestEmptyBatch tests behavior with empty batch
func TestEmptyBatch(t *testing.T) {
	tmpDir := t.TempDir()
	storage, err := NewMVCCStorage(tmpDir)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = storage.Close() }()

	ctx := context.Background()
	initialRev := storage.CurrentRevision()

	// Store empty batch - should succeed but not increment revision
	emptyEvents := []ChangeEvent{}
	if err := storage.StoreChangeEventBatch(ctx, emptyEvents); err != nil {
		t.Errorf("Empty batch should succeed: %v", err)
	}

	finalRev := storage.CurrentRevision()
	if finalRev != initialRev {
		t.Errorf("Empty batch changed revision: %d → %d", initialRev, finalRev)
	}
}

// TestMixedEventTypes ensures different event types don't interfere
func TestMixedEventTypes(t *testing.T) {
	tmpDir := t.TempDir()
	storage, err := NewMVCCStorage(tmpDir)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = storage.Close() }()

	ctx := context.Background()

	// Store one of each type
	changeEvent := ChangeEvent{
		ResourceID: "r1",
		ChangeType: "created",
		Timestamp:  time.Now(),
	}
	if err := storage.StoreChangeEvent(ctx, changeEvent); err != nil {
		t.Fatal(err)
	}

	driftEvent := DriftEvent{
		ResourceID: "r2",
		DriftType:  "config",
		Field:      "size",
		Expected:   "t2.micro",
		Actual:     "t2.small",
		Severity:   "medium",
		Timestamp:  time.Now(),
	}
	if err := storage.StoreDriftEvent(ctx, driftEvent); err != nil {
		t.Fatal(err)
	}

	wastePattern := WastePattern{
		PatternType: "idle",
		ResourceIDs: []string{"r3"},
		Confidence:  0.9,
		Reason:      "low usage",
		Timestamp:   time.Now(),
	}
	if err := storage.StoreWastePattern(ctx, wastePattern); err != nil {
		t.Fatal(err)
	}

	// Query each type independently
	changes, err := storage.QueryChangesSince(ctx, time.Now().Add(-1*time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	if len(changes) != 1 {
		t.Errorf("Changes: got %d, want 1", len(changes))
	}

	drifts, err := storage.QueryDriftEvents(ctx, time.Now().Add(-1*time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	if len(drifts) != 1 {
		t.Errorf("Drifts: got %d, want 1", len(drifts))
	}

	wastes, err := storage.QueryWastePatterns(ctx, time.Now().Add(-1*time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	if len(wastes) != 1 {
		t.Errorf("Wastes: got %d, want 1", len(wastes))
	}

	// Verify total revision
	finalRev := storage.CurrentRevision()
	if finalRev != 3 {
		t.Errorf("Revision = %d, want 3", finalRev)
	}
}
