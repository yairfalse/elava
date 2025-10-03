package storage

import (
	"strings"
	"testing"
	"time"
)

func TestValidateChangeEvent(t *testing.T) {
	tests := []struct {
		name    string
		event   ChangeEvent
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid event",
			event: ChangeEvent{
				ResourceID: "i-123",
				ChangeType: "created",
				Timestamp:  time.Now(),
			},
			wantErr: false,
		},
		{
			name: "empty resource_id",
			event: ChangeEvent{
				ResourceID: "",
				ChangeType: "created",
				Timestamp:  time.Now(),
			},
			wantErr: true,
			errMsg:  "resource_id cannot be empty",
		},
		{
			name: "empty change_type",
			event: ChangeEvent{
				ResourceID: "i-123",
				ChangeType: "",
				Timestamp:  time.Now(),
			},
			wantErr: true,
			errMsg:  "change_type cannot be empty",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateChangeEvent(tt.event)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateChangeEvent() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.wantErr && !strings.Contains(err.Error(), tt.errMsg) {
				t.Errorf("error message = %q, want to contain %q", err.Error(), tt.errMsg)
			}
		})
	}
}

func TestValidateDriftEvent(t *testing.T) {
	tests := []struct {
		name    string
		event   DriftEvent
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid event",
			event: DriftEvent{
				ResourceID: "i-456",
				DriftType:  "config_drift",
				Field:      "instance_type",
				Expected:   "t2.micro",
				Actual:     "t2.small",
				Severity:   "medium",
				Timestamp:  time.Now(),
			},
			wantErr: false,
		},
		{
			name: "empty resource_id",
			event: DriftEvent{
				DriftType: "config_drift",
				Field:     "instance_type",
				Severity:  "medium",
			},
			wantErr: true,
			errMsg:  "resource_id cannot be empty",
		},
		{
			name: "empty drift_type",
			event: DriftEvent{
				ResourceID: "i-456",
				Field:      "instance_type",
				Severity:   "medium",
			},
			wantErr: true,
			errMsg:  "drift_type cannot be empty",
		},
		{
			name: "empty field",
			event: DriftEvent{
				ResourceID: "i-456",
				DriftType:  "config_drift",
				Severity:   "medium",
			},
			wantErr: true,
			errMsg:  "field cannot be empty",
		},
		{
			name: "empty severity",
			event: DriftEvent{
				ResourceID: "i-456",
				DriftType:  "config_drift",
				Field:      "instance_type",
			},
			wantErr: true,
			errMsg:  "severity cannot be empty",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateDriftEvent(tt.event)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateDriftEvent() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.wantErr && !strings.Contains(err.Error(), tt.errMsg) {
				t.Errorf("error message = %q, want to contain %q", err.Error(), tt.errMsg)
			}
		})
	}
}

func TestValidateWastePattern(t *testing.T) {
	tests := []struct {
		name    string
		pattern WastePattern
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid pattern",
			pattern: WastePattern{
				PatternType: "idle",
				ResourceIDs: []string{"i-789"},
				Confidence:  0.95,
				Reason:      "Low CPU usage",
				Timestamp:   time.Now(),
			},
			wantErr: false,
		},
		{
			name: "empty pattern_type",
			pattern: WastePattern{
				ResourceIDs: []string{"i-789"},
				Confidence:  0.95,
				Reason:      "test",
			},
			wantErr: true,
			errMsg:  "pattern_type cannot be empty",
		},
		{
			name: "empty resource_ids",
			pattern: WastePattern{
				PatternType: "idle",
				ResourceIDs: []string{},
				Confidence:  0.95,
				Reason:      "test",
			},
			wantErr: true,
			errMsg:  "resource_ids cannot be empty",
		},
		{
			name: "confidence too low",
			pattern: WastePattern{
				PatternType: "idle",
				ResourceIDs: []string{"i-789"},
				Confidence:  -0.1,
				Reason:      "test",
			},
			wantErr: true,
			errMsg:  "confidence must be between 0.0 and 1.0",
		},
		{
			name: "confidence too high",
			pattern: WastePattern{
				PatternType: "idle",
				ResourceIDs: []string{"i-789"},
				Confidence:  1.5,
				Reason:      "test",
			},
			wantErr: true,
			errMsg:  "confidence must be between 0.0 and 1.0",
		},
		{
			name: "empty reason",
			pattern: WastePattern{
				PatternType: "idle",
				ResourceIDs: []string{"i-789"},
				Confidence:  0.95,
				Reason:      "",
			},
			wantErr: true,
			errMsg:  "reason cannot be empty",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateWastePattern(tt.pattern)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateWastePattern() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.wantErr && !strings.Contains(err.Error(), tt.errMsg) {
				t.Errorf("error message = %q, want to contain %q", err.Error(), tt.errMsg)
			}
		})
	}
}
