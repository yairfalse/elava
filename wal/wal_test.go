package wal

import (
	"encoding/json"
	"fmt"
	"io"
	"path/filepath"
	"testing"
	"time"

	"github.com/yairfalse/ovi/types"
)

func TestWAL_AppendAndRead(t *testing.T) {
	dir := t.TempDir()
	
	// Create WAL
	w, err := Open(dir)
	if err != nil {
		t.Fatalf("Failed to open WAL: %v", err)
	}
	
	// Test data
	resource := types.Resource{
		ID:   "i-123456",
		Type: "ec2",
		Tags: types.Tags{
			OviOwner: "team-web",
		},
	}
	
	// Append entries
	if err := w.Append(EntryObserved, resource.ID, resource); err != nil {
		t.Fatalf("Failed to append observed entry: %v", err)
	}
	
	decision := types.Decision{
		Action:     "delete",
		ResourceID: resource.ID,
		Reason:     "not in desired state",
	}
	
	if err := w.Append(EntryDecided, resource.ID, decision); err != nil {
		t.Fatalf("Failed to append decided entry: %v", err)
	}
	
	if err := w.Append(EntryExecuting, resource.ID, decision); err != nil {
		t.Fatalf("Failed to append executing entry: %v", err)
	}
	
	if err := w.Append(EntryExecuted, resource.ID, decision); err != nil {
		t.Fatalf("Failed to append executed entry: %v", err)
	}
	
	// Close WAL
	if err := w.Close(); err != nil {
		t.Fatalf("Failed to close WAL: %v", err)
	}
	
	// Read back entries
	files, _ := filepath.Glob(filepath.Join(dir, "ovi-*.wal"))
	if len(files) == 0 {
		t.Fatal("No WAL files found")
	}
	
	reader, err := NewReader(files[0])
	if err != nil {
		t.Fatalf("Failed to create reader: %v", err)
	}
	defer reader.Close()
	
	// Verify entries
	expectedTypes := []EntryType{
		EntryObserved,
		EntryDecided,
		EntryExecuting,
		EntryExecuted,
	}
	
	for i, expected := range expectedTypes {
		entry, err := reader.Next()
		if err != nil {
			t.Fatalf("Failed to read entry %d: %v", i, err)
		}
		
		if entry.Type != expected {
			t.Errorf("Entry %d: type = %v, want %v", i, entry.Type, expected)
		}
		
		if entry.ResourceID != resource.ID {
			t.Errorf("Entry %d: resource_id = %v, want %v", i, entry.ResourceID, resource.ID)
		}
		
		if entry.Sequence != int64(i+1) {
			t.Errorf("Entry %d: sequence = %v, want %v", i, entry.Sequence, i+1)
		}
	}
	
	// Should be EOF
	_, err = reader.Next()
	if err != io.EOF {
		t.Errorf("Expected EOF, got %v", err)
	}
}

func TestWAL_AppendError(t *testing.T) {
	dir := t.TempDir()
	
	w, err := Open(dir)
	if err != nil {
		t.Fatalf("Failed to open WAL: %v", err)
	}
	defer w.Close()
	
	decision := types.Decision{
		Action:     "delete",
		ResourceID: "i-123456",
		Reason:     "not in desired state",
	}
	
	testErr := fmt.Errorf("permission denied")
	
	if err := w.AppendError(EntryFailed, decision.ResourceID, decision, testErr); err != nil {
		t.Fatalf("Failed to append error entry: %v", err)
	}
	
	w.Close()
	
	// Read back
	files, _ := filepath.Glob(filepath.Join(dir, "ovi-*.wal"))
	reader, _ := NewReader(files[0])
	defer reader.Close()
	
	entry, err := reader.Next()
	if err != nil {
		t.Fatalf("Failed to read entry: %v", err)
	}
	
	if entry.Type != EntryFailed {
		t.Errorf("Entry type = %v, want %v", entry.Type, EntryFailed)
	}
	
	if entry.Error != testErr.Error() {
		t.Errorf("Entry error = %v, want %v", entry.Error, testErr.Error())
	}
}

func TestWAL_Replay(t *testing.T) {
	dir := t.TempDir()
	
	w, err := Open(dir)
	if err != nil {
		t.Fatalf("Failed to open WAL: %v", err)
	}
	
	// Add entries with different timestamps
	
	// Old entry (should be skipped)
	w.Append(EntryObserved, "old-resource", map[string]string{"age": "old"})
	
	// Sleep to ensure time difference
	time.Sleep(10 * time.Millisecond)
	cutoff := time.Now()
	time.Sleep(10 * time.Millisecond)
	
	// New entries (should be replayed)
	w.Append(EntryObserved, "new-resource-1", map[string]string{"age": "new"})
	w.Append(EntryObserved, "new-resource-2", map[string]string{"age": "new"})
	
	w.Close()
	
	// Replay entries after cutoff
	var replayed []string
	err = Replay(dir, cutoff, func(entry *Entry) error {
		replayed = append(replayed, entry.ResourceID)
		return nil
	})
	
	if err != nil {
		t.Fatalf("Replay failed: %v", err)
	}
	
	// Should only have replayed the new entries
	if len(replayed) != 2 {
		t.Errorf("Replayed %d entries, want 2", len(replayed))
	}
	
	expectedIDs := []string{"new-resource-1", "new-resource-2"}
	for i, id := range replayed {
		if id != expectedIDs[i] {
			t.Errorf("Replayed[%d] = %v, want %v", i, id, expectedIDs[i])
		}
	}
}

func TestWAL_SequenceNumbers(t *testing.T) {
	dir := t.TempDir()
	
	w, err := Open(dir)
	if err != nil {
		t.Fatalf("Failed to open WAL: %v", err)
	}
	
	// Append multiple entries
	for i := 0; i < 5; i++ {
		w.Append(EntryObserved, fmt.Sprintf("resource-%d", i), nil)
	}
	
	w.Close()
	
	// Read and verify sequences
	files, _ := filepath.Glob(filepath.Join(dir, "ovi-*.wal"))
	reader, _ := NewReader(files[0])
	defer reader.Close()
	
	for i := 0; i < 5; i++ {
		entry, err := reader.Next()
		if err != nil {
			t.Fatalf("Failed to read entry %d: %v", i, err)
		}
		
		if entry.Sequence != int64(i+1) {
			t.Errorf("Entry %d: sequence = %d, want %d", i, entry.Sequence, i+1)
		}
	}
}

func TestWAL_DataIntegrity(t *testing.T) {
	dir := t.TempDir()
	
	w, err := Open(dir)
	if err != nil {
		t.Fatalf("Failed to open WAL: %v", err)
	}
	
	// Complex data structure
	decision := types.Decision{
		Action:     "delete",
		ResourceID: "i-complex",
		Reason:     "complex reason with special chars: \"quotes\" and \nnewlines",
	}
	
	w.Append(EntryDecided, decision.ResourceID, decision)
	w.Close()
	
	// Read back and verify
	files, _ := filepath.Glob(filepath.Join(dir, "ovi-*.wal"))
	reader, _ := NewReader(files[0])
	defer reader.Close()
	
	entry, _ := reader.Next()
	
	// Unmarshal the data
	var recovered types.Decision
	if err := json.Unmarshal(entry.Data, &recovered); err != nil {
		t.Fatalf("Failed to unmarshal data: %v", err)
	}
	
	if recovered.Action != decision.Action {
		t.Errorf("Action = %v, want %v", recovered.Action, decision.Action)
	}
	
	if recovered.Reason != decision.Reason {
		t.Errorf("Reason = %v, want %v", recovered.Reason, decision.Reason)
	}
}