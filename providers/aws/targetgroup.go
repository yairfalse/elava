package aws

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2"
	elbv2types "github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2/types"

	"github.com/yairfalse/elava/types"
)

// ListTargetGroups scans all ELB target groups
func (p *RealAWSProvider) ListTargetGroups(ctx context.Context) ([]types.Resource, error) {
	paginator := elasticloadbalancingv2.NewDescribeTargetGroupsPaginator(
		p.elbv2Client,
		&elasticloadbalancingv2.DescribeTargetGroupsInput{},
	)

	var resources []types.Resource
	for paginator.HasMorePages() {
		output, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to describe target groups: %w", err)
		}

		for _, tg := range output.TargetGroups {
			resource := buildTargetGroupResource(tg, p.region, p.accountID)
			resources = append(resources, resource)
		}
	}

	return resources, nil
}

// buildTargetGroupResource converts ELBv2 target group to types.Resource
func buildTargetGroupResource(tg elbv2types.TargetGroup, region, accountID string) types.Resource {
	return types.Resource{
		ID:         aws.ToString(tg.TargetGroupArn),
		Type:       "target_group",
		Provider:   "aws",
		Region:     region,
		AccountID:  accountID,
		Name:       aws.ToString(tg.TargetGroupName),
		Status:     "active",     // Target groups don't have explicit status
		Tags:       types.Tags{}, // Tags need separate API call
		LastSeenAt: time.Now(),
		Metadata: types.ResourceMetadata{
			VpcID:                      aws.ToString(tg.VpcId),
			TargetType:                 string(tg.TargetType),
			Protocol:                   string(tg.Protocol),
			Port:                       aws.ToInt32(tg.Port),
			LoadBalancerARNs:           extractLoadBalancerARNs(tg.LoadBalancerArns),
			HealthCheckProtocol:        string(tg.HealthCheckProtocol),
			HealthCheckPort:            aws.ToString(tg.HealthCheckPort),
			HealthCheckPath:            aws.ToString(tg.HealthCheckPath),
			HealthCheckIntervalSeconds: aws.ToInt32(tg.HealthCheckIntervalSeconds),
			HealthCheckTimeoutSeconds:  aws.ToInt32(tg.HealthCheckTimeoutSeconds),
			HealthyThresholdCount:      aws.ToInt32(tg.HealthyThresholdCount),
			UnhealthyThresholdCount:    aws.ToInt32(tg.UnhealthyThresholdCount),
		},
	}
}

// extractLoadBalancerARNs converts load balancer ARN slice to comma-separated string
func extractLoadBalancerARNs(arns []string) string {
	if len(arns) == 0 {
		return ""
	}
	return strings.Join(arns, ",")
}

// convertELBv2Tags converts ELBv2 tags to Elava tags
func convertELBv2Tags(tags []elbv2types.Tag) types.Tags {
	result := types.Tags{}
	for _, tag := range tags {
		key := aws.ToString(tag.Key)
		value := aws.ToString(tag.Value)

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
