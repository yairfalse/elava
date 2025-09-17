package policy

import (
	"context"
	"testing"
	"time"

	"github.com/yairfalse/elava/storage"
	"github.com/yairfalse/elava/types"
)

func TestPolicyEngine_Basic(t *testing.T) {
	// Create temp storage for testing
	tmpDir := t.TempDir()
	storage, err := storage.NewMVCCStorage(tmpDir)
	if err != nil {
		t.Fatalf("Failed to create storage: %v", err)
	}
	defer func() { _ = storage.Close() }()

	// Create policy engine
	engine := NewPolicyEngine(storage)

	// Load a simple test policy that always matches
	testPolicy := `package elava

import rego.v1

# Always allow for test
decision := "allow" if {
	input.resource.type == "ec2"
}

action := "ignore" if {
	decision == "allow"
}

reason := "test policy" if {
	decision == "allow"
}

confidence := 0.5 if {
	decision == "allow"
}

risk := "low" if {
	decision == "allow"
}`

	ctx := context.Background()
	err = engine.LoadPolicy(ctx, "test", testPolicy)
	if err != nil {
		t.Fatalf("Failed to load policy: %v", err)
	}

	// Create test resource
	resource := types.Resource{
		ID:       "i-123456",
		Type:     "ec2",
		Provider: "aws",
		Region:   "us-east-1",
		Status:   "running",
		Tags: types.Tags{
			Name:        "test-instance",
			Environment: "dev",
		},
		CreatedAt:  time.Now().Add(-24 * time.Hour),
		LastSeenAt: time.Now(),
	}

	// Build policy input
	input, err := engine.BuildPolicyInput(ctx, resource)
	if err != nil {
		t.Fatalf("Failed to build policy input: %v", err)
	}

	// Evaluate policies
	result, err := engine.Evaluate(ctx, input)
	if err != nil {
		t.Fatalf("Failed to evaluate policies: %v", err)
	}

	// Check results
	if result.Decision != "allow" {
		t.Errorf("Expected decision 'allow', got '%s'", result.Decision)
	}
	if result.Action != "ignore" {
		t.Errorf("Expected action 'ignore', got '%s'", result.Action)
	}
	if result.Confidence != 0.5 {
		t.Errorf("Expected confidence 0.5, got %f", result.Confidence)
	}
	if len(result.Policies) != 1 || result.Policies[0] != "test" {
		t.Errorf("Expected policies ['test'], got %v", result.Policies)
	}
}

func TestPolicyEngine_NoMatch(t *testing.T) {
	tmpDir := t.TempDir()
	storage, err := storage.NewMVCCStorage(tmpDir)
	if err != nil {
		t.Fatalf("Failed to create storage: %v", err)
	}
	defer func() { _ = storage.Close() }()

	engine := NewPolicyEngine(storage)

	// Create test resource
	resource := types.Resource{
		ID:     "i-123456",
		Type:   "ec2",
		Status: "running",
	}

	ctx := context.Background()
	input, err := engine.BuildPolicyInput(ctx, resource)
	if err != nil {
		t.Fatalf("Failed to build policy input: %v", err)
	}

	// Evaluate with no loaded policies
	result, err := engine.Evaluate(ctx, input)
	if err != nil {
		t.Fatalf("Failed to evaluate policies: %v", err)
	}

	// Should get default result
	if result.Decision != "allow" {
		t.Errorf("Expected default decision 'allow', got '%s'", result.Decision)
	}
	if result.Action != "ignore" {
		t.Errorf("Expected default action 'ignore', got '%s'", result.Action)
	}
}

func TestPolicyLoader_LoadPolicies(t *testing.T) {
	tmpDir := t.TempDir()
	storage, err := storage.NewMVCCStorage(tmpDir)
	if err != nil {
		t.Fatalf("Failed to create storage: %v", err)
	}
	defer func() { _ = storage.Close() }()

	engine := NewPolicyEngine(storage)
	loader := NewPolicyLoader("./policies", engine)

	ctx := context.Background()
	err = loader.LoadDefaultPolicies(ctx)
	if err != nil {
		t.Fatalf("Failed to load default policies: %v", err)
	}

	// Verify at least the default policy was loaded
	if len(engine.queries) == 0 {
		t.Error("Expected at least one policy to be loaded")
	}
}
