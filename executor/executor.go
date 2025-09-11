package executor

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/yairfalse/ovi/providers"
	"github.com/yairfalse/ovi/storage"
	"github.com/yairfalse/ovi/types"
	"github.com/yairfalse/ovi/wal"
)

// Engine implements the Executor interface with comprehensive safety checks
type Engine struct {
	providers       map[string]providers.CloudProvider
	storage         *storage.MVCCStorage
	wal             *wal.WAL
	options         ExecutorOptions
	safetyChecker   SafetyChecker
	confirmer       Confirmer
	rollbackManager RollbackManager
	mu              sync.RWMutex
}

// NewEngine creates a new executor engine
func NewEngine(
	providerMap map[string]providers.CloudProvider,
	storage *storage.MVCCStorage,
	walInstance *wal.WAL,
	options ExecutorOptions,
) *Engine {
	engine := &Engine{
		providers: providerMap,
		storage:   storage,
		wal:       walInstance,
		options:   options,
		mu:        sync.RWMutex{},
	}

	// Set defaults if not provided
	if engine.safetyChecker == nil {
		engine.safetyChecker = NewDefaultSafetyChecker()
	}
	if engine.rollbackManager == nil {
		engine.rollbackManager = NewDefaultRollbackManager(storage, walInstance)
	}

	return engine
}

// Execute executes a batch of decisions with comprehensive error handling
func (e *Engine) Execute(ctx context.Context, decisions []types.Decision) (*ExecutionResult, error) {
	startTime := time.Now()

	result := &ExecutionResult{
		StartTime:      startTime,
		TotalDecisions: len(decisions),
		Results:        make([]SingleExecutionResult, 0, len(decisions)),
	}

	// Log execution start
	if err := e.wal.Append(wal.EntryExecuting, "", ExecutionStart{
		DecisionCount: len(decisions),
		StartTime:     startTime,
		Options:       e.options,
	}); err != nil {
		return nil, fmt.Errorf("failed to log execution start: %w", err)
	}

	// Execute decisions
	for _, decision := range decisions {
		singleResult, err := e.ExecuteSingle(ctx, decision)
		if err != nil {
			// Log the error but continue with other decisions if configured
			singleResult = &SingleExecutionResult{
				Decision:  decision,
				Status:    StatusFailed,
				StartTime: time.Now(),
				EndTime:   time.Now(),
				Error:     err.Error(),
			}
		}

		result.Results = append(result.Results, *singleResult)
		e.updateResultCounts(result, *singleResult)

		// Check if we should stop on failure
		if singleResult.Status == StatusFailed && !e.options.ContinueOnFailure {
			result.PartialFailure = true
			break
		}
	}

	result.EndTime = time.Now()
	result.Duration = result.EndTime.Sub(result.StartTime)
	result.PartialFailure = result.FailedCount > 0
	result.RollbackRequired = result.PartialFailure && e.options.RollbackOnPartialFail

	// Log execution completion
	if err := e.wal.Append(wal.EntryExecuted, "", result); err != nil {
		return result, fmt.Errorf("failed to log execution result: %w", err)
	}

	return result, nil
}

// ExecuteSingle executes a single decision with full safety checks
//
//nolint:gocyclo // This function coordinates many necessary steps for safe execution
func (e *Engine) ExecuteSingle(ctx context.Context, decision types.Decision) (*SingleExecutionResult, error) {
	startTime := time.Now()

	result := &SingleExecutionResult{
		Decision:  decision,
		Status:    StatusPending,
		StartTime: startTime,
	}

	// Validate decision
	if err := decision.Validate(); err != nil {
		return e.failResult(result, "invalid decision", err), nil
	}

	// Get provider
	provider, err := e.getProviderForDecision(decision)
	if err != nil {
		return e.failResult(result, "provider error", err), nil
	}

	result.ProviderInfo = ProviderInfo{
		Provider: provider.Name(),
		Region:   provider.Region(),
	}

	// Pre-execution safety checks
	if shouldSkip, reason := e.shouldSkipDecision(ctx, decision, provider); shouldSkip {
		result.Status = StatusSkipped
		result.SkipReason = reason
		result.EndTime = time.Now()
		result.Duration = result.EndTime.Sub(result.StartTime)
		return result, nil
	}

	// Request confirmation if needed
	if decision.RequiresConfirmation() && !e.options.SkipConfirmation {
		if confirmed, err := e.requestConfirmation(ctx, decision); err != nil {
			return e.failResult(result, "confirmation error", err), nil
		} else if !confirmed {
			result.Status = StatusSkipped
			result.SkipReason = "user declined confirmation"
			result.EndTime = time.Now()
			result.Duration = result.EndTime.Sub(result.StartTime)
			return result, nil
		}
	}

	// Execute the decision
	result.Status = StatusExecuting

	// Log execution start
	if err := e.wal.Append(wal.EntryExecuting, decision.ResourceID, decision); err != nil {
		return e.failResult(result, "failed to log execution start", err), nil
	}

	resourceID, err := e.executeDecision(ctx, decision, provider)
	if err != nil {
		// Log failure
		if walErr := e.wal.AppendError(wal.EntryFailed, decision.ResourceID, decision, err); walErr != nil {
			return e.failResult(result, "execution failed and WAL error", fmt.Errorf("execution: %w, wal: %w", err, walErr)), nil
		}
		return e.failResult(result, "execution failed", err), nil
	}

	// Success
	result.Status = StatusSuccess
	result.ResourceID = resourceID
	result.EndTime = time.Now()
	result.Duration = result.EndTime.Sub(result.StartTime)

	// Log success
	if err := e.wal.Append(wal.EntryExecuted, decision.ResourceID, result); err != nil {
		// Decision executed successfully but WAL failed - still return success
		// but log the WAL error separately
		fmt.Printf("Warning: execution succeeded but WAL logging failed: %v\n", err)
	}

	// Record for potential rollback
	if e.options.EnableRollback {
		if err := e.recordForRollback(ctx, decision, resourceID); err != nil {
			// Don't fail the execution for rollback recording errors
			fmt.Printf("Warning: failed to record rollback entry: %v\n", err)
		}
	}

	return result, nil
}

// DryRun simulates execution without making changes
func (e *Engine) DryRun(ctx context.Context, decisions []types.Decision) (*DryRunResult, error) {
	result := &DryRunResult{
		TotalDecisions:   len(decisions),
		BlockedDecisions: make([]BlockedDecision, 0),
	}

	for _, decision := range decisions {
		// Check if decision would be blocked
		if blocked, reason, severity := e.wouldBeBlocked(ctx, decision); blocked {
			result.BlockedDecisions = append(result.BlockedDecisions, BlockedDecision{
				Decision: decision,
				Reason:   reason,
				Severity: severity,
			})
		} else {
			result.SafeDecisions++
		}

		// Categorize decisions
		if decision.IsDestructive() {
			result.DestructiveDecisions++
		}
		if decision.IsBlessed {
			result.BlessedDecisions++
		}
	}

	// Estimate duration (simple heuristic)
	result.EstimatedDuration = time.Duration(len(decisions)) * 5 * time.Second

	return result, nil
}

// Helper methods

func (e *Engine) updateResultCounts(result *ExecutionResult, singleResult SingleExecutionResult) {
	switch singleResult.Status {
	case StatusSuccess:
		result.SuccessfulCount++
	case StatusFailed:
		result.FailedCount++
	case StatusSkipped:
		result.SkippedCount++
	}
}

func (e *Engine) failResult(result *SingleExecutionResult, context string, err error) *SingleExecutionResult {
	result.Status = StatusFailed
	result.Error = fmt.Sprintf("%s: %v", context, err)
	result.EndTime = time.Now()
	result.Duration = result.EndTime.Sub(result.StartTime)
	return result
}

func (e *Engine) getProviderForDecision(decision types.Decision) (providers.CloudProvider, error) {
	// Extract provider from resource type or use default
	// For now, we only have AWS, but this could be expanded
	providerName := "aws" // Default to AWS for now

	// Try to find the provider
	for name, provider := range e.providers {
		// Use the first available provider if we only have one
		if len(e.providers) == 1 {
			return provider, nil
		}
		// Otherwise match by name
		if name == providerName {
			return provider, nil
		}
	}

	return nil, fmt.Errorf("no suitable provider found for decision")
}

func (e *Engine) shouldSkipDecision(ctx context.Context, decision types.Decision, provider providers.CloudProvider) (bool, string) {
	// Skip destructive actions if not allowed
	if decision.IsDestructive() && !e.options.AllowDestructive {
		return true, "destructive actions disabled"
	}

	// Skip blessed resource changes if not allowed
	if decision.IsBlessed && !e.options.AllowBlessedChanges {
		return true, "blessed resource protection enabled"
	}

	// Additional safety checks
	if e.safetyChecker != nil {
		checks, err := e.safetyChecker.CheckSafety(ctx, decision, provider)
		if err != nil {
			return true, fmt.Sprintf("safety check error: %v", err)
		}

		for _, check := range checks {
			if !check.Passed && check.Severity == SeverityCritical {
				return true, fmt.Sprintf("critical safety check failed: %s", check.Message)
			}
		}
	}

	return false, ""
}

func (e *Engine) wouldBeBlocked(ctx context.Context, decision types.Decision) (bool, string, BlockSeverity) {
	if decision.IsDestructive() && !e.options.AllowDestructive {
		return true, "destructive actions disabled", SeverityError
	}

	if decision.IsBlessed && !e.options.AllowBlessedChanges {
		return true, "blessed resource protection enabled", SeverityCritical
	}

	return false, "", SeverityWarning
}

func (e *Engine) requestConfirmation(ctx context.Context, decision types.Decision) (bool, error) {
	if e.confirmer == nil {
		// No confirmer configured - default to requiring manual override
		return false, fmt.Errorf("confirmation required but no confirmer configured")
	}

	req := ConfirmationRequest{
		Decision:  decision,
		Message:   fmt.Sprintf("Execute %s on %s?", decision.Action, decision.ResourceID),
		Severity:  SeverityWarning,
		DefaultNo: decision.IsDestructive(),
		Timeout:   30 * time.Second,
	}

	if decision.IsDestructive() {
		req.Severity = SeverityError
	}
	if decision.IsBlessed {
		req.Severity = SeverityCritical
	}

	response, err := e.confirmer.RequestConfirmation(ctx, req)
	if err != nil {
		return false, err
	}

	return response.Approved, nil
}

func (e *Engine) recordForRollback(ctx context.Context, decision types.Decision, resourceID string) error {
	if e.rollbackManager == nil {
		return nil // No rollback manager configured
	}

	entry := RollbackEntry{
		Decision:      decision,
		ExecutedAt:    time.Now(),
		CanRollback:   e.canRollbackAction(decision.Action),
		ReverseAction: e.getReverseAction(decision.Action),
	}

	return e.rollbackManager.RecordExecution(ctx, entry)
}

func (e *Engine) canRollbackAction(action string) bool {
	switch action {
	case types.ActionCreate:
		return true // Can delete what we created
	case types.ActionTag:
		return true // Can remove tags
	case types.ActionUpdate:
		return false // Updates are hard to rollback
	case types.ActionDelete, types.ActionTerminate:
		return false // Can't bring back deleted resources
	default:
		return false
	}
}

func (e *Engine) getReverseAction(action string) string {
	switch action {
	case types.ActionCreate:
		return types.ActionDelete
	case types.ActionTag:
		return "untag"
	default:
		return types.ActionNoop
	}
}

// ExecutionStart represents the start of an execution batch
type ExecutionStart struct {
	DecisionCount int             `json:"decision_count"`
	StartTime     time.Time       `json:"start_time"`
	Options       ExecutorOptions `json:"options"`
}
