package policy

import (
	"context"
	"fmt"
	"time"

	"github.com/yairfalse/elava/providers"
	"github.com/yairfalse/elava/storage"
	"github.com/yairfalse/elava/telemetry"
	"github.com/yairfalse/elava/types"
)

// Enforcer executes policy decisions
type Enforcer struct {
	logger   *telemetry.Logger
	provider providers.CloudProvider
	storage  *storage.MVCCStorage
}

// NewEnforcer creates a new enforcer without provider (dry-run mode)
func NewEnforcer() *Enforcer {
	return &Enforcer{
		logger: telemetry.NewLogger("policy-enforcer"),
	}
}

// NewEnforcerWithProvider creates an enforcer that can tag resources
func NewEnforcerWithProvider(provider providers.CloudProvider) *Enforcer {
	return &Enforcer{
		logger:   telemetry.NewLogger("policy-enforcer"),
		provider: provider,
	}
}

// NewEnforcerWithStorage creates an enforcer that stores events
func NewEnforcerWithStorage(storage *storage.MVCCStorage) *Enforcer {
	return &Enforcer{
		logger:  telemetry.NewLogger("policy-enforcer"),
		storage: storage,
	}
}

// NewEnforcerWithStorageAndProvider creates a full enforcer
func NewEnforcerWithStorageAndProvider(storage *storage.MVCCStorage, provider providers.CloudProvider) *Enforcer {
	return &Enforcer{
		logger:   telemetry.NewLogger("policy-enforcer"),
		storage:  storage,
		provider: provider,
	}
}

// Execute enforces a policy decision on a resource
func (e *Enforcer) Execute(ctx context.Context, decision PolicyResult, resource types.Resource) error {
	e.logEnforcementStart(ctx, decision, resource)

	event := e.createEnforcementEvent(decision, resource)
	err := e.executeAction(ctx, decision, resource, &event)

	e.handleEnforcementResult(&event, err)
	e.storeEnforcementAsync(event)

	return err
}

// logEnforcementStart logs the start of enforcement
func (e *Enforcer) logEnforcementStart(ctx context.Context, decision PolicyResult, resource types.Resource) {
	e.logger.WithContext(ctx).Info().
		Str("resource_id", resource.ID).
		Str("resource_type", resource.Type).
		Str("action", decision.Action).
		Str("reason", decision.Reason).
		Msg("executing policy enforcement")
}

// createEnforcementEvent creates an enforcement event
func (e *Enforcer) createEnforcementEvent(decision PolicyResult, resource types.Resource) types.EnforcementEvent {
	return types.EnforcementEvent{
		Timestamp:    time.Now(),
		ResourceID:   resource.ID,
		ResourceType: resource.Type,
		Provider:     resource.Provider,
		Action:       decision.Action,
		Decision:     decision.Decision,
		Reason:       decision.Reason,
		Success:      true,
	}
}

// executeAction executes the enforcement action
func (e *Enforcer) executeAction(ctx context.Context, decision PolicyResult, resource types.Resource, event *types.EnforcementEvent) error {
	switch decision.Action {
	case "ignore":
		return nil
	case "notify":
		return e.notify(ctx, resource, decision.Reason)
	case "flag":
		tags := map[string]string{
			"elava:policy-flag":   decision.Decision,
			"elava:policy-reason": decision.Reason,
		}
		event.Tags = tags
		return e.flag(ctx, resource, decision)
	default:
		e.logger.WithContext(ctx).Warn().
			Str("action", decision.Action).
			Msg("unknown enforcement action")
		return nil
	}
}

// handleEnforcementResult updates event based on result
func (e *Enforcer) handleEnforcementResult(event *types.EnforcementEvent, err error) {
	if err != nil {
		event.Success = false
		event.Error = err.Error()
	}
}

// storeEnforcementAsync stores event asynchronously
func (e *Enforcer) storeEnforcementAsync(event types.EnforcementEvent) {
	if e.storage != nil {
		go func() {
			if storeErr := e.storage.StoreEnforcement(context.Background(), event); storeErr != nil {
				e.logger.Error().
					Err(storeErr).
					Str("resource_id", event.ResourceID).
					Msg("failed to store enforcement event")
			}
		}()
	}
}

func (e *Enforcer) notify(ctx context.Context, resource types.Resource, reason string) error {
	message := fmt.Sprintf("Policy Alert: Resource %s (%s) - %s", resource.ID, resource.Type, reason)

	e.logger.WithContext(ctx).Info().
		Str("notification", message).
		Msg("notification sent")

	// Print notification to stdout for now
	fmt.Println(message)
	return nil
}

func (e *Enforcer) flag(ctx context.Context, resource types.Resource, decision PolicyResult) error {
	tags := map[string]string{
		"elava:policy-flag":   decision.Decision,
		"elava:policy-reason": decision.Reason,
	}

	e.logger.WithContext(ctx).Info().
		Str("resource_id", resource.ID).
		Interface("tags", tags).
		Msg("flagging resource with policy tags")

	// Apply tags if provider is configured
	if e.provider != nil {
		if err := e.provider.TagResource(ctx, resource.ID, tags); err != nil {
			e.logger.WithContext(ctx).Error().
				Err(err).
				Str("resource_id", resource.ID).
				Msg("failed to tag resource")
			return fmt.Errorf("failed to tag resource %s: %w", resource.ID, err)
		}
		e.logger.WithContext(ctx).Info().
			Str("resource_id", resource.ID).
			Msg("successfully tagged resource")
	} else {
		e.logger.WithContext(ctx).Debug().
			Msg("dry-run mode: tags not applied")
	}

	return nil
}
