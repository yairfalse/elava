package wal

import (
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// Cleanup removes WAL files older than retention period
func Cleanup(dir string, config Config) error {
	files := listOldWALFiles(dir, config)
	return removeFiles(files)
}

// listOldWALFiles finds WAL files older than retention period
func listOldWALFiles(dir string, config Config) []string {
	cutoff := calculateCutoffTime(config.RetentionDays)
	allFiles := findAllWALFiles(dir, config.FilePrefix)
	return filterOldFiles(allFiles, cutoff)
}

// calculateCutoffTime returns the time before which files should be removed
func calculateCutoffTime(retentionDays int) time.Time {
	return time.Now().AddDate(0, 0, -retentionDays)
}

// findAllWALFiles returns all WAL files in directory
func findAllWALFiles(dir, prefix string) []string {
	pattern := filepath.Join(dir, prefix+"-*.wal")
	files, err := filepath.Glob(pattern)
	if err != nil {
		return nil
	}
	return files
}

// filterOldFiles returns only files older than cutoff time
func filterOldFiles(files []string, cutoff time.Time) []string {
	var oldFiles []string
	for _, file := range files {
		if isOlderThan(file, cutoff) {
			oldFiles = append(oldFiles, file)
		}
	}
	return oldFiles
}

// isOlderThan checks if file modification time is before cutoff
func isOlderThan(path string, cutoff time.Time) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	return info.ModTime().Before(cutoff)
}

// removeFiles deletes all files in the list
func removeFiles(files []string) error {
	for _, file := range files {
		if err := removeFile(file); err != nil {
			return err
		}
	}
	return nil
}

// removeFile deletes a single file
func removeFile(path string) error {
	if err := os.Remove(path); err != nil {
		return fmt.Errorf("failed to remove %s: %w", path, err)
	}
	return nil
}

// CleanupStats tracks cleanup operation results
type CleanupStats struct {
	FilesRemoved  int
	BytesFreed    int64
	OldestRemoved time.Time
	NewestRemoved time.Time
}

// CleanupWithStats removes old files and returns statistics
func CleanupWithStats(dir string, config Config) (CleanupStats, error) {
	stats := CleanupStats{}
	files := listOldWALFiles(dir, config)

	if len(files) == 0 {
		return stats, nil
	}

	stats.FilesRemoved = len(files)
	stats.BytesFreed = calculateTotalSize(files)
	stats.OldestRemoved, stats.NewestRemoved = findTimeRange(files)

	err := removeFiles(files)
	return stats, err
}

// calculateTotalSize sums file sizes
func calculateTotalSize(files []string) int64 {
	var total int64
	for _, file := range files {
		info, err := os.Stat(file)
		if err == nil {
			total += info.Size()
		}
	}
	return total
}

// findTimeRange returns oldest and newest file modification times
func findTimeRange(files []string) (oldest, newest time.Time) {
	if len(files) == 0 {
		return time.Time{}, time.Time{}
	}

	for i, file := range files {
		info, err := os.Stat(file)
		if err != nil {
			continue
		}

		modTime := info.ModTime()
		if i == 0 {
			oldest = modTime
			newest = modTime
			continue
		}

		if modTime.Before(oldest) {
			oldest = modTime
		}
		if modTime.After(newest) {
			newest = modTime
		}
	}

	return oldest, newest
}
