package types

import (
	"testing"
)

func TestDecision_Validate(t *testing.T) {
	tests := []struct {
		name     string
		decision Decision
		wantErr  bool
	}{
		{
			name: "valid create decision",
			decision: Decision{
				Action: ActionCreate,
				Reason: "Resource doesn't exist",
			},
			wantErr: false,
		},
		{
			name: "valid delete decision",
			decision: Decision{
				Action:     ActionDelete,
				ResourceID: "i-123456",
				Reason:     "Orphaned resource",
			},
			wantErr: false,
		},
		{
			name: "valid notify decision",
			decision: Decision{
				Action:     ActionNotify,
				ResourceID: "i-789456",
				Reason:     "Resource requires attention",
			},
			wantErr: false,
		},
		{
			name: "invalid - empty action",
			decision: Decision{
				ResourceID: "i-123456",
				Reason:     "Some reason",
			},
			wantErr: true,
		},
		{
			name: "valid - create with empty resource ID",
			decision: Decision{
				Action: ActionCreate,
				Reason: "New resource will be created",
			},
			wantErr: false, // Create actions don't require ResourceID
		},
		{
			name: "invalid - delete with empty resource ID",
			decision: Decision{
				Action: ActionDelete,
				Reason: "Delete something",
			},
			wantErr: true, // Non-create actions require ResourceID
		},
		{
			name: "invalid - empty reason",
			decision: Decision{
				Action:     ActionCreate,
				ResourceID: "i-123456",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.decision.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestDecision_IsDestructive(t *testing.T) {
	tests := []struct {
		name     string
		decision Decision
		want     bool
	}{
		{
			name:     "delete is destructive",
			decision: Decision{Action: ActionDelete},
			want:     true,
		},
		{
			name:     "terminate is destructive",
			decision: Decision{Action: ActionTerminate},
			want:     true,
		},
		{
			name:     "create is not destructive",
			decision: Decision{Action: ActionCreate},
			want:     false,
		},
		{
			name:     "update is not destructive",
			decision: Decision{Action: ActionUpdate},
			want:     false,
		},
		{
			name:     "notify is not destructive",
			decision: Decision{Action: ActionNotify},
			want:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.decision.IsDestructive(); got != tt.want {
				t.Errorf("IsDestructive() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestDecision_RequiresConfirmation(t *testing.T) {
	tests := []struct {
		name     string
		decision Decision
		want     bool
	}{
		{
			name: "delete requires confirmation",
			decision: Decision{
				Action:     ActionDelete,
				ResourceID: "i-123456",
				Reason:     "Orphaned",
			},
			want: true,
		},
		{
			name: "blessed resource requires confirmation",
			decision: Decision{
				Action:     ActionUpdate,
				ResourceID: "i-123456",
				Reason:     "Update config",
				IsBlessed:  true,
			},
			want: true,
		},
		{
			name: "create doesn't require confirmation",
			decision: Decision{
				Action:     ActionCreate,
				ResourceID: "new-resource",
				Reason:     "Scaling up",
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.decision.RequiresConfirmation(); got != tt.want {
				t.Errorf("RequiresConfirmation() = %v, want %v", got, tt.want)
			}
		})
	}
}
