package scanner

import (
	"context"
	"strings"
	"time"

	"github.com/yairfalse/ovi/types"
)

// Tracker identifies untracked resources
type Tracker struct {
	rules []TrackingRule
}

// TrackingRule defines criteria for identifying untracked resources
type TrackingRule struct {
	Name        string
	Description string
	Check       TrackingCheckFunc
	Severity    string // "low", "medium", "high"
}

// TrackingCheckFunc checks if a resource is properly tracked
type TrackingCheckFunc func(resource *types.Resource) bool

// NewTracker creates a tracker with default rules
func NewTracker() *Tracker {
	return &Tracker{
		rules: []TrackingRule{
			{
				Name:        "missing_owner",
				Description: "No owner or team assigned",
				Check:       checkMissingOwner,
				Severity:    "high",
			},
			{
				Name:        "missing_project",
				Description: "No project or name tags",
				Check:       checkMissingProject,
				Severity:    "medium",
			},
			{
				Name:        "no_iac_management",
				Description: "Not managed by IaC tools",
				Check:       checkNoIaCManagement,
				Severity:    "medium",
			},
			{
				Name:        "dead_resource",
				Description: "Stopped/terminated but still exists",
				Check:       checkDeadResource,
				Severity:    "high",
			},
		},
	}
}

// FindUntracked analyzes resources and identifies untracked ones
func (t *Tracker) FindUntracked(ctx context.Context, resources []types.Resource) []UntrackedResource {
	var untracked []UntrackedResource

	for _, resource := range resources {
		if t.isUntracked(&resource) {
			untracked = append(untracked, UntrackedResource{
				Resource: resource,
				Issues:   t.getTrackingIssues(&resource),
				Risk:     t.assessRisk(&resource),
				Action:   t.recommendAction(&resource),
			})
		}
	}

	return untracked
}

// isUntracked checks if a resource lacks proper tracking
func (t *Tracker) isUntracked(resource *types.Resource) bool {
	for _, rule := range t.rules {
		if rule.Check(resource) {
			return true
		}
	}
	return false
}

// getTrackingIssues gets all tracking issues for a resource
func (t *Tracker) getTrackingIssues(resource *types.Resource) []string {
	var issues []string
	for _, rule := range t.rules {
		if rule.Check(resource) {
			issues = append(issues, rule.Description)
		}
	}
	return issues
}

// assessRisk determines risk level of untracked resource
func (t *Tracker) assessRisk(resource *types.Resource) string {
	// High risk: expensive resources (RDS, large EC2) or dead resources
	if resource.Type == "rds" || resource.Status == "stopped" {
		return "high"
	}

	// Medium risk: compute resources without clear ownership
	if resource.Type == "ec2" && resource.Tags.OviOwner == "" {
		return "medium"
	}

	return "low"
}

// recommendAction suggests what to do with untracked resource
func (t *Tracker) recommendAction(resource *types.Resource) string {
	if resource.Status == "stopped" || resource.Status == "terminated" {
		return "cleanup"
	}

	if resource.Tags.OviOwner == "" && resource.Tags.Team == "" {
		return "tag_owner"
	}

	if !hasIaCTags(resource) {
		return "verify_management"
	}

	return "investigate"
}

// UntrackedResource represents a resource that lacks proper tracking
type UntrackedResource struct {
	Resource types.Resource `json:"resource"`
	Issues   []string       `json:"issues"`
	Risk     string         `json:"risk"`   // "low", "medium", "high"
	Action   string         `json:"action"` // "investigate", "tag_owner", "cleanup", "verify_management"
}

// Tracking rule implementations

func checkMissingOwner(resource *types.Resource) bool {
	return resource.Tags.OviOwner == "" && resource.Tags.Team == ""
}

func checkMissingProject(resource *types.Resource) bool {
	return resource.Tags.Project == "" && resource.Tags.Name == ""
}

func checkNoIaCManagement(resource *types.Resource) bool {
	// Already managed by Ovi
	if resource.Tags.OviManaged {
		return false
	}

	return !hasIaCTags(resource)
}

func checkDeadResource(resource *types.Resource) bool {
	deadStates := []string{"stopped", "terminated", "shutting-down"}

	for _, state := range deadStates {
		if resource.Status == state {
			// If stopped for more than 7 days, likely untracked
			if !resource.CreatedAt.IsZero() && time.Since(resource.CreatedAt) > 7*24*time.Hour {
				return true
			}
		}
	}

	return false
}

// hasIaCTags checks if resource has Infrastructure as Code indicators
func hasIaCTags(resource *types.Resource) bool {
	// Check for common IaC tool indicators
	iacIndicators := []string{
		"terraform",
		"cloudformation",
		"cdk",
		"pulumi",
		"ansible",
		"managed-by",
		"created-by",
		"stack-name",
	}

	// Look in all tag fields
	allTagText := strings.ToLower(
		resource.Tags.Name + " " +
			resource.Tags.Project + " " +
			resource.Tags.Environment + " " +
			resource.Tags.OviOwner,
	)

	for _, indicator := range iacIndicators {
		if strings.Contains(allTagText, indicator) {
			return true
		}
	}

	return false
}

// ScanForUntracked is a convenience function
func ScanForUntracked(ctx context.Context, resources []types.Resource) []UntrackedResource {
	tracker := NewTracker()
	return tracker.FindUntracked(ctx, resources)
}
