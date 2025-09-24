package policy

import (
	"context"
	"fmt"

	"github.com/yairfalse/elava/telemetry"
	"github.com/yairfalse/elava/types"
)

// Enforcer executes policy decisions
type Enforcer struct {
	logger *telemetry.Logger
}

// NewEnforcer creates a new enforcer
func NewEnforcer() *Enforcer {
	return &Enforcer{
		logger: telemetry.NewLogger("policy-enforcer"),
	}
}

// Execute enforces a policy decision on a resource
func (e *Enforcer) Execute(ctx context.Context, decision PolicyResult, resource types.Resource) error {
	e.logger.WithContext(ctx).Info().
		Str("resource_id", resource.ID).
		Str("resource_type", resource.Type).
		Str("action", decision.Action).
		Str("reason", decision.Reason).
		Msg("executing policy enforcement")

	switch decision.Action {
	case "ignore":
		return nil
	case "notify":
		return e.notify(ctx, resource, decision.Reason)
	case "flag":
		return e.flag(ctx, resource, decision)
	default:
		e.logger.WithContext(ctx).Warn().
			Str("action", decision.Action).
			Msg("unknown enforcement action")
		return nil
	}
}

func (e *Enforcer) notify(ctx context.Context, resource types.Resource, reason string) error {
	message := fmt.Sprintf("Policy Alert: Resource %s (%s) - %s", resource.ID, resource.Type, reason)

	e.logger.WithContext(ctx).Info().
		Str("notification", message).
		Msg("notification sent")

	// TODO: Future - send to Slack/Discord
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

	// TODO: Future - call provider.TagResource
	return nil
}
