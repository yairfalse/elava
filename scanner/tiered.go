package scanner

import (
	"fmt"
	"strings"
	"time"

	"github.com/yairfalse/elava/config"
	"github.com/yairfalse/elava/types"
)

// TieredScanner implements intelligent tiered scanning based on configuration
type TieredScanner struct {
	config     *config.Config
	lastScans  map[string]time.Time        // tier -> last scan time
	tierStates map[string][]types.Resource // tier -> resources
}

// NewTieredScanner creates a new tiered scanner
func NewTieredScanner(cfg *config.Config) *TieredScanner {
	return &TieredScanner{
		config:     cfg,
		lastScans:  make(map[string]time.Time),
		tierStates: make(map[string][]types.Resource),
	}
}

// ClassifyResource determines which tier a resource belongs to
func (ts *TieredScanner) ClassifyResource(resource types.Resource) string {
	// Check each tier in order (critical first)
	tierOrder := []string{"critical", "production", "standard", "archive"}

	for _, tierName := range tierOrder {
		tier, exists := ts.config.Scanning.Tiers[tierName]
		if !exists {
			continue
		}

		if ts.matchesTier(resource, tier) {
			return tierName
		}
	}

	// Default to standard if no match
	return "standard"
}

// matchesTier checks if a resource matches any pattern in a tier
func (ts *TieredScanner) matchesTier(resource types.Resource, tier config.TierConfig) bool {
	for _, pattern := range tier.Patterns {
		if ts.matchesPattern(resource, pattern) {
			return true
		}
	}
	return false
}

// matchesPattern checks if a resource matches a specific pattern
func (ts *TieredScanner) matchesPattern(resource types.Resource, pattern config.TierPattern) bool {
	if !ts.matchesType(resource, pattern) {
		return false
	}
	if !ts.matchesStatus(resource, pattern) {
		return false
	}
	if !ts.matchesTags(resource, pattern) {
		return false
	}
	if !ts.matchesInstanceType(resource, pattern) {
		return false
	}
	return ts.hasAnyPattern(pattern)
}

func (ts *TieredScanner) matchesType(resource types.Resource, pattern config.TierPattern) bool {
	if pattern.Type != "" && resource.Type != pattern.Type {
		return false
	}
	if len(pattern.Types) > 0 {
		for _, t := range pattern.Types {
			if resource.Type == t {
				return true
			}
		}
		return false
	}
	return true
}

func (ts *TieredScanner) matchesStatus(resource types.Resource, pattern config.TierPattern) bool {
	return pattern.Status == "" || resource.Status == pattern.Status
}

func (ts *TieredScanner) matchesTags(resource types.Resource, pattern config.TierPattern) bool {
	for key, value := range pattern.Tags {
		if resource.Tags.Get(key) != value {
			return false
		}
	}
	return true
}

func (ts *TieredScanner) matchesInstanceType(resource types.Resource, pattern config.TierPattern) bool {
	if pattern.InstanceTypePattern == "" || resource.Type != "ec2" {
		return true
	}

	instanceType := ts.getInstanceType(resource)
	return matchesGlob(instanceType, pattern.InstanceTypePattern)
}

func (ts *TieredScanner) getInstanceType(resource types.Resource) string {
	instanceType := resource.Tags.Get("instance_type")
	if instanceType != "" {
		return instanceType
	}

	if metadata, ok := resource.Metadata["instance_type"]; ok {
		if instanceTypeStr, ok := metadata.(string); ok {
			return instanceTypeStr
		}
	}
	return ""
}

func (ts *TieredScanner) hasAnyPattern(pattern config.TierPattern) bool {
	return pattern.Type != "" || len(pattern.Types) > 0 || pattern.Status != "" ||
		len(pattern.Tags) > 0 || pattern.InstanceTypePattern != ""
}

// matchesGlob performs simple glob matching (* wildcards)
func matchesGlob(text, pattern string) bool {
	if pattern == "*" {
		return true
	}

	if strings.Contains(pattern, "*") {
		// Simple wildcard matching
		parts := strings.Split(pattern, "*")
		if len(parts) == 2 {
			prefix := parts[0]
			suffix := parts[1]
			return strings.HasPrefix(text, prefix) && strings.HasSuffix(text, suffix)
		}
	}

	return text == pattern
}

// GetTiersDueForScan returns tiers that need scanning based on their intervals
func (ts *TieredScanner) GetTiersDueForScan() []string {
	now := time.Now()
	var dueFor []string

	// Adjust time for adaptive hours
	if ts.config.Scanning.AdaptiveHours && ts.isWorkingHours(now) {
		// During work hours, scan 2x more frequently
		now = now.Add(-30 * time.Minute)
	}

	for tierName, tier := range ts.config.Scanning.Tiers {
		lastScan, exists := ts.lastScans[tierName]
		if !exists || now.Sub(lastScan) >= tier.ScanInterval {
			dueFor = append(dueFor, tierName)
		}
	}

	return dueFor
}

// isWorkingHours checks if it's currently working hours (9 AM - 6 PM, weekdays)
func (ts *TieredScanner) isWorkingHours(t time.Time) bool {
	hour := t.Hour()
	weekday := t.Weekday()

	// 9 AM - 6 PM, Monday-Friday
	return hour >= 9 && hour <= 18 && weekday >= time.Monday && weekday <= time.Friday
}

// FilterResourcesByTier filters resources to only those belonging to specified tiers
func (ts *TieredScanner) FilterResourcesByTier(resources []types.Resource, tierNames []string) map[string][]types.Resource {
	tierMap := make(map[string]bool)
	for _, name := range tierNames {
		tierMap[name] = true
	}

	result := make(map[string][]types.Resource)

	for _, resource := range resources {
		tier := ts.ClassifyResource(resource)
		if tierMap[tier] {
			result[tier] = append(result[tier], resource)
		}
	}

	return result
}

// MarkTierScanned records that a tier was scanned at the current time
func (ts *TieredScanner) MarkTierScanned(tierName string, resources []types.Resource) {
	ts.lastScans[tierName] = time.Now()
	ts.tierStates[tierName] = resources
}

// GetScanSummary returns a summary of the current scanning state
func (ts *TieredScanner) GetScanSummary() ScanSummary {
	summary := ScanSummary{
		Tiers: make(map[string]TierSummary),
	}

	for tierName, tier := range ts.config.Scanning.Tiers {
		lastScan := ts.lastScans[tierName]
		resources := ts.tierStates[tierName]

		var nextScan time.Time
		if !lastScan.IsZero() {
			nextScan = lastScan.Add(tier.ScanInterval)
		}

		summary.Tiers[tierName] = TierSummary{
			Description:   tier.Description,
			ScanInterval:  tier.ScanInterval,
			LastScan:      lastScan,
			NextScan:      nextScan,
			ResourceCount: len(resources),
			DueForScan:    time.Now().After(nextScan),
		}
	}

	return summary
}

// ScanSummary provides an overview of tiered scanning status
type ScanSummary struct {
	Tiers map[string]TierSummary `json:"tiers"`
}

// TierSummary provides summary information for a single tier
type TierSummary struct {
	Description   string        `json:"description"`
	ScanInterval  time.Duration `json:"scan_interval"`
	LastScan      time.Time     `json:"last_scan"`
	NextScan      time.Time     `json:"next_scan"`
	ResourceCount int           `json:"resource_count"`
	DueForScan    bool          `json:"due_for_scan"`
}

// String returns a human-readable summary
func (ss ScanSummary) String() string {
	var result strings.Builder
	result.WriteString("Tiered Scanning Status:\n")

	tierOrder := []string{"critical", "production", "standard", "archive"}

	for _, tierName := range tierOrder {
		if tier, exists := ss.Tiers[tierName]; exists {
			status := "✓"
			if tier.DueForScan {
				status = "⏰"
			}

			result.WriteString(fmt.Sprintf("  %s %s: %s (%d resources)\n",
				status, tierName, tier.Description, tier.ResourceCount))

			if !tier.LastScan.IsZero() {
				result.WriteString(fmt.Sprintf("    Last: %s, Next: %s\n",
					tier.LastScan.Format("15:04"), tier.NextScan.Format("15:04")))
			} else {
				result.WriteString("    Never scanned\n")
			}
		}
	}

	return result.String()
}
