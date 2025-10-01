package aws

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
	"github.com/aws/aws-sdk-go-v2/service/sqs/types"

	elavatypes "github.com/yairfalse/elava/types"
)

// listSQSQueues discovers SQS queues
func (p *RealAWSProvider) listSQSQueues(ctx context.Context, filter elavatypes.ResourceFilter) ([]elavatypes.Resource, error) {
	var resources []elavatypes.Resource

	input := &sqs.ListQueuesInput{}

	// Apply prefix filter if queue names specified in IDs
	if len(filter.IDs) > 0 {
		input.QueueNamePrefix = &filter.IDs[0]
	}

	output, err := p.sqsClient.ListQueues(ctx, input)
	if err != nil {
		return nil, fmt.Errorf("failed to list SQS queues: %w", err)
	}

	for _, queueURL := range output.QueueUrls {
		resource, err := p.convertSQSQueue(ctx, queueURL)
		if err != nil {
			// Log error but continue processing other queues
			continue
		}
		resources = append(resources, resource)
	}

	return resources, nil
}

// convertSQSQueue converts SQS queue URL to Elava resource
func (p *RealAWSProvider) convertSQSQueue(ctx context.Context, queueURL string) (elavatypes.Resource, error) {
	queueName := extractQueueName(queueURL)

	// Get queue attributes for detailed metadata
	attrs, err := p.getSQSQueueAttributes(ctx, queueURL)
	if err != nil {
		return elavatypes.Resource{}, fmt.Errorf("failed to get queue attributes: %w", err)
	}

	// Get queue tags for ownership detection
	tags, err := p.getSQSQueueTags(ctx, queueURL)
	if err != nil {
		// Tags might not be available, continue without them
		tags = elavatypes.Tags{}
	}

	isOrphaned := p.isResourceOrphaned(tags)

	return elavatypes.Resource{
		ID:         queueURL,
		Type:       "sqs",
		Provider:   "aws",
		Region:     p.region,
		AccountID:  p.accountID,
		Status:     "available", // SQS queues don't have complex status
		Tags:       tags,
		CreatedAt:  parseQueueCreatedTime(attrs),
		IsOrphaned: isOrphaned,
		Metadata:   buildSQSMetadata(attrs, queueName),
	}, nil
}

// getSQSQueueAttributes retrieves queue attributes for metadata
func (p *RealAWSProvider) getSQSQueueAttributes(ctx context.Context, queueURL string) (map[string]string, error) {
	input := &sqs.GetQueueAttributesInput{
		QueueUrl: aws.String(queueURL),
		AttributeNames: []types.QueueAttributeName{
			types.QueueAttributeNameCreatedTimestamp,
			types.QueueAttributeNameLastModifiedTimestamp,
			types.QueueAttributeNameApproximateNumberOfMessages,
			types.QueueAttributeNameApproximateNumberOfMessagesNotVisible,
			types.QueueAttributeNameApproximateNumberOfMessagesDelayed,
			types.QueueAttributeNameVisibilityTimeout,
			types.QueueAttributeNameMessageRetentionPeriod,
			types.QueueAttributeNameDelaySeconds,
			types.QueueAttributeNameReceiveMessageWaitTimeSeconds,
			types.QueueAttributeNameQueueArn,
		},
	}

	output, err := p.sqsClient.GetQueueAttributes(ctx, input)
	if err != nil {
		return nil, fmt.Errorf("failed to get queue attributes: %w", err)
	}

	return output.Attributes, nil
}

// getSQSQueueTags retrieves queue tags for ownership detection
func (p *RealAWSProvider) getSQSQueueTags(ctx context.Context, queueURL string) (elavatypes.Tags, error) {
	input := &sqs.ListQueueTagsInput{
		QueueUrl: aws.String(queueURL),
	}

	output, err := p.sqsClient.ListQueueTags(ctx, input)
	if err != nil {
		return elavatypes.Tags{}, err
	}

	return convertSQSTagsToElava(output.Tags), nil
}

// extractQueueName extracts queue name from queue URL
func extractQueueName(queueURL string) string {
	parts := strings.Split(queueURL, "/")
	if len(parts) > 0 {
		return parts[len(parts)-1]
	}
	return queueURL
}

// parseQueueCreatedTime parses creation timestamp from attributes
func parseQueueCreatedTime(attrs map[string]string) time.Time {
	if createdStr, exists := attrs["CreatedTimestamp"]; exists {
		if timestamp, err := strconv.ParseInt(createdStr, 10, 64); err == nil {
			return time.Unix(timestamp, 0)
		}
	}
	return time.Time{}
}

// buildSQSMetadata builds metadata from queue attributes
func buildSQSMetadata(attrs map[string]string, queueName string) elavatypes.ResourceMetadata {
	metadata := elavatypes.ResourceMetadata{}

	// Parse message counts
	if msgCount, exists := attrs["ApproximateNumberOfMessages"]; exists {
		if count, err := strconv.Atoi(msgCount); err == nil {
			metadata.ItemCount = int64(count)
		}
	}

	// Check if queue appears empty/idle
	metadata.IsEmpty = metadata.ItemCount == 0

	// Parse visibility timeout
	if visTimeout, exists := attrs["VisibilityTimeout"]; exists {
		if timeout, err := strconv.Atoi(visTimeout); err == nil {
			metadata.Timeout = int32(timeout)
		}
	}

	// Parse message retention period
	if retention, exists := attrs["MessageRetentionPeriod"]; exists {
		if period, err := strconv.Atoi(retention); err == nil {
			metadata.BackupRetentionPeriod = period
		}
	}

	// Parse last modified time for age calculation
	if modifiedStr, exists := attrs["LastModifiedTimestamp"]; exists {
		if timestamp, err := strconv.ParseInt(modifiedStr, 10, 64); err == nil {
			modifiedTime := time.Unix(timestamp, 0)
			metadata.ModifiedTime = modifiedTime
			metadata.DaysSinceModified = int(time.Since(modifiedTime).Hours() / 24)
		}
	}

	return metadata
}

// convertSQSTagsToElava converts SQS tags to Elava tag format
func convertSQSTagsToElava(sqsTags map[string]string) elavatypes.Tags {
	return elavatypes.Tags{
		ElavaOwner:   sqsTags["elava:owner"],
		ElavaManaged: sqsTags["elava:managed"] == "true",
		Environment:  sqsTags["Environment"],
		Project:      sqsTags["Project"],
		Team:         sqsTags["Team"],
		Name:         sqsTags["Name"],
		CostCenter:   sqsTags["CostCenter"],
		Owner:        sqsTags["Owner"],
	}
}
