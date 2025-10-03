package storage

import "fmt"

// validateChangeEvent validates a ChangeEvent before storage
func validateChangeEvent(e ChangeEvent) error {
	if e.ResourceID == "" {
		return fmt.Errorf("change event resource_id cannot be empty")
	}
	if e.ChangeType == "" {
		return fmt.Errorf("change event change_type cannot be empty")
	}
	return nil
}

// validateDriftEvent validates a DriftEvent before storage
func validateDriftEvent(e DriftEvent) error {
	if e.ResourceID == "" {
		return fmt.Errorf("drift event resource_id cannot be empty")
	}
	if e.DriftType == "" {
		return fmt.Errorf("drift event drift_type cannot be empty")
	}
	if e.Field == "" {
		return fmt.Errorf("drift event field cannot be empty")
	}
	if e.Severity == "" {
		return fmt.Errorf("drift event severity cannot be empty")
	}
	return nil
}

// validateWastePattern validates a WastePattern before storage
func validateWastePattern(p WastePattern) error {
	if p.PatternType == "" {
		return fmt.Errorf("waste pattern pattern_type cannot be empty")
	}
	if len(p.ResourceIDs) == 0 {
		return fmt.Errorf("waste pattern resource_ids cannot be empty")
	}
	if p.Confidence < 0.0 || p.Confidence > 1.0 {
		return fmt.Errorf("waste pattern confidence must be between 0.0 and 1.0, got %f", p.Confidence)
	}
	if p.Reason == "" {
		return fmt.Errorf("waste pattern reason cannot be empty")
	}
	return nil
}
