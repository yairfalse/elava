package wal

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// EntryType defines the type of WAL entry
type EntryType string

const (
	EntryObserved  EntryType = "observed"
	EntryDecided   EntryType = "decided"
	EntryExecuting EntryType = "executing"
	EntryExecuted  EntryType = "executed"
	EntryFailed    EntryType = "failed"
	EntrySkipped   EntryType = "skipped"
)

// Entry represents a single WAL entry
type Entry struct {
	Timestamp  time.Time       `json:"timestamp"`
	Sequence   int64           `json:"sequence"`
	Type       EntryType       `json:"type"`
	ResourceID string          `json:"resource_id,omitempty"`
	Data       json.RawMessage `json:"data"`
	Error      string          `json:"error,omitempty"`
}

// Config holds WAL configuration
type Config struct {
	// Maximum file size before rotation (bytes)
	MaxFileSize int64
	// Retention period for old WAL files
	RetentionDays int
	// Buffer size for writes
	BufferSize int
	// File name prefix
	FilePrefix string
}

// DefaultConfig returns sensible default configuration
func DefaultConfig() Config {
	return Config{
		MaxFileSize:   100 * 1024 * 1024, // 100MB
		RetentionDays: 30,                // 30 days
		BufferSize:    4096,              // 4KB buffer
		FilePrefix:    "elava",
	}
}

// WAL provides Write-Ahead Logging for audit and recovery
type WAL struct {
	mu       sync.Mutex
	file     *os.File
	writer   *bufio.Writer
	sequence int64
	dir      string
	config   Config
}

// Open creates or opens a WAL in the specified directory with default config
func Open(dir string) (*WAL, error) {
	return OpenWithConfig(dir, DefaultConfig())
}

// OpenWithConfig creates or opens a WAL with custom configuration
func OpenWithConfig(dir string, config Config) (*WAL, error) {
	if err := os.MkdirAll(dir, 0750); err != nil {
		return nil, fmt.Errorf("failed to create WAL directory: %w", err)
	}

	// Use timestamp in filename for rotation
	filename := fmt.Sprintf("%s-%s.wal", config.FilePrefix, time.Now().Format("20060102-150405"))
	path := filepath.Join(dir, filename)

	file, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600) // #nosec G304 - path is constructed from trusted dir
	if err != nil {
		return nil, fmt.Errorf("failed to open WAL file: %w", err)
	}

	w := &WAL{
		file:   file,
		writer: bufio.NewWriterSize(file, config.BufferSize),
		dir:    dir,
		config: config,
	}

	// Load last sequence number from existing WAL files
	w.loadSequence()

	return w, nil
}

// Close flushes and closes the WAL
func (w *WAL) Close() error {
	w.mu.Lock()
	defer w.mu.Unlock()

	if err := w.writer.Flush(); err != nil {
		return err
	}
	return w.file.Close()
}

// Append adds an entry to the WAL
func (w *WAL) Append(entryType EntryType, resourceID string, data interface{}) error {
	w.mu.Lock()
	defer w.mu.Unlock()

	w.sequence++

	jsonData, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("failed to marshal data: %w", err)
	}

	entry := Entry{
		Timestamp:  time.Now(),
		Sequence:   w.sequence,
		Type:       entryType,
		ResourceID: resourceID,
		Data:       jsonData,
	}

	return w.writeEntry(entry)
}

// AppendError adds an error entry to the WAL
func (w *WAL) AppendError(entryType EntryType, resourceID string, data interface{}, errToLog error) error {
	w.mu.Lock()
	defer w.mu.Unlock()

	w.sequence++

	jsonData, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("failed to marshal data: %w", err)
	}

	entry := Entry{
		Timestamp:  time.Now(),
		Sequence:   w.sequence,
		Type:       entryType,
		ResourceID: resourceID,
		Data:       jsonData,
		Error:      errToLog.Error(),
	}

	return w.writeEntry(entry)
}

// writeEntry writes a single entry to the WAL
func (w *WAL) writeEntry(entry Entry) error {
	line, err := json.Marshal(entry)
	if err != nil {
		return fmt.Errorf("failed to marshal entry: %w", err)
	}

	if _, err := w.writer.Write(line); err != nil {
		return fmt.Errorf("failed to write entry: %w", err)
	}

	if _, err := w.writer.WriteString("\n"); err != nil {
		return fmt.Errorf("failed to write newline: %w", err)
	}

	// Flush immediately for durability
	if err := w.writer.Flush(); err != nil {
		return fmt.Errorf("failed to flush: %w", err)
	}

	return w.file.Sync()
}

// loadSequence finds the last sequence number from existing WAL files
func (w *WAL) loadSequence() {
	maxSeq := w.findMaxSequence()
	w.sequence = maxSeq
}

// findMaxSequence scans all WAL files to find the highest sequence number
func (w *WAL) findMaxSequence() int64 {
	files := w.listWALFiles()
	if len(files) == 0 {
		return 0
	}

	return w.scanFilesForMaxSequence(files)
}

// listWALFiles returns all WAL files in the directory, sorted by name
func (w *WAL) listWALFiles() []string {
	pattern := filepath.Join(w.dir, w.config.FilePrefix+"-*.wal")
	files, err := filepath.Glob(pattern)
	if err != nil {
		return nil
	}
	return files
}

// scanFilesForMaxSequence reads files to find the highest sequence number
func (w *WAL) scanFilesForMaxSequence(files []string) int64 {
	maxSeq := int64(0)

	// Start from the last file (most recent) for efficiency
	for i := len(files) - 1; i >= 0; i-- {
		fileMax := w.getMaxSequenceFromFile(files[i])
		if fileMax > maxSeq {
			maxSeq = fileMax
		}
	}

	return maxSeq
}

// getMaxSequenceFromFile reads a single file and returns its max sequence
func (w *WAL) getMaxSequenceFromFile(path string) int64 {
	reader, err := NewReader(path)
	if err != nil {
		return 0
	}
	defer func() { _ = reader.Close() }()

	maxSeq := int64(0)
	for {
		entry, err := reader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			continue // Skip corrupted entries
		}
		if entry.Sequence > maxSeq {
			maxSeq = entry.Sequence
		}
	}

	return maxSeq
}

// Reader provides WAL replay functionality
type Reader struct {
	scanner *bufio.Scanner
	file    *os.File
}

// NewReader creates a WAL reader for the specified file
func NewReader(path string) (*Reader, error) {
	file, err := os.Open(path) // #nosec G304 - path is constructed from trusted dir
	if err != nil {
		return nil, fmt.Errorf("failed to open WAL file: %w", err)
	}

	return &Reader{
		scanner: bufio.NewScanner(file),
		file:    file,
	}, nil
}

// Next reads the next entry from the WAL
func (r *Reader) Next() (*Entry, error) {
	if !r.scanner.Scan() {
		if err := r.scanner.Err(); err != nil {
			return nil, err
		}
		return nil, io.EOF
	}

	var entry Entry
	if err := json.Unmarshal(r.scanner.Bytes(), &entry); err != nil {
		return nil, fmt.Errorf("failed to unmarshal entry: %w", err)
	}

	return &entry, nil
}

// Close closes the reader
func (r *Reader) Close() error {
	return r.file.Close()
}

// Replay replays WAL entries from a specific time
func Replay(dir string, since time.Time, handler func(*Entry) error) error {
	files, err := filepath.Glob(filepath.Join(dir, "elava-*.wal"))
	if err != nil {
		return fmt.Errorf("failed to list WAL files: %w", err)
	}

	for _, file := range files {
		reader, err := NewReader(file)
		if err != nil {
			return err
		}
		defer func() {
			_ = reader.Close()
		}()

		for {
			entry, err := reader.Next()
			if err == io.EOF {
				break
			}
			if err != nil {
				return err
			}

			if entry.Timestamp.After(since) {
				if err := handler(entry); err != nil {
					return err
				}
			}
		}
	}

	return nil
}
