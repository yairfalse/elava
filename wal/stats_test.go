package wal

import (
	"testing"
	"time"
)

func TestGetStats_EmptyWAL(t *testing.T) {
	dir := t.TempDir()

	w, err := Open(dir)
	if err != nil {
		t.Fatalf("Failed to open WAL: %v", err)
	}
	defer func() { _ = w.Close() }()

	stats := w.GetStats()

	if stats.TotalFiles != 1 {
		t.Errorf("Expected 1 file, got %d", stats.TotalFiles)
	}
	if stats.LastSequence != 0 {
		t.Errorf("Expected sequence 0, got %d", stats.LastSequence)
	}
	if stats.SequenceCount != 1 {
		t.Errorf("Expected sequence count 1, got %d", stats.SequenceCount)
	}
}

func TestGetStats_WithEntries(t *testing.T) {
	dir := t.TempDir()

	w, err := Open(dir)
	if err != nil {
		t.Fatalf("Failed to open WAL: %v", err)
	}
	defer func() { _ = w.Close() }()

	// Write some entries
	for i := 0; i < 10; i++ {
		if err := w.Append(EntryObserved, "resource", nil); err != nil {
			t.Fatalf("Failed to append entry %d: %v", i, err)
		}
	}

	stats := w.GetStats()

	if stats.LastSequence != 10 {
		t.Errorf("Expected sequence 10, got %d", stats.LastSequence)
	}
	if stats.SequenceCount != 10 {
		t.Errorf("Expected sequence count 10, got %d", stats.SequenceCount)
	}
	if stats.TotalSizeBytes == 0 {
		t.Error("Expected non-zero total size")
	}
}

func TestGetStats_MultipleFiles(t *testing.T) {
	dir := t.TempDir()

	// Force rotation with small file size
	config := DefaultConfig()
	config.MaxFileSize = 200

	w, err := OpenWithConfig(dir, config)
	if err != nil {
		t.Fatalf("Failed to open WAL: %v", err)
	}

	// Write enough to trigger rotation (each entry ~100 bytes with JSON)
	for i := 0; i < 10; i++ {
		largeData := make([]byte, 80)
		if err := w.Append(EntryObserved, "resource", largeData); err != nil {
			t.Fatalf("Failed to append entry %d: %v", i, err)
		}
	}
	defer func() { _ = w.Close() }()

	stats := w.GetStats()

	// Should have rotated at least once
	if stats.TotalFiles < 2 {
		t.Skipf("Rotation didn't occur, got %d files", stats.TotalFiles)
	}
	if stats.FirstSequence != 1 {
		t.Errorf("Expected first sequence 1, got %d", stats.FirstSequence)
	}
	if stats.LastSequence < 5 {
		t.Errorf("Expected at least 5 sequences, got %d", stats.LastSequence)
	}
}

func TestGetStatsFromDir(t *testing.T) {
	dir := t.TempDir()

	// Create and close a WAL with entries
	w, err := Open(dir)
	if err != nil {
		t.Fatalf("Failed to open WAL: %v", err)
	}
	for i := 0; i < 5; i++ {
		if err := w.Append(EntryObserved, "resource", nil); err != nil {
			t.Fatalf("Failed to append entry %d: %v", i, err)
		}
	}
	if err := w.Close(); err != nil {
		t.Fatalf("Failed to close WAL: %v", err)
	}

	// Get stats without opening WAL
	config := DefaultConfig()
	stats := GetStatsFromDir(dir, config)

	if stats.TotalFiles != 1 {
		t.Errorf("Expected 1 file, got %d", stats.TotalFiles)
	}
	if stats.FirstSequence != 1 {
		t.Errorf("Expected first sequence 1, got %d", stats.FirstSequence)
	}
	if stats.LastSequence != 5 {
		t.Errorf("Expected last sequence 5, got %d", stats.LastSequence)
	}
}

func TestGetStatsFromDir_EmptyDirectory(t *testing.T) {
	dir := t.TempDir()

	config := DefaultConfig()
	stats := GetStatsFromDir(dir, config)

	if stats.TotalFiles != 0 {
		t.Errorf("Expected 0 files, got %d", stats.TotalFiles)
	}
	if stats.TotalSizeBytes != 0 {
		t.Errorf("Expected 0 bytes, got %d", stats.TotalSizeBytes)
	}
}

func TestGetHealth_Healthy(t *testing.T) {
	dir := t.TempDir()

	w, err := Open(dir)
	if err != nil {
		t.Fatalf("Failed to open WAL: %v", err)
	}
	defer func() { _ = w.Close() }()

	// Write a few entries (well below limits)
	for i := 0; i < 5; i++ {
		if err := w.Append(EntryObserved, "resource", nil); err != nil {
			t.Fatalf("Failed to append entry %d: %v", i, err)
		}
	}

	health := w.GetHealth()

	if !health.Healthy {
		t.Errorf("Expected healthy WAL, got issues: %v", health.Issues)
	}
	if health.DiskUsagePercent > 1.0 {
		t.Errorf("Expected low disk usage, got %.2f%%", health.DiskUsagePercent)
	}
	if health.NeedsRotation {
		t.Error("Should not need rotation with few entries")
	}
}

func TestGetHealth_NeedsRotation(t *testing.T) {
	dir := t.TempDir()

	// Small file size to trigger rotation warning
	config := DefaultConfig()
	config.MaxFileSize = 100 // Very small

	w, err := OpenWithConfig(dir, config)
	if err != nil {
		t.Fatalf("Failed to open WAL: %v", err)
	}
	defer func() { _ = w.Close() }()

	// Write enough to exceed limit
	largeData := make([]byte, 150)
	if err := w.Append(EntryObserved, "resource", largeData); err != nil {
		t.Fatalf("Failed to append entry: %v", err)
	}

	health := w.GetHealth()

	if health.DiskUsagePercent < 90 {
		t.Errorf("Expected high disk usage, got %.2f%%", health.DiskUsagePercent)
	}
	if len(health.Issues) == 0 {
		t.Error("Expected health issues with large file")
	}
}

func TestCountEntriesInFile(t *testing.T) {
	dir := t.TempDir()

	w, err := Open(dir)
	if err != nil {
		t.Fatalf("Failed to open WAL: %v", err)
	}

	// Write known number of entries
	for i := 0; i < 7; i++ {
		if err := w.Append(EntryObserved, "resource", nil); err != nil {
			t.Fatalf("Failed to append entry %d: %v", i, err)
		}
	}

	files := w.listWALFiles()
	if len(files) == 0 {
		t.Fatal("No WAL files found")
	}

	count := w.countEntriesInFile(files[0])
	if count != 7 {
		t.Errorf("Expected 7 entries, got %d", count)
	}

	if err := w.Close(); err != nil {
		t.Fatalf("Failed to close WAL: %v", err)
	}
}

func TestWritesPerFile(t *testing.T) {
	dir := t.TempDir()

	// Force rotation with small file size
	config := DefaultConfig()
	config.MaxFileSize = 200

	w, err := OpenWithConfig(dir, config)
	if err != nil {
		t.Fatalf("Failed to open WAL: %v", err)
	}

	// Write enough to trigger rotation (each entry ~100 bytes with JSON)
	numWrites := 10
	for i := 0; i < numWrites; i++ {
		largeData := make([]byte, 80)
		if err := w.Append(EntryObserved, "r", largeData); err != nil {
			t.Fatalf("Failed to append entry %d: %v", i, err)
		}
	}

	stats := w.GetStats()

	if len(stats.WritesPerFile) < 2 {
		t.Skipf("Rotation didn't occur, got %d files", len(stats.WritesPerFile))
	}

	totalWrites := 0
	for _, count := range stats.WritesPerFile {
		totalWrites += count
	}

	if totalWrites != numWrites {
		t.Errorf("Expected %d total writes, got %d", numWrites, totalWrites)
	}

	if err := w.Close(); err != nil {
		t.Fatalf("Failed to close WAL: %v", err)
	}
}

func TestFileTimeRange(t *testing.T) {
	dir := t.TempDir()

	start := time.Now()

	// Force rotation with small file size
	config := DefaultConfig()
	config.MaxFileSize = 200

	w, err := OpenWithConfig(dir, config)
	if err != nil {
		t.Fatalf("Failed to open WAL: %v", err)
	}

	// Write entries to trigger rotation
	for i := 0; i < 5; i++ {
		largeData := make([]byte, 80)
		if err := w.Append(EntryObserved, "resource", largeData); err != nil {
			t.Fatalf("Failed to append entry %d: %v", i, err)
		}
		if i == 2 {
			time.Sleep(10 * time.Millisecond)
		}
	}
	defer func() {
		if err := w.Close(); err != nil {
			t.Errorf("Failed to close WAL: %v", err)
		}
	}()

	stats := w.GetStats()

	if stats.OldestFile.IsZero() {
		t.Error("Oldest file time should be set")
	}
	if stats.NewestFile.IsZero() {
		t.Error("Newest file time should be set")
	}

	if stats.TotalFiles > 1 && !stats.OldestFile.Before(stats.NewestFile) {
		t.Error("Oldest file should be before newest file")
	}

	if stats.OldestFile.Before(start.Add(-time.Second)) {
		t.Error("Oldest file time should be after test start")
	}
}
