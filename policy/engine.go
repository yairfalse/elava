package policy

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/open-policy-agent/opa/v1/rego"
	"github.com/yairfalse/elava/storage"
	"github.com/yairfalse/elava/telemetry"
	"github.com/yairfalse/elava/types"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

// PolicyEngine evaluates OPA policies against resources with MVCC context
type PolicyEngine struct {
	storage *storage.MVCCStorage
	logger  *telemetry.Logger
	tracer  trace.Tracer
	queries map[string]rego.PreparedEvalQuery
}

// PolicyInput represents the input data for policy evaluation
type PolicyInput struct {
	Resource    types.Resource   `json:"resource"`
	History     []types.Resource `json:"history,omitempty"`
	Context     PolicyContext    `json:"context"`
	Environment string           `json:"environment"`
	Timestamp   time.Time        `json:"timestamp"`
}

// PolicyContext provides rich context for policy decisions
type PolicyContext struct {
	Account          string           `json:"account"`
	Region           string           `json:"region"`
	Environment      string           `json:"environment"`
	TeamPolicies     []string         `json:"team_policies"`
	ResourceAge      int              `json:"resource_age_days"`
	LastSeenDays     int              `json:"last_seen_days"`
	ChangeFrequency  int              `json:"change_frequency"`
	RelatedResources []types.Resource `json:"related_resources,omitempty"`
}

// PolicyResult contains the result of policy evaluation
type PolicyResult struct {
	Decision   string         `json:"decision"` // "allow", "deny", "require_approval", "flag"
	Action     string         `json:"action"`   // "delete", "tag", "notify", "ignore"
	Reason     string         `json:"reason"`
	Confidence float64        `json:"confidence"`
	Policies   []string       `json:"policies"` // Which policies matched
	Risk       string         `json:"risk"`     // "high", "medium", "low"
	Metadata   map[string]any `json:"metadata"`
}

// NewPolicyEngine creates a new policy engine
func NewPolicyEngine(storage *storage.MVCCStorage) *PolicyEngine {
	return &PolicyEngine{
		storage: storage,
		logger:  telemetry.NewLogger("policy-engine"),
		tracer:  otel.Tracer("policy-engine"),
		queries: make(map[string]rego.PreparedEvalQuery),
	}
}

// LoadPolicy loads and compiles a Rego policy
func (pe *PolicyEngine) LoadPolicy(ctx context.Context, name string, regoCode string) error {
	ctx, span := pe.tracer.Start(ctx, "policy_engine.load_policy",
		trace.WithAttributes(attribute.String("policy.name", name)))
	defer span.End()

	pe.logger.WithContext(ctx).Info().
		Str("policy_name", name).
		Msg("loading policy")

	// Compile the Rego query
	query := rego.New(
		rego.Query("data.elava"), // Query root namespace
		rego.Module(name, regoCode),
	)

	prepared, err := query.PrepareForEval(ctx)
	if err != nil {
		pe.logger.LogStorageError(ctx, "compile_policy", err)
		return fmt.Errorf("failed to compile policy %s: %w", name, err)
	}

	pe.queries[name] = prepared

	pe.logger.WithContext(ctx).Info().
		Str("policy_name", name).
		Msg("policy loaded successfully")

	return nil
}

// BuildPolicyInput creates policy input with rich context from MVCC storage
func (pe *PolicyEngine) BuildPolicyInput(ctx context.Context, resource types.Resource) (PolicyInput, error) {
	ctx, span := pe.tracer.Start(ctx, "policy_engine.build_input",
		trace.WithAttributes(attribute.String("resource.id", resource.ID)))
	defer span.End()

	// Get resource history from MVCC storage
	history, err := pe.getResourceHistory(ctx, resource.ID, 30) // 30 days
	if err != nil {
		pe.logger.WithContext(ctx).Warn().
			Err(err).
			Str("resource_id", resource.ID).
			Msg("failed to get resource history")
		// Continue without history
		history = []types.Resource{}
	}

	// Build context
	policyContext, err := pe.buildPolicyContext(ctx, resource, history)
	if err != nil {
		return PolicyInput{}, fmt.Errorf("failed to build policy context: %w", err)
	}

	input := PolicyInput{
		Resource:    resource,
		History:     history,
		Context:     policyContext,
		Environment: detectEnvironment(resource),
		Timestamp:   time.Now(),
	}

	pe.logger.WithContext(ctx).Debug().
		Str("resource_id", resource.ID).
		Str("environment", input.Environment).
		Int("history_entries", len(history)).
		Msg("built policy input")

	return input, nil
}

// Evaluate runs all loaded policies against the input
func (pe *PolicyEngine) Evaluate(ctx context.Context, input PolicyInput) (PolicyResult, error) {
	ctx, span := pe.tracer.Start(ctx, "policy_engine.evaluate",
		trace.WithAttributes(
			attribute.String("resource.id", input.Resource.ID),
			attribute.String("resource.type", input.Resource.Type)))
	defer span.End()

	pe.logger.WithContext(ctx).Info().
		Str("resource_id", input.Resource.ID).
		Str("resource_type", input.Resource.Type).
		Str("environment", input.Environment).
		Int("loaded_policies", len(pe.queries)).
		Msg("evaluating policies")

	var allResults []PolicyResult
	matchedPolicies := []string{}

	// Evaluate each loaded policy
	for policyName, query := range pe.queries {
		result, err := pe.evaluatePolicy(ctx, policyName, query, input)
		if err != nil {
			pe.logger.WithContext(ctx).Error().
				Err(err).
				Str("policy_name", policyName).
				Msg("policy evaluation failed")
			continue
		}

		if result.Decision != "" {
			allResults = append(allResults, result)
			matchedPolicies = append(matchedPolicies, policyName)
		}
	}

	// Aggregate results into final decision
	finalResult := pe.aggregateResults(allResults)
	finalResult.Policies = matchedPolicies

	pe.logger.WithContext(ctx).Info().
		Str("resource_id", input.Resource.ID).
		Str("final_decision", finalResult.Decision).
		Str("final_action", finalResult.Action).
		Str("risk", finalResult.Risk).
		Strs("matched_policies", matchedPolicies).
		Float64("confidence", finalResult.Confidence).
		Msg("policy evaluation complete")

	return finalResult, nil
}

// evaluatePolicy evaluates a single policy
func (pe *PolicyEngine) evaluatePolicy(ctx context.Context, name string, query rego.PreparedEvalQuery, input PolicyInput) (PolicyResult, error) {
	results, err := query.Eval(ctx, rego.EvalInput(input))
	if err != nil {
		return PolicyResult{}, fmt.Errorf("evaluation failed: %w", err)
	}

	if len(results) == 0 {
		return PolicyResult{}, nil // No match
	}

	result := PolicyResult{
		Policies: []string{name},
		Metadata: make(map[string]any),
	}

	pe.parseEvalResults(results, &result)
	return result, nil
}

func (pe *PolicyEngine) parseEvalResults(results rego.ResultSet, result *PolicyResult) {
	for _, res := range results {
		for key, value := range res.Bindings {
			pe.bindPolicyValue(key, value, result)
		}
	}
}

func (pe *PolicyEngine) bindPolicyValue(key string, value interface{}, result *PolicyResult) {
	switch key {
	case "decision":
		if str, ok := value.(string); ok {
			result.Decision = str
		}
	case "action":
		if str, ok := value.(string); ok {
			result.Action = str
		}
	case "reason":
		if str, ok := value.(string); ok {
			result.Reason = str
		}
	case "risk":
		if str, ok := value.(string); ok {
			result.Risk = str
		}
	case "confidence":
		if num, ok := value.(float64); ok {
			result.Confidence = num
		}
	default:
		result.Metadata[key] = value
	}
}

// aggregateResults combines multiple policy results into a final decision
func (pe *PolicyEngine) aggregateResults(results []PolicyResult) PolicyResult {
	if len(results) == 0 {
		return PolicyResult{
			Decision:   "allow",
			Action:     "ignore",
			Risk:       "low",
			Confidence: 1.0,
			Reason:     "no policies matched",
		}
	}

	// Simple aggregation - in practice this would be more sophisticated
	finalResult := PolicyResult{
		Decision:   "allow",
		Action:     "ignore",
		Risk:       "low",
		Confidence: 0.0,
		Metadata:   make(map[string]any),
	}

	// Find highest priority decision
	priorityOrder := map[string]int{
		"deny":             4,
		"require_approval": 3,
		"flag":             2,
		"allow":            1,
	}

	riskOrder := map[string]int{
		"high":   3,
		"medium": 2,
		"low":    1,
	}

	maxPriority := 0
	maxRisk := 0
	var reasons []string

	for _, result := range results {
		if priority := priorityOrder[result.Decision]; priority > maxPriority {
			maxPriority = priority
			finalResult.Decision = result.Decision
			finalResult.Action = result.Action
		}

		if risk := riskOrder[result.Risk]; risk > maxRisk {
			maxRisk = risk
			finalResult.Risk = result.Risk
		}

		if result.Confidence > finalResult.Confidence {
			finalResult.Confidence = result.Confidence
		}

		if result.Reason != "" {
			reasons = append(reasons, result.Reason)
		}
	}

	if len(reasons) > 0 {
		finalResult.Reason = fmt.Sprintf("Multiple policies: %v", reasons)
	}

	return finalResult
}

// Helper functions

func (pe *PolicyEngine) getResourceHistory(ctx context.Context, resourceID string, days int) ([]types.Resource, error) {
	// This would query MVCC storage for resource history
	// For now, return empty slice
	return []types.Resource{}, nil
}

func (pe *PolicyEngine) buildPolicyContext(ctx context.Context, resource types.Resource, history []types.Resource) (PolicyContext, error) {
	context := PolicyContext{
		Account:      resource.AccountID,
		Region:       resource.Region,
		Environment:  detectEnvironment(resource),
		TeamPolicies: []string{}, // Would be loaded from config
	}

	// Calculate resource age
	if !resource.CreatedAt.IsZero() {
		context.ResourceAge = int(time.Since(resource.CreatedAt).Hours() / 24)
	}

	// Calculate last seen days
	if !resource.LastSeenAt.IsZero() {
		context.LastSeenDays = int(time.Since(resource.LastSeenAt).Hours() / 24)
	}

	// Calculate change frequency from history
	context.ChangeFrequency = len(history)

	return context, nil
}

func detectEnvironment(resource types.Resource) string {
	// Simple environment detection based on tags
	if env := resource.Tags.Environment; env != "" {
		return env
	}

	// Fallback detection based on resource patterns
	if resource.Tags.Name != "" {
		name := resource.Tags.Name
		if contains(name, "prod") {
			return "prod"
		}
		if contains(name, "stag") {
			return "staging"
		}
		if contains(name, "dev") || contains(name, "test") {
			return "dev"
		}
	}

	return "unknown"
}

func contains(s, substr string) bool {
	return strings.Contains(s, substr)
}
