package types

import (
	"testing"
	"time"
)

func TestResource_IsManaged(t *testing.T) {
	tests := []struct {
		name     string
		resource Resource
		want     bool
	}{
		{
			name: "managed with owner tag",
			resource: Resource{
				Tags: map[string]string{"ovi:owner": "team-web"},
			},
			want: true,
		},
		{
			name: "managed with managed tag",
			resource: Resource{
				Tags: map[string]string{"ovi:managed": "true"},
			},
			want: true,
		},
		{
			name: "not managed - no ovi tags",
			resource: Resource{
				Tags: map[string]string{"Name": "test"},
			},
			want: false,
		},
		{
			name:     "not managed - nil tags",
			resource: Resource{Tags: nil},
			want:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.resource.IsManaged(); got != tt.want {
				t.Errorf("IsManaged() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestResource_IsBlessed(t *testing.T) {
	tests := []struct {
		name     string
		resource Resource
		want     bool
	}{
		{
			name: "blessed resource",
			resource: Resource{
				Tags: map[string]string{"ovi:blessed": "true"},
			},
			want: true,
		},
		{
			name: "not blessed - false value",
			resource: Resource{
				Tags: map[string]string{"ovi:blessed": "false"},
			},
			want: false,
		},
		{
			name: "not blessed - no tag",
			resource: Resource{
				Tags: map[string]string{"Name": "test"},
			},
			want: false,
		},
		{
			name:     "not blessed - nil tags",
			resource: Resource{Tags: nil},
			want:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.resource.IsBlessed(); got != tt.want {
				t.Errorf("IsBlessed() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestResource_Matches(t *testing.T) {
	testResource := Resource{
		ID:       "i-123456",
		Type:     "ec2",
		Provider: "aws",
		Region:   "us-east-1",
		Tags: map[string]string{
			"env":  "prod",
			"team": "platform",
		},
	}

	tests := []struct {
		name   string
		filter ResourceFilter
		want   bool
	}{
		{
			name:   "matches type",
			filter: ResourceFilter{Type: "ec2"},
			want:   true,
		},
		{
			name:   "no match - wrong type",
			filter: ResourceFilter{Type: "rds"},
			want:   false,
		},
		{
			name:   "matches region",
			filter: ResourceFilter{Region: "us-east-1"},
			want:   true,
		},
		{
			name:   "matches provider",
			filter: ResourceFilter{Provider: "aws"},
			want:   true,
		},
		{
			name:   "matches ID in list",
			filter: ResourceFilter{IDs: []string{"i-123456", "i-789"}},
			want:   true,
		},
		{
			name:   "no match - ID not in list",
			filter: ResourceFilter{IDs: []string{"i-789", "i-456"}},
			want:   false,
		},
		{
			name:   "matches tags",
			filter: ResourceFilter{Tags: map[string]string{"env": "prod"}},
			want:   true,
		},
		{
			name:   "no match - wrong tag value",
			filter: ResourceFilter{Tags: map[string]string{"env": "dev"}},
			want:   false,
		},
		{
			name:   "matches multiple criteria",
			filter: ResourceFilter{Type: "ec2", Region: "us-east-1", Tags: map[string]string{"team": "platform"}},
			want:   true,
		},
		{
			name:   "empty filter matches all",
			filter: ResourceFilter{},
			want:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := testResource.Matches(tt.filter); got != tt.want {
				t.Errorf("Matches() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestResourceCreation(t *testing.T) {
	r := Resource{
		ID:        "i-123456",
		Type:      "ec2",
		Provider:  "aws",
		Region:    "us-east-1",
		Name:      "web-server-1",
		Status:    "running",
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		Tags: map[string]string{
			"ovi:owner": "team-web",
			"env":       "prod",
		},
	}

	if r.ID == "" {
		t.Error("Resource must have ID")
	}
	if r.Type == "" {
		t.Error("Resource must have Type")
	}
	if r.Provider == "" {
		t.Error("Resource must have Provider")
	}
}
