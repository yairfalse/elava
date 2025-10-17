package aws

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/eks"
	ekstypes "github.com/aws/aws-sdk-go-v2/service/eks/types"

	"github.com/yairfalse/elava/types"
)

// ListEKSClusters scans all EKS clusters
func (p *RealAWSProvider) ListEKSClusters(ctx context.Context) ([]types.Resource, error) {
	listOutput, err := p.eksClient.ListClusters(ctx, &eks.ListClustersInput{})
	if err != nil {
		return nil, fmt.Errorf("failed to list EKS clusters: %w", err)
	}

	resources := make([]types.Resource, 0, len(listOutput.Clusters))
	for _, clusterName := range listOutput.Clusters {
		describeOutput, err := p.eksClient.DescribeCluster(ctx, &eks.DescribeClusterInput{
			Name: aws.String(clusterName),
		})
		if err != nil {
			return nil, fmt.Errorf("failed to describe EKS cluster %s: %w", clusterName, err)
		}

		resource := buildEKSClusterResource(*describeOutput.Cluster, p.region, p.accountID)
		resources = append(resources, resource)
	}

	return resources, nil
}

// ListEKSNodeGroups scans all EKS node groups across all clusters
func (p *RealAWSProvider) ListEKSNodeGroups(ctx context.Context) ([]types.Resource, error) {
	// First get all clusters
	listClustersOutput, err := p.eksClient.ListClusters(ctx, &eks.ListClustersInput{})
	if err != nil {
		return nil, fmt.Errorf("failed to list EKS clusters: %w", err)
	}

	var allNodeGroups []types.Resource

	// For each cluster, list its node groups
	for _, clusterName := range listClustersOutput.Clusters {
		listNGOutput, err := p.eksClient.ListNodegroups(ctx, &eks.ListNodegroupsInput{
			ClusterName: aws.String(clusterName),
		})
		if err != nil {
			return nil, fmt.Errorf("failed to list node groups for cluster %s: %w", clusterName, err)
		}

		// Describe each node group
		for _, ngName := range listNGOutput.Nodegroups {
			describeOutput, err := p.eksClient.DescribeNodegroup(ctx, &eks.DescribeNodegroupInput{
				ClusterName:   aws.String(clusterName),
				NodegroupName: aws.String(ngName),
			})
			if err != nil {
				return nil, fmt.Errorf("failed to describe node group %s/%s: %w", clusterName, ngName, err)
			}

			resource := buildEKSNodeGroupResource(*describeOutput.Nodegroup, p.region, p.accountID)
			allNodeGroups = append(allNodeGroups, resource)
		}
	}

	return allNodeGroups, nil
}

// buildEKSClusterResource converts AWS EKS cluster to types.Resource
func buildEKSClusterResource(cluster ekstypes.Cluster, region, accountID string) types.Resource {
	var vpcID, subnetIDs, securityGroupIDs string
	if cluster.ResourcesVpcConfig != nil {
		vpcID = aws.ToString(cluster.ResourcesVpcConfig.VpcId)
		subnetIDs = extractSubnetIDs(cluster.ResourcesVpcConfig.SubnetIds)
		securityGroupIDs = extractSecurityGroupIDs(cluster.ResourcesVpcConfig.SecurityGroupIds)
	}

	return types.Resource{
		ID:         aws.ToString(cluster.Name),
		Type:       "eks_cluster",
		Provider:   "aws",
		Region:     region,
		AccountID:  accountID,
		Name:       aws.ToString(cluster.Name),
		Status:     string(cluster.Status),
		Tags:       convertEKSTags(cluster.Tags),
		CreatedAt:  aws.ToTime(cluster.CreatedAt),
		LastSeenAt: time.Now(),
		Metadata: types.ResourceMetadata{
			ClusterVersion:   aws.ToString(cluster.Version),
			Endpoint:         aws.ToString(cluster.Endpoint),
			RoleArn:          aws.ToString(cluster.RoleArn),
			VpcID:            vpcID,
			SubnetIDs:        subnetIDs,
			SecurityGroupIDs: securityGroupIDs,
		},
	}
}

// buildEKSNodeGroupResource converts AWS EKS node group to types.Resource
func buildEKSNodeGroupResource(ng ekstypes.Nodegroup, region, accountID string) types.Resource {
	var desiredSize, minSize, maxSize int32
	if ng.ScalingConfig != nil {
		desiredSize = aws.ToInt32(ng.ScalingConfig.DesiredSize)
		minSize = aws.ToInt32(ng.ScalingConfig.MinSize)
		maxSize = aws.ToInt32(ng.ScalingConfig.MaxSize)
	}

	clusterName := aws.ToString(ng.ClusterName)
	nodeGroupName := aws.ToString(ng.NodegroupName)

	return types.Resource{
		ID:         fmt.Sprintf("%s/%s", clusterName, nodeGroupName),
		Type:       "eks_node_group",
		Provider:   "aws",
		Region:     region,
		AccountID:  accountID,
		Name:       nodeGroupName,
		Status:     string(ng.Status),
		Tags:       convertEKSTags(ng.Tags),
		CreatedAt:  aws.ToTime(ng.CreatedAt),
		LastSeenAt: time.Now(),
		Metadata: types.ResourceMetadata{
			ClusterName:          clusterName,
			DesiredCapacity:      desiredSize,
			MinSize:              minSize,
			MaxSize:              maxSize,
			InstanceTypes:        extractInstanceTypes(ng.InstanceTypes),
			SubnetIDs:            extractSubnetIDs(ng.Subnets),
			AutoScalingGroupName: extractASGName(ng.Resources),
			NodeLabels:           ng.Labels,
			NodeTaints:           formatNodeTaints(ng.Taints),
		},
	}
}

// extractSubnetIDs converts subnet slice to comma-separated string
func extractSubnetIDs(subnets []string) string {
	if len(subnets) == 0 {
		return ""
	}
	return strings.Join(subnets, ",")
}

// extractSecurityGroupIDs converts security group slice to comma-separated string
func extractSecurityGroupIDs(sgs []string) string {
	if len(sgs) == 0 {
		return ""
	}
	return strings.Join(sgs, ",")
}

// extractInstanceTypes converts instance type slice to comma-separated string
func extractInstanceTypes(types []string) string {
	if len(types) == 0 {
		return ""
	}
	return strings.Join(types, ",")
}

// extractASGName extracts the first ASG name from node group resources
func extractASGName(resources *ekstypes.NodegroupResources) string {
	if resources == nil || len(resources.AutoScalingGroups) == 0 {
		return ""
	}
	return aws.ToString(resources.AutoScalingGroups[0].Name)
}

// formatNodeTaints converts taints to "key=value:effect" format
func formatNodeTaints(taints []ekstypes.Taint) string {
	if len(taints) == 0 {
		return ""
	}

	formatted := make([]string, len(taints))
	for i, taint := range taints {
		key := aws.ToString(taint.Key)
		value := aws.ToString(taint.Value)
		effect := formatTaintEffect(taint.Effect)
		formatted[i] = fmt.Sprintf("%s=%s:%s", key, value, effect)
	}
	return strings.Join(formatted, ",")
}

// formatTaintEffect converts AWS taint effect to K8s format
func formatTaintEffect(effect ekstypes.TaintEffect) string {
	switch effect {
	case ekstypes.TaintEffectNoSchedule:
		return "NoSchedule"
	case ekstypes.TaintEffectNoExecute:
		return "NoExecute"
	case ekstypes.TaintEffectPreferNoSchedule:
		return "PreferNoSchedule"
	default:
		return string(effect)
	}
}

// convertEKSTags converts EKS tags to Elava tags
func convertEKSTags(tags map[string]string) types.Tags {
	result := types.Tags{}
	for key, value := range tags {
		// Map common tag keys to struct fields
		switch key {
		case "Name":
			result.Name = value
		case "Environment":
			result.Environment = value
		case "Team":
			result.Team = value
		case "Project":
			result.Project = value
		case "Application":
			result.Application = value
		case "Owner":
			result.Owner = value
		case "Contact":
			result.Contact = value
		case "CostCenter", "cost-center":
			result.CostCenter = value
		case "CreatedBy", "created-by":
			result.CreatedBy = value
		case "CreatedDate", "created-date":
			result.CreatedDate = value
		case "elava:owner":
			result.ElavaOwner = value
		case "elava:managed":
			result.ElavaManaged = value == "true"
		case "elava:blessed":
			result.ElavaBlessed = value == "true"
		case "elava:generation":
			result.ElavaGeneration = value
		case "elava:claimed_at":
			result.ElavaClaimedAt = value
		}
	}
	return result
}
