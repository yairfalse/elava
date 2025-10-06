package wal

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestCleanup_NoFiles(t *testing.T) {
	dir := t.TempDir()
	config := DefaultConfig()

	err := Cleanup(dir, config)
	if err != nil {
		t.Errorf("Cleanup failed on empty directory: %v", err)
	}
}

func TestCleanup_AllFilesNew(t *testing.T) {
	dir := t.TempDir()

	// Create fresh WAL files
	w, _ := Open(dir)
	_ = w.Append(EntryObserved, "r1", nil)
	_ = w.Close()

	config := DefaultConfig()
	config.RetentionDays = 30

	err := Cleanup(dir, config)
	if err != nil {
		t.Errorf("Cleanup failed: %v", err)
	}

	// Files should still exist
	files, _ := filepath.Glob(filepath.Join(dir, "elava-*.wal"))
	if len(files) != 1 {
		t.Errorf("Expected 1 file to remain, got %d", len(files))
	}
}

func TestCleanup_OldFilesRemoved(t *testing.T) {
	dir := t.TempDir()

	// Create a WAL file
	testFile := filepath.Join(dir, "elava-20200101-120000.wal")
	f, _ := os.Create(testFile)
	_ = f.Close()

	// Set modification time to 60 days ago
	oldTime := time.Now().AddDate(0, 0, -60)
	_ = os.Chtimes(testFile, oldTime, oldTime)

	config := DefaultConfig()
	config.RetentionDays = 30

	err := Cleanup(dir, config)
	if err != nil {
		t.Errorf("Cleanup failed: %v", err)
	}

	// Old file should be removed
	files, _ := filepath.Glob(filepath.Join(dir, "elava-*.wal"))
	if len(files) != 0 {
		t.Errorf("Expected 0 files after cleanup, got %d", len(files))
	}
}

func TestCleanup_MixedAges(t *testing.T) {
	dir := t.TempDir()

	// Create old file (60 days)
	oldFile := filepath.Join(dir, "elava-20200101-120000.wal")
	f1, _ := os.Create(oldFile)
	_ = f1.Close()
	oldTime := time.Now().AddDate(0, 0, -60)
	_ = os.Chtimes(oldFile, oldTime, oldTime)

	// Create recent file (10 days)
	recentFile := filepath.Join(dir, "elava-20240101-120000.wal")
	f2, _ := os.Create(recentFile)
	_ = f2.Close()
	recentTime := time.Now().AddDate(0, 0, -10)
	_ = os.Chtimes(recentFile, recentTime, recentTime)

	config := DefaultConfig()
	config.RetentionDays = 30

	err := Cleanup(dir, config)
	if err != nil {
		t.Errorf("Cleanup failed: %v", err)
	}

	// Only recent file should remain
	files, _ := filepath.Glob(filepath.Join(dir, "elava-*.wal"))
	if len(files) != 1 {
		t.Errorf("Expected 1 file to remain, got %d", len(files))
	}

	// Verify it's the recent file
	if _, err := os.Stat(recentFile); os.IsNotExist(err) {
		t.Error("Recent file was incorrectly removed")
	}
	if _, err := os.Stat(oldFile); !os.IsNotExist(err) {
		t.Error("Old file was not removed")
	}
}

func TestCleanupWithStats_EmptyDirectory(t *testing.T) {
	dir := t.TempDir()
	config := DefaultConfig()

	stats, err := CleanupWithStats(dir, config)
	if err != nil {
		t.Errorf("CleanupWithStats failed: %v", err)
	}

	if stats.FilesRemoved != 0 {
		t.Errorf("Expected 0 files removed, got %d", stats.FilesRemoved)
	}
	if stats.BytesFreed != 0 {
		t.Errorf("Expected 0 bytes freed, got %d", stats.BytesFreed)
	}
}

func TestCleanupWithStats_ReportsCorrectly(t *testing.T) {
	dir := t.TempDir()

	// Create old files
	for i := 0; i < 3; i++ {
		filename := filepath.Join(dir, "elava-2020010"+string(rune('1'+i))+"-120000.wal")
		_ = os.WriteFile(filename, []byte("test data"), 0600)
		oldTime := time.Now().AddDate(0, 0, -60)
		_ = os.Chtimes(filename, oldTime, oldTime)
	}

	config := DefaultConfig()
	config.RetentionDays = 30

	stats, err := CleanupWithStats(dir, config)
	if err != nil {
		t.Errorf("CleanupWithStats failed: %v", err)
	}

	if stats.FilesRemoved != 3 {
		t.Errorf("Expected 3 files removed, got %d", stats.FilesRemoved)
	}
	if stats.BytesFreed == 0 {
		t.Error("Expected bytes freed > 0")
	}
	if stats.OldestRemoved.IsZero() {
		t.Error("Expected oldest removed time to be set")
	}
}

func TestCalculateCutoffTime(t *testing.T) {
	now := time.Now()
	cutoff := calculateCutoffTime(30)

	// Should be approximately 30 days ago
	diff := now.Sub(cutoff)
	expectedDiff := 30 * 24 * time.Hour

	// Allow 1 minute tolerance
	if diff < expectedDiff-time.Minute || diff > expectedDiff+time.Minute {
		t.Errorf("Cutoff time incorrect: got %v, expected ~%v", diff, expectedDiff)
	}
}

func TestIsOlderThan(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "test.wal")

	_ = os.WriteFile(file, []byte("test"), 0600)

	// Set time to 10 days ago
	oldTime := time.Now().AddDate(0, 0, -10)
	_ = os.Chtimes(file, oldTime, oldTime)

	// File should be older than 5 days ago
	fiveDaysAgo := time.Now().AddDate(0, 0, -5)
	if !isOlderThan(file, fiveDaysAgo) {
		t.Error("File should be older than 5 days ago")
	}

	// File should not be older than 20 days ago
	twentyDaysAgo := time.Now().AddDate(0, 0, -20)
	if isOlderThan(file, twentyDaysAgo) {
		t.Error("File should not be older than 20 days ago")
	}
}

func TestCleanup_ZeroRetention(t *testing.T) {
	dir := t.TempDir()

	// Create a file
	w, _ := Open(dir)
	_ = w.Append(EntryObserved, "r1", nil)
	_ = w.Close()

	config := DefaultConfig()
	config.RetentionDays = 0 // Remove all files

	err := Cleanup(dir, config)
	if err != nil {
		t.Errorf("Cleanup failed: %v", err)
	}

	// All files should be removed
	files, _ := filepath.Glob(filepath.Join(dir, "elava-*.wal"))
	if len(files) != 0 {
		t.Errorf("Expected 0 files with zero retention, got %d", len(files))
	}
}
