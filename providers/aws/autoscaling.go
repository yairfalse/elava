package aws

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/autoscaling"
	autoscalingtypes "github.com/aws/aws-sdk-go-v2/service/autoscaling/types"

	"github.com/yairfalse/elava/types"
)

// ListAutoScalingGroups scans all Auto Scaling Groups
func (p *RealAWSProvider) ListAutoScalingGroups(ctx context.Context) ([]types.Resource, error) {
	var allASGs []autoscalingtypes.AutoScalingGroup
	var nextToken *string

	for {
		input := &autoscaling.DescribeAutoScalingGroupsInput{
			NextToken: nextToken,
		}
		output, err := p.asgClient.DescribeAutoScalingGroups(ctx, input)
		if err != nil {
			return nil, fmt.Errorf("failed to describe auto scaling groups: %w", err)
		}
		allASGs = append(allASGs, output.AutoScalingGroups...)
		if output.NextToken == nil {
			break
		}
		nextToken = output.NextToken
	}

	resources := make([]types.Resource, 0, len(allASGs))
	for _, asg := range allASGs {

  output, err := p.asgClient.DescribeAutoScalingGroups(ctx, &autoscaling.DescribeAutoScalingGroupsInput{})
	if err != nil {
		return nil, fmt.Errorf("failed to describe auto scaling groups: %w", err)
	}

	resources := make([]types.Resource, 0, len(output.AutoScalingGroups))
	for _, asg := range output.AutoScalingGroups {
		resource := buildASGResource(asg, p.region, p.accountID)
		resources = append(resources, resource)
	}

	return resources, nil
}

// buildASGResource converts AWS ASG to types.Resource
func buildASGResource(asg autoscalingtypes.AutoScalingGroup, region, accountID string) types.Resource {
	instanceIDs := extractInstanceIDs(asg.Instances)
	targetGroupARNs := extractTargetGroupARNs(asg.TargetGroupARNs)
	launchTemplate := extractLaunchTemplateName(asg.LaunchTemplate)

	return types.Resource{
		ID:         aws.ToString(asg.AutoScalingGroupName),
		Type:       "autoscaling_group",
		Provider:   "aws",
		Region:     region,
		AccountID:  accountID,
		Name:       aws.ToString(asg.AutoScalingGroupName),
		Status:     "active",
		Tags:       convertASGTags(asg.Tags),
		CreatedAt:  aws.ToTime(asg.CreatedTime),
		LastSeenAt: time.Now(),
		Metadata: types.ResourceMetadata{
			MinSize:            aws.ToInt32(asg.MinSize),
			MaxSize:            aws.ToInt32(asg.MaxSize),
			DesiredCapacity:    aws.ToInt32(asg.DesiredCapacity),
			CurrentSize:        int32(len(asg.Instances)),
			InstanceIDs:        instanceIDs,
			LaunchTemplate:     launchTemplate,
			TargetGroupARNs:    targetGroupARNs,
			VPCZoneIdentifiers: aws.ToString(asg.VPCZoneIdentifier),
		},
	}
}

// extractInstanceIDs extracts instance IDs from ASG instances
func extractInstanceIDs(instances []autoscalingtypes.Instance) string {
	if len(instances) == 0 {
		return ""
	}

	ids := make([]string, len(instances))
	for i, inst := range instances {
		ids[i] = aws.ToString(inst.InstanceId)
	}
	return strings.Join(ids, ",")
}

// extractTargetGroupARNs converts target group ARN slice to comma-separated string
func extractTargetGroupARNs(arns []string) string {
	if len(arns) == 0 {
		return ""
	}
	return strings.Join(arns, ",")
}

// extractLaunchTemplateName extracts launch template name or ID
func extractLaunchTemplateName(template *autoscalingtypes.LaunchTemplateSpecification) string {
	if template == nil {
		return ""
	}

	// Prefer name over ID
	if template.LaunchTemplateName != nil {
		return aws.ToString(template.LaunchTemplateName)
	}

	return aws.ToString(template.LaunchTemplateId)
}

// convertASGTags converts ASG tags to Elava tags
func convertASGTags(tags []autoscalingtypes.TagDescription) types.Tags {
	result := types.Tags{}
	for _, tag := range tags {
		key := aws.ToString(tag.Key)
		value := aws.ToString(tag.Value)

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
