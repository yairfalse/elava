package storage

import (
	"context"
	"encoding/json"
	"testing"
	"time"
)

type testChangeEvent struct {
	Revision   int64
	Timestamp  time.Time
	ResourceID string
	Type       string
}

type testDriftEvent struct {
	ResourceID string
	Field      string
	Severity   string
}

type testWastePattern struct {
	Type        string
	ResourceIDs []string
	Confidence  float64
}

func TestMVCCStorage_StoreAndQueryChangeEvents(t *testing.T) {
	tmpDir := t.TempDir()
	storage, err := NewMVCCStorage(tmpDir)
	if err != nil {
		t.Fatalf("Failed to create storage: %v", err)
	}
	defer func() { _ = storage.Close() }()

	event := testChangeEvent{
		Revision:   1,
		Timestamp:  time.Now(),
		ResourceID: "i-123",
		Type:       "created",
	}

	ctx := context.Background()
	if err := storage.StoreChangeEvent(ctx, event); err != nil {
		t.Fatalf("StoreChangeEvent failed: %v", err)
	}

	events, err := storage.QueryChangesSince(ctx, time.Now().Add(-1*time.Hour))
	if err != nil {
		t.Fatalf("QueryChangesSince failed: %v", err)
	}

	if len(events) != 1 {
		t.Errorf("Expected 1 event, got %d", len(events))
	}

	var retrieved testChangeEvent
	if err := json.Unmarshal(events[0], &retrieved); err != nil {
		t.Fatalf("Failed to unmarshal: %v", err)
	}

	if retrieved.ResourceID != "i-123" {
		t.Errorf("ResourceID = %s, want i-123", retrieved.ResourceID)
	}
}

func TestMVCCStorage_StoreAndQueryDriftEvents(t *testing.T) {
	tmpDir := t.TempDir()
	storage, err := NewMVCCStorage(tmpDir)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = storage.Close() }()

	event := testDriftEvent{
		ResourceID: "i-456",
		Field:      "instance_type",
		Severity:   "medium",
	}

	ctx := context.Background()
	if err := storage.StoreDriftEvent(ctx, event); err != nil {
		t.Fatalf("StoreDriftEvent failed: %v", err)
	}

	events, err := storage.QueryDriftEvents(ctx, time.Now().Add(-1*time.Hour))
	if err != nil {
		t.Fatalf("QueryDriftEvents failed: %v", err)
	}

	if len(events) != 1 {
		t.Errorf("Expected 1 event, got %d", len(events))
	}

	var retrieved testDriftEvent
	if err := json.Unmarshal(events[0], &retrieved); err != nil {
		t.Fatalf("Failed to unmarshal: %v", err)
	}

	if retrieved.Field != "instance_type" {
		t.Errorf("Field = %s, want instance_type", retrieved.Field)
	}
}

func TestMVCCStorage_StoreAndQueryWastePatterns(t *testing.T) {
	tmpDir := t.TempDir()
	storage, err := NewMVCCStorage(tmpDir)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = storage.Close() }()

	pattern := testWastePattern{
		Type:        "orphaned",
		ResourceIDs: []string{"i-789", "i-abc"},
		Confidence:  0.95,
	}

	ctx := context.Background()
	if err := storage.StoreWastePattern(ctx, pattern); err != nil {
		t.Fatalf("StoreWastePattern failed: %v", err)
	}

	patterns, err := storage.QueryWastePatterns(ctx, time.Now().Add(-1*time.Hour))
	if err != nil {
		t.Fatalf("QueryWastePatterns failed: %v", err)
	}

	if len(patterns) != 1 {
		t.Errorf("Expected 1 pattern, got %d", len(patterns))
	}

	var retrieved testWastePattern
	if err := json.Unmarshal(patterns[0], &retrieved); err != nil {
		t.Fatalf("Failed to unmarshal: %v", err)
	}

	if len(retrieved.ResourceIDs) != 2 {
		t.Errorf("Expected 2 resource IDs, got %d", len(retrieved.ResourceIDs))
	}
}
