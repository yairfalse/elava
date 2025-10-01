package storage

import "testing"

// TestInterfaceCompliance verifies MVCCStorage implements all interfaces
// This test ensures compile-time verification of interface compliance
func TestInterfaceCompliance(t *testing.T) {
	// Complete Storage interface
	var _ Storage = (*MVCCStorage)(nil)

	// Observation interfaces
	var _ ObservationStorage = (*MVCCStorage)(nil)
	var _ ObservationWriter = (*MVCCStorage)(nil)
	var _ ObservationReader = (*MVCCStorage)(nil)

	// Analyzer event interfaces
	var _ AnalyzerEventStorage = (*MVCCStorage)(nil)
	var _ AnalyzerEventWriter = (*MVCCStorage)(nil)
	var _ AnalyzerEventReader = (*MVCCStorage)(nil)

	// Maintenance interfaces
	var _ Compactor = (*MVCCStorage)(nil)
	var _ StorageStats = (*MVCCStorage)(nil)
	var _ Lifecycle = (*MVCCStorage)(nil)
}
