package reconciler

import (
	"fmt"
	"strings"
	"time"

	"github.com/yairfalse/elava/types"
)

// BaselineSummary contains summary information from a baseline scan
type BaselineSummary struct {
	Total          int
	ByType         map[string]int
	ByEnvironment  map[string]int
	Untagged       []string
	NoOwner        []string
	OldestResource time.Time
	NewestResource time.Time
}

// SummarizeBaseline analyzes resources and generates baseline summary
func SummarizeBaseline(resources []types.Resource) BaselineSummary {
	summary := BaselineSummary{
		Total:         len(resources),
		ByType:        make(map[string]int),
		ByEnvironment: make(map[string]int),
	}

	for _, r := range resources {
		// Count by type
		summary.ByType[r.Type]++

		// Count by environment (or "unknown")
		env := r.Tags.Environment
		if env == "" {
			env = "unknown"
		}
		summary.ByEnvironment[env]++

		// Flag completely untagged
		if len(r.Tags.ToMap()) == 0 {
			summary.Untagged = append(summary.Untagged, r.ID)
		}

		// Flag missing owner
		if r.Tags.ElavaOwner == "" && r.Tags.Team == "" {
			summary.NoOwner = append(summary.NoOwner, r.ID)
		}

		// Track oldest/newest
		if summary.OldestResource.IsZero() || r.CreatedAt.Before(summary.OldestResource) {
			summary.OldestResource = r.CreatedAt
		}
		if summary.NewestResource.IsZero() || r.CreatedAt.After(summary.NewestResource) {
			summary.NewestResource = r.CreatedAt
		}
	}

	return summary
}

// FormatBaselineSummary generates formatted output for baseline summary
func (s BaselineSummary) FormatBaselineSummary() string {
	var b strings.Builder

	// Header
	b.WriteString("════════════════════════════════════════════════════\n")
	b.WriteString(fmt.Sprintf("  BASELINE SCAN COMPLETE - %d resources discovered\n", s.Total))
	b.WriteString("════════════════════════════════════════════════════\n\n")

	// Infrastructure age
	if !s.OldestResource.IsZero() && !s.NewestResource.IsZero() {
		b.WriteString("INFRASTRUCTURE AGE\n")
		b.WriteString(fmt.Sprintf("  Oldest resource created %s\n", formatAge(s.OldestResource)))
		b.WriteString(fmt.Sprintf("  Newest resource created %s\n\n", formatAge(s.NewestResource)))
	}

	// Resource breakdown
	if len(s.ByType) > 0 {
		b.WriteString("RESOURCE BREAKDOWN\n")
		for resType, count := range s.ByType {
			b.WriteString(fmt.Sprintf("  %3d  %s\n", count, formatResourceType(resType)))
		}
		b.WriteString("\n")
	}

	// Environment distribution
	if len(s.ByEnvironment) > 0 {
		b.WriteString("ENVIRONMENT DISTRIBUTION\n")
		for env, count := range s.ByEnvironment {
			if env == "unknown" {
				b.WriteString(fmt.Sprintf("  %3d  (no environment tag)\n", count))
			} else {
				b.WriteString(fmt.Sprintf("  %3d  %s\n", count, env))
			}
		}
		b.WriteString("\n")
	}

	// Tagging status
	b.WriteString("TAGGING STATUS\n")
	b.WriteString(fmt.Sprintf("  %3d  resources have no tags\n", len(s.Untagged)))
	b.WriteString(fmt.Sprintf("  %3d  resources missing owner\n\n", len(s.NoOwner)))

	// Footer
	b.WriteString("────────────────────────────────────────────────────\n")
	b.WriteString("Baseline saved. Next scan will detect changes.\n")
	b.WriteString("════════════════════════════════════════════════════\n")

	return b.String()
}

// formatAge converts a timestamp to human-readable age
func formatAge(t time.Time) string {
	duration := time.Since(t)

	years := int(duration.Hours() / 24 / 365)
	if years > 0 {
		months := int(duration.Hours()/24/30) % 12
		if months > 0 {
			return fmt.Sprintf("%d years, %d months ago", years, months)
		}
		return fmt.Sprintf("%d years ago", years)
	}

	months := int(duration.Hours() / 24 / 30)
	if months > 0 {
		return fmt.Sprintf("%d months ago", months)
	}

	days := int(duration.Hours() / 24)
	if days > 0 {
		return fmt.Sprintf("%d days ago", days)
	}

	hours := int(duration.Hours())
	if hours > 0 {
		return fmt.Sprintf("%d hours ago", hours)
	}

	return "just now"
}

// formatResourceType converts resource type to display name
func formatResourceType(resType string) string {
	displayNames := map[string]string{
		"ec2": "EC2 Instances",
		"rds": "RDS Databases",
		"s3":  "S3 Buckets",
		"vpc": "VPC Networks",
		"sg":  "Security Groups",
		"ebs": "EBS Volumes",
		"elb": "Load Balancers",
		"r53": "Route53 Zones",
	}

	if name, ok := displayNames[resType]; ok {
		return name
	}
	return resType
}
