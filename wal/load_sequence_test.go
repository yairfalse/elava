package wal

import (
	"testing"
)

func TestLoadSequence_EmptyDirectory(t *testing.T) {
	dir := t.TempDir()

	w, err := Open(dir)
	if err != nil {
		t.Fatalf("Failed to open WAL: %v", err)
	}
	defer func() { _ = w.Close() }()

	if w.sequence != 0 {
		t.Errorf("Empty directory should start at sequence 0, got %d", w.sequence)
	}
}

func TestLoadSequence_ExistingEntries(t *testing.T) {
	dir := t.TempDir()

	// Create first WAL and write some entries
	w1, err := Open(dir)
	if err != nil {
		t.Fatalf("Failed to open WAL: %v", err)
	}

	_ = w1.Append(EntryObserved, "resource-1", nil)
	_ = w1.Append(EntryObserved, "resource-2", nil)
	_ = w1.Append(EntryObserved, "resource-3", nil)

	_ = w1.Close()

	// Open new WAL in same directory - should continue sequence
	w2, err := Open(dir)
	if err != nil {
		t.Fatalf("Failed to open second WAL: %v", err)
	}
	defer func() { _ = w2.Close() }()

	// Sequence should be loaded from existing file (3)
	if w2.sequence != 3 {
		t.Errorf("Expected sequence 3, got %d", w2.sequence)
	}

	// Next append should be sequence 4
	_ = w2.Append(EntryObserved, "resource-4", nil)

	if w2.sequence != 4 {
		t.Errorf("Expected sequence 4 after append, got %d", w2.sequence)
	}
}

func TestLoadSequence_MultipleFiles(t *testing.T) {
	dir := t.TempDir()

	// Create multiple WAL files
	w1, _ := Open(dir)
	_ = w1.Append(EntryObserved, "r1", nil)
	_ = w1.Append(EntryObserved, "r2", nil)
	_ = w1.Close()

	w2, _ := Open(dir)
	_ = w2.Append(EntryObserved, "r3", nil)
	_ = w2.Append(EntryObserved, "r4", nil)
	_ = w2.Append(EntryObserved, "r5", nil)
	_ = w2.Close()

	// New WAL should find max sequence across all files (5)
	w3, err := Open(dir)
	if err != nil {
		t.Fatalf("Failed to open third WAL: %v", err)
	}
	defer func() { _ = w3.Close() }()

	if w3.sequence != 5 {
		t.Errorf("Expected sequence 5, got %d", w3.sequence)
	}
}
