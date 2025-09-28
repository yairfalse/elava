package executor

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/yairfalse/elava/providers"
	"github.com/yairfalse/elava/storage"
	"github.com/yairfalse/elava/types"
	"github.com/yairfalse/elava/wal"
)

// MockProvider for testing
type MockProvider struct {
	resources      []types.Resource
	createCalls    []types.ResourceSpec
	deleteCalls    []string
	tagCalls       []TagCall
	shouldFailNext bool
	failureError   error
	callCount      int
	failOnCall     int // Fail on the Nth call (0 = don't fail)
}

type TagCall struct {
	ResourceID string
	Tags       map[string]string
}

func (m *MockProvider) Name() string {
	return "mock"
}

func (m *MockProvider) Region() string {
	return "mock-region"
}

func (m *MockProvider) ListResources(ctx context.Context, filter types.ResourceFilter) ([]types.Resource, error) {
	if m.shouldFailNext {
		m.shouldFailNext = false
		return nil, m.failureError
	}

	var result []types.Resource
	for _, r := range m.resources {
		// Simple filter implementation
		if len(filter.IDs) > 0 {
			found := false
			for _, id := range filter.IDs {
				if r.ID == id {
					found = true
					break
				}
			}
			if !found {
				continue
			}
		}
		result = append(result, r)
	}
	return result, nil
}

func (m *MockProvider) CreateResource(ctx context.Context, spec types.ResourceSpec) (*types.Resource, error) {
	m.callCount++
	m.createCalls = append(m.createCalls, spec)

	// Check if we should fail on this specific call
	if m.failOnCall > 0 && m.callCount == m.failOnCall {
		return nil, m.failureError
	}

	if m.shouldFailNext {
		m.shouldFailNext = false
		return nil, m.failureError
	}

	resource := types.Resource{
		ID:       "new-resource-123",
		Type:     spec.Type,
		Provider: "mock",
		Status:   "running",
		Tags:     spec.Tags,
	}
	m.resources = append(m.resources, resource)
	return &resource, nil
}

func (m *MockProvider) DeleteResource(ctx context.Context, resourceID string) error {
	if m.shouldFailNext {
		m.shouldFailNext = false
		return m.failureError
	}

	m.deleteCalls = append(m.deleteCalls, resourceID)

	// Remove from resources
	for i, r := range m.resources {
		if r.ID == resourceID {
			m.resources = append(m.resources[:i], m.resources[i+1:]...)
			break
		}
	}
	return nil
}

func (m *MockProvider) TagResource(ctx context.Context, resourceID string, tags map[string]string) error {
	if m.shouldFailNext {
		m.shouldFailNext = false
		return m.failureError
	}

	m.tagCalls = append(m.tagCalls, TagCall{ResourceID: resourceID, Tags: tags})
	return nil
}

// MockConfirmer for testing
type MockConfirmer struct {
	shouldApprove bool
	calls         []ConfirmationRequest
}

func (m *MockConfirmer) RequestConfirmation(ctx context.Context, req ConfirmationRequest) (*ConfirmationResponse, error) {
	m.calls = append(m.calls, req)
	return &ConfirmationResponse{
		Approved: m.shouldApprove,
		Message:  "test response",
	}, nil
}

func TestEngine_ExecuteSingle_Create(t *testing.T) {
	tmpDir := t.TempDir()

	// Setup
	storage, _ := storage.NewMVCCStorage(tmpDir)
	defer func() { _ = storage.Close() }()

	walInstance, _ := wal.Open(tmpDir)
	defer func() { _ = walInstance.Close() }()

	mockProvider := &MockProvider{}
	providers := map[string]providers.CloudProvider{
		"aws": mockProvider,
	}

	engine := NewEngine(providers, storage, walInstance, ExecutorOptions{})

	// Create decision
	decision := types.Decision{
		Action:       types.ActionCreate,
		ResourceType: "ec2",
		Reason:       "Scaling up",
		CreatedAt:    time.Now(),
	}

	// Execute
	ctx := context.Background()
	result, err := engine.ExecuteSingle(ctx, decision)

	// Verify
	if err != nil {
		t.Fatalf("ExecuteSingle failed: %v", err)
	}

	if result.Status != StatusSuccess {
		t.Errorf("Status = %v, want %v", result.Status, StatusSuccess)
	}

	if result.ResourceID == "" {
		t.Error("ResourceID should not be empty")
	}

	if len(mockProvider.createCalls) != 1 {
		t.Errorf("Create calls = %d, want 1", len(mockProvider.createCalls))
	}
}

func TestEngine_ExecuteSingle_Delete(t *testing.T) {
	tmpDir := t.TempDir()

	// Setup
	storage, _ := storage.NewMVCCStorage(tmpDir)
	defer func() { _ = storage.Close() }()

	walInstance, _ := wal.Open(tmpDir)
	defer func() { _ = walInstance.Close() }()

	mockProvider := &MockProvider{
		resources: []types.Resource{
			{
				ID:     "resource-to-delete",
				Type:   "ec2",
				Status: "running",
				Tags: types.Tags{
					ElavaOwner: "test",
				},
			},
		},
	}
	providers := map[string]providers.CloudProvider{
		"aws": mockProvider,
	}

	engine := NewEngine(providers, storage, walInstance, ExecutorOptions{
		AllowDestructive: true,
		SkipConfirmation: true, // Skip confirmation for testing
	})

	// Delete decision
	decision := types.Decision{
		Action:       types.ActionDelete,
		ResourceID:   "resource-to-delete",
		ResourceType: "ec2",
		Reason:       "No longer needed",
		CreatedAt:    time.Now(),
	}

	// Execute
	ctx := context.Background()
	result, err := engine.ExecuteSingle(ctx, decision)

	// Verify
	if err != nil {
		t.Fatalf("ExecuteSingle failed: %v", err)
	}

	if result.Status != StatusSuccess {
		t.Errorf("Status = %v, want %v", result.Status, StatusSuccess)
		if result.Error != "" {
			t.Errorf("Error: %s", result.Error)
		}
		if result.SkipReason != "" {
			t.Errorf("SkipReason: %s", result.SkipReason)
		}
	}

	if len(mockProvider.deleteCalls) != 1 {
		t.Errorf("Delete calls = %d, want 1", len(mockProvider.deleteCalls))
	} else if mockProvider.deleteCalls[0] != "resource-to-delete" {
		t.Errorf("Deleted resource = %v, want resource-to-delete", mockProvider.deleteCalls[0])
	}
}

func TestEngine_ExecuteSingle_BlessedProtection(t *testing.T) {
	tmpDir := t.TempDir()

	// Setup
	storage, _ := storage.NewMVCCStorage(tmpDir)
	defer func() { _ = storage.Close() }()

	walInstance, _ := wal.Open(tmpDir)
	defer func() { _ = walInstance.Close() }()

	mockProvider := &MockProvider{
		resources: []types.Resource{
			{
				ID:     "blessed-resource",
				Type:   "rds",
				Status: "running",
				Tags: types.Tags{
					ElavaOwner:   "production",
					ElavaBlessed: true,
				},
			},
		},
	}
	providers := map[string]providers.CloudProvider{
		"aws": mockProvider,
	}

	engine := NewEngine(providers, storage, walInstance, ExecutorOptions{
		AllowDestructive:    true,
		AllowBlessedChanges: false, // Blessed protection enabled
	})

	// Try to delete blessed resource
	decision := types.Decision{
		Action:       types.ActionDelete,
		ResourceID:   "blessed-resource",
		ResourceType: "rds",
		Reason:       "Attempted deletion",
		IsBlessed:    true,
		CreatedAt:    time.Now(),
	}

	// Execute
	ctx := context.Background()
	result, err := engine.ExecuteSingle(ctx, decision)

	// Verify - should be skipped
	if err != nil {
		t.Fatalf("ExecuteSingle failed: %v", err)
	}

	if result.Status != StatusSkipped {
		t.Errorf("Status = %v, want %v", result.Status, StatusSkipped)
	}

	if result.SkipReason == "" {
		t.Error("SkipReason should not be empty")
	}

	if len(mockProvider.deleteCalls) != 0 {
		t.Error("Delete should not have been called for blessed resource")
	}
}

// setupTestEngine creates a test engine with mock provider
func setupTestEngine(t *testing.T, opts ExecutorOptions) (*Engine, *MockProvider, func()) {
	t.Helper()
	tmpDir := t.TempDir()

	storage, _ := storage.NewMVCCStorage(tmpDir)
	walInstance, _ := wal.Open(tmpDir)

	mockProvider := &MockProvider{
		resources: []types.Resource{
			{ID: "resource-1", Type: "ec2", Tags: types.Tags{ElavaOwner: "test"}},
			{ID: "resource-2", Type: "ec2", Tags: types.Tags{ElavaOwner: "test"}},
		},
	}
	providers := map[string]providers.CloudProvider{
		"aws": mockProvider,
	}

	engine := NewEngine(providers, storage, walInstance, opts)

	cleanup := func() {
		_ = storage.Close()
		_ = walInstance.Close()
	}

	return engine, mockProvider, cleanup
}

func TestEngine_Execute_Batch(t *testing.T) {
	engine, mockProvider, cleanup := setupTestEngine(t, ExecutorOptions{
		AllowDestructive:  true,
		ContinueOnFailure: true,
		SkipConfirmation:  true,
	})
	defer cleanup()

	// Multiple decisions
	decisions := []types.Decision{
		{
			Action:       types.ActionCreate,
			ResourceType: "ec2",
			Reason:       "New resource",
			CreatedAt:    time.Now(),
		},
		{
			Action:       types.ActionTag,
			ResourceID:   "resource-1",
			ResourceType: "ec2",
			Reason:       "Update tags",
			CreatedAt:    time.Now(),
		},
		{
			Action:       types.ActionDelete,
			ResourceID:   "resource-2",
			ResourceType: "ec2",
			Reason:       "Cleanup",
			CreatedAt:    time.Now(),
		},
	}

	// Execute batch
	ctx := context.Background()
	result, err := engine.Execute(ctx, decisions)

	// Verify
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	validateBatchResult(t, result, 3, 3, false)
	validateProviderCalls(t, mockProvider, 1, 1, 1)
}

// validateBatchResult validates batch execution results
func validateBatchResult(t *testing.T, result *ExecutionResult, total, successful int, partial bool) {
	t.Helper()

	if result.TotalDecisions != total {
		t.Errorf("TotalDecisions = %d, want %d", result.TotalDecisions, total)
	}

	if result.SuccessfulCount != successful {
		t.Errorf("SuccessfulCount = %d, want %d", result.SuccessfulCount, successful)
	}

	if result.PartialFailure != partial {
		t.Errorf("PartialFailure = %v, want %v", result.PartialFailure, partial)
	}
}

// validateProviderCalls validates provider method call counts
func validateProviderCalls(t *testing.T, p *MockProvider, creates, tags, deletes int) {
	t.Helper()

	if len(p.createCalls) != creates {
		t.Errorf("Create calls = %d, want %d", len(p.createCalls), creates)
	}

	if len(p.tagCalls) != tags {
		t.Errorf("Tag calls = %d, want %d", len(p.tagCalls), tags)
	}

	if len(p.deleteCalls) != deletes {
		t.Errorf("Delete calls = %d, want %d", len(p.deleteCalls), deletes)
	}
}

func TestEngine_Execute_PartialFailure(t *testing.T) {
	tmpDir := t.TempDir()

	// Setup
	storage, _ := storage.NewMVCCStorage(tmpDir)
	defer func() { _ = storage.Close() }()

	walInstance, _ := wal.Open(tmpDir)
	defer func() { _ = walInstance.Close() }()

	mockProvider := &MockProvider{
		failureError: errors.New("provider error"),
		failOnCall:   2, // Fail on the second call
	}
	providers := map[string]providers.CloudProvider{
		"aws": mockProvider,
	}

	engine := NewEngine(providers, storage, walInstance, ExecutorOptions{
		ContinueOnFailure: false, // Stop on first failure
	})

	// Multiple decisions
	decisions := []types.Decision{
		{
			Action:       types.ActionCreate,
			ResourceType: "ec2",
			Reason:       "Will succeed",
			CreatedAt:    time.Now(),
		},
		{
			Action:       types.ActionCreate,
			ResourceType: "ec2",
			Reason:       "Will fail",
			CreatedAt:    time.Now(),
		},
		{
			Action:       types.ActionCreate,
			ResourceType: "ec2",
			Reason:       "Won't be reached",
			CreatedAt:    time.Now(),
		},
	}

	// Execute batch
	ctx := context.Background()
	result, err := engine.Execute(ctx, decisions)

	// Verify
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	if !result.PartialFailure {
		t.Error("PartialFailure should be true")
	}

	// Should have executed 2 decisions (success + failure)
	if len(result.Results) != 2 {
		t.Errorf("Results count = %d, want 2", len(result.Results))
	}

	if result.SuccessfulCount != 1 {
		t.Errorf("SuccessfulCount = %d, want 1", result.SuccessfulCount)
	}

	if result.FailedCount != 1 {
		t.Errorf("FailedCount = %d, want 1", result.FailedCount)
	}
}

func TestEngine_DryRun(t *testing.T) {
	tmpDir := t.TempDir()

	// Setup
	storage, _ := storage.NewMVCCStorage(tmpDir)
	defer func() { _ = storage.Close() }()

	walInstance, _ := wal.Open(tmpDir)
	defer func() { _ = walInstance.Close() }()

	mockProvider := &MockProvider{}
	providers := map[string]providers.CloudProvider{
		"aws": mockProvider,
	}

	engine := NewEngine(providers, storage, walInstance, ExecutorOptions{
		DryRun:              true,
		AllowDestructive:    false, // Destructive actions disabled
		AllowBlessedChanges: false,
	})

	// Mix of decisions
	decisions := []types.Decision{
		{
			Action:       types.ActionCreate,
			ResourceType: "ec2",
			Reason:       "Safe create",
		},
		{
			Action:       types.ActionDelete,
			ResourceID:   "resource-1",
			ResourceType: "ec2",
			Reason:       "Destructive delete",
		},
		{
			Action:     types.ActionDelete,
			ResourceID: "blessed-resource",
			IsBlessed:  true,
			Reason:     "Blessed delete",
		},
	}

	// Execute dry run
	ctx := context.Background()
	result, err := engine.DryRun(ctx, decisions)

	// Verify
	if err != nil {
		t.Fatalf("DryRun failed: %v", err)
	}

	if result.TotalDecisions != 3 {
		t.Errorf("TotalDecisions = %d, want 3", result.TotalDecisions)
	}

	if result.SafeDecisions != 1 {
		t.Errorf("SafeDecisions = %d, want 1", result.SafeDecisions)
	}

	if result.DestructiveDecisions != 2 {
		t.Errorf("DestructiveDecisions = %d, want 2", result.DestructiveDecisions)
	}

	if result.BlessedDecisions != 1 {
		t.Errorf("BlessedDecisions = %d, want 1", result.BlessedDecisions)
	}

	if len(result.BlockedDecisions) != 2 {
		t.Errorf("BlockedDecisions = %d, want 2", len(result.BlockedDecisions))
	}

	// Verify no actual operations were performed
	if len(mockProvider.createCalls) != 0 {
		t.Error("No create calls should be made in dry run")
	}

	if len(mockProvider.deleteCalls) != 0 {
		t.Error("No delete calls should be made in dry run")
	}
}

func TestEngine_Confirmation(t *testing.T) {
	tmpDir := t.TempDir()

	// Setup
	storage, _ := storage.NewMVCCStorage(tmpDir)
	defer func() { _ = storage.Close() }()

	walInstance, _ := wal.Open(tmpDir)
	defer func() { _ = walInstance.Close() }()

	mockProvider := &MockProvider{
		resources: []types.Resource{
			{ID: "resource-1", Type: "ec2", Tags: types.Tags{ElavaOwner: "test"}},
		},
	}
	providers := map[string]providers.CloudProvider{
		"aws": mockProvider,
	}

	mockConfirmer := &MockConfirmer{shouldApprove: false}

	engine := NewEngine(providers, storage, walInstance, ExecutorOptions{
		AllowDestructive: true,
		SkipConfirmation: false,
	})
	engine.confirmer = mockConfirmer

	// Destructive decision requiring confirmation
	decision := types.Decision{
		Action:       types.ActionDelete,
		ResourceID:   "resource-1",
		ResourceType: "ec2",
		Reason:       "Requires confirmation",
		CreatedAt:    time.Now(),
	}

	// Execute - should be skipped due to declined confirmation
	ctx := context.Background()
	result, err := engine.ExecuteSingle(ctx, decision)

	// Verify
	if err != nil {
		t.Fatalf("ExecuteSingle failed: %v", err)
	}

	if result.Status != StatusSkipped {
		t.Errorf("Status = %v, want %v", result.Status, StatusSkipped)
	}

	if len(mockConfirmer.calls) != 1 {
		t.Errorf("Confirmation calls = %d, want 1", len(mockConfirmer.calls))
	}

	if len(mockProvider.deleteCalls) != 0 {
		t.Error("Delete should not have been called after declined confirmation")
	}

	// Now approve and try again
	mockConfirmer.shouldApprove = true
	result, err = engine.ExecuteSingle(ctx, decision)

	if err != nil {
		t.Fatalf("ExecuteSingle with approval failed: %v", err)
	}

	if result.Status != StatusSuccess {
		t.Errorf("Status = %v, want %v", result.Status, StatusSuccess)
	}

	if len(mockProvider.deleteCalls) != 1 {
		t.Error("Delete should have been called after approval")
	}
}
