package filter

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/yairfalse/elava/pkg/resource"
)

func TestShouldScanType_NoExclusions(t *testing.T) {
	f := New(nil, nil, nil)
	assert.True(t, f.ShouldScanType("ec2"))
	assert.True(t, f.ShouldScanType("rds"))
}

func TestShouldScanType_WithExclusions(t *testing.T) {
	f := New([]string{"iam_role", "cloudwatch_logs"}, nil, nil)
	assert.True(t, f.ShouldScanType("ec2"))
	assert.True(t, f.ShouldScanType("rds"))
	assert.False(t, f.ShouldScanType("iam_role"))
	assert.False(t, f.ShouldScanType("cloudwatch_logs"))
}

func TestShouldIncludeResource_NoFilters(t *testing.T) {
	f := New(nil, nil, nil)
	r := resource.Resource{
		ID:     "i-123",
		Type:   "ec2",
		Labels: map[string]string{"env": "prod"},
	}
	assert.True(t, f.ShouldIncludeResource(r))
}

func TestShouldIncludeResource_IncludeTags_Match(t *testing.T) {
	f := New(nil, map[string]string{"env": "prod"}, nil)
	r := resource.Resource{
		ID:     "i-123",
		Type:   "ec2",
		Labels: map[string]string{"env": "prod", "team": "platform"},
	}
	assert.True(t, f.ShouldIncludeResource(r))
}

func TestShouldIncludeResource_IncludeTags_NoMatch(t *testing.T) {
	f := New(nil, map[string]string{"env": "prod"}, nil)
	r := resource.Resource{
		ID:     "i-123",
		Type:   "ec2",
		Labels: map[string]string{"env": "staging"},
	}
	assert.False(t, f.ShouldIncludeResource(r))
}

func TestShouldIncludeResource_IncludeTags_MultipleRequired(t *testing.T) {
	f := New(nil, map[string]string{"env": "prod", "team": "platform"}, nil)

	// Has both tags - should include
	r1 := resource.Resource{
		ID:     "i-123",
		Type:   "ec2",
		Labels: map[string]string{"env": "prod", "team": "platform"},
	}
	assert.True(t, f.ShouldIncludeResource(r1))

	// Missing one tag - should exclude
	r2 := resource.Resource{
		ID:     "i-456",
		Type:   "ec2",
		Labels: map[string]string{"env": "prod"},
	}
	assert.False(t, f.ShouldIncludeResource(r2))
}

func TestShouldIncludeResource_ExcludeTags_Match(t *testing.T) {
	f := New(nil, nil, map[string]string{"do-not-scan": "true"})
	r := resource.Resource{
		ID:     "i-123",
		Type:   "ec2",
		Labels: map[string]string{"do-not-scan": "true"},
	}
	assert.False(t, f.ShouldIncludeResource(r))
}

func TestShouldIncludeResource_ExcludeTags_NoMatch(t *testing.T) {
	f := New(nil, nil, map[string]string{"do-not-scan": "true"})
	r := resource.Resource{
		ID:     "i-123",
		Type:   "ec2",
		Labels: map[string]string{"env": "prod"},
	}
	assert.True(t, f.ShouldIncludeResource(r))
}

func TestShouldIncludeResource_ExcludeTags_AnyMatch(t *testing.T) {
	// If ANY exclude tag matches, resource is excluded
	f := New(nil, nil, map[string]string{"skip": "true", "ignore": "yes"})

	// Matches first exclude tag
	r1 := resource.Resource{
		ID:     "i-123",
		Type:   "ec2",
		Labels: map[string]string{"skip": "true"},
	}
	assert.False(t, f.ShouldIncludeResource(r1))

	// Matches second exclude tag
	r2 := resource.Resource{
		ID:     "i-456",
		Type:   "ec2",
		Labels: map[string]string{"ignore": "yes"},
	}
	assert.False(t, f.ShouldIncludeResource(r2))

	// Matches neither
	r3 := resource.Resource{
		ID:     "i-789",
		Type:   "ec2",
		Labels: map[string]string{"env": "prod"},
	}
	assert.True(t, f.ShouldIncludeResource(r3))
}

func TestShouldIncludeResource_BothIncludeAndExclude(t *testing.T) {
	// Include takes precedence - must match include AND not match exclude
	f := New(nil, map[string]string{"env": "prod"}, map[string]string{"skip": "true"})

	// Matches include, no exclude - included
	r1 := resource.Resource{
		ID:     "i-123",
		Type:   "ec2",
		Labels: map[string]string{"env": "prod"},
	}
	assert.True(t, f.ShouldIncludeResource(r1))

	// Matches include AND exclude - excluded
	r2 := resource.Resource{
		ID:     "i-456",
		Type:   "ec2",
		Labels: map[string]string{"env": "prod", "skip": "true"},
	}
	assert.False(t, f.ShouldIncludeResource(r2))

	// Doesn't match include - excluded
	r3 := resource.Resource{
		ID:     "i-789",
		Type:   "ec2",
		Labels: map[string]string{"env": "staging"},
	}
	assert.False(t, f.ShouldIncludeResource(r3))
}

func TestShouldIncludeResource_EmptyLabels(t *testing.T) {
	f := New(nil, map[string]string{"env": "prod"}, nil)
	r := resource.Resource{
		ID:     "i-123",
		Type:   "ec2",
		Labels: map[string]string{},
	}
	assert.False(t, f.ShouldIncludeResource(r))
}

func TestShouldIncludeResource_NilLabels(t *testing.T) {
	f := New(nil, map[string]string{"env": "prod"}, nil)
	r := resource.Resource{
		ID:     "i-123",
		Type:   "ec2",
		Labels: nil,
	}
	assert.False(t, f.ShouldIncludeResource(r))
}

func TestFilterResources(t *testing.T) {
	f := New(nil, map[string]string{"env": "prod"}, nil)
	resources := []resource.Resource{
		{ID: "i-1", Labels: map[string]string{"env": "prod"}},
		{ID: "i-2", Labels: map[string]string{"env": "staging"}},
		{ID: "i-3", Labels: map[string]string{"env": "prod"}},
	}

	filtered := f.FilterResources(resources)
	assert.Len(t, filtered, 2)
	assert.Equal(t, "i-1", filtered[0].ID)
	assert.Equal(t, "i-3", filtered[1].ID)
}

func TestIsEmpty(t *testing.T) {
	assert.True(t, New(nil, nil, nil).IsEmpty())
	assert.False(t, New([]string{"ec2"}, nil, nil).IsEmpty())
	assert.False(t, New(nil, map[string]string{"env": "prod"}, nil).IsEmpty())
	assert.False(t, New(nil, nil, map[string]string{"skip": "true"}).IsEmpty())
}
