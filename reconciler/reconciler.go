package reconciler

import (
	"context"
	"fmt"
	"time"

	"github.com/yairfalse/elava/storage"
	"github.com/yairfalse/elava/telemetry"
	"github.com/yairfalse/elava/types"
	"github.com/yairfalse/elava/wal"
	"go.opentelemetry.io/otel/trace"
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
	day2Metrics         *telemetry.Day2Metrics
	tracer              trace.Tracer
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
	// Initialize Day 2 metrics
	day2Metrics, err := telemetry.InitDay2Metrics(telemetry.Meter)
	if err != nil {
		// Log error but don't fail - telemetry is optional
		telemetry.NewLogger("reconciler-engine").Error().Err(err).Msg("failed to initialize Day 2 metrics")
	}

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
		day2Metrics:         day2Metrics,
		tracer:              telemetry.Tracer,
	}
}

// Reconcile performs a full reconciliation cycle
func (e *Engine) Reconcile(ctx context.Context, config Config) ([]types.Decision, error) {
	startTime := time.Now()

	// Start reconciliation span (if tracer available)
	var reconSpan *telemetry.ReconciliationSpan
	scanType := e.determineScanType()
	if e.tracer != nil {
		ctx, reconSpan = telemetry.StartReconciliation(ctx, e.tracer, config.Provider, config.Region, scanType)
		defer reconSpan.End()
	}

	if err := e.logReconcileStart(); err != nil {
		return nil, err
	}

	// Step 1: Observe current state (don't store yet)
	current, err := e.observeCurrentState(ctx, config)
	if err != nil {
		if reconSpan != nil {
			telemetry.RecordError(reconSpan.Span, err.Error(), "ObservationError")
		}
		return nil, err
	}
	if reconSpan != nil {
		reconSpan.SetResourceCount(int64(len(current)))
	}

	// Step 2: Detect changes (compares against MVCC from BEFORE this scan)
	decisions, err := e.detectAndDecide(ctx, current, config)
	if err != nil {
		if reconSpan != nil {
			telemetry.RecordError(reconSpan.Span, err.Error(), "DetectDecideError")
		}
		return nil, err
	}

	// Step 3: Store observations (updates MVCC for next scan)
	if err := e.storeObservations(ctx, current); err != nil {
		if reconSpan != nil {
			telemetry.RecordError(reconSpan.Span, err.Error(), "StorageError")
		}
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

	// Record reconciliation duration metric
	durationMs := float64(time.Since(startTime).Milliseconds())
	if e.day2Metrics != nil {
		e.day2Metrics.RecordReconcileDuration(ctx, scanType, config.Provider, config.Region, durationMs)
	}

	return decisions, nil
}

// determineScanType determines if this is a baseline or normal scan
func (e *Engine) determineScanType() string {
	// Check if storage is empty (baseline scan)
	resourceCount, _, _ := e.storage.Stats()
	if resourceCount == 0 {
		return "baseline"
	}
	return "normal"
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
func (e *Engine) detectAndDecide(ctx context.Context, current []types.Resource, config Config) ([]types.Decision, error) {
	// Start detect span (if tracer available)
	detectStart := time.Now()
	var detectSpan trace.Span
	if e.tracer != nil {
		ctx, detectSpan = telemetry.StartDetect(ctx, e.tracer, config.Provider, config.Region)
	}

	changes, err := e.changeDetector.DetectChanges(ctx, current)
	if err != nil {
		if detectSpan != nil {
			telemetry.RecordError(detectSpan, err.Error(), "ChangeDetectionError")
			detectSpan.End()
		}
		return nil, fmt.Errorf("failed to detect changes: %w", err)
	}

	// Count change types and record metrics
	appeared, disappeared, tagDrift, statusChanged := e.countChangeTypes(changes)
	if detectSpan != nil {
		telemetry.EndDetect(detectSpan, int64(len(changes)), appeared, disappeared, tagDrift, statusChanged)
	}

	// Record change metrics
	if e.day2Metrics != nil {
		environment := e.determineEnvironment(config)
		if appeared > 0 {
			e.day2Metrics.RecordChangeDetected(ctx, "appeared", "mixed", environment, "info", config.Provider, config.Region, appeared)
		}
		if disappeared > 0 {
			e.day2Metrics.RecordChangeDetected(ctx, "disappeared", "mixed", environment, "warning", config.Provider, config.Region, disappeared)
		}
		if tagDrift > 0 {
			e.day2Metrics.RecordChangeDetected(ctx, "tag_drift", "mixed", environment, "warning", config.Provider, config.Region, tagDrift)
		}
	}

	// Record detect duration
	if e.day2Metrics != nil {
		detectDurationMs := float64(time.Since(detectStart).Milliseconds())
		e.day2Metrics.DetectDuration.Record(ctx, detectDurationMs)
	}

	// Start decide span (if tracer available)
	var decideSpan trace.Span
	if e.tracer != nil {
		ctx, decideSpan = telemetry.StartDecide(ctx, e.tracer)
	}

	decisions, err := e.policyDecisionMaker.Decide(ctx, changes)
	if err != nil {
		if decideSpan != nil {
			telemetry.RecordError(decideSpan, err.Error(), "DecisionMakingError")
			decideSpan.End()
		}
		return nil, fmt.Errorf("failed to make decisions: %w", err)
	}

	// Count decision types
	notifyCount, alertCount, enforceCount := e.countDecisionTypes(decisions)
	if decideSpan != nil {
		telemetry.EndDecide(decideSpan, int64(len(decisions)), notifyCount, alertCount, enforceCount, 0)
	}

	// Record decision metrics
	if e.day2Metrics != nil {
		environment := e.determineEnvironment(config)
		if notifyCount > 0 {
			e.day2Metrics.RecordDecisionMade(ctx, "notify", "mixed", environment, false, notifyCount)
		}
		if alertCount > 0 {
			e.day2Metrics.RecordDecisionMade(ctx, "alert", "mixed", environment, false, alertCount)
		}
		if enforceCount > 0 {
			e.day2Metrics.RecordDecisionMade(ctx, "enforce", "mixed", environment, false, enforceCount)
		}
	}

	return decisions, nil
}

// countChangeTypes counts changes by type
func (e *Engine) countChangeTypes(changes []Change) (appeared, disappeared, tagDrift, statusChanged int64) {
	for _, change := range changes {
		switch change.Type {
		case ChangeAppeared:
			appeared++
		case ChangeDisappeared:
			disappeared++
		case ChangeTagDrift:
			tagDrift++
		case ChangeStatusChanged:
			statusChanged++
		}
	}
	return
}

// countDecisionTypes counts decisions by action
func (e *Engine) countDecisionTypes(decisions []types.Decision) (notify, alert, enforce int64) {
	for _, decision := range decisions {
		switch decision.Action {
		case "notify":
			notify++
		case "alert":
			alert++
		case "enforce", "enforce_tags":
			enforce++
		}
	}
	return
}

// determineEnvironment determines environment from config or tags
func (e *Engine) determineEnvironment(config Config) string {
	// Could be enhanced to read from config or resource tags
	return "unknown"
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
	// Start observe span (if tracer available)
	observeStart := time.Now()
	var observeSpan trace.Span
	if e.tracer != nil {
		ctx, observeSpan = telemetry.StartObserve(ctx, e.tracer, config.Provider, config.Region)
	}

	filter := types.ResourceFilter{
		Provider: config.Provider,
		Region:   config.Region,
	}

	resources, err := e.observer.Observe(ctx, filter)
	if err != nil {
		if observeSpan != nil {
			telemetry.RecordError(observeSpan, err.Error(), "ObservationError")
			observeSpan.End()
		}
		return nil, fmt.Errorf("observation failed: %w", err)
	}

	// End observe span with metrics
	if observeSpan != nil {
		durationSeconds := time.Since(observeStart).Seconds()
		telemetry.EndObserve(observeSpan, int64(len(resources)), durationSeconds)
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
