package analyzer

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/yairfalse/elava/storage"
	"github.com/yairfalse/elava/types"
)

// ResourceMetadata represents resource metadata for analysis
type ResourceMetadata map[string]interface{}

// WasteAnalyzerImpl identifies wasted resources
type WasteAnalyzerImpl struct {
	storage     *storage.MVCCStorage
	queryEngine *QueryEngineImpl
}

// NewWasteAnalyzer creates a new waste analyzer
func NewWasteAnalyzer(s *storage.MVCCStorage) *WasteAnalyzerImpl {
	return &WasteAnalyzerImpl{
		storage:     s,
		queryEngine: NewQueryEngine(s),
	}
}

// AnalyzeWaste finds potentially wasted resources
func (w *WasteAnalyzerImpl) AnalyzeWaste(ctx context.Context) ([]WastePattern, error) {
	var patterns []WastePattern

	// Get all current resources
	resources, err := w.queryEngine.QueryByTimeRange(ctx, time.Now().Add(-24*time.Hour), time.Now())
	if err != nil {
		return nil, fmt.Errorf("failed to query resources: %w", err)
	}

	// Detect different waste patterns
	orphaned := w.detectOrphaned(resources)
	if len(orphaned.ResourceIDs) > 0 {
		patterns = append(patterns, orphaned)
	}

	idle := w.detectIdle(resources)
	if len(idle.ResourceIDs) > 0 {
		patterns = append(patterns, idle)
	}

	oversized := w.detectOversized(resources)
	if len(oversized.ResourceIDs) > 0 {
		patterns = append(patterns, oversized)
	}

	unattached := w.detectUnattached(resources)
	if len(unattached.ResourceIDs) > 0 {
		patterns = append(patterns, unattached)
	}

	obsolete := w.detectObsolete(resources)
	if len(obsolete.ResourceIDs) > 0 {
		patterns = append(patterns, obsolete)
	}

	return patterns, nil
}

// detectOrphaned finds orphaned resources
func (w *WasteAnalyzerImpl) detectOrphaned(resources []types.Resource) WastePattern {
	pattern := WastePattern{
		Type:        WasteOrphaned,
		Reason:      "Resources without owner or project tags",
		FirstSeen:   time.Now(),
		Confidence:  0.8,
		ResourceIDs: []string{},
	}

	var totalCost float64
	for _, r := range resources {
		if w.isOrphaned(r) {
			pattern.ResourceIDs = append(pattern.ResourceIDs, r.ID)
			if cost, ok := r.Metadata["monthly_cost_estimate"].(float64); ok {
				totalCost += cost
			}
		}
	}

	// Impact calculation removed - let FinOps tools handle
	return pattern
}

// isOrphaned checks if resource lacks ownership
func (w *WasteAnalyzerImpl) isOrphaned(r types.Resource) bool {
	// Already marked as orphaned
	if r.IsOrphaned {
		return true
	}

	// No owner or team tags
	if r.Tags.ElavaOwner == "" && r.Tags.Team == "" {
		return true
	}

	// Default security groups/VPCs are often orphaned
	if r.Type == "security_group" && strings.Contains(r.Name, "default") {
		return true
	}

	return false
}

// detectIdle finds idle resources
func (w *WasteAnalyzerImpl) detectIdle(resources []types.Resource) WastePattern {
	pattern := WastePattern{
		Type:        WasteIdle,
		Reason:      "Resources showing no activity",
		FirstSeen:   time.Now(),
		Confidence:  0.7,
		ResourceIDs: []string{},
	}

	var totalCost float64
	for _, r := range resources {
		if w.isIdle(r) {
			pattern.ResourceIDs = append(pattern.ResourceIDs, r.ID)
			if cost, ok := r.Metadata["monthly_cost_estimate"].(float64); ok {
				totalCost += cost
			}
		}
	}

	// Impact calculation removed - let FinOps tools handle
	return pattern
}

// isIdle checks if resource is idle
func (w *WasteAnalyzerImpl) isIdle(r types.Resource) bool {
	// Check specific idle indicators by type
	switch r.Type {
	case "ec2":
		return r.Status == "stopped"
	case "rds", "aurora":
		return w.checkMetadataBool(r.Metadata, "is_idle")
	case "redshift":
		return w.checkMetadataBool(r.Metadata, "is_paused")
	case "lambda":
		return w.checkMetadataInt(r.Metadata, "days_since_modified") > 30
	case "nat_gateway":
		// NAT gateways cost money even when idle
		return r.Status != "available"
	}
	return false
}

// detectOversized finds oversized resources
func (w *WasteAnalyzerImpl) detectOversized(resources []types.Resource) WastePattern {
	pattern := WastePattern{
		Type:        WasteOversized,
		Reason:      "Resources potentially oversized for workload",
		FirstSeen:   time.Now(),
		Confidence:  0.6,
		ResourceIDs: []string{},
	}

	var totalCost float64
	for _, r := range resources {
		if w.isOversized(r) {
			pattern.ResourceIDs = append(pattern.ResourceIDs, r.ID)
			if cost, ok := r.Metadata["monthly_cost_estimate"].(float64); ok {
				totalCost += cost * 0.3 // Estimate 30% waste
			}
		}
	}

	// Impact calculation removed - let FinOps tools handle
	return pattern
}

// isOversized checks if resource might be oversized
func (w *WasteAnalyzerImpl) isOversized(r types.Resource) bool {
	switch r.Type {
	case "ec2":
		// Large instances in dev/test environments
		if r.Tags.Environment == "dev" || r.Tags.Environment == "test" {
			instanceType := w.getMetadataString(r.Metadata, "instance_type")
			return w.isLargeInstance(instanceType)
		}
	case "rds", "aurora":
		// Multi-AZ in non-prod
		if r.Tags.Environment != "prod" && r.Tags.Environment != "production" {
			return w.checkMetadataBool(r.Metadata, "multi_az")
		}
	case "redshift":
		// Large clusters with few nodes
		nodeCount := w.checkMetadataInt(r.Metadata, "node_count")
		return nodeCount > 4 && r.Tags.Environment != "prod"
	}
	return false
}

// isLargeInstance checks if EC2 instance is large
func (w *WasteAnalyzerImpl) isLargeInstance(instanceType string) bool {
	largeTypes := []string{"xlarge", "2xlarge", "4xlarge", "8xlarge", "metal"}
	for _, t := range largeTypes {
		if strings.Contains(instanceType, t) {
			return true
		}
	}
	return false
}

// detectUnattached finds unattached resources
func (w *WasteAnalyzerImpl) detectUnattached(resources []types.Resource) WastePattern {
	pattern := WastePattern{
		Type:        WasteUnattached,
		Reason:      "Storage resources not attached to any instance",
		FirstSeen:   time.Now(),
		Confidence:  0.9,
		ResourceIDs: []string{},
	}

	var totalCost float64
	for _, r := range resources {
		if w.isUnattached(r) {
			pattern.ResourceIDs = append(pattern.ResourceIDs, r.ID)
			if cost, ok := r.Metadata["monthly_cost_estimate"].(float64); ok {
				totalCost += cost
			}
		}
	}

	// Impact calculation removed - let FinOps tools handle
	return pattern
}

// isUnattached checks if resource is unattached
func (w *WasteAnalyzerImpl) isUnattached(r types.Resource) bool {
	switch r.Type {
	case "ebs":
		return r.Status == "unattached" || !w.checkMetadataBool(r.Metadata, "is_attached")
	case "elastic_ip":
		return r.Status == "unassociated" || !w.checkMetadataBool(r.Metadata, "is_associated")
	case "network_interface":
		return r.Metadata["attachment"] == nil
	}
	return false
}

// detectObsolete finds obsolete resources
func (w *WasteAnalyzerImpl) detectObsolete(resources []types.Resource) WastePattern {
	pattern := WastePattern{
		Type:        WasteObsolete,
		Reason:      "Old snapshots, AMIs, and backups",
		FirstSeen:   time.Now(),
		Confidence:  0.75,
		ResourceIDs: []string{},
	}

	var totalCost float64
	for _, r := range resources {
		if w.isObsolete(r) {
			pattern.ResourceIDs = append(pattern.ResourceIDs, r.ID)
			if cost, ok := r.Metadata["monthly_cost_estimate"].(float64); ok {
				totalCost += cost
			}
		}
	}

	// Impact calculation removed - let FinOps tools handle
	return pattern
}

// isObsolete checks if resource is obsolete
func (w *WasteAnalyzerImpl) isObsolete(r types.Resource) bool {
	switch r.Type {
	case "snapshot", "ami", "rds_snapshot", "redshift_snapshot", "dynamodb_backup":
		ageDays := w.checkMetadataInt(r.Metadata, "age_days")
		isOld := w.checkMetadataBool(r.Metadata, "is_old")
		isTemp := w.checkMetadataBool(r.Metadata, "is_temp")

		// Old backups or temp resources
		return ageDays > 30 || isOld || isTemp
	}
	return false
}

// FindOrphans returns orphaned resources
func (w *WasteAnalyzerImpl) FindOrphans(ctx context.Context, since time.Time) ([]types.Resource, error) {
	resources, err := w.queryEngine.QueryByTimeRange(ctx, since, time.Now())
	if err != nil {
		return nil, err
	}

	var orphans []types.Resource
	for _, r := range resources {
		if w.isOrphaned(r) {
			orphans = append(orphans, r)
		}
	}

	return orphans, nil
}

// FindIdleResources returns idle resources
func (w *WasteAnalyzerImpl) FindIdleResources(ctx context.Context, idleThreshold time.Duration) ([]types.Resource, error) {
	resources, err := w.queryEngine.QueryByTimeRange(ctx, time.Now().Add(-idleThreshold), time.Now())
	if err != nil {
		return nil, err
	}

	var idle []types.Resource
	for _, r := range resources {
		if w.isIdle(r) {
			idle = append(idle, r)
		}
	}

	return idle, nil
}

// Helper methods for metadata access

func (w *WasteAnalyzerImpl) checkMetadataBool(meta ResourceMetadata, key string) bool {
	if val, ok := meta[key].(bool); ok {
		return val
	}
	return false
}

func (w *WasteAnalyzerImpl) checkMetadataInt(meta ResourceMetadata, key string) int {
	if val, ok := meta[key].(int); ok {
		return val
	}
	if val, ok := meta[key].(float64); ok {
		return int(val)
	}
	return 0
}

func (w *WasteAnalyzerImpl) getMetadataString(meta ResourceMetadata, key string) string {
	if val, ok := meta[key].(string); ok {
		return val
	}
	return ""
}
