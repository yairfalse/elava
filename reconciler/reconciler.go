package reconciler

import (
	"context"
	"fmt"
	"time"

	"github.com/yairfalse/elava/storage"
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
	observer      Observer
	comparator    Comparator
	decisionMaker DecisionMaker
	coordinator   Coordinator
	storage       *storage.MVCCStorage
	wal           *wal.WAL
	instanceID    string
	options       ReconcilerOptions
}

// NewEngine creates a new reconciler engine
func NewEngine(
	observer Observer,
	comparator Comparator,
	decisionMaker DecisionMaker,
	coordinator Coordinator,
	storage *storage.MVCCStorage,
	walInstance *wal.WAL,
	instanceID string,
	options ReconcilerOptions,
) *Engine {
	return &Engine{
		observer:      observer,
		comparator:    comparator,
		decisionMaker: decisionMaker,
		coordinator:   coordinator,
		storage:       storage,
		wal:           walInstance,
		instanceID:    instanceID,
		options:       options,
	}
}

// Reconcile performs a full reconciliation cycle
func (e *Engine) Reconcile(ctx context.Context, config Config) ([]types.Decision, error) {
	startTime := time.Now()

	if err := e.logReconcileStart(); err != nil {
		return nil, err
	}

	current, err := e.observeAndStore(ctx, config)
	if err != nil {
		return nil, err
	}

	decisions, err := e.compareAndDecide(config, current)
	if err != nil {
		return nil, err
	}

	if err := e.logDecisions(decisions); err != nil {
		return nil, err
	}

	if err := e.logReconcileResult(startTime, current, decisions); err != nil {
		return nil, err
	}

	return decisions, nil
}

// logReconcileStart logs the start of reconciliation
func (e *Engine) logReconcileStart() error {
	if err := e.wal.Append(wal.EntryObserved, "", "reconcile_start"); err != nil {
		return fmt.Errorf("failed to log reconcile start: %w", err)
	}
	return nil
}

// observeAndStore observes current state and stores it
func (e *Engine) observeAndStore(ctx context.Context, config Config) ([]types.Resource, error) {
	current, err := e.observeCurrentState(ctx, config)
	if err != nil {
		return nil, fmt.Errorf("failed to observe current state: %w", err)
	}

	_, err = e.storage.RecordObservationBatch(current)
	if err != nil {
		return nil, fmt.Errorf("failed to store observations: %w", err)
	}

	return current, nil
}

// compareAndDecide compares states and makes decisions
func (e *Engine) compareAndDecide(config Config, current []types.Resource) ([]types.Decision, error) {
	desired := e.buildDesiredState(config)
	diffs, err := e.comparator.Compare(current, desired)
	if err != nil {
		return nil, fmt.Errorf("failed to compare states: %w", err)
	}

	decisions, err := e.decisionMaker.Decide(diffs)
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

// buildDesiredState converts config specs to desired resources
//
// Deprecated: buildDesiredState is deprecated and will be removed in v2.0.
// Elava has pivoted from IaC state management to Day 2 operations.
// Instead of declaring desired state, Elava now observes actual infrastructure,
// detects changes, and enforces policies. Use tag-based resource tracking.
// See docs/design/day2-reconciler.md for the new approach.
func (e *Engine) buildDesiredState(config Config) []types.Resource {
	// TODO(v2.0): Remove this function entirely
	// For now, return empty to avoid creating resources from config
	return []types.Resource{}
}
