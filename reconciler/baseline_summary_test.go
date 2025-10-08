package reconciler

import (
	"strings"
	"testing"
	"time"

	"github.com/yairfalse/elava/types"
)

func TestSummarizeBaseline(t *testing.T) {
	now := time.Now()
	oldTime := now.Add(-3 * 365 * 24 * time.Hour) // 3 years ago
	newTime := now.Add(-2 * time.Hour)            // 2 hours ago

	resources := []types.Resource{
		{
			ID:        "i-123",
			Type:      "ec2",
			CreatedAt: oldTime,
			Tags: types.Tags{
				Environment: "production",
				ElavaOwner:  "team-infra",
			},
		},
		{
			ID:        "i-456",
			Type:      "ec2",
			CreatedAt: newTime,
			Tags: types.Tags{
				Environment: "staging",
				Team:        "team-web",
			},
		},
		{
			ID:        "db-789",
			Type:      "rds",
			CreatedAt: now.Add(-30 * 24 * time.Hour),
			Tags:      types.Tags{}, // No tags
		},
		{
			ID:        "s3-abc",
			Type:      "s3",
			CreatedAt: now.Add(-60 * 24 * time.Hour),
			Tags: types.Tags{
				Environment: "production",
				// No owner
			},
		},
	}

	summary := SummarizeBaseline(resources)

	// Test total count
	if summary.Total != 4 {
		t.Errorf("expected Total=4, got %d", summary.Total)
	}

	// Test by type
	if summary.ByType["ec2"] != 2 {
		t.Errorf("expected 2 EC2 instances, got %d", summary.ByType["ec2"])
	}
	if summary.ByType["rds"] != 1 {
		t.Errorf("expected 1 RDS database, got %d", summary.ByType["rds"])
	}
	if summary.ByType["s3"] != 1 {
		t.Errorf("expected 1 S3 bucket, got %d", summary.ByType["s3"])
	}

	// Test by environment
	if summary.ByEnvironment["production"] != 2 {
		t.Errorf("expected 2 production resources, got %d", summary.ByEnvironment["production"])
	}
	if summary.ByEnvironment["staging"] != 1 {
		t.Errorf("expected 1 staging resource, got %d", summary.ByEnvironment["staging"])
	}
	if summary.ByEnvironment["unknown"] != 1 {
		t.Errorf("expected 1 unknown environment, got %d", summary.ByEnvironment["unknown"])
	}

	// Test untagged
	if len(summary.Untagged) != 1 {
		t.Errorf("expected 1 untagged resource, got %d", len(summary.Untagged))
	}
	if len(summary.Untagged) > 0 && summary.Untagged[0] != "db-789" {
		t.Errorf("expected db-789 to be untagged, got %s", summary.Untagged[0])
	}

	// Test no owner
	if len(summary.NoOwner) != 2 {
		t.Errorf("expected 2 resources without owner, got %d", len(summary.NoOwner))
	}

	// Test age tracking
	if !summary.OldestResource.Equal(oldTime) {
		t.Errorf("expected oldest resource time %v, got %v", oldTime, summary.OldestResource)
	}
	if !summary.NewestResource.Equal(newTime) {
		t.Errorf("expected newest resource time %v, got %v", newTime, summary.NewestResource)
	}
}

func TestFormatBaselineSummary(t *testing.T) {
	now := time.Now()
	summary := BaselineSummary{
		Total: 100,
		ByType: map[string]int{
			"ec2": 60,
			"rds": 25,
			"s3":  15,
		},
		ByEnvironment: map[string]int{
			"production":  50,
			"staging":     30,
			"development": 15,
			"unknown":     5,
		},
		Untagged:       make([]string, 10),
		NoOwner:        make([]string, 20),
		OldestResource: now.Add(-2 * 365 * 24 * time.Hour),
		NewestResource: now.Add(-1 * time.Hour),
	}

	output := summary.FormatBaselineSummary()

	// Check header
	if !strings.Contains(output, "BASELINE SCAN COMPLETE - 100 resources discovered") {
		t.Error("output should contain header with resource count")
	}

	// Check sections
	if !strings.Contains(output, "INFRASTRUCTURE AGE") {
		t.Error("output should contain infrastructure age section")
	}
	if !strings.Contains(output, "RESOURCE BREAKDOWN") {
		t.Error("output should contain resource breakdown section")
	}
	if !strings.Contains(output, "ENVIRONMENT DISTRIBUTION") {
		t.Error("output should contain environment distribution section")
	}
	if !strings.Contains(output, "TAGGING STATUS") {
		t.Error("output should contain tagging status section")
	}

	// Check resource types appear
	if !strings.Contains(output, "EC2 Instances") {
		t.Error("output should contain EC2 Instances")
	}
	if !strings.Contains(output, "RDS Databases") {
		t.Error("output should contain RDS Databases")
	}

	// Check tagging counts
	if !strings.Contains(output, "10  resources have no tags") {
		t.Error("output should show untagged count")
	}
	if !strings.Contains(output, "20  resources missing owner") {
		t.Error("output should show no-owner count")
	}

	// Check footer
	if !strings.Contains(output, "Baseline saved. Next scan will detect changes.") {
		t.Error("output should contain footer message")
	}
}

func TestFormatAge(t *testing.T) {
	now := time.Now()

	tests := []struct {
		name     string
		time     time.Time
		expected string
	}{
		{
			name:     "3 years ago",
			time:     now.Add(-3 * 365 * 24 * time.Hour),
			expected: "3 years ago",
		},
		{
			name:     "1 year 2 months ago",
			time:     now.Add(-14 * 30 * 24 * time.Hour),
			expected: "1 years, 2 months ago",
		},
		{
			name:     "3 months ago",
			time:     now.Add(-3 * 30 * 24 * time.Hour),
			expected: "3 months ago",
		},
		{
			name:     "15 days ago",
			time:     now.Add(-15 * 24 * time.Hour),
			expected: "15 days ago",
		},
		{
			name:     "5 hours ago",
			time:     now.Add(-5 * time.Hour),
			expected: "5 hours ago",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := formatAge(tt.time)
			if result != tt.expected {
				t.Errorf("formatAge() = %q, want %q", result, tt.expected)
			}
		})
	}
}

func TestFormatResourceType(t *testing.T) {
	tests := []struct {
		resType  string
		expected string
	}{
		{"ec2", "EC2 Instances"},
		{"rds", "RDS Databases"},
		{"s3", "S3 Buckets"},
		{"vpc", "VPC Networks"},
		{"sg", "Security Groups"},
		{"unknown", "unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.resType, func(t *testing.T) {
			result := formatResourceType(tt.resType)
			if result != tt.expected {
				t.Errorf("formatResourceType(%q) = %q, want %q", tt.resType, result, tt.expected)
			}
		})
	}
}
