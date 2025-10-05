package orchestrator

import (
	"context"
	"fmt"
	"time"

	"github.com/yairfalse/elava/policy"
	"github.com/yairfalse/elava/storage"
	"github.com/yairfalse/elava/telemetry"
	"github.com/yairfalse/elava/types"
)

// Orchestrator coordinates scan → policy → enforce flow
type Orchestrator struct {
	storage      *storage.MVCCStorage
	scanner      Scanner
	policyEngine *policy.PolicyEngine
	enforcer     *policy.Enforcer
	logger       *telemetry.Logger
}

// NewOrchestrator creates a new orchestrator
func NewOrchestrator(storage *storage.MVCCStorage) *Orchestrator {
	return &Orchestrator{
		storage:      storage,
		policyEngine: policy.NewPolicyEngine(storage),
		enforcer:     policy.NewEnforcerWithStorage(storage),
		logger:       telemetry.NewLogger("orchestrator"),
	}
}

// WithScanner sets the scanner
func (o *Orchestrator) WithScanner(s Scanner) *Orchestrator {
	o.scanner = s
	return o
}

// PolicyEngine returns the policy engine for loading policies
func (o *Orchestrator) PolicyEngine() *policy.PolicyEngine {
	return o.policyEngine
}

// RunCycle runs one reconciliation cycle
func (o *Orchestrator) RunCycle(ctx context.Context) (*CycleResult, error) {
	result := &CycleResult{
		StartTime: time.Now(),
		Success:   true,
	}

	o.logger.WithContext(ctx).Info().
		Msg("starting reconciliation cycle")

	// 1. Scan resources
	resources, err := o.scanResources(ctx)
	if err != nil {
		result.Errors = append(result.Errors, fmt.Sprintf("scan failed: %v", err))
		result.Success = false
		return o.finishCycle(result), err
	}
	result.ResourcesScanned = len(resources)

	// 2. Store resources
	if err := o.storeResources(ctx, resources); err != nil {
		result.Errors = append(result.Errors, fmt.Sprintf("storage failed: %v", err))
		// Continue anyway
	}

	// 3. Evaluate and enforce policies
	for _, resource := range resources {
		if err := o.processResource(ctx, resource, result); err != nil {
			result.Errors = append(result.Errors, err.Error())
			// Continue with other resources
		}
	}

	return o.finishCycle(result), nil
}

func (o *Orchestrator) scanResources(ctx context.Context) ([]types.Resource, error) {
	if o.scanner == nil {
		return nil, fmt.Errorf("no scanner configured")
	}

	o.logger.WithContext(ctx).Info().
		Msg("scanning resources")

	return o.scanner.Scan(ctx)
}

func (o *Orchestrator) storeResources(ctx context.Context, resources []types.Resource) error {
	_, err := o.storage.RecordObservationBatch(resources)
	if err != nil {
		o.logger.WithContext(ctx).Error().
			Err(err).
			Int("count", len(resources)).
			Msg("failed to store resources")
	}
	return err
}

func (o *Orchestrator) processResource(ctx context.Context, resource types.Resource, result *CycleResult) error {
	// Build policy input
	input, err := o.policyEngine.BuildPolicyInput(ctx, resource)
	if err != nil {
		return fmt.Errorf("policy input failed for %s: %w", resource.ID, err)
	}

	// Evaluate policies
	decision, err := o.policyEngine.Evaluate(ctx, input)
	if err != nil {
		return fmt.Errorf("policy evaluation failed for %s: %w", resource.ID, err)
	}
	result.PoliciesEvaluated++

	// Enforce if needed
	if decision.Action != "ignore" {
		if err := o.enforcer.Execute(ctx, decision, resource); err != nil {
			return fmt.Errorf("enforcement failed for %s: %w", resource.ID, err)
		}
		result.EnforcementActions++
	}

	return nil
}

func (o *Orchestrator) finishCycle(result *CycleResult) *CycleResult {
	result.EndTime = time.Now()
	result.Duration = result.EndTime.Sub(result.StartTime)

	o.logger.Info().
		Int("resources", result.ResourcesScanned).
		Int("policies", result.PoliciesEvaluated).
		Int("actions", result.EnforcementActions).
		Dur("duration", result.Duration).
		Bool("success", result.Success).
		Msg("reconciliation cycle complete")

	return result
}
