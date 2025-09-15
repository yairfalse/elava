package aws

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs/types"

	ovitypes "github.com/yairfalse/elava/types"
)

// listCloudWatchLogs discovers CloudWatch Log Groups - CLAUDE.md: Small focused function
func (p *RealAWSProvider) listCloudWatchLogs(ctx context.Context, filter ovitypes.ResourceFilter) ([]ovitypes.Resource, error) {
	var resources []ovitypes.Resource

	// List all log groups in the region
	paginator := cloudwatchlogs.NewDescribeLogGroupsPaginator(p.cwLogsClient, &cloudwatchlogs.DescribeLogGroupsInput{})

	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to list CloudWatch log groups: %w", err)
		}

		for _, logGroup := range page.LogGroups {
			resource := convertLogGroupToResource(&logGroup, p.region)

			// Apply filter if specified
			if resource.Matches(filter) {
				resources = append(resources, resource)
			}
		}
	}

	return resources, nil
}

// convertLogGroupToResource converts AWS LogGroup to Ovi Resource - CLAUDE.md: Small focused function
func convertLogGroupToResource(logGroup *types.LogGroup, region string) ovitypes.Resource {
	// Determine status based on retention and activity
	status := "active"
	if logGroup.RetentionInDays == nil {
		status = "no_retention" // Expensive: infinite retention
	} else if !hasRecentActivity(logGroup) {
		status = "inactive" // Potentially unused
	}

	// Extract resource name from log group name
	name := ""
	if logGroup.LogGroupName != nil {
		name = *logGroup.LogGroupName
	}

	// Build creation time
	createdAt := time.Time{}
	if logGroup.CreationTime != nil {
		createdAt = time.UnixMilli(*logGroup.CreationTime)
	}

	// Build tags from log group name patterns
	tags := ovitypes.Tags{
		Name: name,
	}

	return ovitypes.Resource{
		ID:        name,
		Type:      "cloudwatch_logs",
		Provider:  "aws",
		Region:    region,
		Status:    status,
		CreatedAt: createdAt,
		Tags:      tags,
	}
}

// detectWastefulLogGroup checks if log group is wasteful - CLAUDE.md: Small focused function
func detectWastefulLogGroup(logGroup *types.LogGroup) bool {
	// No retention policy = infinite storage costs
	if logGroup.RetentionInDays == nil {
		return true
	}

	// Check for suspicious naming patterns
	if logGroup.LogGroupName != nil {
		name := strings.ToLower(*logGroup.LogGroupName)
		suspiciousPatterns := []string{"test", "temp", "old", "backup", "debug", "dev"}

		for _, pattern := range suspiciousPatterns {
			if strings.Contains(name, pattern) {
				return true
			}
		}
	}

	return false
}

// hasRecentActivity checks if log group has been active recently - CLAUDE.md: Small focused function
func hasRecentActivity(logGroup *types.LogGroup) bool {
	// For now, we'll consider log groups as potentially active
	// In a more complete implementation, we could query log events
	// but that would be expensive for many log groups

	// If log group has stored bytes, it has some activity
	if logGroup.StoredBytes != nil && *logGroup.StoredBytes > 0 {
		return true
	}

	return false
}
