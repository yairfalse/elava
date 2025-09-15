package reconciler

import (
	"context"
	"testing"
	"time"

	"github.com/yairfalse/elava/storage"
	"github.com/yairfalse/elava/types"
	"github.com/yairfalse/elava/wal"
)

// MockObserver for testing
type MockObserver struct {
	resources []types.Resource
	err       error
}

func (m *MockObserver) Observe(ctx context.Context, filter types.ResourceFilter) ([]types.Resource, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.resources, nil
}

// MockComparator for testing
type MockComparator struct {
	diffs []Diff
	err   error
}

func (m *MockComparator) Compare(current, desired []types.Resource) ([]Diff, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.diffs, nil
}

// MockDecisionMaker for testing
type MockDecisionMaker struct {
	decisions []types.Decision
	err       error
}

func (m *MockDecisionMaker) Decide(diffs []Diff) ([]types.Decision, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.decisions, nil
}

// MockCoordinator for testing
type MockCoordinator struct {
	claims map[string]bool
	err    error
}

func NewMockCoordinator() *MockCoordinator {
	return &MockCoordinator{
		claims: make(map[string]bool),
	}
}

func (m *MockCoordinator) ClaimResources(ctx context.Context, resourceIDs []string, ttl time.Duration) error {
	if m.err != nil {
		return m.err
	}
	for _, id := range resourceIDs {
		m.claims[id] = true
	}
	return nil
}

func (m *MockCoordinator) ReleaseResources(ctx context.Context, resourceIDs []string) error {
	if m.err != nil {
		return m.err
	}
	for _, id := range resourceIDs {
		delete(m.claims, id)
	}
	return nil
}

func (m *MockCoordinator) IsResourceClaimed(ctx context.Context, resourceID string) (bool, error) {
	if m.err != nil {
		return false, m.err
	}
	return m.claims[resourceID], nil
}

func TestEngine_Reconcile(t *testing.T) {
	// Setup test storage and WAL
	tmpDir := t.TempDir()
	storage, err := storage.NewMVCCStorage(tmpDir)
	if err != nil {
		t.Fatalf("Failed to create storage: %v", err)
	}
	defer func() { _ = storage.Close() }()

	walInstance, err := wal.Open(tmpDir)
	if err != nil {
		t.Fatalf("Failed to create WAL: %v", err)
	}
	defer func() { _ = walInstance.Close() }()

	tests := []struct {
		name              string
		observer          Observer
		comparator        Comparator
		decisionMaker     DecisionMaker
		config            Config
		expectedDecisions int
		shouldError       bool
	}{
		{
			name: "successful reconciliation",
			observer: &MockObserver{
				resources: []types.Resource{
					{
						ID:       "i-existing",
						Type:     "ec2",
						Provider: "aws",
						Tags:     types.Tags{ElavaManaged: true},
					},
				},
			},
			comparator: &MockComparator{
				diffs: []Diff{
					{
						Type:       DiffMissing,
						ResourceID: "i-missing",
						Reason:     "Test missing resource",
					},
				},
			},
			decisionMaker: &MockDecisionMaker{
				decisions: []types.Decision{
					{
						Action:     "create",
						ResourceID: "i-missing",
						Reason:     "Test decision",
					},
				},
			},
			config: Config{
				Provider: "aws",
				Region:   "us-east-1",
				Resources: []types.ResourceSpec{
					{
						Type:  "ec2",
						Count: 1,
						Tags:  types.Tags{ElavaManaged: true},
					},
				},
			},
			expectedDecisions: 1,
			shouldError:       false,
		},
		{
			name: "empty reconciliation",
			observer: &MockObserver{
				resources: []types.Resource{},
			},
			comparator: &MockComparator{
				diffs: []Diff{},
			},
			decisionMaker: &MockDecisionMaker{
				decisions: []types.Decision{},
			},
			config: Config{
				Provider:  "aws",
				Region:    "us-east-1",
				Resources: []types.ResourceSpec{},
			},
			expectedDecisions: 0,
			shouldError:       false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			coordinator := NewMockCoordinator()
			options := ReconcilerOptions{
				DryRun:          false,
				MaxConcurrency:  1,
				ClaimTTL:        time.Minute,
				SkipDestructive: false,
			}

			engine := NewEngine(
				tt.observer,
				tt.comparator,
				tt.decisionMaker,
				coordinator,
				storage,
				walInstance,
				"test-instance",
				options,
			)

			ctx := context.Background()
			decisions, err := engine.Reconcile(ctx, tt.config)

			if tt.shouldError {
				if err == nil {
					t.Errorf("Reconcile() expected error but got none")
				}
				return
			}

			if err != nil {
				t.Fatalf("Reconcile() error = %v", err)
			}

			if len(decisions) != tt.expectedDecisions {
				t.Errorf("Reconcile() got %d decisions, want %d", len(decisions), tt.expectedDecisions)
			}
		})
	}
}

func TestEngine_BuildDesiredState(t *testing.T) {
	engine := &Engine{}

	config := Config{
		Provider: "aws",
		Region:   "us-east-1",
		Resources: []types.ResourceSpec{
			{
				Type:  "ec2",
				Count: 2,
				Tags:  types.Tags{Environment: "prod"},
			},
			{
				Type:  "rds",
				Count: 1,
				Tags:  types.Tags{Environment: "prod"},
			},
		},
	}

	desired := engine.buildDesiredState(config)

	expectedCount := 3 // 2 EC2 + 1 RDS
	if len(desired) != expectedCount {
		t.Errorf("buildDesiredState() got %d resources, want %d", len(desired), expectedCount)
	}

	// Check that all resources are marked as Elava-managed
	for i, resource := range desired {
		if !resource.Tags.ElavaManaged {
			t.Errorf("Resource[%d] not marked as ElavaManaged", i)
		}

		if resource.Tags.ElavaOwner == "" {
			t.Errorf("Resource[%d] missing ElavaOwner", i)
		}

		if resource.Provider != config.Provider {
			t.Errorf("Resource[%d].Provider = %v, want %v", i, resource.Provider, config.Provider)
		}
	}
}

func TestEngine_ObserveCurrentState(t *testing.T) {
	tmpDir := t.TempDir()
	walInstance, err := wal.Open(tmpDir)
	if err != nil {
		t.Fatalf("Failed to create WAL: %v", err)
	}
	defer func() { _ = walInstance.Close() }()

	mockObserver := &MockObserver{
		resources: []types.Resource{
			{ID: "i-123", Type: "ec2", Provider: "aws"},
			{ID: "i-456", Type: "ec2", Provider: "aws"},
		},
	}

	engine := &Engine{
		observer: mockObserver,
		wal:      walInstance,
	}

	config := Config{
		Provider: "aws",
		Region:   "us-east-1",
	}

	ctx := context.Background()
	resources, err := engine.observeCurrentState(ctx, config)

	if err != nil {
		t.Fatalf("observeCurrentState() error = %v", err)
	}

	if len(resources) != 2 {
		t.Errorf("observeCurrentState() got %d resources, want 2", len(resources))
	}
}

func TestNewEngine(t *testing.T) {
	tmpDir := t.TempDir()
	storage, _ := storage.NewMVCCStorage(tmpDir)
	defer func() { _ = storage.Close() }()

	walInstance, _ := wal.Open(tmpDir)
	defer func() { _ = walInstance.Close() }()

	observer := &MockObserver{}
	comparator := &MockComparator{}
	decisionMaker := &MockDecisionMaker{}
	coordinator := NewMockCoordinator()
	options := ReconcilerOptions{}

	engine := NewEngine(
		observer,
		comparator,
		decisionMaker,
		coordinator,
		storage,
		walInstance,
		"test-instance",
		options,
	)

	if engine == nil {
		t.Error("NewEngine() returned nil")
		return
	}

	if engine.instanceID != "test-instance" {
		t.Errorf("NewEngine().instanceID = %v, want test-instance", engine.instanceID)
	}

	if engine.observer != observer {
		t.Error("NewEngine() observer not set correctly")
	}
}
