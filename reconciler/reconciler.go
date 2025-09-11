package reconciler

import (
	"context"
	"fmt"
	"time"

	"github.com/yairfalse/ovi/storage"
	"github.com/yairfalse/ovi/types"
	"github.com/yairfalse/ovi/wal"
)

// Engine implements the main reconciliation logic
type Engine struct {
	observer     Observer
	comparator   Comparator
	decisionMaker DecisionMaker
	coordinator  Coordinator
	storage      *storage.MVCCStorage
	wal          *wal.WAL
	instanceID   string
	options      ReconcilerOptions
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
	
	// Step 1: Observe current state
	if err := e.wal.Append(wal.EntryObserved, "", "reconcile_start"); err != nil {
		return nil, fmt.Errorf("failed to log reconcile start: %w", err)
	}

	current, err := e.observeCurrentState(ctx, config)
	if err != nil {
		return nil, fmt.Errorf("failed to observe current state: %w", err)
	}

	// Step 2: Store observations
	_, err = e.storage.RecordObservationBatch(current)
	if err != nil {
		return nil, fmt.Errorf("failed to store observations: %w", err)
	}

	// Step 3: Compare with desired state
	desired := e.buildDesiredState(config)
	diffs, err := e.comparator.Compare(current, desired)
	if err != nil {
		return nil, fmt.Errorf("failed to compare states: %w", err)
	}

	// Step 4: Make decisions
	decisions, err := e.decisionMaker.Decide(diffs)
	if err != nil {
		return nil, fmt.Errorf("failed to make decisions: %w", err)
	}

	// Step 5: Log decisions
	for _, decision := range decisions {
		if err := e.wal.Append(wal.EntryDecided, decision.ResourceID, decision); err != nil {
			return nil, fmt.Errorf("failed to log decision: %w", err)
		}
	}

	// Log reconcile completion
	result := ReconcileResult{
		Timestamp:      startTime,
		ResourcesFound: len(current),
		DiffsDetected:  len(diffs),
		DecisionsMade:  len(decisions),
		Duration:       time.Since(startTime),
		Decisions:      decisions,
	}

	if err := e.wal.Append(wal.EntryExecuted, "", result); err != nil {
		return nil, fmt.Errorf("failed to log reconcile result: %w", err)
	}

	return decisions, nil
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
	observationData := map[string]interface{}{
		"provider":        config.Provider,
		"region":          config.Region,
		"resources_found": len(resources),
	}

	if err := e.wal.Append(wal.EntryObserved, "", observationData); err != nil {
		return nil, fmt.Errorf("failed to log observation: %w", err)
	}

	return resources, nil
}

// buildDesiredState converts config specs to desired resources
func (e *Engine) buildDesiredState(config Config) []types.Resource {
	var desired []types.Resource

	for i, spec := range config.Resources {
		for j := 0; j < max(spec.Count, 1); j++ {
			resource := types.Resource{
				ID:       fmt.Sprintf("%s-%d-%d", spec.Type, i, j),
				Type:     spec.Type,
				Provider: config.Provider,
				Region:   spec.Region,
				Tags:     spec.Tags,
			}

			// Mark as Ovi-managed
			resource.Tags.OviManaged = true
			if resource.Tags.OviOwner == "" {
				resource.Tags.OviOwner = "ovi"
			}

			desired = append(desired, resource)
		}
	}

	return desired
}

// max returns the maximum of two integers
func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}