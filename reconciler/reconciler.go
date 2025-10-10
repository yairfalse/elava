package reconciler

import (
	"context"
	"fmt"
	"time"

	"github.com/yairfalse/elava/storage"
	"github.com/yairfalse/elava/telemetry"
	"github.com/yairfalse/elava/types"
	"github.com/yairfalse/elava/wal"
)

// ObservationData represents data logged during observation
type ObservationData struct {
	Provider       string `json:"provider"`
	Region         string `json:"region"`
	ResourcesFound int    `json:"resources_found"`
}

// Engine implements the main reconciliation logic
type Engine struct {
	observer            Observer
	changeDetector      ChangeDetector
	policyDecisionMaker PolicyDecisionMaker
	coordinator         Coordinator
	storage             *storage.MVCCStorage
	wal                 *wal.WAL
	logger              *telemetry.Logger
	instanceID          string
	options             ReconcilerOptions
}

// NewEngine creates a new reconciler engine
func NewEngine(
	observer Observer,
	changeDetector ChangeDetector,
	policyDecisionMaker PolicyDecisionMaker,
	coordinator Coordinator,
	storage *storage.MVCCStorage,
	walInstance *wal.WAL,
	instanceID string,
	options ReconcilerOptions,
) *Engine {
	return &Engine{
		observer:            observer,
		changeDetector:      changeDetector,
		policyDecisionMaker: policyDecisionMaker,
		coordinator:         coordinator,
		storage:             storage,
		wal:                 walInstance,
		logger:              telemetry.NewLogger("reconciler-engine"),
		instanceID:          instanceID,
		options:             options,
	}
}

// Reconcile performs a full reconciliation cycle
func (e *Engine) Reconcile(ctx context.Context, config Config) ([]types.Decision, error) {
	startTime := time.Now()

	if err := e.logReconcileStart(); err != nil {
		return nil, err
	}

	// Step 1: Observe current state (don't store yet)
	current, err := e.observeCurrentState(ctx, config)
	if err != nil {
		return nil, err
	}

	// Step 2: Detect changes (compares against MVCC from BEFORE this scan)
	decisions, err := e.detectAndDecide(ctx, current)
	if err != nil {
		return nil, err
	}

	// Step 3: Store observations (updates MVCC for next scan)
	if err := e.storeObservations(ctx, current); err != nil {
		return nil, err
	}

	// Handle baseline scan (first scan)
	decisions = e.handleBaselineScan(current, decisions)

	if err := e.logDecisions(decisions); err != nil {
		return nil, err
	}

	if err := e.logReconcileResult(startTime, current, decisions); err != nil {
		return nil, err
	}

	return decisions, nil
}

// handleBaselineScan detects first scan and displays baseline summary
func (e *Engine) handleBaselineScan(resources []types.Resource, decisions []types.Decision) []types.Decision {
	// Check if all decisions are for baseline changes
	if len(decisions) == 0 {
		return decisions
	}

	isBaseline := true
	for _, decision := range decisions {
		if decision.Action != "audit" {
			isBaseline = false
			break
		}
	}

	// If this is a baseline scan, log summary
	if isBaseline && len(resources) > 0 {
		summary := SummarizeBaseline(resources)
		e.logger.Info().
			Int("total_resources", summary.Total).
			Int("resource_types", len(summary.ByType)).
			Int("untagged", len(summary.Untagged)).
			Int("no_owner", len(summary.NoOwner)).
			Str("oldest_resource", formatAge(summary.OldestResource)).
			Str("newest_resource", formatAge(summary.NewestResource)).
			Msg("baseline scan complete")

		// Still print formatted summary to stdout for user visibility
		fmt.Println(summary.FormatBaselineSummary())
	}

	return decisions
}

// logReconcileStart logs the start of reconciliation
func (e *Engine) logReconcileStart() error {
	if err := e.wal.Append(wal.EntryObserved, "", "reconcile_start"); err != nil {
		return fmt.Errorf("failed to log reconcile start: %w", err)
	}
	return nil
}

// storeObservations stores current observations in MVCC
func (e *Engine) storeObservations(ctx context.Context, resources []types.Resource) error {
	_, err := e.storage.RecordObservationBatch(resources)
	if err != nil {
		return fmt.Errorf("failed to store observations: %w", err)
	}
	return nil
}

// detectAndDecide detects changes and makes policy-based decisions
func (e *Engine) detectAndDecide(ctx context.Context, current []types.Resource) ([]types.Decision, error) {
	changes, err := e.changeDetector.DetectChanges(ctx, current)
	if err != nil {
		return nil, fmt.Errorf("failed to detect changes: %w", err)
	}

	decisions, err := e.policyDecisionMaker.Decide(ctx, changes)
	if err != nil {
		return nil, fmt.Errorf("failed to make decisions: %w", err)
	}

	return decisions, nil
}

// logDecisions logs all decisions to WAL
func (e *Engine) logDecisions(decisions []types.Decision) error {
	for _, decision := range decisions {
		if err := e.wal.Append(wal.EntryDecided, decision.ResourceID, decision); err != nil {
			return fmt.Errorf("failed to log decision: %w", err)
		}
	}
	return nil
}

// logReconcileResult logs the reconciliation result
func (e *Engine) logReconcileResult(startTime time.Time, current []types.Resource, decisions []types.Decision) error {
	result := ReconcileResult{
		Timestamp:      startTime,
		ResourcesFound: len(current),
		DiffsDetected:  len(decisions), // Simplified - could track separately
		DecisionsMade:  len(decisions),
		Duration:       time.Since(startTime),
		Decisions:      decisions,
	}

	if err := e.wal.Append(wal.EntryExecuted, "", result); err != nil {
		return fmt.Errorf("failed to log reconcile result: %w", err)
	}

	return nil
}

// observeCurrentState polls all configured providers for current resources
func (e *Engine) observeCurrentState(ctx context.Context, config Config) ([]types.Resource, error) {
	filter := types.ResourceFilter{
		Provider: config.Provider,
		Region:   config.Region,
	}

	resources, err := e.observer.Observe(ctx, filter)
	if err != nil {
		return nil, fmt.Errorf("observation failed: %w", err)
	}

	// Log observation
	observationData := ObservationData{
		Provider:       config.Provider,
		Region:         config.Region,
		ResourcesFound: len(resources),
	}

	if err := e.wal.Append(wal.EntryObserved, "", observationData); err != nil {
		return nil, fmt.Errorf("failed to log observation: %w", err)
	}

	return resources, nil
}
