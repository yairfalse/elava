package scanner

import (
	"testing"
	"time"

	"github.com/yairfalse/elava/config"
	"github.com/yairfalse/elava/types"
)

func TestTieredScanner_ClassifyResource(t *testing.T) {
	cfg := config.LoadDefault()
	scanner := NewTieredScanner(cfg)

	tests := []struct {
		name     string
		resource types.Resource
		want     string
	}{
		{
			name: "production RDS should be critical",
			resource: types.Resource{
				Type: "rds",
				Tags: types.Tags{Environment: "production"},
			},
			want: "critical",
		},
		{
			name: "NAT gateway should be critical",
			resource: types.Resource{
				Type: "nat_gateway",
			},
			want: "critical",
		},
		{
			name: "large EC2 should be critical",
			resource: types.Resource{
				Type: "ec2",
				Metadata: types.ResourceMetadata{
					InstanceType: "m5.xlarge",
				},
			},
			want: "critical",
		},
		{
			name: "production EC2 should be production tier",
			resource: types.Resource{
				Type: "ec2",
				Tags: types.Tags{Environment: "production"},
				Metadata: types.ResourceMetadata{
					InstanceType: "t3.micro",
				},
			},
			want: "production",
		},
		{
			name: "development resource should be standard",
			resource: types.Resource{
				Type: "ec2",
				Tags: types.Tags{Environment: "development"},
			},
			want: "standard",
		},
		{
			name: "stopped instance should be archive",
			resource: types.Resource{
				Type:   "ec2",
				Status: "stopped",
			},
			want: "archive",
		},
		{
			name: "snapshot should be archive",
			resource: types.Resource{
				Type: "snapshot",
			},
			want: "archive",
		},
		{
			name: "untagged resource should be standard",
			resource: types.Resource{
				Type: "s3",
			},
			want: "standard",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := scanner.ClassifyResource(tt.resource)
			if got != tt.want {
				t.Errorf("ClassifyResource() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestTieredScanner_GetTiersDueForScan(t *testing.T) {
	cfg := config.LoadDefault()
	scanner := NewTieredScanner(cfg)

	// Initially, all tiers should be due for scan
	due := scanner.GetTiersDueForScan()
	if len(due) == 0 {
		t.Error("Expected some tiers to be due for scan initially")
	}

	// Mark a tier as scanned
	scanner.MarkTierScanned("critical", []types.Resource{})

	// Check that critical tier is no longer due (but others still are)
	due = scanner.GetTiersDueForScan()
	for _, tier := range due {
		if tier == "critical" {
			t.Error("Critical tier should not be due immediately after scanning")
		}
	}
}

func TestTieredScanner_FilterResourcesByTier(t *testing.T) {
	cfg := config.LoadDefault()
	scanner := NewTieredScanner(cfg)

	resources := []types.Resource{
		{Type: "rds", Tags: types.Tags{Environment: "production"}},  // critical
		{Type: "ec2", Tags: types.Tags{Environment: "production"}},  // production
		{Type: "ec2", Tags: types.Tags{Environment: "development"}}, // standard
		{Type: "snapshot"}, // archive
	}

	// Filter for critical and production tiers only
	filtered := scanner.FilterResourcesByTier(resources, []string{"critical", "production"})

	if len(filtered["critical"]) != 1 {
		t.Errorf("Expected 1 critical resource, got %d", len(filtered["critical"]))
	}
	if len(filtered["production"]) != 1 {
		t.Errorf("Expected 1 production resource, got %d", len(filtered["production"]))
	}
	if len(filtered["standard"]) != 0 {
		t.Errorf("Expected 0 standard resources, got %d", len(filtered["standard"]))
	}
	if len(filtered["archive"]) != 0 {
		t.Errorf("Expected 0 archive resources, got %d", len(filtered["archive"]))
	}
}

func TestTieredScanner_IsWorkingHours(t *testing.T) {
	cfg := config.LoadDefault()
	scanner := NewTieredScanner(cfg)

	// Monday 10 AM should be working hours
	monday10AM := time.Date(2024, 1, 8, 10, 0, 0, 0, time.UTC)
	if !scanner.isWorkingHours(monday10AM) {
		t.Error("Monday 10 AM should be working hours")
	}

	// Saturday 10 AM should not be working hours
	saturday10AM := time.Date(2024, 1, 6, 10, 0, 0, 0, time.UTC)
	if scanner.isWorkingHours(saturday10AM) {
		t.Error("Saturday 10 AM should not be working hours")
	}

	// Monday 8 AM should not be working hours (before 9 AM)
	monday8AM := time.Date(2024, 1, 8, 8, 0, 0, 0, time.UTC)
	if scanner.isWorkingHours(monday8AM) {
		t.Error("Monday 8 AM should not be working hours")
	}

	// Monday 7 PM should not be working hours (after 6 PM)
	monday7PM := time.Date(2024, 1, 8, 19, 0, 0, 0, time.UTC)
	if scanner.isWorkingHours(monday7PM) {
		t.Error("Monday 7 PM should not be working hours")
	}
}

func TestMatchesGlob(t *testing.T) {
	tests := []struct {
		text    string
		pattern string
		want    bool
	}{
		{"m5.xlarge", "*xlarge", true},
		{"t3.micro", "*xlarge", false},
		{"r5.2xlarge", "*xlarge", true},
		{"anything", "*", true},
		{"exact", "exact", true},
		{"not-exact", "exact", false},
	}

	for _, tt := range tests {
		t.Run(tt.text+"_"+tt.pattern, func(t *testing.T) {
			got := matchesGlob(tt.text, tt.pattern)
			if got != tt.want {
				t.Errorf("matchesGlob(%q, %q) = %v, want %v", tt.text, tt.pattern, got, tt.want)
			}
		})
	}
}

func TestTieredScanner_GetScanSummary(t *testing.T) {
	cfg := config.LoadDefault()
	scanner := NewTieredScanner(cfg)

	// Mark some tiers as scanned
	scanner.MarkTierScanned("critical", []types.Resource{{Type: "rds"}})
	scanner.MarkTierScanned("production", []types.Resource{{Type: "ec2"}, {Type: "s3"}})

	summary := scanner.GetScanSummary()

	// Check that we have tiers in the summary
	if len(summary.Tiers) == 0 {
		t.Error("Expected tiers in summary")
	}

	// Check critical tier
	if critical, exists := summary.Tiers["critical"]; exists {
		if critical.ResourceCount != 1 {
			t.Errorf("Expected 1 critical resource, got %d", critical.ResourceCount)
		}
		if critical.LastScan.IsZero() {
			t.Error("Expected critical tier to have a last scan time")
		}
	} else {
		t.Error("Expected critical tier in summary")
	}

	// Check production tier
	if production, exists := summary.Tiers["production"]; exists {
		if production.ResourceCount != 2 {
			t.Errorf("Expected 2 production resources, got %d", production.ResourceCount)
		}
	} else {
		t.Error("Expected production tier in summary")
	}
}
