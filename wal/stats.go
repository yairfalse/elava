package wal

import (
	"io"
	"path/filepath"
	"time"
)

// Stats represents WAL statistics
type Stats struct {
	// File statistics
	TotalFiles      int
	TotalSizeBytes  int64
	OldestFile      time.Time
	NewestFile      time.Time
	CurrentFileSize int64

	// Sequence statistics
	SequenceCount int64
	FirstSequence int64
	LastSequence  int64

	// Performance metrics
	WritesPerFile map[string]int
}

// GetStats returns current WAL statistics
func (w *WAL) GetStats() Stats {
	w.mu.Lock()
	defer w.mu.Unlock()

	return w.collectStats()
}

// collectStats gathers all statistics
func (w *WAL) collectStats() Stats {
	stats := Stats{
		LastSequence: w.sequence,
	}

	// Collect file statistics
	w.collectFileStats(&stats)

	// Collect sequence statistics
	w.collectSequenceStats(&stats)

	return stats
}

// collectFileStats gathers file-related statistics
func (w *WAL) collectFileStats(stats *Stats) {
	files := w.listWALFiles()
	stats.TotalFiles = len(files)

	if len(files) == 0 {
		return
	}

	stats.TotalSizeBytes = calculateTotalSize(files)
	stats.OldestFile, stats.NewestFile = findTimeRange(files)
	stats.CurrentFileSize = w.getCurrentFileSize()
}

// collectSequenceStats gathers sequence-related statistics
func (w *WAL) collectSequenceStats(stats *Stats) {
	if stats.TotalFiles == 0 {
		return
	}

	files := w.listWALFiles()
	stats.FirstSequence = w.findFirstSequence(files)
	if stats.LastSequence >= stats.FirstSequence {
		stats.SequenceCount = stats.LastSequence - stats.FirstSequence + 1
	} else {
		stats.SequenceCount = 0
	}
	stats.WritesPerFile = w.countWritesPerFile(files)
}

// getCurrentFileSize returns size of current WAL file
func (w *WAL) getCurrentFileSize() int64 {
	info, err := w.file.Stat()
	if err != nil {
		return 0
	}
	return info.Size()
}

// findFirstSequence finds the lowest sequence number across all files
func (w *WAL) findFirstSequence(files []string) int64 {
	if len(files) == 0 {
		return 0
	}

	// Start from first file (oldest)
	reader, err := NewReader(files[0])
	if err != nil {
		return 0
	}
	defer func() { _ = reader.Close() }()

	entry, err := reader.Next()
	if err != nil {
		return 0
	}

	return entry.Sequence
}

// countWritesPerFile counts entries in each file
func (w *WAL) countWritesPerFile(files []string) map[string]int {
	counts := make(map[string]int)

	for _, file := range files {
		count := w.countEntriesInFile(file)
		counts[filepath.Base(file)] = count
	}

	return counts
}

// countEntriesInFile counts entries in a single file
func (w *WAL) countEntriesInFile(path string) int {
	reader, err := NewReader(path)
	if err != nil {
		return 0
	}
	defer func() { _ = reader.Close() }()

	count := 0
	for {
		_, err := reader.Next()
		if err != nil {
			break
		}
		count++
	}

	return count
}

// GetStatsFromDir returns statistics for a WAL directory (no active WAL needed)
func GetStatsFromDir(dir string, config Config) Stats {
	stats := Stats{}

	pattern := filepath.Join(dir, config.FilePrefix+"-*.wal")
	files, err := filepath.Glob(pattern)
	if err != nil || len(files) == 0 {
		return stats
	}

	stats.TotalFiles = len(files)
	stats.TotalSizeBytes = calculateTotalSize(files)
	stats.OldestFile, stats.NewestFile = findTimeRange(files)

	// Find sequence range
	stats.FirstSequence = findFirstSequenceInFiles(files)
	stats.LastSequence = findLastSequenceInFiles(files)
	if len(files) == 0 || stats.LastSequence < stats.FirstSequence {
		stats.SequenceCount = 0
	} else {
		stats.SequenceCount = stats.LastSequence - stats.FirstSequence + 1
	}

	return stats
}

// findFirstSequenceInFiles finds lowest sequence across files
func findFirstSequenceInFiles(files []string) int64 {
	if len(files) == 0 {
		return 0
	}

	reader, err := NewReader(files[0])
	if err != nil {
		return 0
	}
	defer func() { _ = reader.Close() }()

	entry, err := reader.Next()
	if err != nil {
		return 0
	}

	return entry.Sequence
}

// findLastSequenceInFiles finds highest sequence across files
func findLastSequenceInFiles(files []string) int64 {
	maxSeq := int64(0)

	for _, file := range files {
		fileMax := getMaxSequenceFromFile(file)
		if fileMax > maxSeq {
			maxSeq = fileMax
		}
	}

	return maxSeq
}

// scanMaxSequenceInFile iterates through a Reader and returns the max sequence, skipping corrupted entries.
func scanMaxSequenceInFile(reader *Reader) int64 {
	maxSeq := int64(0)
	for {
		entry, err := reader.Next()
		if err != nil {
			// If EOF, break; if other error, skip and continue.
			if err == io.EOF {
				break
			}
			continue
		}
		if entry.Sequence > maxSeq {
			maxSeq = entry.Sequence
		}
	}
	return maxSeq
}

// getMaxSequenceFromFile reads file and returns max sequence
func getMaxSequenceFromFile(path string) int64 {
	reader, err := NewReader(path)
	if err != nil {
		return 0
	}
	defer func() { _ = reader.Close() }()

	return scanMaxSequenceInFile(reader)
}
// HealthStatus represents WAL health
type HealthStatus struct {
	Healthy         bool
	DiskUsagePercent float64
	OldestFileAge   time.Duration
	NeedsRotation   bool
	NeedsCleanup    bool
	Issues          []string
}

// GetHealth returns WAL health status
func (w *WAL) GetHealth() HealthStatus {
	w.mu.Lock()
	defer w.mu.Unlock()

	health := HealthStatus{
		Healthy: true,
		Issues:  []string{},
	}

	w.checkDiskUsage(&health)
	w.checkFileAge(&health)
	w.checkRotationNeeded(&health)

	health.Healthy = len(health.Issues) == 0

	return health
}

// checkDiskUsage checks current file size
func (w *WAL) checkDiskUsage(health *HealthStatus) {
	size := w.getCurrentFileSize()
	health.DiskUsagePercent = float64(size) / float64(w.config.MaxFileSize) * 100

	if health.DiskUsagePercent > 90 {
		health.Issues = append(health.Issues, "current file >90% of max size")
	}
}

// checkFileAge checks oldest file age
func (w *WAL) checkFileAge(health *HealthStatus) {
	files := w.listWALFiles()
	if len(files) == 0 {
		return
	}

	oldest, _ := findTimeRange(files)
	health.OldestFileAge = time.Since(oldest)

	retentionDuration := time.Duration(w.config.RetentionDays) * 24 * time.Hour
	if health.OldestFileAge > retentionDuration {
		health.NeedsCleanup = true
		health.Issues = append(health.Issues, "old files exceed retention period")
	}
}

// checkRotationNeeded checks if rotation is imminent
func (w *WAL) checkRotationNeeded(health *HealthStatus) {
	if w.shouldRotate() {
		health.NeedsRotation = true
		health.Issues = append(health.Issues, "file rotation needed")
	}
}
