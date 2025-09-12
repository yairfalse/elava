package executor

import (
	"context"
	"fmt"

	"github.com/yairfalse/ovi/providers"
	"github.com/yairfalse/ovi/types"
)

// executeDecision executes a single decision against the appropriate provider
func (e *Engine) executeDecision(ctx context.Context, decision types.Decision, provider providers.CloudProvider) (string, error) {
	switch decision.Action {
	case types.ActionCreate:
		return e.executeCreate(ctx, decision, provider)
	case types.ActionUpdate:
		return e.executeUpdate(ctx, decision, provider)
	case types.ActionDelete:
		return e.executeDelete(ctx, decision, provider)
	case types.ActionTerminate:
		return e.executeTerminate(ctx, decision, provider)
	case types.ActionTag:
		return e.executeTag(ctx, decision, provider)
	case types.ActionNotify:
		return e.executeNotify(ctx, decision, provider)
	case types.ActionNoop:
		return e.executeNoop(ctx, decision, provider)
	default:
		return "", fmt.Errorf("unknown action: %s", decision.Action)
	}
}

// executeCreate creates a new resource
func (e *Engine) executeCreate(ctx context.Context, decision types.Decision, provider providers.CloudProvider) (string, error) {
	// Build resource spec from decision
	spec, err := e.buildResourceSpec(decision)
	if err != nil {
		return "", fmt.Errorf("failed to build resource spec: %w", err)
	}

	// Create the resource
	resource, err := provider.CreateResource(ctx, spec)
	if err != nil {
		return "", fmt.Errorf("failed to create resource: %w", err)
	}

	// Update storage with the new resource
	if _, err := e.storage.RecordObservation(*resource); err != nil {
		// Log warning but don't fail the execution
		fmt.Printf("Warning: failed to record new resource in storage: %v\n", err)
	}

	return resource.ID, nil
}

// executeUpdate updates an existing resource
func (e *Engine) executeUpdate(ctx context.Context, decision types.Decision, provider providers.CloudProvider) (string, error) {
	// For now, updates are handled through tagging
	// In a full implementation, this would support various update operations
	return e.executeTag(ctx, decision, provider)
}

// executeDelete deletes a resource
func (e *Engine) executeDelete(ctx context.Context, decision types.Decision, provider providers.CloudProvider) (string, error) {
	// Verify resource is not blessed
	if decision.IsBlessed {
		return "", fmt.Errorf("cannot delete blessed resource %s", decision.ResourceID)
	}

	// Delete the resource
	if err := provider.DeleteResource(ctx, decision.ResourceID); err != nil {
		return "", fmt.Errorf("failed to delete resource: %w", err)
	}

	// Record disappearance in storage
	if _, err := e.storage.RecordDisappearance(decision.ResourceID); err != nil {
		// Log warning but don't fail the execution
		fmt.Printf("Warning: failed to record resource disappearance: %v\n", err)
	}

	return decision.ResourceID, nil
}

// executeTerminate terminates a resource (similar to delete but more forceful)
func (e *Engine) executeTerminate(ctx context.Context, decision types.Decision, provider providers.CloudProvider) (string, error) {
	// Verify resource is not blessed
	if decision.IsBlessed {
		return "", fmt.Errorf("cannot terminate blessed resource %s", decision.ResourceID)
	}

	// For now, terminate is the same as delete
	// In a full implementation, this might use force-delete operations
	return e.executeDelete(ctx, decision, provider)
}

// executeTag applies tags to a resource
func (e *Engine) executeTag(ctx context.Context, decision types.Decision, provider providers.CloudProvider) (string, error) {
	// Build tags map from decision
	tags, err := e.buildTagsMap(decision)
	if err != nil {
		return "", fmt.Errorf("failed to build tags: %w", err)
	}

	// Apply tags
	if err := provider.TagResource(ctx, decision.ResourceID, tags); err != nil {
		return "", fmt.Errorf("failed to tag resource: %w", err)
	}

	return decision.ResourceID, nil
}

// executeNotify sends a notification (placeholder for future implementation)
func (e *Engine) executeNotify(ctx context.Context, decision types.Decision, provider providers.CloudProvider) (string, error) {
	// This would integrate with notification systems
	// For now, just log the notification
	fmt.Printf("NOTIFICATION: %s for resource %s - %s\n",
		decision.Action, decision.ResourceID, decision.Reason)

	return decision.ResourceID, nil
}

// executeNoop does nothing (used for testing and dry runs)
func (e *Engine) executeNoop(ctx context.Context, decision types.Decision, provider providers.CloudProvider) (string, error) {
	// No-op: just return success
	return decision.ResourceID, nil
}

// buildResourceSpec constructs a ResourceSpec from a decision
func (e *Engine) buildResourceSpec(decision types.Decision) (types.ResourceSpec, error) {
	spec := types.ResourceSpec{
		Type: decision.ResourceType,
		Tags: types.Tags{
			OviManaged: true,
			OviOwner:   "ovi",
		},
	}

	// In a full implementation, this would extract more details from the decision
	// or from associated configuration data

	return spec, nil
}

// buildTagsMap constructs a tags map from a decision
func (e *Engine) buildTagsMap(decision types.Decision) (map[string]string, error) {
	// Basic Ovi management tags
	tags := map[string]string{
		"ovi:managed": "true",
		"ovi:owner":   "ovi",
	}

	// In a full implementation, this would extract specific tags to apply
	// from the decision context or associated data

	return tags, nil
}
