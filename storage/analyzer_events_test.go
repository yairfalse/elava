package storage

import (
	"context"
	"testing"
	"time"
)

func TestMVCCStorage_StoreAndQueryChangeEvents(t *testing.T) {
	tmpDir := t.TempDir()
	storage, err := NewMVCCStorage(tmpDir)
	if err != nil {
		t.Fatalf("Failed to create storage: %v", err)
	}
	defer func() { _ = storage.Close() }()

	event := ChangeEvent{
		ResourceID: "i-123",
		ChangeType: "created",
		Timestamp:  time.Now(),
		Revision:   1,
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

	if events[0].ResourceID != "i-123" {
		t.Errorf("ResourceID = %s, want i-123", events[0].ResourceID)
	}

	if events[0].ChangeType != "created" {
		t.Errorf("ChangeType = %s, want created", events[0].ChangeType)
	}
}

func TestMVCCStorage_StoreAndQueryDriftEvents(t *testing.T) {
	tmpDir := t.TempDir()
	storage, err := NewMVCCStorage(tmpDir)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = storage.Close() }()

	event := DriftEvent{
		ResourceID: "i-456",
		DriftType:  "config_drift",
		Timestamp:  time.Now(),
		Field:      "instance_type",
		Expected:   "t2.micro",
		Actual:     "t2.small",
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

	if events[0].Field != "instance_type" {
		t.Errorf("Field = %s, want instance_type", events[0].Field)
	}

	if events[0].Severity != "medium" {
		t.Errorf("Severity = %s, want medium", events[0].Severity)
	}
}

func TestMVCCStorage_StoreAndQueryWastePatterns(t *testing.T) {
	tmpDir := t.TempDir()
	storage, err := NewMVCCStorage(tmpDir)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = storage.Close() }()

	pattern := WastePattern{
		PatternType: "orphaned",
		ResourceIDs: []string{"i-789", "i-abc"},
		Timestamp:   time.Now(),
		Confidence:  0.95,
		Reason:      "No owner tag",
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

	if len(patterns[0].ResourceIDs) != 2 {
		t.Errorf("Expected 2 resource IDs, got %d", len(patterns[0].ResourceIDs))
	}

	if patterns[0].Confidence != 0.95 {
		t.Errorf("Confidence = %f, want 0.95", patterns[0].Confidence)
	}
}
