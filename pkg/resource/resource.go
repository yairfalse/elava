// Package resource defines the unified resource model for Elava.
package resource

import "time"

// Resource represents a cloud resource in unified format.
// This is emitted as metrics/logs - no storage, no state.
type Resource struct {
	ID        string            `json:"id"`         // Unique identifier (e.g., "i-abc123")
	Type      string            `json:"type"`       // Resource type (e.g., "ec2", "rds")
	Provider  string            `json:"provider"`   // Cloud provider (e.g., "aws", "gcp")
	Region    string            `json:"region"`     // Region (e.g., "us-east-1")
	Account   string            `json:"account"`    // Account/Project ID
	Name      string            `json:"name"`       // Human-readable name
	Status    string            `json:"status"`     // Current status (e.g., "running")
	Labels    map[string]string `json:"labels"`     // Normalized labels/tags
	Attrs     map[string]string `json:"attrs"`      // Provider-specific attributes
	ScannedAt time.Time         `json:"scanned_at"` // When this was scanned
}

// ScanResult holds the result of a plugin scan.
type ScanResult struct {
	Provider  string
	Region    string
	Resources []Resource
	Duration  time.Duration
	Error     error
}
