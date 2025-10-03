package storage

import (
	"context"
	"strings"
	"testing"
	"time"
)

// TestStoreChangeEvent_ValidationFailure tests that invalid events are rejected
func TestStoreChangeEvent_ValidationFailure(t *testing.T) {
	tmpDir := t.TempDir()
	storage, err := NewMVCCStorage(tmpDir)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = storage.Close() }()

	ctx := context.Background()

	// Try to store event with empty resource_id
	invalidEvent := ChangeEvent{
		ResourceID: "",
		ChangeType: "created",
		Timestamp:  time.Now(),
	}

	err = storage.StoreChangeEvent(ctx, invalidEvent)
	if err == nil {
		t.Fatal("Expected error for invalid event, got nil")
	}

	if !strings.Contains(err.Error(), "resource_id cannot be empty") {
		t.Errorf("Error message = %q, want to contain 'resource_id cannot be empty'", err.Error())
	}

	// Verify nothing was stored
	events, err := storage.QueryChangesSince(ctx, time.Now().Add(-1*time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 0 {
		t.Errorf("Expected 0 events stored, got %d", len(events))
	}
}

// TestStoreDriftEvent_ValidationFailure tests drift event validation
func TestStoreDriftEvent_ValidationFailure(t *testing.T) {
	tmpDir := t.TempDir()
	storage, err := NewMVCCStorage(tmpDir)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = storage.Close() }()

	ctx := context.Background()

	// Try to store event with empty field
	invalidEvent := DriftEvent{
		ResourceID: "i-123",
		DriftType:  "config_drift",
		Field:      "", // Invalid!
		Severity:   "medium",
		Timestamp:  time.Now(),
	}

	err = storage.StoreDriftEvent(ctx, invalidEvent)
	if err == nil {
		t.Fatal("Expected error for invalid event, got nil")
	}

	if !strings.Contains(err.Error(), "field cannot be empty") {
		t.Errorf("Error message = %q, want to contain 'field cannot be empty'", err.Error())
	}
}

// TestStoreWastePattern_ValidationFailure tests waste pattern validation
func TestStoreWastePattern_ValidationFailure(t *testing.T) {
	tmpDir := t.TempDir()
	storage, err := NewMVCCStorage(tmpDir)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = storage.Close() }()

	ctx := context.Background()

	// Try to store pattern with invalid confidence
	invalidPattern := WastePattern{
		PatternType: "idle",
		ResourceIDs: []string{"i-123"},
		Confidence:  1.5, // Invalid! > 1.0
		Reason:      "test",
		Timestamp:   time.Now(),
	}

	err = storage.StoreWastePattern(ctx, invalidPattern)
	if err == nil {
		t.Fatal("Expected error for invalid pattern, got nil")
	}

	if !strings.Contains(err.Error(), "confidence must be between 0.0 and 1.0") {
		t.Errorf("Error message = %q, want to contain confidence validation error", err.Error())
	}
}

// TestStoreChangeEventBatch_ValidationFailure tests batch validation
func TestStoreChangeEventBatch_ValidationFailure(t *testing.T) {
	tmpDir := t.TempDir()
	storage, err := NewMVCCStorage(tmpDir)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = storage.Close() }()

	ctx := context.Background()

	// Batch with one invalid event (second one)
	events := []ChangeEvent{
		{ResourceID: "i-001", ChangeType: "created", Timestamp: time.Now()},
		{ResourceID: "", ChangeType: "created", Timestamp: time.Now()}, // Invalid!
		{ResourceID: "i-003", ChangeType: "created", Timestamp: time.Now()},
	}

	err = storage.StoreChangeEventBatch(ctx, events)
	if err == nil {
		t.Fatal("Expected error for invalid event in batch, got nil")
	}

	if !strings.Contains(err.Error(), "at index 1") {
		t.Errorf("Error should specify index 1, got: %q", err.Error())
	}

	if !strings.Contains(err.Error(), "resource_id cannot be empty") {
		t.Errorf("Error message = %q, want to contain 'resource_id cannot be empty'", err.Error())
	}

	// Verify NOTHING was stored (atomic failure)
	storedEvents, err := storage.QueryChangesSince(ctx, time.Now().Add(-1*time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	if len(storedEvents) != 0 {
		t.Errorf("Expected 0 events stored (atomic failure), got %d", len(storedEvents))
	}
}

// TestStoreChangeEventBatch_AllValidSuccess tests successful batch storage
func TestStoreChangeEventBatch_AllValidSuccess(t *testing.T) {
	tmpDir := t.TempDir()
	storage, err := NewMVCCStorage(tmpDir)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = storage.Close() }()

	ctx := context.Background()

	// All valid events
	events := []ChangeEvent{
		{ResourceID: "i-001", ChangeType: "created", Timestamp: time.Now()},
		{ResourceID: "i-002", ChangeType: "modified", Timestamp: time.Now()},
		{ResourceID: "i-003", ChangeType: "disappeared", Timestamp: time.Now()},
	}

	err = storage.StoreChangeEventBatch(ctx, events)
	if err != nil {
		t.Fatalf("Expected success for valid events, got error: %v", err)
	}

	// Verify all stored
	storedEvents, err := storage.QueryChangesSince(ctx, time.Now().Add(-1*time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	if len(storedEvents) != 3 {
		t.Errorf("Expected 3 events stored, got %d", len(storedEvents))
	}
}
