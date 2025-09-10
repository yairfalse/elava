package aws

import (
	"context"
	"fmt"
	"time"

	"github.com/yairfalse/ovi/types"
)

// EC2Client interface for AWS EC2 operations
type EC2Client interface {
	DescribeInstances(ctx context.Context, filter InstanceFilter) ([]EC2Instance, error)
	RunInstances(ctx context.Context, spec InstanceSpec) (*EC2Instance, error)
	TerminateInstances(ctx context.Context, instanceIDs []string) error
	CreateTags(ctx context.Context, instanceID string, tags map[string]string) error
}

// Types moved to types.go

// AWSProvider implements CloudProvider for AWS
type AWSProvider struct {
	client EC2Client
	region string
}

// NewAWSProvider creates a new AWS provider
func NewAWSProvider(client EC2Client, region string) *AWSProvider {
	return &AWSProvider{
		client: client,
		region: region,
	}
}

// Name returns provider name
func (p *AWSProvider) Name() string {
	return "aws"
}

// Region returns provider region
func (p *AWSProvider) Region() string {
	return p.region
}

// ListResources lists EC2 instances matching filter
func (p *AWSProvider) ListResources(ctx context.Context, filter types.ResourceFilter) ([]types.Resource, error) {
	awsFilter := p.convertToInstanceFilter(filter)

	instances, err := p.client.DescribeInstances(ctx, awsFilter)
	if err != nil {
		return nil, fmt.Errorf("failed to describe instances: %w", err)
	}

	var resources []types.Resource
	for _, instance := range instances {
		resource := p.convertToResource(instance)
		if resource.Matches(filter) {
			resources = append(resources, resource)
		}
	}

	return resources, nil
}

// CreateResource creates a new EC2 instance
func (p *AWSProvider) CreateResource(ctx context.Context, spec types.ResourceSpec) (*types.Resource, error) {
	instanceSpec := InstanceSpec{
		InstanceType: p.getInstanceType(spec.Size),
		Tags:         spec.Tags,
	}

	instance, err := p.client.RunInstances(ctx, instanceSpec)
	if err != nil {
		return nil, fmt.Errorf("failed to run instance: %w", err)
	}

	resource := p.convertToResource(*instance)
	return &resource, nil
}

// DeleteResource terminates an EC2 instance
func (p *AWSProvider) DeleteResource(ctx context.Context, id string) error {
	err := p.client.TerminateInstances(ctx, []string{id})
	if err != nil {
		return fmt.Errorf("failed to terminate instance %s: %w", id, err)
	}
	return nil
}

// TagResource adds tags to an EC2 instance
func (p *AWSProvider) TagResource(ctx context.Context, id string, tags map[string]string) error {
	err := p.client.CreateTags(ctx, id, tags)
	if err != nil {
		return fmt.Errorf("failed to tag instance %s: %w", id, err)
	}
	return nil
}

// convertToInstanceFilter converts ResourceFilter to InstanceFilter
func (p *AWSProvider) convertToInstanceFilter(filter types.ResourceFilter) InstanceFilter {
	awsFilter := InstanceFilter{
		States: []string{"pending", "running", "stopping", "stopped"},
		Tags:   make(map[string]string),
	}

	// Copy tag filters
	for k, v := range filter.Tags {
		awsFilter.Tags[k] = v
	}

	return awsFilter
}

// convertToResource converts EC2Instance to Resource
func (p *AWSProvider) convertToResource(instance EC2Instance) types.Resource {
	return types.Resource{
		ID:        instance.InstanceID,
		Type:      "ec2",
		Provider:  "aws",
		Region:    p.region,
		Name:      instance.Tags["Name"],
		Status:    instance.State,
		Tags:      instance.Tags,
		CreatedAt: instance.LaunchTime,
		UpdatedAt: time.Now(),
	}
}

// getInstanceType maps size to AWS instance type
func (p *AWSProvider) getInstanceType(size string) string {
	if size == "" {
		return "t3.micro"
	}
	return size
}
