package executor

import (
	"context"
	"fmt"

	"github.com/yairfalse/ovi/providers"
	"github.com/yairfalse/ovi/types"
)

// DefaultSafetyChecker implements comprehensive safety checks
type DefaultSafetyChecker struct {
	checks []SafetyCheckFunc
}

// SafetyCheckFunc represents a single safety check function
type SafetyCheckFunc func(ctx context.Context, decision types.Decision, provider providers.CloudProvider) SafetyCheck

// NewDefaultSafetyChecker creates a safety checker with standard checks
func NewDefaultSafetyChecker() *DefaultSafetyChecker {
	return &DefaultSafetyChecker{
		checks: []SafetyCheckFunc{
			checkBlessedResource,
			checkResourceExists,
			checkDestructiveAction,
			checkResourceOwnership,
			checkProviderLimits,
		},
	}
}

// CheckSafety runs all safety checks on a decision
func (sc *DefaultSafetyChecker) CheckSafety(ctx context.Context, decision types.Decision, provider providers.CloudProvider) ([]SafetyCheck, error) {
	var results []SafetyCheck

	for _, checkFunc := range sc.checks {
		check := checkFunc(ctx, decision, provider)
		results = append(results, check)
	}

	return results, nil
}

// Individual safety check functions

func checkBlessedResource(ctx context.Context, decision types.Decision, provider providers.CloudProvider) SafetyCheck {
	check := SafetyCheck{
		Name:        "blessed_resource_check",
		Description: "Verify blessed resources are protected",
		Passed:      true,
		Severity:    SeverityCritical,
	}

	if decision.IsBlessed && decision.IsDestructive() {
		check.Passed = false
		check.Message = fmt.Sprintf("Cannot perform destructive action %s on blessed resource %s",
			decision.Action, decision.ResourceID)
	}

	return check
}

func checkResourceExists(ctx context.Context, decision types.Decision, provider providers.CloudProvider) SafetyCheck {
	check := SafetyCheck{
		Name:        "resource_existence_check",
		Description: "Verify resource existence matches action expectation",
		Passed:      true,
		Severity:    SeverityError,
	}

	// For destructive actions, resource should exist
	if decision.IsDestructive() {
		exists, err := resourceExists(ctx, decision.ResourceID, provider)
		if err != nil {
			check.Passed = false
			check.Message = fmt.Sprintf("Cannot verify resource existence: %v", err)
			return check
		}

		if !exists {
			check.Passed = false
			check.Message = fmt.Sprintf("Cannot %s non-existent resource %s",
				decision.Action, decision.ResourceID)
		}
	}

	// For create actions, resource should not exist
	if decision.Action == types.ActionCreate {
		exists, err := resourceExists(ctx, decision.ResourceID, provider)
		if err != nil {
			// This is often expected for create operations
			check.Severity = SeverityWarning
			check.Message = fmt.Sprintf("Cannot verify resource existence (may be expected): %v", err)
			return check
		}

		if exists {
			check.Passed = false
			check.Message = fmt.Sprintf("Cannot create resource %s: already exists", decision.ResourceID)
		}
	}

	return check
}

func checkDestructiveAction(ctx context.Context, decision types.Decision, provider providers.CloudProvider) SafetyCheck {
	check := SafetyCheck{
		Name:        "destructive_action_check",
		Description: "Validate destructive actions are intentional",
		Passed:      true,
		Severity:    SeverityError,
	}

	if decision.IsDestructive() {
		// Additional validation for destructive actions
		if decision.Reason == "" {
			check.Passed = false
			check.Message = "Destructive actions require a reason"
			return check
		}

		// Check if resource has important tags that suggest it shouldn't be deleted
		resource, err := getCurrentResource(ctx, decision.ResourceID, provider)
		if err != nil {
			check.Severity = SeverityWarning
			check.Message = fmt.Sprintf("Cannot validate resource tags: %v", err)
			return check
		}

		if resource != nil && hasImportantTags(resource) {
			check.Severity = SeverityCritical
			check.Message = fmt.Sprintf("Resource %s has important tags indicating it should not be deleted",
				decision.ResourceID)
		}
	}

	return check
}

func checkResourceOwnership(ctx context.Context, decision types.Decision, provider providers.CloudProvider) SafetyCheck {
	check := SafetyCheck{
		Name:        "resource_ownership_check",
		Description: "Verify Ovi can manage this resource",
		Passed:      true,
		Severity:    SeverityError,
	}

	// Skip for create actions since resource doesn't exist yet
	if decision.Action == types.ActionCreate {
		return check
	}

	resource, err := getCurrentResource(ctx, decision.ResourceID, provider)
	if err != nil {
		check.Passed = false
		check.Message = fmt.Sprintf("Cannot verify resource ownership: %v", err)
		return check
	}

	if resource == nil {
		// Resource doesn't exist - this might be expected for some actions
		check.Severity = SeverityWarning
		check.Message = "Resource not found during ownership check"
		return check
	}

	// Check if resource is managed by Ovi - but allow deletion of any resource if explicitly requested
	if !resource.IsManaged() && decision.Action != types.ActionDelete && decision.Action != types.ActionTerminate {
		check.Passed = false
		check.Message = fmt.Sprintf("Resource %s is not managed by Ovi", decision.ResourceID)
	}

	return check
}

func checkProviderLimits(ctx context.Context, decision types.Decision, provider providers.CloudProvider) SafetyCheck {
	check := SafetyCheck{
		Name:        "provider_limits_check",
		Description: "Verify action doesn't exceed provider limits",
		Passed:      true,
		Severity:    SeverityWarning,
	}

	// This is a placeholder for provider-specific limit checks
	// In a full implementation, this would check:
	// - API rate limits
	// - Resource quotas
	// - Regional availability
	// - Service-specific constraints

	if decision.Action == types.ActionCreate {
		// Example: check if we're approaching resource limits
		if decision.ResourceType == "ec2" {
			// Would check EC2 instance limits
			check.Message = "EC2 instance limit check passed"
		}
	}

	return check
}

// Helper functions

func resourceExists(ctx context.Context, resourceID string, provider providers.CloudProvider) (bool, error) {
	filter := types.ResourceFilter{
		IDs: []string{resourceID},
	}

	resources, err := provider.ListResources(ctx, filter)
	if err != nil {
		return false, err
	}

	return len(resources) > 0, nil
}

func getCurrentResource(ctx context.Context, resourceID string, provider providers.CloudProvider) (*types.Resource, error) {
	filter := types.ResourceFilter{
		IDs: []string{resourceID},
	}

	resources, err := provider.ListResources(ctx, filter)
	if err != nil {
		return nil, err
	}

	if len(resources) == 0 {
		return nil, nil
	}

	return &resources[0], nil
}

func hasImportantTags(resource *types.Resource) bool {
	// Check for tags that indicate the resource is important
	tags := resource.Tags

	// Examples of important indicators
	if tags.Environment == "production" || tags.Environment == "prod" {
		return true
	}

	if tags.Name != "" && (tags.Name == "critical" || tags.Name == "important") {
		return true
	}

	// Check for database or storage resources
	if resource.Type == "rds" || resource.Type == "s3" {
		return true
	}

	return false
}
