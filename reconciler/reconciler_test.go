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
	storage, walInstance := setupTestInfrastructure(t)
	defer func() { _ = storage.Close() }()
	defer func() { _ = walInstance.Close() }()

	tests := getReconcileTestCases()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			engine := setupTestEngine(tt, storage, walInstance)
			runReconcileTest(t, engine, tt)
		})
	}
}

// setupTestInfrastructure creates test storage and WAL
func setupTestInfrastructure(t *testing.T) (*storage.MVCCStorage, *wal.WAL) {
	tmpDir := t.TempDir()

	storage, err := storage.NewMVCCStorage(tmpDir)
	if err != nil {
		t.Fatalf("Failed to create storage: %v", err)
	}

	walInstance, err := wal.Open(tmpDir)
	if err != nil {
		t.Fatalf("Failed to create WAL: %v", err)
	}

	return storage, walInstance
}

// reconcileTestCase defines a test case for reconciliation
type reconcileTestCase struct {
	name              string
	observer          Observer
	comparator        Comparator
	decisionMaker     DecisionMaker
	config            Config
	expectedDecisions int
	shouldError       bool
}

// getReconcileTestCases returns all test cases
func getReconcileTestCases() []reconcileTestCase {
	return []reconcileTestCase{
		{
			name:              "successful reconciliation",
			observer:          createMockObserverWithResources(),
			comparator:        createMockComparatorWithDiffs(),
			decisionMaker:     createMockDecisionMakerWithDecisions(),
			config:            createTestConfig(),
			expectedDecisions: 1,
			shouldError:       false,
		},
		{
			name:              "empty reconciliation",
			observer:          &MockObserver{resources: []types.Resource{}},
			comparator:        &MockComparator{diffs: []Diff{}},
			decisionMaker:     &MockDecisionMaker{decisions: []types.Decision{}},
			config:            createEmptyTestConfig(),
			expectedDecisions: 0,
			shouldError:       false,
		},
	}
}

// createMockObserverWithResources creates a mock observer with test resources
func createMockObserverWithResources() *MockObserver {
	return &MockObserver{
		resources: []types.Resource{
			{
				ID:       "i-existing",
				Type:     "ec2",
				Provider: "aws",
				Tags:     types.Tags{ElavaManaged: true},
			},
		},
	}
}

// createMockComparatorWithDiffs creates a mock comparator with test diffs
func createMockComparatorWithDiffs() *MockComparator {
	return &MockComparator{
		diffs: []Diff{
			{
				Type:       DiffMissing,
				ResourceID: "i-missing",
				Reason:     "Test missing resource",
			},
		},
	}
}

// createMockDecisionMakerWithDecisions creates a mock decision maker with test decisions
func createMockDecisionMakerWithDecisions() *MockDecisionMaker {
	return &MockDecisionMaker{
		decisions: []types.Decision{
			{
				Action:     "create",
				ResourceID: "i-missing",
				Reason:     "Test decision",
			},
		},
	}
}

// createTestConfig creates a test configuration
func createTestConfig() Config {
	return Config{
		Provider: "aws",
		Region:   "us-east-1",
		Resources: []types.ResourceSpec{
			{
				Type:  "ec2",
				Count: 1,
				Tags:  types.Tags{ElavaManaged: true},
			},
		},
	}
}

// createEmptyTestConfig creates an empty test configuration
func createEmptyTestConfig() Config {
	return Config{
		Provider:  "aws",
		Region:    "us-east-1",
		Resources: []types.ResourceSpec{},
	}
}

// setupTestEngine creates and configures a test engine
func setupTestEngine(tt reconcileTestCase, storage *storage.MVCCStorage, walInstance *wal.WAL) *Engine {
	coordinator := NewMockCoordinator()
	options := ReconcilerOptions{
		DryRun:          false,
		MaxConcurrency:  1,
		ClaimTTL:        time.Minute,
		SkipDestructive: false,
	}

	return NewEngine(
		tt.observer,
		tt.comparator,
		tt.decisionMaker,
		coordinator,
		storage,
		walInstance,
		"test-instance",
		options,
	)
}

// runReconcileTest executes a reconcile test case
func runReconcileTest(t *testing.T, engine *Engine, tt reconcileTestCase) {
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
}

// TestEngine_BuildDesiredState tests the deprecated buildDesiredState function
//
// Deprecated: This test validates deprecated functionality.
// buildDesiredState is deprecated as part of Day 2 operations pivot.
// TODO(v2.0): Remove this test when buildDesiredState is removed.
func TestEngine_BuildDesiredState(t *testing.T) {
	t.Skip("buildDesiredState is deprecated - Elava pivoted to Day 2 operations")

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

	// Should return empty slice (deprecated behavior)
	if len(desired) != 0 {
		t.Errorf("buildDesiredState() should return empty (deprecated), got %d resources", len(desired))
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
