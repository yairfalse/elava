package reconciler

import (
	"testing"

	"github.com/yairfalse/elava/types"
)

//nolint:gocognit // Test complexity is acceptable for thorough coverage
func TestSimpleDecisionMaker_Decide(t *testing.T) {
	tests := []struct {
		name            string
		skipDestructive bool
		diffs           []Diff
		expectedActions []string
		shouldError     bool
	}{
		{
			name:            "no diffs",
			skipDestructive: false,
			diffs:           []Diff{},
			expectedActions: []string{},
			shouldError:     false,
		},
		{
			name:            "missing resource",
			skipDestructive: false,
			diffs: []Diff{
				{
					Type:       DiffMissing,
					ResourceID: "i-missing",
					Desired: &types.Resource{
						ID:   "i-missing",
						Type: "ec2",
						Tags: types.Tags{OviManaged: true},
					},
					Reason: "Resource specified in config but not found",
				},
			},
			expectedActions: []string{"create"},
			shouldError:     false,
		},
		{
			name:            "unwanted resource - destructive allowed",
			skipDestructive: false,
			diffs: []Diff{
				{
					Type:       DiffUnwanted,
					ResourceID: "i-unwanted",
					Current: &types.Resource{
						ID:   "i-unwanted",
						Type: "ec2",
						Tags: types.Tags{OviManaged: true},
					},
					Reason: "Resource not in config",
				},
			},
			expectedActions: []string{"delete"},
			shouldError:     false,
		},
		{
			name:            "unwanted resource - destructive skipped",
			skipDestructive: true,
			diffs: []Diff{
				{
					Type:       DiffUnwanted,
					ResourceID: "i-unwanted",
					Current: &types.Resource{
						ID:   "i-unwanted",
						Type: "ec2",
						Tags: types.Tags{OviManaged: true},
					},
					Reason: "Resource not in config",
				},
			},
			expectedActions: []string{"notify"},
			shouldError:     false,
		},
		{
			name:            "blessed resource",
			skipDestructive: false,
			diffs: []Diff{
				{
					Type:       DiffUnwanted,
					ResourceID: "i-blessed",
					Current: &types.Resource{
						ID:   "i-blessed",
						Type: "ec2",
						Tags: types.Tags{OviManaged: true, OviBlessed: true},
					},
					Reason: "Resource not in config",
				},
			},
			expectedActions: []string{"notify"},
			shouldError:     false,
		},
		{
			name:            "drifted resource",
			skipDestructive: false,
			diffs: []Diff{
				{
					Type:       DiffDrifted,
					ResourceID: "i-drifted",
					Current: &types.Resource{
						ID:   "i-drifted",
						Type: "ec2",
						Tags: types.Tags{Environment: "prod"},
					},
					Desired: &types.Resource{
						ID:   "i-drifted",
						Type: "ec2",
						Tags: types.Tags{Environment: "staging"},
					},
					Reason: "Configuration differs",
				},
			},
			expectedActions: []string{"update"},
			shouldError:     false,
		},
		{
			name:            "unmanaged resource",
			skipDestructive: false,
			diffs: []Diff{
				{
					Type:       DiffUnmanaged,
					ResourceID: "i-unmanaged",
					Current: &types.Resource{
						ID:   "i-unmanaged",
						Type: "ec2",
						Tags: types.Tags{}, // No Ovi tags
					},
					Reason: "Not managed by Ovi",
				},
			},
			expectedActions: []string{"notify"},
			shouldError:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dm := NewSimpleDecisionMaker(tt.skipDestructive)
			decisions, err := dm.Decide(tt.diffs)

			if tt.shouldError {
				if err == nil {
					t.Errorf("Decide() expected error but got none")
				}
				return
			}

			if err != nil {
				t.Fatalf("Decide() error = %v", err)
			}

			if len(decisions) != len(tt.expectedActions) {
				t.Errorf("Decide() got %d decisions, want %d", len(decisions), len(tt.expectedActions))
				return
			}

			for i, decision := range decisions {
				if decision.Action != tt.expectedActions[i] {
					t.Errorf("Decision[%d].Action = %v, want %v", i, decision.Action, tt.expectedActions[i])
				}
			}
		})
	}
}

func TestSimpleDecisionMaker_DecideSingle(t *testing.T) {
	dm := NewSimpleDecisionMaker(false)

	tests := []struct {
		name           string
		diff           Diff
		expectedAction string
		shouldError    bool
	}{
		{
			name: "missing resource decision",
			diff: Diff{
				Type:       DiffMissing,
				ResourceID: "i-missing",
				Desired: &types.Resource{
					ID:   "i-missing",
					Type: "ec2",
				},
				Reason: "Test missing",
			},
			expectedAction: "create",
			shouldError:    false,
		},
		{
			name: "missing diff without desired state",
			diff: Diff{
				Type:       DiffMissing,
				ResourceID: "i-missing",
				Desired:    nil,
				Reason:     "Test missing",
			},
			expectedAction: "",
			shouldError:    true,
		},
		{
			name: "unwanted resource decision",
			diff: Diff{
				Type:       DiffUnwanted,
				ResourceID: "i-unwanted",
				Current: &types.Resource{
					ID:   "i-unwanted",
					Type: "ec2",
					Tags: types.Tags{OviManaged: true},
				},
				Reason: "Test unwanted",
			},
			expectedAction: "delete",
			shouldError:    false,
		},
		{
			name: "drifted resource decision",
			diff: Diff{
				Type:       DiffDrifted,
				ResourceID: "i-drifted",
				Current: &types.Resource{
					ID:   "i-drifted",
					Type: "ec2",
				},
				Desired: &types.Resource{
					ID:   "i-drifted",
					Type: "ec2",
				},
				Reason: "Test drifted",
			},
			expectedAction: "update",
			shouldError:    false,
		},
		{
			name: "unmanaged resource decision",
			diff: Diff{
				Type:       DiffUnmanaged,
				ResourceID: "i-unmanaged",
				Current: &types.Resource{
					ID:   "i-unmanaged",
					Type: "ec2",
				},
				Reason: "Test unmanaged",
			},
			expectedAction: "notify",
			shouldError:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			decision, err := dm.decideSingle(tt.diff)

			if tt.shouldError {
				if err == nil {
					t.Errorf("decideSingle() expected error but got none")
				}
				return
			}

			if err != nil {
				t.Fatalf("decideSingle() error = %v", err)
			}

			if decision == nil {
				t.Fatalf("decideSingle() returned nil decision")
			}

			if decision.Action != tt.expectedAction {
				t.Errorf("decideSingle().Action = %v, want %v", decision.Action, tt.expectedAction)
			}

			if decision.ResourceID != tt.diff.ResourceID {
				t.Errorf("decideSingle().ResourceID = %v, want %v", decision.ResourceID, tt.diff.ResourceID)
			}
		})
	}
}

func TestSimpleDecisionMaker_DecideMissing(t *testing.T) {
	dm := NewSimpleDecisionMaker(false)

	resource := &types.Resource{
		ID:   "i-test",
		Type: "ec2",
		Tags: types.Tags{OviManaged: true},
	}

	diff := Diff{
		Type:       DiffMissing,
		ResourceID: "i-test",
		Desired:    resource,
		Reason:     "Test reason",
	}

	decision, err := dm.decideMissing(diff)
	if err != nil {
		t.Fatalf("decideMissing() error = %v", err)
	}

	if decision.Action != "create" {
		t.Errorf("decideMissing().Action = %v, want create", decision.Action)
	}

	if decision.ResourceID != resource.ID {
		t.Errorf("decideMissing().ResourceID = %v, want %v", decision.ResourceID, resource.ID)
	}
}

func TestSimpleDecisionMaker_DecideUnwanted_Blessed(t *testing.T) {
	dm := NewSimpleDecisionMaker(false)

	resource := &types.Resource{
		ID:   "i-blessed",
		Type: "ec2",
		Tags: types.Tags{OviManaged: true, OviBlessed: true},
	}

	diff := Diff{
		Type:       DiffUnwanted,
		ResourceID: "i-blessed",
		Current:    resource,
		Reason:     "Test blessed",
	}

	decision, err := dm.decideUnwanted(diff)
	if err != nil {
		t.Fatalf("decideUnwanted() error = %v", err)
	}

	if decision.Action != "notify" {
		t.Errorf("decideUnwanted() blessed resource should notify, got %v", decision.Action)
	}
}
