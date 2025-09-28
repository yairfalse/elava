package aws

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/autoscaling"
	asgtypes "github.com/aws/aws-sdk-go-v2/service/autoscaling/types"
	"github.com/aws/aws-sdk-go-v2/service/ecs"
	ecstypes "github.com/aws/aws-sdk-go-v2/service/ecs/types"
	"github.com/aws/aws-sdk-go-v2/service/eks"
	"github.com/aws/aws-sdk-go-v2/service/lambda"
	lambdatypes "github.com/aws/aws-sdk-go-v2/service/lambda/types"
	"github.com/yairfalse/elava/types"
)

// listLambdaFunctions discovers Lambda functions
func (p *RealAWSProvider) listLambdaFunctions(ctx context.Context, filter types.ResourceFilter) ([]types.Resource, error) {
	var resources []types.Resource
	paginator := lambda.NewListFunctionsPaginator(p.lambdaClient, &lambda.ListFunctionsInput{})

	for paginator.HasMorePages() {
		output, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to list Lambda functions: %w", err)
		}

		for _, function := range output.Functions {
			resource := p.processLambdaFunction(ctx, function)
			resources = append(resources, resource)
		}
	}

	return resources, nil
}

// processLambdaFunction processes a single Lambda function
func (p *RealAWSProvider) processLambdaFunction(ctx context.Context, function lambdatypes.FunctionConfiguration) types.Resource {
	tags := p.getLambdaTags(ctx, function.FunctionArn)
	lastInvoke := p.getLambdaLastInvoke(ctx, function.FunctionName)
	isOrphaned := p.isLambdaOrphaned(tags, lastInvoke)

	return types.Resource{
		ID:         aws.ToString(function.FunctionArn),
		Type:       "lambda",
		Provider:   "aws",
		Region:     p.region,
		AccountID:  p.accountID,
		Name:       aws.ToString(function.FunctionName),
		Status:     string(function.State),
		Tags:       tags,
		LastSeenAt: time.Now(),
		IsOrphaned: isOrphaned,
		Metadata: map[string]interface{}{
			"runtime":      string(function.Runtime),
			"last_invoked": lastInvoke,
			"memory_size":  aws.ToInt32(function.MemorySize),
			"timeout":      aws.ToInt32(function.Timeout),
		},
	}
}

// getLambdaTags retrieves tags for a Lambda function
func (p *RealAWSProvider) getLambdaTags(ctx context.Context, functionArn *string) types.Tags {
	tags := types.Tags{}
	tagsOutput, err := p.lambdaClient.ListTags(ctx, &lambda.ListTagsInput{
		Resource: functionArn,
	})
	if err == nil && tagsOutput.Tags != nil {
		tags = p.convertTagsToElava(tagsOutput.Tags)
	}
	return tags
}

// getLambdaLastInvoke gets the last invocation time for a Lambda function
func (p *RealAWSProvider) getLambdaLastInvoke(ctx context.Context, functionName *string) *time.Time {
	// This would need CloudWatch metrics to get actual invocation time
	// For now, return nil to indicate unknown
	return nil
}

// isLambdaOrphaned checks if a Lambda function is orphaned
func (p *RealAWSProvider) isLambdaOrphaned(tags types.Tags, lastInvoke *time.Time) bool {
	if p.isResourceOrphaned(tags) {
		return true
	}
	// Consider orphaned if not invoked in 30 days
	if lastInvoke != nil && time.Since(*lastInvoke) > 30*24*time.Hour {
		return true
	}
	return false
}

// listEKSClusters discovers EKS clusters
func (p *RealAWSProvider) listEKSClusters(ctx context.Context, filter types.ResourceFilter) ([]types.Resource, error) {
	var resources []types.Resource

	// List cluster names
	listOutput, err := p.eksClient.ListClusters(ctx, &eks.ListClustersInput{})
	if err != nil {
		return nil, fmt.Errorf("failed to list EKS clusters: %w", err)
	}

	// Describe each cluster
	for _, clusterName := range listOutput.Clusters {
		resource := p.describeEKSCluster(ctx, clusterName)
		if resource != nil {
			resources = append(resources, *resource)
		}
	}

	return resources, nil
}

// describeEKSCluster describes a single EKS cluster
func (p *RealAWSProvider) describeEKSCluster(ctx context.Context, clusterName string) *types.Resource {
	describeOutput, err := p.eksClient.DescribeCluster(ctx, &eks.DescribeClusterInput{
		Name: aws.String(clusterName),
	})
	if err != nil {
		return nil
	}

	cluster := describeOutput.Cluster
	tags := types.Tags{}
	if cluster.Tags != nil {
		tags = p.convertTagsToElava(cluster.Tags)
	}

	return &types.Resource{
		ID:         aws.ToString(cluster.Arn),
		Type:       "eks",
		Provider:   "aws",
		Region:     p.region,
		AccountID:  p.accountID,
		Name:       aws.ToString(cluster.Name),
		Status:     string(cluster.Status),
		Tags:       tags,
		CreatedAt:  p.safeTimeValue(cluster.CreatedAt),
		LastSeenAt: time.Now(),
		IsOrphaned: p.isResourceOrphaned(tags),
		Metadata: map[string]interface{}{
			"version":     aws.ToString(cluster.Version),
			"platform":    aws.ToString(cluster.PlatformVersion),
			"endpoint":    aws.ToString(cluster.Endpoint),
			"node_groups": p.getNodeGroupCount(ctx, clusterName),
		},
	}
}

// getNodeGroupCount gets the number of node groups for an EKS cluster
func (p *RealAWSProvider) getNodeGroupCount(ctx context.Context, clusterName string) int {
	nodeGroups, err := p.eksClient.ListNodegroups(ctx, &eks.ListNodegroupsInput{
		ClusterName: aws.String(clusterName),
	})
	if err != nil {
		return 0
	}
	return len(nodeGroups.Nodegroups)
}

// listECSClusters discovers ECS clusters
func (p *RealAWSProvider) listECSClusters(ctx context.Context, filter types.ResourceFilter) ([]types.Resource, error) {
	var resources []types.Resource

	// List cluster ARNs
	listOutput, err := p.ecsClient.ListClusters(ctx, &ecs.ListClustersInput{})
	if err != nil {
		return nil, fmt.Errorf("failed to list ECS clusters: %w", err)
	}

	if len(listOutput.ClusterArns) == 0 {
		return resources, nil
	}

	// Describe clusters in batch
	describeOutput, err := p.ecsClient.DescribeClusters(ctx, &ecs.DescribeClustersInput{
		Clusters: listOutput.ClusterArns,
		Include:  []ecstypes.ClusterField{ecstypes.ClusterFieldTags, ecstypes.ClusterFieldStatistics},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to describe ECS clusters: %w", err)
	}

	for _, cluster := range describeOutput.Clusters {
		resource := p.processECSCluster(cluster)
		resources = append(resources, resource)
	}

	return resources, nil
}

// processECSCluster processes a single ECS cluster
func (p *RealAWSProvider) processECSCluster(cluster ecstypes.Cluster) types.Resource {
	tags := p.convertECSTags(cluster.Tags)

	// Extract statistics
	runningTasks := int32(0)
	pendingTasks := int32(0)
	if cluster.Statistics != nil {
		for _, stat := range cluster.Statistics {
			if aws.ToString(stat.Name) == "runningTasksCount" {
				if stat.Value != nil {
					if val, err := strconv.ParseInt(*stat.Value, 10, 32); err == nil {
						runningTasks = int32(val)
					}
				}
			}
			if aws.ToString(stat.Name) == "pendingTasksCount" {
				if stat.Value != nil {
					if val, err := strconv.ParseInt(*stat.Value, 10, 32); err == nil {
						pendingTasks = int32(val)
					}
				}
			}
		}
	}

	isOrphaned := p.isResourceOrphaned(tags) ||
		(runningTasks == 0 && cluster.ActiveServicesCount == 0)

	return types.Resource{
		ID:         aws.ToString(cluster.ClusterArn),
		Type:       "ecs",
		Provider:   "aws",
		Region:     p.region,
		AccountID:  p.accountID,
		Name:       aws.ToString(cluster.ClusterName),
		Status:     aws.ToString(cluster.Status),
		Tags:       tags,
		LastSeenAt: time.Now(),
		IsOrphaned: isOrphaned,
		Metadata: map[string]interface{}{
			"running_tasks":        runningTasks,
			"pending_tasks":        pendingTasks,
			"active_services":      cluster.ActiveServicesCount,
			"registered_instances": cluster.RegisteredContainerInstancesCount,
		},
	}
}

// convertECSTags converts ECS tags to Elava tags
func (p *RealAWSProvider) convertECSTags(ecsTags []ecstypes.Tag) types.Tags {
	return p.convertTagsToElava(ecsTags)
}

// listAutoScalingGroups discovers Auto Scaling Groups
func (p *RealAWSProvider) listAutoScalingGroups(ctx context.Context, filter types.ResourceFilter) ([]types.Resource, error) {
	var resources []types.Resource
	paginator := autoscaling.NewDescribeAutoScalingGroupsPaginator(p.asgClient, &autoscaling.DescribeAutoScalingGroupsInput{})

	for paginator.HasMorePages() {
		output, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to list Auto Scaling Groups: %w", err)
		}

		for _, asg := range output.AutoScalingGroups {
			resource := p.processAutoScalingGroup(asg)
			resources = append(resources, resource)
		}
	}

	return resources, nil
}

// processAutoScalingGroup processes a single Auto Scaling Group
func (p *RealAWSProvider) processAutoScalingGroup(asg asgtypes.AutoScalingGroup) types.Resource {
	tags := p.convertTagsToElava(asg.Tags)

	status := "active"
	if aws.ToInt32(asg.DesiredCapacity) == 0 {
		status = "stopped"
	}

	isOrphaned := p.isResourceOrphaned(tags) ||
		(aws.ToInt32(asg.DesiredCapacity) == 0 && len(asg.Instances) == 0)

	return types.Resource{
		ID:         aws.ToString(asg.AutoScalingGroupARN),
		Type:       "asg",
		Provider:   "aws",
		Region:     p.region,
		AccountID:  p.accountID,
		Name:       aws.ToString(asg.AutoScalingGroupName),
		Status:     status,
		Tags:       tags,
		CreatedAt:  p.safeTimeValue(asg.CreatedTime),
		LastSeenAt: time.Now(),
		IsOrphaned: isOrphaned,
		Metadata: map[string]interface{}{
			"desired_capacity": aws.ToInt32(asg.DesiredCapacity),
			"min_size":         aws.ToInt32(asg.MinSize),
			"max_size":         aws.ToInt32(asg.MaxSize),
			"instances":        len(asg.Instances),
		},
	}
}
