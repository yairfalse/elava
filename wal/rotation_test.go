package wal

import (
	"path/filepath"
	"testing"
)

func TestFileRotation_SizeLimit(t *testing.T) {
	t.Skip("Rotation tested in SequenceContinuity test")
}

func TestFileRotation_SequenceContinuity(t *testing.T) {
	dir := t.TempDir()

	// Create WAL with small file size for quick rotation
	config := DefaultConfig()
	config.MaxFileSize = 500 // Very small to force rotation

	w, err := OpenWithConfig(dir, config)
	if err != nil {
		t.Fatalf("Failed to open WAL: %v", err)
	}
	defer func() { _ = w.Close() }()

	// Write entries that will span multiple files
	for i := 0; i < 20; i++ {
		_ = w.Append(EntryObserved, "resource", "some data")
	}

	// Sequence should be continuous (20)
	if w.sequence != 20 {
		t.Errorf("Expected sequence 20, got %d", w.sequence)
	}

	// Verify all entries are readable across files
	count := 0
	files, _ := filepath.Glob(filepath.Join(dir, "elava-*.wal"))
	for _, file := range files {
		reader, _ := NewReader(file)
		for {
			_, err := reader.Next()
			if err != nil {
				break
			}
			count++
		}
		_ = reader.Close()
	}

	if count != 20 {
		t.Errorf("Expected 20 entries across all files, got %d", count)
	}
}

func TestFileRotation_NoRotationWhenBelowLimit(t *testing.T) {
	dir := t.TempDir()

	// Large file size - should not rotate
	config := DefaultConfig()
	config.MaxFileSize = 100 * 1024 * 1024 // 100MB

	w, err := OpenWithConfig(dir, config)
	if err != nil {
		t.Fatalf("Failed to open WAL: %v", err)
	}
	defer func() { _ = w.Close() }()

	// Write a few small entries
	for i := 0; i < 10; i++ {
		_ = w.Append(EntryObserved, "resource", "data")
	}

	// Should only have 1 file
	files := w.listWALFiles()
	if len(files) != 1 {
		t.Errorf("Expected 1 WAL file (no rotation), got %d", len(files))
	}
}
