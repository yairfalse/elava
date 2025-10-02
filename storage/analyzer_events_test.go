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

func TestMVCCStorage_StoreChangeEventBatch(t *testing.T) {
	tmpDir := t.TempDir()
	storage, err := NewMVCCStorage(tmpDir)
	if err != nil {
		t.Fatalf("Failed to create storage: %v", err)
	}
	defer func() { _ = storage.Close() }()

	events := []ChangeEvent{
		{ResourceID: "i-001", ChangeType: "created", Timestamp: time.Now(), Revision: 1},
		{ResourceID: "i-002", ChangeType: "modified", Timestamp: time.Now(), Revision: 2},
		{ResourceID: "i-003", ChangeType: "disappeared", Timestamp: time.Now(), Revision: 3},
	}

	ctx := context.Background()
	if err := storage.StoreChangeEventBatch(ctx, events); err != nil {
		t.Fatalf("StoreChangeEventBatch failed: %v", err)
	}

	retrieved, err := storage.QueryChangesSince(ctx, time.Now().Add(-1*time.Hour))
	if err != nil {
		t.Fatalf("QueryChangesSince failed: %v", err)
	}

	if len(retrieved) != 3 {
		t.Errorf("Expected 3 events, got %d", len(retrieved))
	}

	for i, event := range retrieved {
		if event.ResourceID != events[i].ResourceID {
			t.Errorf("Event %d: ResourceID = %s, want %s", i, event.ResourceID, events[i].ResourceID)
		}
		if event.ChangeType != events[i].ChangeType {
			t.Errorf("Event %d: ChangeType = %s, want %s", i, event.ChangeType, events[i].ChangeType)
		}
	}
}

func TestMVCCStorage_StoreDriftEventBatch(t *testing.T) {
	tmpDir := t.TempDir()
	storage, err := NewMVCCStorage(tmpDir)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = storage.Close() }()

	events := []DriftEvent{
		{ResourceID: "i-001", DriftType: "tag_drift", Field: "env", Expected: "prod", Actual: "dev", Severity: "high", Timestamp: time.Now()},
		{ResourceID: "i-002", DriftType: "config_drift", Field: "type", Expected: "t2.micro", Actual: "t2.small", Severity: "medium", Timestamp: time.Now()},
	}

	ctx := context.Background()
	if err := storage.StoreDriftEventBatch(ctx, events); err != nil {
		t.Fatalf("StoreDriftEventBatch failed: %v", err)
	}

	retrieved, err := storage.QueryDriftEvents(ctx, time.Now().Add(-1*time.Hour))
	if err != nil {
		t.Fatalf("QueryDriftEvents failed: %v", err)
	}

	if len(retrieved) != 2 {
		t.Errorf("Expected 2 events, got %d", len(retrieved))
	}

	for i, event := range retrieved {
		if event.ResourceID != events[i].ResourceID {
			t.Errorf("Event %d: ResourceID = %s, want %s", i, event.ResourceID, events[i].ResourceID)
		}
		if event.Severity != events[i].Severity {
			t.Errorf("Event %d: Severity = %s, want %s", i, event.Severity, events[i].Severity)
		}
	}
}

func TestMVCCStorage_StoreWastePatternBatch(t *testing.T) {
	tmpDir := t.TempDir()
	storage, err := NewMVCCStorage(tmpDir)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = storage.Close() }()

	patterns := []WastePattern{
		{PatternType: "idle", ResourceIDs: []string{"i-001"}, Confidence: 0.9, Reason: "Low CPU usage", Timestamp: time.Now()},
		{PatternType: "orphaned", ResourceIDs: []string{"i-002", "i-003"}, Confidence: 0.95, Reason: "No owner tag", Timestamp: time.Now()},
	}

	ctx := context.Background()
	if err := storage.StoreWastePatternBatch(ctx, patterns); err != nil {
		t.Fatalf("StoreWastePatternBatch failed: %v", err)
	}

	retrieved, err := storage.QueryWastePatterns(ctx, time.Now().Add(-1*time.Hour))
	if err != nil {
		t.Fatalf("QueryWastePatterns failed: %v", err)
	}

	if len(retrieved) != 2 {
		t.Errorf("Expected 2 patterns, got %d", len(retrieved))
	}

	for i, pattern := range retrieved {
		if pattern.PatternType != patterns[i].PatternType {
			t.Errorf("Pattern %d: PatternType = %s, want %s", i, pattern.PatternType, patterns[i].PatternType)
		}
		if pattern.Confidence != patterns[i].Confidence {
			t.Errorf("Pattern %d: Confidence = %f, want %f", i, pattern.Confidence, patterns[i].Confidence)
		}
	}
}

func TestMVCCStorage_BatchRevisionNumbering(t *testing.T) {
	tmpDir := t.TempDir()
	storage, err := NewMVCCStorage(tmpDir)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = storage.Close() }()

	initialRev := storage.CurrentRevision()

	events := []ChangeEvent{
		{ResourceID: "i-001", ChangeType: "created", Timestamp: time.Now()},
		{ResourceID: "i-002", ChangeType: "created", Timestamp: time.Now()},
		{ResourceID: "i-003", ChangeType: "created", Timestamp: time.Now()},
	}

	ctx := context.Background()
	if err := storage.StoreChangeEventBatch(ctx, events); err != nil {
		t.Fatalf("StoreChangeEventBatch failed: %v", err)
	}

	finalRev := storage.CurrentRevision()
	expectedRev := initialRev + int64(len(events))

	if finalRev != expectedRev {
		t.Errorf("Revision = %d, want %d (increment of %d)", finalRev, expectedRev, len(events))
	}
}
