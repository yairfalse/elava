package aws

import (
	"context"
	"testing"
	"time"

	"github.com/yairfalse/ovi/providers"
	"github.com/yairfalse/ovi/types"
)

// MockEC2Client for testing
type MockEC2Client struct {
	instances []EC2Instance
	tagCalls  []TagOperation
}

// Using EC2Instance from ec2.go

// Using TagOperation from types.go

func (m *MockEC2Client) DescribeInstances(ctx context.Context, filter InstanceFilter) ([]EC2Instance, error) {
	var result []EC2Instance
	for _, instance := range m.instances {
		if m.matchesFilter(instance, filter) {
			result = append(result, instance)
		}
	}
	return result, nil
}

func (m *MockEC2Client) RunInstances(ctx context.Context, spec InstanceSpec) (*EC2Instance, error) {
	instance := EC2Instance{
		InstanceID:   "i-newinstance123",
		InstanceType: spec.InstanceType,
		State:        "running",
		LaunchTime:   time.Now(),
		Tags:         spec.Tags,
	}
	m.instances = append(m.instances, instance)
	return &instance, nil
}

func (m *MockEC2Client) TerminateInstances(ctx context.Context, instanceIDs []string) error {
	for i, instance := range m.instances {
		for _, id := range instanceIDs {
			if instance.InstanceID == id {
				m.instances[i].State = "terminating"
			}
		}
	}
	return nil
}

func (m *MockEC2Client) CreateTags(ctx context.Context, instanceID string, tags map[string]string) error {
	m.tagCalls = append(m.tagCalls, TagOperation{
		InstanceID: instanceID,
		Tags:       tags,
	})

	// Update instance tags
	for i, instance := range m.instances {
		if instance.InstanceID == instanceID {
			if m.instances[i].Tags == nil {
				m.instances[i].Tags = make(map[string]string)
			}
			for k, v := range tags {
				m.instances[i].Tags[k] = v
			}
		}
	}
	return nil
}

func (m *MockEC2Client) matchesFilter(instance EC2Instance, filter InstanceFilter) bool {
	// Check state filter
	if len(filter.States) > 0 {
		found := false
		for _, state := range filter.States {
			if instance.State == state {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}

	// Check tag filters
	for key, value := range filter.Tags {
		if instance.Tags[key] != value {
			return false
		}
	}

	return true
}

func TestAWSProvider_ListResources(t *testing.T) {
	mockClient := &MockEC2Client{
		instances: []EC2Instance{
			{
				InstanceID:   "i-123456",
				InstanceType: "t3.micro",
				State:        "running",
				LaunchTime:   time.Now().Add(-time.Hour),
				Tags: map[string]string{
					"Name":      "web-server",
					"ovi:owner": "team-web",
				},
			},
			{
				InstanceID:   "i-789012",
				InstanceType: "t3.small",
				State:        "stopped",
				LaunchTime:   time.Now().Add(-2 * time.Hour),
				Tags: map[string]string{
					"Name": "test-server",
				},
			},
		},
	}

	provider := &AWSProvider{
		client: mockClient,
		region: "us-east-1",
	}

	ctx := context.Background()

	// Test list all resources
	resources, err := provider.ListResources(ctx, types.ResourceFilter{})
	if err != nil {
		t.Fatalf("ListResources() error = %v", err)
	}

	if len(resources) != 2 {
		t.Errorf("ListResources() returned %d resources, want 2", len(resources))
	}

	// Verify first resource
	r := resources[0]
	if r.ID != "i-123456" {
		t.Errorf("Resource ID = %v, want i-123456", r.ID)
	}
	if r.Type != "ec2" {
		t.Errorf("Resource Type = %v, want ec2", r.Type)
	}
	if r.Provider != "aws" {
		t.Errorf("Resource Provider = %v, want aws", r.Provider)
	}
	if !r.IsManaged() {
		t.Error("Resource should be managed (has ovi:owner tag)")
	}
}

func TestAWSProvider_ListResources_WithFilter(t *testing.T) {
	mockClient := &MockEC2Client{
		instances: []EC2Instance{
			{
				InstanceID: "i-managed",
				State:      "running",
				Tags: map[string]string{
					"ovi:owner": "team-web",
				},
			},
			{
				InstanceID: "i-unmanaged",
				State:      "running",
				Tags: map[string]string{
					"Name": "external-server",
				},
			},
		},
	}

	provider := &AWSProvider{
		client: mockClient,
		region: "us-east-1",
	}

	ctx := context.Background()

	// Test filter by tags
	filter := types.ResourceFilter{
		Tags: map[string]string{
			"ovi:owner": "team-web",
		},
	}

	resources, err := provider.ListResources(ctx, filter)
	if err != nil {
		t.Fatalf("ListResources() error = %v", err)
	}

	if len(resources) != 1 {
		t.Errorf("ListResources() returned %d resources, want 1", len(resources))
	}

	if resources[0].ID != "i-managed" {
		t.Errorf("Resource ID = %v, want i-managed", resources[0].ID)
	}
}

func TestAWSProvider_CreateResource(t *testing.T) {
	mockClient := &MockEC2Client{}

	provider := &AWSProvider{
		client: mockClient,
		region: "us-east-1",
	}

	ctx := context.Background()
	spec := types.ResourceSpec{
		Type: "ec2",
		Size: "t3.micro",
		Tags: map[string]string{
			"ovi:owner": "team-web",
			"env":       "test",
		},
	}

	resource, err := provider.CreateResource(ctx, spec)
	if err != nil {
		t.Fatalf("CreateResource() error = %v", err)
	}

	if resource.Type != "ec2" {
		t.Errorf("Resource Type = %v, want ec2", resource.Type)
	}
	if resource.Provider != "aws" {
		t.Errorf("Resource Provider = %v, want aws", resource.Provider)
	}
	if !resource.IsManaged() {
		t.Error("Created resource should be managed")
	}

	// Verify instance was created in mock
	if len(mockClient.instances) != 1 {
		t.Errorf("Expected 1 instance created, got %d", len(mockClient.instances))
	}
}

func TestAWSProvider_TagResource(t *testing.T) {
	mockClient := &MockEC2Client{
		instances: []EC2Instance{
			{
				InstanceID: "i-123456",
				State:      "running",
				Tags:       map[string]string{"Name": "test"},
			},
		},
	}

	provider := &AWSProvider{
		client: mockClient,
		region: "us-east-1",
	}

	ctx := context.Background()
	tags := map[string]string{
		"ovi:managed": "true",
		"env":         "prod",
	}

	err := provider.TagResource(ctx, "i-123456", tags)
	if err != nil {
		t.Fatalf("TagResource() error = %v", err)
	}

	// Verify tag call was made
	if len(mockClient.tagCalls) != 1 {
		t.Errorf("Expected 1 tag call, got %d", len(mockClient.tagCalls))
	}

	call := mockClient.tagCalls[0]
	if call.InstanceID != "i-123456" {
		t.Errorf("Tag call instance ID = %v, want i-123456", call.InstanceID)
	}
	if call.Tags["ovi:managed"] != "true" {
		t.Errorf("Tag call missing ovi:managed tag")
	}
}

func TestAWSProvider_DeleteResource(t *testing.T) {
	mockClient := &MockEC2Client{
		instances: []EC2Instance{
			{
				InstanceID: "i-123456",
				State:      "running",
			},
		},
	}

	provider := &AWSProvider{
		client: mockClient,
		region: "us-east-1",
	}

	ctx := context.Background()
	err := provider.DeleteResource(ctx, "i-123456")
	if err != nil {
		t.Fatalf("DeleteResource() error = %v", err)
	}

	// Verify instance state changed to terminating
	if mockClient.instances[0].State != "terminating" {
		t.Errorf("Instance state = %v, want terminating", mockClient.instances[0].State)
	}
}

func TestAWSProvider_Interface(t *testing.T) {
	// Ensure AWSProvider implements CloudProvider
	var _ providers.CloudProvider = (*AWSProvider)(nil)

	provider := &AWSProvider{
		client: &MockEC2Client{},
		region: "us-west-2",
	}

	if provider.Name() != "aws" {
		t.Errorf("Name() = %v, want aws", provider.Name())
	}
	if provider.Region() != "us-west-2" {
		t.Errorf("Region() = %v, want us-west-2", provider.Region())
	}
}
