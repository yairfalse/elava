package executor

import (
	"context"
	"testing"

	"github.com/yairfalse/elava/types"
)

func TestDefaultSafetyChecker_CheckBlessedResource(t *testing.T) {
	checker := NewDefaultSafetyChecker()
	ctx := context.Background()
	mockProvider := &MockProvider{
		resources: []types.Resource{
			{
				ID:   "blessed-db",
				Type: "rds",
				Tags: types.Tags{
					ElavaBlessed: true,
					ElavaOwner:   "production",
				},
			},
		},
	}

	tests := []struct {
		name     string
		decision types.Decision
		wantPass bool
		severity BlockSeverity
	}{
		{
			name: "blessed resource with destructive action",
			decision: types.Decision{
				Action:     types.ActionDelete,
				ResourceID: "blessed-db",
				IsBlessed:  true,
			},
			wantPass: false,
			severity: SeverityCritical,
		},
		{
			name: "blessed resource with safe action",
			decision: types.Decision{
				Action:     types.ActionTag,
				ResourceID: "blessed-db",
				IsBlessed:  true,
			},
			wantPass: true,
			severity: SeverityCritical,
		},
		{
			name: "non-blessed resource with destructive action",
			decision: types.Decision{
				Action:     types.ActionDelete,
				ResourceID: "regular-resource",
				IsBlessed:  false,
			},
			wantPass: true,
			severity: SeverityCritical,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			checks, err := checker.CheckSafety(ctx, tt.decision, mockProvider)
			if err != nil {
				t.Fatalf("CheckSafety() error = %v", err)
			}

			// Find the blessed resource check
			var blessedCheck *SafetyCheck
			for _, check := range checks {
				if check.Name == "blessed_resource_check" {
					blessedCheck = &check
					break
				}
			}

			if blessedCheck == nil {
				t.Fatal("blessed_resource_check not found")
			}

			if blessedCheck.Passed != tt.wantPass {
				t.Errorf("Check passed = %v, want %v", blessedCheck.Passed, tt.wantPass)
			}

			if blessedCheck.Severity != tt.severity {
				t.Errorf("Check severity = %v, want %v", blessedCheck.Severity, tt.severity)
			}
		})
	}
}

func TestDefaultSafetyChecker_CheckResourceExists(t *testing.T) {
	checker := NewDefaultSafetyChecker()
	ctx := context.Background()
	mockProvider := &MockProvider{
		resources: []types.Resource{
			{
				ID:   "existing-resource",
				Type: "ec2",
				Tags: types.Tags{
					ElavaOwner: "test",
				},
			},
		},
	}

	tests := []struct {
		name     string
		decision types.Decision
		wantPass bool
		severity BlockSeverity
	}{
		{
			name: "delete existing resource",
			decision: types.Decision{
				Action:     types.ActionDelete,
				ResourceID: "existing-resource",
			},
			wantPass: true,
			severity: SeverityError,
		},
		{
			name: "delete non-existent resource",
			decision: types.Decision{
				Action:     types.ActionDelete,
				ResourceID: "non-existent",
			},
			wantPass: false,
			severity: SeverityError,
		},
		{
			name: "create new resource",
			decision: types.Decision{
				Action:     types.ActionCreate,
				ResourceID: "new-resource",
			},
			wantPass: true,
			severity: SeverityWarning, // Create checks are less severe
		},
		{
			name: "create existing resource",
			decision: types.Decision{
				Action:     types.ActionCreate,
				ResourceID: "existing-resource",
			},
			wantPass: false,
			severity: SeverityError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			checks, err := checker.CheckSafety(ctx, tt.decision, mockProvider)
			if err != nil {
				t.Fatalf("CheckSafety() error = %v", err)
			}

			// Find the existence check
			var existCheck *SafetyCheck
			for _, check := range checks {
				if check.Name == "resource_existence_check" {
					existCheck = &check
					break
				}
			}

			if existCheck == nil {
				t.Fatal("resource_existence_check not found")
			}

			if existCheck.Passed != tt.wantPass {
				t.Errorf("Check passed = %v, want %v", existCheck.Passed, tt.wantPass)
			}
		})
	}
}

//nolint:gocognit // Test function with multiple test cases
func TestDefaultSafetyChecker_CheckDestructiveAction(t *testing.T) {
	checker := NewDefaultSafetyChecker()
	ctx := context.Background()
	mockProvider := &MockProvider{
		resources: []types.Resource{
			{
				ID:   "prod-db",
				Type: "rds",
				Tags: types.Tags{
					ElavaOwner:  "production",
					Environment: "production",
				},
			},
			{
				ID:   "dev-server",
				Type: "ec2",
				Tags: types.Tags{
					ElavaOwner:  "development",
					Environment: "dev",
				},
			},
		},
	}

	tests := []struct {
		name     string
		decision types.Decision
		wantPass bool
		message  string
	}{
		{
			name: "destructive action with reason",
			decision: types.Decision{
				Action:     types.ActionDelete,
				ResourceID: "dev-server",
				Reason:     "No longer needed",
			},
			wantPass: true,
		},
		{
			name: "destructive action without reason",
			decision: types.Decision{
				Action:     types.ActionDelete,
				ResourceID: "dev-server",
				Reason:     "",
			},
			wantPass: false,
			message:  "Destructive actions require a reason",
		},
		{
			name: "destructive action on production resource",
			decision: types.Decision{
				Action:     types.ActionDelete,
				ResourceID: "prod-db",
				Reason:     "Cleanup",
			},
			wantPass: true, // Pass but with critical warning
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			checks, err := checker.CheckSafety(ctx, tt.decision, mockProvider)
			if err != nil {
				t.Fatalf("CheckSafety() error = %v", err)
			}

			// Find the destructive action check
			var destructiveCheck *SafetyCheck
			for _, check := range checks {
				if check.Name == "destructive_action_check" {
					destructiveCheck = &check
					break
				}
			}

			if destructiveCheck == nil {
				t.Fatal("destructive_action_check not found")
			}

			if destructiveCheck.Passed != tt.wantPass {
				t.Errorf("Check passed = %v, want %v", destructiveCheck.Passed, tt.wantPass)
			}

			if tt.message != "" && destructiveCheck.Message != tt.message {
				t.Errorf("Check message = %v, want %v", destructiveCheck.Message, tt.message)
			}
		})
	}
}

func TestDefaultSafetyChecker_CheckResourceOwnership(t *testing.T) {
	checker := NewDefaultSafetyChecker()
	ctx := context.Background()
	mockProvider := &MockProvider{
		resources: []types.Resource{
			{
				ID:   "managed-resource",
				Type: "ec2",
				Tags: types.Tags{
					ElavaOwner:   "team-web",
					ElavaManaged: true,
				},
			},
			{
				ID:   "unmanaged-resource",
				Type: "ec2",
				Tags: types.Tags{
					Name: "external-server",
				},
			},
		},
	}

	tests := []struct {
		name     string
		decision types.Decision
		wantPass bool
	}{
		{
			name: "action on managed resource",
			decision: types.Decision{
				Action:     types.ActionUpdate,
				ResourceID: "managed-resource",
			},
			wantPass: true,
		},
		{
			name: "action on unmanaged resource",
			decision: types.Decision{
				Action:     types.ActionUpdate,
				ResourceID: "unmanaged-resource",
			},
			wantPass: false,
		},
		{
			name: "create action (skip ownership check)",
			decision: types.Decision{
				Action:     types.ActionCreate,
				ResourceID: "new-resource",
			},
			wantPass: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			checks, err := checker.CheckSafety(ctx, tt.decision, mockProvider)
			if err != nil {
				t.Fatalf("CheckSafety() error = %v", err)
			}

			// Find the ownership check
			var ownershipCheck *SafetyCheck
			for _, check := range checks {
				if check.Name == "resource_ownership_check" {
					ownershipCheck = &check
					break
				}
			}

			if ownershipCheck == nil {
				t.Fatal("resource_ownership_check not found")
			}

			if ownershipCheck.Passed != tt.wantPass {
				t.Errorf("Check passed = %v, want %v", ownershipCheck.Passed, tt.wantPass)
			}
		})
	}
}

func TestDefaultSafetyChecker_AllChecks(t *testing.T) {
	checker := NewDefaultSafetyChecker()
	ctx := context.Background()
	mockProvider := &MockProvider{
		resources: []types.Resource{
			{
				ID:   "test-resource",
				Type: "ec2",
				Tags: types.Tags{
					ElavaOwner:   "test",
					ElavaManaged: true,
				},
			},
		},
	}

	// Normal update decision - all checks should pass
	decision := types.Decision{
		Action:     types.ActionUpdate,
		ResourceID: "test-resource",
		Reason:     "Update configuration",
	}

	checks, err := checker.CheckSafety(ctx, decision, mockProvider)
	if err != nil {
		t.Fatalf("CheckSafety() error = %v", err)
	}

	// Should have all 5 checks
	expectedChecks := []string{
		"blessed_resource_check",
		"resource_existence_check",
		"destructive_action_check",
		"resource_ownership_check",
		"provider_limits_check",
	}

	if len(checks) != len(expectedChecks) {
		t.Errorf("Number of checks = %d, want %d", len(checks), len(expectedChecks))
	}

	// Verify all expected checks are present
	checkMap := make(map[string]bool)
	for _, check := range checks {
		checkMap[check.Name] = true
	}

	for _, expected := range expectedChecks {
		if !checkMap[expected] {
			t.Errorf("Missing expected check: %s", expected)
		}
	}

	// All checks should pass for this normal update
	for _, check := range checks {
		if !check.Passed {
			t.Errorf("Check %s failed unexpectedly: %s", check.Name, check.Message)
		}
	}
}

func TestHasImportantTags(t *testing.T) {
	tests := []struct {
		name     string
		resource types.Resource
		want     bool
	}{
		{
			name: "production environment",
			resource: types.Resource{
				Type: "ec2",
				Tags: types.Tags{
					Environment: "production",
				},
			},
			want: true,
		},
		{
			name: "prod environment",
			resource: types.Resource{
				Type: "ec2",
				Tags: types.Tags{
					Environment: "prod",
				},
			},
			want: true,
		},
		{
			name: "critical name",
			resource: types.Resource{
				Type: "ec2",
				Tags: types.Tags{
					Name: "critical",
				},
			},
			want: true,
		},
		{
			name: "important name",
			resource: types.Resource{
				Type: "ec2",
				Tags: types.Tags{
					Name: "important",
				},
			},
			want: true,
		},
		{
			name: "RDS resource",
			resource: types.Resource{
				Type: "rds",
				Tags: types.Tags{
					Environment: "dev",
				},
			},
			want: true,
		},
		{
			name: "S3 resource",
			resource: types.Resource{
				Type: "s3",
				Tags: types.Tags{
					Environment: "test",
				},
			},
			want: true,
		},
		{
			name: "dev environment EC2",
			resource: types.Resource{
				Type: "ec2",
				Tags: types.Tags{
					Environment: "dev",
					Name:        "test-server",
				},
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := hasImportantTags(&tt.resource); got != tt.want {
				t.Errorf("hasImportantTags() = %v, want %v", got, tt.want)
			}
		})
	}
}
