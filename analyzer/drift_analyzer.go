package analyzer

import (
	"context"
	"fmt"
	"reflect"
	"time"

	"github.com/yairfalse/elava/storage"
	"github.com/yairfalse/elava/types"
)

// DriftAnalyzerImpl analyzes configuration drift
type DriftAnalyzerImpl struct {
	storage     *storage.MVCCStorage
	queryEngine *QueryEngineImpl
}

// NewDriftAnalyzer creates a new drift analyzer
func NewDriftAnalyzer(s *storage.MVCCStorage) *DriftAnalyzerImpl {
	return &DriftAnalyzerImpl{
		storage:     s,
		queryEngine: NewQueryEngine(s),
	}
}

// AnalyzeDrift detects drift between two time points
func (d *DriftAnalyzerImpl) AnalyzeDrift(ctx context.Context, from, to time.Time) ([]DriftEvent, error) {
	var driftEvents []DriftEvent

	// Get resources at both time points
	fromResources, err := d.getResourcesAtTime(ctx, from)
	if err != nil {
		return nil, fmt.Errorf("failed to get resources at from time: %w", err)
	}

	toResources, err := d.getResourcesAtTime(ctx, to)
	if err != nil {
		return nil, fmt.Errorf("failed to get resources at to time: %w", err)
	}

	// Compare resources
	fromMap := d.buildResourceMap(fromResources)
	toMap := d.buildResourceMap(toResources)

	// Check for drift in existing resources
	for id, fromRes := range fromMap {
		if toRes, exists := toMap[id]; exists {
			events := d.compareResources(fromRes, toRes)
			driftEvents = append(driftEvents, events...)
		}
	}

	return driftEvents, nil
}

// getResourcesAtTime gets resources at specific time
func (d *DriftAnalyzerImpl) getResourcesAtTime(ctx context.Context, t time.Time) ([]types.Resource, error) {
	// Query a window around the time point
	start := t.Add(-time.Hour)
	end := t.Add(time.Hour)
	return d.queryEngine.QueryByTimeRange(ctx, start, end)
}

// buildResourceMap creates ID->Resource map
func (d *DriftAnalyzerImpl) buildResourceMap(resources []types.Resource) map[string]types.Resource {
	m := make(map[string]types.Resource)
	for _, r := range resources {
		m[r.ID] = r
	}
	return m
}

// compareResources compares two resource states
func (d *DriftAnalyzerImpl) compareResources(from, to types.Resource) []DriftEvent {
	var events []DriftEvent

	// Check status change
	if from.Status != to.Status {
		events = append(events, DriftEvent{
			ResourceID: from.ID,
			Timestamp:  to.LastSeenAt,
			Type:       from.Type,
			Field:      "status",
			OldValue:   from.Status,
			NewValue:   to.Status,
			Severity:   d.assessStatusDriftSeverity(from.Status, to.Status),
		})
	}

	// Check tag changes
	tagDrift := d.compareTagDrift(from.Tags, to.Tags)
	for _, drift := range tagDrift {
		drift.ResourceID = from.ID
		drift.Timestamp = to.LastSeenAt
		drift.Type = from.Type
		events = append(events, drift)
	}

	// Check metadata changes
	metaDrift := d.compareMetadata(from.Metadata, to.Metadata)
	for _, drift := range metaDrift {
		drift.ResourceID = from.ID
		drift.Timestamp = to.LastSeenAt
		drift.Type = from.Type
		events = append(events, drift)
	}

	return events
}

// assessStatusDriftSeverity determines severity of status change
func (d *DriftAnalyzerImpl) assessStatusDriftSeverity(from, to string) DriftSeverity {
	// Define severity rules
	criticalTransitions := map[string]string{
		"running_terminated":  "critical",
		"available_failed":    "critical",
		"healthy_terminating": "critical",
	}

	highTransitions := map[string]string{
		"running_stopping":   "high",
		"healthy_unhealthy":  "high",
		"available_degraded": "high",
	}

	key := from + "_" + to
	if _, ok := criticalTransitions[key]; ok {
		return DriftCritical
	}
	if _, ok := highTransitions[key]; ok {
		return DriftHigh
	}
	if from == "running" && to == "stopped" {
		return DriftMedium
	}
	return DriftLow
}

// compareTagDrift compares tag changes
func (d *DriftAnalyzerImpl) compareTagDrift(from, to types.Tags) []DriftEvent {
	var events []DriftEvent

	if from.ElavaOwner != to.ElavaOwner {
		events = append(events, DriftEvent{
			Field:    "tags.owner",
			OldValue: from.ElavaOwner,
			NewValue: to.ElavaOwner,
			Severity: DriftHigh, // Ownership changes are important
		})
	}

	if from.Team != to.Team {
		events = append(events, DriftEvent{
			Field:    "tags.team",
			OldValue: from.Team,
			NewValue: to.Team,
			Severity: DriftMedium,
		})
	}

	if from.Environment != to.Environment {
		events = append(events, DriftEvent{
			Field:    "tags.environment",
			OldValue: from.Environment,
			NewValue: to.Environment,
			Severity: DriftHigh, // Environment changes are critical
		})
	}

	if from.ElavaManaged != to.ElavaManaged {
		events = append(events, DriftEvent{
			Field:    "tags.elava_managed",
			OldValue: from.ElavaManaged,
			NewValue: to.ElavaManaged,
			Severity: DriftCritical, // Management status is critical
		})
	}

	return events
}

// compareMetadata compares metadata changes
func (d *DriftAnalyzerImpl) compareMetadata(from, to map[string]interface{}) []DriftEvent {
	var events []DriftEvent

	// Check for important metadata changes
	importantFields := []string{
		"instance_type", "node_count", "is_encrypted",
		"public_ip", "deletion_protection", "backup_retention",
	}

	for _, field := range importantFields {
		if d.metadataChanged(from, to, field) {
			events = append(events, DriftEvent{
				Field:    fmt.Sprintf("metadata.%s", field),
				OldValue: from[field],
				NewValue: to[field],
				Severity: d.assessMetadataDriftSeverity(field),
			})
		}
	}

	// Check cost changes
	if d.costChanged(from, to) {
		events = append(events, DriftEvent{
			Field:    "metadata.monthly_cost_estimate",
			OldValue: from["monthly_cost_estimate"],
			NewValue: to["monthly_cost_estimate"],
			Severity: DriftHigh,
			Metadata: map[string]interface{}{
				"cost_increase": d.calculateCostIncrease(from, to),
			},
		})
	}

	return events
}

// metadataChanged checks if metadata field changed
func (d *DriftAnalyzerImpl) metadataChanged(from, to map[string]interface{}, field string) bool {
	fromVal, fromOk := from[field]
	toVal, toOk := to[field]

	if fromOk != toOk {
		return true
	}
	if !fromOk {
		return false
	}

	return !reflect.DeepEqual(fromVal, toVal)
}

// costChanged checks if cost changed significantly
func (d *DriftAnalyzerImpl) costChanged(from, to map[string]interface{}) bool {
	fromCost, _ := from["monthly_cost_estimate"].(float64)
	toCost, _ := to["monthly_cost_estimate"].(float64)

	// Consider >10% change significant
	if fromCost == 0 {
		return toCost > 0
	}
	change := (toCost - fromCost) / fromCost
	return change > 0.1 || change < -0.1
}

// calculateCostIncrease calculates percentage cost increase
func (d *DriftAnalyzerImpl) calculateCostIncrease(from, to map[string]interface{}) float64 {
	fromCost, _ := from["monthly_cost_estimate"].(float64)
	toCost, _ := to["monthly_cost_estimate"].(float64)

	if fromCost == 0 {
		return 100.0
	}
	return ((toCost - fromCost) / fromCost) * 100
}

// assessMetadataDriftSeverity determines metadata drift severity
func (d *DriftAnalyzerImpl) assessMetadataDriftSeverity(field string) DriftSeverity {
	criticalFields := map[string]bool{
		"deletion_protection": true,
		"is_encrypted":        true,
		"public_ip":           true,
	}

	highFields := map[string]bool{
		"instance_type":    true,
		"node_count":       true,
		"backup_retention": true,
	}

	if criticalFields[field] {
		return DriftCritical
	}
	if highFields[field] {
		return DriftHigh
	}
	return DriftMedium
}

// GetResourceDrift gets drift for specific resource
func (d *DriftAnalyzerImpl) GetResourceDrift(ctx context.Context, resourceID string, period time.Duration) ([]DriftEvent, error) {
	var driftEvents []DriftEvent

	// Get resource history
	history, err := d.queryEngine.QueryResourceHistory(ctx, resourceID)
	if err != nil {
		return nil, fmt.Errorf("failed to get resource history: %w", err)
	}

	// Filter by period
	cutoff := time.Now().Add(-period)
	var relevantHistory []ResourceRevision
	for _, rev := range history {
		if rev.Timestamp.After(cutoff) {
			relevantHistory = append(relevantHistory, rev)
		}
	}

	// Compare consecutive revisions
	for i := 1; i < len(relevantHistory); i++ {
		prev := relevantHistory[i-1].Resource
		curr := relevantHistory[i].Resource

		events := d.compareResources(prev, curr)
		driftEvents = append(driftEvents, events...)
	}

	return driftEvents, nil
}
