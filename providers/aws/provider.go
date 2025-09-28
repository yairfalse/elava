package aws

import (
	"context"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/autoscaling"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/aws/aws-sdk-go-v2/service/ecr"
	"github.com/aws/aws-sdk-go-v2/service/ecs"
	"github.com/aws/aws-sdk-go-v2/service/eks"
	"github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2"
	elbv2types "github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2/types"
	"github.com/aws/aws-sdk-go-v2/service/iam"
	"github.com/aws/aws-sdk-go-v2/service/kms"
	"github.com/aws/aws-sdk-go-v2/service/lambda"
	"github.com/aws/aws-sdk-go-v2/service/memorydb"
	"github.com/aws/aws-sdk-go-v2/service/rds"
	rdstypes "github.com/aws/aws-sdk-go-v2/service/rds/types"
	"github.com/aws/aws-sdk-go-v2/service/redshift"
	"github.com/aws/aws-sdk-go-v2/service/route53"
	"github.com/aws/aws-sdk-go-v2/service/s3"

	"github.com/yairfalse/elava/providers"
	"github.com/yairfalse/elava/types"
)

// Factory function for creating AWS providers
func NewAWSProviderFactory(ctx context.Context, config providers.ProviderConfig) (providers.CloudProvider, error) {
	return NewRealAWSProvider(ctx, config.Region)
}

func init() {
	// Register the AWS provider factory
	providers.RegisterProvider("aws", NewAWSProviderFactory)
}

// ResourceHandler handles listing of a specific resource type
type ResourceHandler struct {
	Name     string
	Critical bool // If true, errors will fail the whole operation
	Handler  func(ctx context.Context, filter types.ResourceFilter) ([]types.Resource, error)
}

// RealAWSProvider implements CloudProvider using AWS SDK v2
type RealAWSProvider struct {
	ec2Client      *ec2.Client
	rdsClient      *rds.Client
	elbv2Client    *elasticloadbalancingv2.Client
	s3Client       *s3.Client
	lambdaClient   *lambda.Client
	cwLogsClient   *cloudwatchlogs.Client
	eksClient      *eks.Client
	ecsClient      *ecs.Client
	asgClient      *autoscaling.Client
	iamClient      *iam.Client
	ecrClient      *ecr.Client
	route53Client  *route53.Client
	kmsClient      *kms.Client
	dynamodbClient *dynamodb.Client
	memorydbClient *memorydb.Client
	redshiftClient *redshift.Client
	region         string
	accountID      string
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
		ec2Client:      ec2Client,
		rdsClient:      rds.NewFromConfig(cfg),
		elbv2Client:    elasticloadbalancingv2.NewFromConfig(cfg),
		s3Client:       s3.NewFromConfig(cfg),
		lambdaClient:   lambda.NewFromConfig(cfg),
		cwLogsClient:   cloudwatchlogs.NewFromConfig(cfg),
		eksClient:      eks.NewFromConfig(cfg),
		ecsClient:      ecs.NewFromConfig(cfg),
		asgClient:      autoscaling.NewFromConfig(cfg),
		iamClient:      iam.NewFromConfig(cfg),
		ecrClient:      ecr.NewFromConfig(cfg),
		route53Client:  route53.NewFromConfig(cfg),
		kmsClient:      kms.NewFromConfig(cfg),
		dynamodbClient: dynamodb.NewFromConfig(cfg),
		memorydbClient: memorydb.NewFromConfig(cfg),
		redshiftClient: redshift.NewFromConfig(cfg),
		region:         region,
		accountID:      accountID,
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
// ListResources lists all AWS resources using the strategy pattern
func (p *RealAWSProvider) ListResources(ctx context.Context, filter types.ResourceFilter) ([]types.Resource, error) {
	handlers := p.getResourceHandlers()
	resources := p.executeHandlers(ctx, handlers, filter)
	return p.applyFilters(resources, filter), nil
}

// getResourceHandlers returns all resource handlers
func (p *RealAWSProvider) getResourceHandlers() []ResourceHandler {
	return []ResourceHandler{
		// Critical resources that must succeed
		{Name: "EC2", Critical: true, Handler: p.listEC2Instances},
		{Name: "RDS", Critical: true, Handler: p.listRDSInstances},
		{Name: "ELB", Critical: true, Handler: p.listLoadBalancers},

		// Non-critical resources (continue on error)
		{Name: "S3", Critical: false, Handler: p.listS3Buckets},
		{Name: "Lambda", Critical: false, Handler: p.listLambdaFunctions},
		{Name: "EBS", Critical: false, Handler: p.listEBSVolumes},
		{Name: "EIP", Critical: false, Handler: p.listElasticIPs},
		{Name: "NAT", Critical: false, Handler: p.listNATGateways},
		{Name: "Snapshots", Critical: false, Handler: p.listSnapshots},
		{Name: "AMI", Critical: false, Handler: p.listAMIs},
		{Name: "CloudWatchLogs", Critical: false, Handler: p.listCloudWatchLogs},
		{Name: "SecurityGroups", Critical: false, Handler: p.listSecurityGroups},
		{Name: "EKS", Critical: false, Handler: p.listEKSClusters},
		{Name: "ECS", Critical: false, Handler: p.listECSClusters},
		{Name: "ASG", Critical: false, Handler: p.listAutoScalingGroups},
		{Name: "VPCEndpoints", Critical: false, Handler: p.listVPCEndpoints},
		{Name: "RDSSnapshots", Critical: false, Handler: p.listRDSSnapshots},
		{Name: "IAMRoles", Critical: false, Handler: p.listIAMRoles},
		{Name: "ENI", Critical: false, Handler: p.listNetworkInterfaces},
		{Name: "ECR", Critical: false, Handler: p.listECRRepositories},
		{Name: "Route53", Critical: false, Handler: p.listRoute53HostedZones},
		{Name: "KMS", Critical: false, Handler: p.listKMSKeys},
		{Name: "Aurora", Critical: false, Handler: p.listAuroraClusters},
		{Name: "Redshift", Critical: false, Handler: p.listRedshiftClusters},
		{Name: "RedshiftSnapshots", Critical: false, Handler: p.listRedshiftSnapshots},
		{Name: "MemoryDB", Critical: false, Handler: p.listMemoryDBClusters},
		{Name: "DynamoDB", Critical: false, Handler: p.listDynamoDBTables},
		{Name: "DynamoDBBackups", Critical: false, Handler: p.listDynamoDBBackups},
	}
}

// executeHandlers runs all handlers and collects resources
func (p *RealAWSProvider) executeHandlers(ctx context.Context, handlers []ResourceHandler, filter types.ResourceFilter) []types.Resource {
	var resources []types.Resource

	for _, handler := range handlers {
		result, err := handler.Handler(ctx, filter)
		if err != nil {
			if handler.Critical {
				fmt.Printf("Critical failure listing %s: %v\n", handler.Name, err)
				continue
			}
			fmt.Printf("Warning: failed to list %s: %v\n", handler.Name, err)
			continue
		}
		resources = append(resources, result...)
	}

	return resources
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

// convertEC2Instance converts AWS EC2 instance to Elava resource
func (p *RealAWSProvider) convertEC2Instance(instance ec2types.Instance) types.Resource {
	tags := p.convertTagsToElava(instance.Tags)

	// Determine if orphaned (no Elava owner or common tags)
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
			"instance_type":     string(instance.InstanceType),
			"availability_zone": aws.ToString(instance.Placement.AvailabilityZone),
			"private_ip":        aws.ToString(instance.PrivateIpAddress),
			"public_ip":         aws.ToString(instance.PublicIpAddress),
			"vpc_id":            aws.ToString(instance.VpcId),
			"subnet_id":         aws.ToString(instance.SubnetId),
		},
	}
}

// convertRDSInstance converts AWS RDS instance to Elava resource
func (p *RealAWSProvider) convertRDSInstance(instance rdstypes.DBInstance) types.Resource {
	tags := p.convertTagsToElava(instance.TagList)
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
			"engine":            aws.ToString(instance.Engine),
			"engine_version":    aws.ToString(instance.EngineVersion),
			"instance_class":    aws.ToString(instance.DBInstanceClass),
			"allocated_storage": aws.ToInt32(instance.AllocatedStorage),
			"db_name":           aws.ToString(instance.DBName),
			"endpoint":          aws.ToString(instance.Endpoint.Address),
			"port":              aws.ToInt32(instance.Endpoint.Port),
		},
	}
}

// convertLoadBalancer converts AWS ELBv2 to Elava resource
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
			"type":     string(lb.Type),
			"scheme":   string(lb.Scheme),
			"vpc_id":   aws.ToString(lb.VpcId),
			"dns_name": aws.ToString(lb.DNSName),
		},
	}
}

// isResourceOrphaned determines if resource lacks proper ownership/management tags
func (p *RealAWSProvider) isResourceOrphaned(tags types.Tags) bool {
	// Resource is orphaned if it lacks key identification tags
	hasOwner := tags.ElavaOwner != "" || tags.Team != ""
	hasProject := tags.Project != "" || tags.Name != ""
	hasManagement := tags.ElavaManaged

	// If it's explicitly managed by Elava, it's not orphaned
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
		if p.shouldIncludeResource(resource, filter) {
			filtered = append(filtered, resource)
		}
	}

	return filtered
}

// shouldIncludeResource checks if a resource matches the filter criteria
func (p *RealAWSProvider) shouldIncludeResource(resource types.Resource, filter types.ResourceFilter) bool {
	// Filter by owner
	if filter.Owner != "" && !p.matchesOwner(resource, filter.Owner) {
		return false
	}

	// Filter by type
	if filter.Type != "" && resource.Type != filter.Type {
		return false
	}

	// Filter by IDs
	if len(filter.IDs) > 0 && !p.matchesID(resource, filter.IDs) {
		return false
	}

	return true
}

// matchesOwner checks if resource matches owner filter
func (p *RealAWSProvider) matchesOwner(resource types.Resource, owner string) bool {
	return resource.Tags.ElavaOwner == owner || resource.Tags.Team == owner
}

// matchesID checks if resource ID matches any in the filter list
func (p *RealAWSProvider) matchesID(resource types.Resource, ids []string) bool {
	for _, id := range ids {
		if resource.ID == id {
			return true
		}
	}
	return false
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
	return nil, fmt.Errorf("Day 2 operations: Elava does not create resources - use your IaC tools")
}

// DeleteResource deletes an AWS resource (placeholder - we recommend, don't execute)
func (p *RealAWSProvider) DeleteResource(ctx context.Context, resourceID string) error {
	return fmt.Errorf("Day 2 operations: Elava recommends cleanup - use your IaC tools or AWS console")
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
		Tags:         elbTags,
	})

	return err
}

// Ensure RealAWSProvider implements CloudProvider interface
var _ providers.CloudProvider = (*RealAWSProvider)(nil)
