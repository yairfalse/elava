package aws

import (
	"context"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2"
	elbv2types "github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2/types"
	"github.com/aws/aws-sdk-go-v2/service/rds"
	rdstypes "github.com/aws/aws-sdk-go-v2/service/rds/types"

	"github.com/yairfalse/ovi/providers"
	"github.com/yairfalse/ovi/types"
)

// Factory function for creating AWS providers
func NewAWSProviderFactory(ctx context.Context, config providers.ProviderConfig) (providers.CloudProvider, error) {
	return NewRealAWSProvider(ctx, config.Region)
}

func init() {
	// Register the AWS provider factory
	providers.RegisterProvider("aws", NewAWSProviderFactory)
}

// RealAWSProvider implements CloudProvider using AWS SDK v2
type RealAWSProvider struct {
	ec2Client    *ec2.Client
	rdsClient    *rds.Client
	elbv2Client  *elasticloadbalancingv2.Client
	region       string
	accountID    string
}

// NewRealAWSProvider creates a new real AWS provider
func NewRealAWSProvider(ctx context.Context, region string) (*RealAWSProvider, error) {
	cfg, err := config.LoadDefaultConfig(ctx, config.WithRegion(region))
	if err != nil {
		return nil, fmt.Errorf("failed to load AWS config: %w", err)
	}

	ec2Client := ec2.NewFromConfig(cfg)
	
	// Get account ID from EC2 describe-account-attributes
	accountOutput, err := ec2Client.DescribeAccountAttributes(ctx, &ec2.DescribeAccountAttributesInput{})
	if err != nil {
		return nil, fmt.Errorf("failed to get account ID: %w", err)
	}

	var accountID string
	for _, attr := range accountOutput.AccountAttributes {
		if aws.ToString(attr.AttributeName) == "account-id" && len(attr.AttributeValues) > 0 {
			accountID = aws.ToString(attr.AttributeValues[0].AttributeValue)
			break
		}
	}

	return &RealAWSProvider{
		ec2Client:   ec2Client,
		rdsClient:   rds.NewFromConfig(cfg),
		elbv2Client: elasticloadbalancingv2.NewFromConfig(cfg),
		region:      region,
		accountID:   accountID,
	}, nil
}

// Name returns the provider name
func (p *RealAWSProvider) Name() string {
	return "aws"
}

// Region returns the AWS region
func (p *RealAWSProvider) Region() string {
	return p.region
}

// ListResources discovers all resources in the AWS account
func (p *RealAWSProvider) ListResources(ctx context.Context, filter types.ResourceFilter) ([]types.Resource, error) {
	var resources []types.Resource

	// List EC2 instances
	ec2Resources, err := p.listEC2Instances(ctx, filter)
	if err != nil {
		return nil, fmt.Errorf("failed to list EC2 instances: %w", err)
	}
	resources = append(resources, ec2Resources...)

	// List RDS instances
	rdsResources, err := p.listRDSInstances(ctx, filter)
	if err != nil {
		return nil, fmt.Errorf("failed to list RDS instances: %w", err)
	}
	resources = append(resources, rdsResources...)

	// List Load Balancers
	elbResources, err := p.listLoadBalancers(ctx, filter)
	if err != nil {
		return nil, fmt.Errorf("failed to list load balancers: %w", err)
	}
	resources = append(resources, elbResources...)

	// Apply filters
	return p.applyFilters(resources, filter), nil
}

// listEC2Instances discovers EC2 instances
func (p *RealAWSProvider) listEC2Instances(ctx context.Context, filter types.ResourceFilter) ([]types.Resource, error) {
	var resources []types.Resource

	input := &ec2.DescribeInstancesInput{}
	
	// Add filters if specified
	if len(filter.IDs) > 0 {
		input.InstanceIds = filter.IDs
	}

	paginator := ec2.NewDescribeInstancesPaginator(p.ec2Client, input)
	
	for paginator.HasMorePages() {
		output, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to describe EC2 instances: %w", err)
		}

		for _, reservation := range output.Reservations {
			for _, instance := range reservation.Instances {
				resource := p.convertEC2Instance(instance)
				resources = append(resources, resource)
			}
		}
	}

	return resources, nil
}

// listRDSInstances discovers RDS instances
func (p *RealAWSProvider) listRDSInstances(ctx context.Context, filter types.ResourceFilter) ([]types.Resource, error) {
	var resources []types.Resource

	input := &rds.DescribeDBInstancesInput{}
	
	paginator := rds.NewDescribeDBInstancesPaginator(p.rdsClient, input)
	
	for paginator.HasMorePages() {
		output, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to describe RDS instances: %w", err)
		}

		for _, instance := range output.DBInstances {
			resource := p.convertRDSInstance(instance)
			resources = append(resources, resource)
		}
	}

	return resources, nil
}

// listLoadBalancers discovers ELBv2 load balancers
func (p *RealAWSProvider) listLoadBalancers(ctx context.Context, filter types.ResourceFilter) ([]types.Resource, error) {
	var resources []types.Resource

	input := &elasticloadbalancingv2.DescribeLoadBalancersInput{}
	
	paginator := elasticloadbalancingv2.NewDescribeLoadBalancersPaginator(p.elbv2Client, input)
	
	for paginator.HasMorePages() {
		output, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to describe load balancers: %w", err)
		}

		for _, lb := range output.LoadBalancers {
			resource := p.convertLoadBalancer(lb)
			resources = append(resources, resource)
		}
	}

	return resources, nil
}

// convertEC2Instance converts AWS EC2 instance to Ovi resource
func (p *RealAWSProvider) convertEC2Instance(instance ec2types.Instance) types.Resource {
	tags := p.convertEC2Tags(instance.Tags)
	
	// Determine if orphaned (no Ovi owner or common tags)
	isOrphaned := p.isResourceOrphaned(tags)
	
	return types.Resource{
		ID:         aws.ToString(instance.InstanceId),
		Type:       "ec2",
		Provider:   "aws",
		Region:     p.region,
		AccountID:  p.accountID,
		Status:     string(instance.State.Name),
		Tags:       tags,
		CreatedAt:  p.safeTimeValue(instance.LaunchTime),
		IsOrphaned: isOrphaned,
		Metadata: map[string]interface{}{
			"instance_type":    string(instance.InstanceType),
			"availability_zone": aws.ToString(instance.Placement.AvailabilityZone),
			"private_ip":       aws.ToString(instance.PrivateIpAddress),
			"public_ip":        aws.ToString(instance.PublicIpAddress),
			"vpc_id":          aws.ToString(instance.VpcId),
			"subnet_id":       aws.ToString(instance.SubnetId),
		},
	}
}

// convertRDSInstance converts AWS RDS instance to Ovi resource
func (p *RealAWSProvider) convertRDSInstance(instance rdstypes.DBInstance) types.Resource {
	tags := p.convertRDSTags(instance.TagList)
	isOrphaned := p.isResourceOrphaned(tags)
	
	return types.Resource{
		ID:         aws.ToString(instance.DBInstanceIdentifier),
		Type:       "rds",
		Provider:   "aws",
		Region:     p.region,
		AccountID:  p.accountID,
		Status:     aws.ToString(instance.DBInstanceStatus),
		Tags:       tags,
		CreatedAt:  p.safeTimeValue(instance.InstanceCreateTime),
		IsOrphaned: isOrphaned,
		Metadata: map[string]interface{}{
			"engine":         aws.ToString(instance.Engine),
			"engine_version": aws.ToString(instance.EngineVersion),
			"instance_class": aws.ToString(instance.DBInstanceClass),
			"allocated_storage": aws.ToInt32(instance.AllocatedStorage),
			"db_name":        aws.ToString(instance.DBName),
			"endpoint":       aws.ToString(instance.Endpoint.Address),
			"port":          aws.ToInt32(instance.Endpoint.Port),
		},
	}
}

// convertLoadBalancer converts AWS ELBv2 to Ovi resource
func (p *RealAWSProvider) convertLoadBalancer(lb elbv2types.LoadBalancer) types.Resource {
	tags := types.Tags{} // ELB tags require separate API call
	isOrphaned := p.isResourceOrphaned(tags)
	
	return types.Resource{
		ID:         aws.ToString(lb.LoadBalancerArn),
		Type:       "elb",
		Provider:   "aws",
		Region:     p.region,
		AccountID:  p.accountID,
		Status:     string(lb.State.Code),
		Tags:       tags,
		CreatedAt:  p.safeTimeValue(lb.CreatedTime),
		IsOrphaned: isOrphaned,
		Metadata: map[string]interface{}{
			"type":      string(lb.Type),
			"scheme":    string(lb.Scheme),
			"vpc_id":    aws.ToString(lb.VpcId),
			"dns_name":  aws.ToString(lb.DNSName),
		},
	}
}

// convertEC2Tags converts EC2 tags to Ovi tags
func (p *RealAWSProvider) convertEC2Tags(ec2Tags []ec2types.Tag) types.Tags {
	tags := types.Tags{}
	
	for _, tag := range ec2Tags {
		key := aws.ToString(tag.Key)
		value := aws.ToString(tag.Value)
		
		switch key {
		case "ovi:owner", "Owner", "owner":
			tags.OviOwner = value
		case "ovi:managed":
			tags.OviManaged = value == "true"
		case "ovi:blessed":
			tags.OviBlessed = value == "true"
		case "Environment", "environment", "env":
			tags.Environment = value
		case "Team", "team":
			tags.Team = value
		case "Name", "name":
			tags.Name = value
		case "Project", "project":
			tags.Project = value
		case "CostCenter", "cost-center", "costcenter":
			tags.CostCenter = value
		}
	}
	
	return tags
}

// convertRDSTags converts RDS tags to Ovi tags  
func (p *RealAWSProvider) convertRDSTags(rdsTags []rdstypes.Tag) types.Tags {
	tags := types.Tags{}
	
	for _, tag := range rdsTags {
		key := aws.ToString(tag.Key)
		value := aws.ToString(tag.Value)
		
		switch key {
		case "ovi:owner", "Owner", "owner":
			tags.OviOwner = value
		case "ovi:managed":
			tags.OviManaged = value == "true"
		case "ovi:blessed":
			tags.OviBlessed = value == "true"
		case "Environment", "environment", "env":
			tags.Environment = value
		case "Team", "team":
			tags.Team = value
		case "Name", "name":
			tags.Name = value
		case "Project", "project":
			tags.Project = value
		case "CostCenter", "cost-center", "costcenter":
			tags.CostCenter = value
		}
	}
	
	return tags
}

// isResourceOrphaned determines if resource lacks proper ownership/management tags
func (p *RealAWSProvider) isResourceOrphaned(tags types.Tags) bool {
	// Resource is orphaned if it lacks key identification tags
	hasOwner := tags.OviOwner != "" || tags.Team != ""
	hasProject := tags.Project != "" || tags.Name != ""
	hasManagement := tags.OviManaged
	
	// If it's explicitly managed by Ovi, it's not orphaned
	if hasManagement {
		return false
	}
	
	// If it has neither owner nor project identification, it's likely orphaned
	return !hasOwner && !hasProject
}

// applyFilters applies ResourceFilter to resources
func (p *RealAWSProvider) applyFilters(resources []types.Resource, filter types.ResourceFilter) []types.Resource {
	if filter.Owner == "" && filter.Type == "" && len(filter.IDs) == 0 {
		return resources
	}
	
	var filtered []types.Resource
	for _, resource := range resources {
		// Filter by owner
		if filter.Owner != "" && resource.Tags.OviOwner != filter.Owner && resource.Tags.Team != filter.Owner {
			continue
		}
		
		// Filter by type
		if filter.Type != "" && resource.Type != filter.Type {
			continue
		}
		
		// Filter by IDs
		if len(filter.IDs) > 0 {
			found := false
			for _, id := range filter.IDs {
				if resource.ID == id {
					found = true
					break
				}
			}
			if !found {
				continue
			}
		}
		
		filtered = append(filtered, resource)
	}
	
	return filtered
}

// safeTimeValue safely converts *time.Time to time.Time
func (p *RealAWSProvider) safeTimeValue(t *time.Time) time.Time {
	if t == nil {
		return time.Time{}
	}
	return *t
}

// CreateResource creates a new AWS resource (placeholder - Day 2 ops doesn't create)
func (p *RealAWSProvider) CreateResource(ctx context.Context, spec types.ResourceSpec) (*types.Resource, error) {
	return nil, fmt.Errorf("Day 2 operations: Ovi does not create resources - use your IaC tools")
}

// DeleteResource deletes an AWS resource (placeholder - we recommend, don't execute)
func (p *RealAWSProvider) DeleteResource(ctx context.Context, resourceID string) error {
	return fmt.Errorf("Day 2 operations: Ovi recommends cleanup - use your IaC tools or AWS console")
}

// TagResource adds tags to an AWS resource
func (p *RealAWSProvider) TagResource(ctx context.Context, resourceID string, tags map[string]string) error {
	// For Day 2 ops, we might tag resources for cleanup or ownership
	// This is one of the few write operations we support
	
	// Determine resource type from ID pattern
	if len(resourceID) > 2 && resourceID[:2] == "i-" {
		// EC2 instance
		return p.tagEC2Instance(ctx, resourceID, tags)
	} else if len(resourceID) > 4 && resourceID[:4] == "arn:" {
		// ARN-based resource (like ELB)
		return p.tagARNResource(ctx, resourceID, tags)
	}
	
	// For RDS and other resources, we'd implement similar logic
	return fmt.Errorf("unsupported resource type for tagging: %s", resourceID)
}

// tagEC2Instance tags an EC2 instance
func (p *RealAWSProvider) tagEC2Instance(ctx context.Context, instanceID string, tags map[string]string) error {
	var ec2Tags []ec2types.Tag
	for key, value := range tags {
		ec2Tags = append(ec2Tags, ec2types.Tag{
			Key:   aws.String(key),
			Value: aws.String(value),
		})
	}
	
	_, err := p.ec2Client.CreateTags(ctx, &ec2.CreateTagsInput{
		Resources: []string{instanceID},
		Tags:      ec2Tags,
	})
	
	return err
}

// tagARNResource tags an ARN-based resource
func (p *RealAWSProvider) tagARNResource(ctx context.Context, arn string, tags map[string]string) error {
	// For ELBv2 and similar ARN-based resources
	var elbTags []elbv2types.Tag
	for key, value := range tags {
		elbTags = append(elbTags, elbv2types.Tag{
			Key:   aws.String(key),
			Value: aws.String(value),
		})
	}
	
	_, err := p.elbv2Client.AddTags(ctx, &elasticloadbalancingv2.AddTagsInput{
		ResourceArns: []string{arn},
		Tags:        elbTags,
	})
	
	return err
}

// Ensure RealAWSProvider implements CloudProvider interface
var _ providers.CloudProvider = (*RealAWSProvider)(nil)