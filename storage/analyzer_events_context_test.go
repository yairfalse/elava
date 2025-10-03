package storage

import (
	"context"
	"strings"
	"testing"
	"time"
)

// TestStoreChangeEventBatch_ContextCancellation tests that cancelled context is respected
func TestStoreChangeEventBatch_ContextCancellation(t *testing.T) {
	tmpDir := t.TempDir()
	storage, err := NewMVCCStorage(tmpDir)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = storage.Close() }()

	// Create a context that's already cancelled
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	// Create a large batch to increase chance of catching cancellation
	events := make([]ChangeEvent, 1000)
	for i := 0; i < 1000; i++ {
		events[i] = ChangeEvent{
			ResourceID: "i-test",
			ChangeType: "created",
			Timestamp:  time.Now(),
		}
	}

	err = storage.StoreChangeEventBatch(ctx, events)
	if err == nil {
		t.Fatal("Expected error for cancelled context, got nil")
	}

	if !strings.Contains(err.Error(), "cancelled") {
		t.Errorf("Error should mention cancellation, got: %q", err.Error())
	}

	// Verify nothing was stored
	storedEvents, err := storage.QueryChangesSince(context.Background(), time.Now().Add(-1*time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	if len(storedEvents) != 0 {
		t.Errorf("Expected 0 events stored after cancellation, got %d", len(storedEvents))
	}
}

// TestStoreDriftEventBatch_ContextTimeout tests timeout handling
func TestStoreDriftEventBatch_ContextTimeout(t *testing.T) {
	tmpDir := t.TempDir()
	storage, err := NewMVCCStorage(tmpDir)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = storage.Close() }()

	// Create a context with very short timeout
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Nanosecond)
	defer cancel()

	// Wait a bit to ensure timeout
	time.Sleep(10 * time.Millisecond)

	events := make([]DriftEvent, 100)
	for i := 0; i < 100; i++ {
		events[i] = DriftEvent{
			ResourceID: "i-test",
			DriftType:  "test",
			Field:      "test",
			Expected:   "a",
			Actual:     "b",
			Severity:   "low",
			Timestamp:  time.Now(),
		}
	}

	err = storage.StoreDriftEventBatch(ctx, events)
	if err == nil {
		t.Fatal("Expected error for timeout context, got nil")
	}

	if !strings.Contains(err.Error(), "cancelled") && !strings.Contains(err.Error(), "deadline") {
		t.Errorf("Error should mention cancellation or deadline, got: %q", err.Error())
	}
}

// TestStoreChangeEvent_ContextRespected tests single event respects context
func TestStoreChangeEvent_ContextRespected(t *testing.T) {
	tmpDir := t.TempDir()
	storage, err := NewMVCCStorage(tmpDir)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = storage.Close() }()

	// Use valid context - should succeed
	ctx := context.Background()
	validEvent := ChangeEvent{
		ResourceID: "i-valid",
		ChangeType: "created",
		Timestamp:  time.Now(),
	}

	err = storage.StoreChangeEvent(ctx, validEvent)
	if err != nil {
		t.Fatalf("Expected success with valid context, got error: %v", err)
	}

	// Verify stored
	events, err := storage.QueryChangesSince(ctx, time.Now().Add(-1*time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 1 {
		t.Errorf("Expected 1 event stored, got %d", len(events))
	}
}
