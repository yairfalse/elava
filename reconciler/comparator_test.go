package reconciler

import (
	"testing"

	"github.com/yairfalse/ovi/types"
)

//nolint:gocognit // Test complexity is acceptable for thorough coverage
func TestSimpleComparator_Compare(t *testing.T) {
	comparator := NewSimpleComparator()

	tests := []struct {
		name     string
		current  []types.Resource
		desired  []types.Resource
		expected []Diff
	}{
		{
			name:     "no differences",
			current:  []types.Resource{},
			desired:  []types.Resource{},
			expected: []Diff{},
		},
		{
			name:    "missing resource",
			current: []types.Resource{},
			desired: []types.Resource{
				{
					ID:       "i-missing",
					Type:     "ec2",
					Provider: "aws",
					Tags:     types.Tags{OviManaged: true},
				},
			},
			expected: []Diff{
				{
					Type:       DiffMissing,
					ResourceID: "i-missing",
					Reason:     "Resource specified in config but not found in cloud",
				},
			},
		},
		{
			name: "unwanted managed resource",
			current: []types.Resource{
				{
					ID:       "i-unwanted",
					Type:     "ec2",
					Provider: "aws",
					Tags:     types.Tags{OviManaged: true, OviOwner: "ovi"},
				},
			},
			desired: []types.Resource{},
			expected: []Diff{
				{
					Type:       DiffUnwanted,
					ResourceID: "i-unwanted",
					Reason:     "Resource managed by Ovi but not in current config",
				},
			},
		},
		{
			name: "unmanaged resource",
			current: []types.Resource{
				{
					ID:       "i-unmanaged",
					Type:     "ec2",
					Provider: "aws",
					Tags:     types.Tags{}, // No Ovi tags
				},
			},
			desired: []types.Resource{},
			expected: []Diff{
				{
					Type:       DiffUnmanaged,
					ResourceID: "i-unmanaged",
					Reason:     "Resource exists but not managed by Ovi",
				},
			},
		},
		{
			name: "drifted resource",
			current: []types.Resource{
				{
					ID:       "i-drifted",
					Type:     "ec2",
					Provider: "aws",
					Tags:     types.Tags{OviManaged: true, Environment: "prod"},
				},
			},
			desired: []types.Resource{
				{
					ID:       "i-drifted",
					Type:     "ec2",
					Provider: "aws",
					Tags:     types.Tags{OviManaged: true, Environment: "staging"},
				},
			},
			expected: []Diff{
				{
					Type:       DiffDrifted,
					ResourceID: "i-drifted",
					Reason:     "Resource configuration differs from desired state",
				},
			},
		},
		{
			name: "matching resources",
			current: []types.Resource{
				{
					ID:       "i-perfect",
					Type:     "ec2",
					Provider: "aws",
					Tags:     types.Tags{OviManaged: true, Environment: "prod"},
				},
			},
			desired: []types.Resource{
				{
					ID:       "i-perfect",
					Type:     "ec2",
					Provider: "aws",
					Tags:     types.Tags{OviManaged: true, Environment: "prod"},
				},
			},
			expected: []Diff{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			diffs, err := comparator.Compare(tt.current, tt.desired)
			if err != nil {
				t.Fatalf("Compare() error = %v", err)
			}

			if len(diffs) != len(tt.expected) {
				t.Errorf("Compare() got %d diffs, want %d", len(diffs), len(tt.expected))
				return
			}

			for i, diff := range diffs {
				expected := tt.expected[i]
				if diff.Type != expected.Type {
					t.Errorf("Diff[%d].Type = %v, want %v", i, diff.Type, expected.Type)
				}
				if diff.ResourceID != expected.ResourceID {
					t.Errorf("Diff[%d].ResourceID = %v, want %v", i, diff.ResourceID, expected.ResourceID)
				}
				if diff.Reason != expected.Reason {
					t.Errorf("Diff[%d].Reason = %v, want %v", i, diff.Reason, expected.Reason)
				}
			}
		})
	}
}

func TestIsDrifted(t *testing.T) {
	tests := []struct {
		name     string
		current  types.Resource
		desired  types.Resource
		expected bool
	}{
		{
			name: "no drift",
			current: types.Resource{
				Type:     "ec2",
				Provider: "aws",
				Region:   "us-east-1",
				Tags:     types.Tags{Environment: "prod"},
			},
			desired: types.Resource{
				Type:     "ec2",
				Provider: "aws",
				Region:   "us-east-1",
				Tags:     types.Tags{Environment: "prod"},
			},
			expected: false,
		},
		{
			name: "type drift",
			current: types.Resource{
				Type:     "ec2",
				Provider: "aws",
				Tags:     types.Tags{},
			},
			desired: types.Resource{
				Type:     "rds",
				Provider: "aws",
				Tags:     types.Tags{},
			},
			expected: true,
		},
		{
			name: "environment drift",
			current: types.Resource{
				Type:     "ec2",
				Provider: "aws",
				Tags:     types.Tags{Environment: "prod"},
			},
			desired: types.Resource{
				Type:     "ec2",
				Provider: "aws",
				Tags:     types.Tags{Environment: "staging"},
			},
			expected: true,
		},
		{
			name: "owner drift",
			current: types.Resource{
				Type:     "ec2",
				Provider: "aws",
				Tags:     types.Tags{OviOwner: "team-web"},
			},
			desired: types.Resource{
				Type:     "ec2",
				Provider: "aws",
				Tags:     types.Tags{OviOwner: "team-api"},
			},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isDrifted(tt.current, tt.desired)
			if result != tt.expected {
				t.Errorf("isDrifted() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestBuildResourceMap(t *testing.T) {
	resources := []types.Resource{
		{ID: "i-123", Type: "ec2"},
		{ID: "i-456", Type: "ec2"},
	}

	resourceMap := buildResourceMap(resources)

	if len(resourceMap) != 2 {
		t.Errorf("buildResourceMap() got %d entries, want 2", len(resourceMap))
	}

	if _, exists := resourceMap["i-123"]; !exists {
		t.Error("buildResourceMap() missing i-123")
	}

	if _, exists := resourceMap["i-456"]; !exists {
		t.Error("buildResourceMap() missing i-456")
	}

	if resourceMap["i-123"].Type != "ec2" {
		t.Errorf("buildResourceMap()['i-123'].Type = %v, want ec2", resourceMap["i-123"].Type)
	}
}
