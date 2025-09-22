package aws

import (
	"context"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/cloudtrail"
	"github.com/aws/aws-sdk-go-v2/service/cloudtrail/types"
	"github.com/yairfalse/elava/telemetry"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/trace"
)

// CloudTrailClient queries AWS CloudTrail for attribution
type CloudTrailClient struct {
	client *cloudtrail.Client
	logger *telemetry.Logger
	tracer trace.Tracer
}

// NewCloudTrailClient creates a new CloudTrail client
func NewCloudTrailClient(cfg aws.Config) *CloudTrailClient {
	return &CloudTrailClient{
		client: cloudtrail.NewFromConfig(cfg),
		logger: telemetry.NewLogger("cloudtrail-client"),
		tracer: otel.Tracer("cloudtrail-client"),
	}
}

// CloudTrailEvent represents an AWS API call
type CloudTrailEvent struct {
	EventID      string
	EventName    string // API call like "RunInstances"
	EventTime    time.Time
	Username     string // Who made the call
	UserType     string // IAMUser, AssumedRole, etc
	SourceIP     string
	UserAgent    string
	ResourceName string
	ResourceID   string
	RequestID    string
	ErrorCode    string
}

// QueryResourceEvents queries CloudTrail for events related to a resource
func (c *CloudTrailClient) QueryResourceEvents(
	ctx context.Context,
	resourceID string,
	timeWindow time.Duration,
) ([]CloudTrailEvent, error) {
	ctx, span := c.tracer.Start(ctx, "QueryResourceEvents")
	defer span.End()

	endTime := time.Now()
	startTime := endTime.Add(-timeWindow)

	return c.lookupEvents(ctx, resourceID, startTime, endTime)
}

// QueryTimeRange queries CloudTrail events in a time range
func (c *CloudTrailClient) QueryTimeRange(
	ctx context.Context,
	start, end time.Time,
) ([]CloudTrailEvent, error) {
	ctx, span := c.tracer.Start(ctx, "QueryTimeRange")
	defer span.End()

	return c.lookupEvents(ctx, "", start, end)
}

// lookupEvents performs the actual CloudTrail API call
func (c *CloudTrailClient) lookupEvents(
	ctx context.Context,
	resourceID string,
	startTime, endTime time.Time,
) ([]CloudTrailEvent, error) {
	var lookupAttributes []types.LookupAttribute

	if resourceID != "" {
		lookupAttributes = append(lookupAttributes, types.LookupAttribute{
			AttributeKey:   types.LookupAttributeKeyResourceName,
			AttributeValue: aws.String(resourceID),
		})
	}

	input := &cloudtrail.LookupEventsInput{
		LookupAttributes: lookupAttributes,
		StartTime:        &startTime,
		EndTime:          &endTime,
		MaxResults:       aws.Int32(50), // Max allowed per request
	}

	result, err := c.client.LookupEvents(ctx, input)
	if err != nil {
		return nil, fmt.Errorf("failed to lookup CloudTrail events: %w", err)
	}

	return c.convertEvents(result.Events), nil
}

// convertEvents converts CloudTrail SDK events to our type
func (c *CloudTrailClient) convertEvents(
	events []types.Event,
) []CloudTrailEvent {
	var result []CloudTrailEvent

	for _, event := range events {
		result = append(result, c.convertEvent(event))
	}

	return result
}

// convertEvent converts a single CloudTrail event
func (c *CloudTrailClient) convertEvent(event types.Event) CloudTrailEvent {
	ce := CloudTrailEvent{
		EventID:   aws.ToString(event.EventId),
		EventName: aws.ToString(event.EventName),
		EventTime: aws.ToTime(event.EventTime),
		Username:  aws.ToString(event.Username),
	}

	// Extract resource information
	for _, resource := range event.Resources {
		if aws.ToString(resource.ResourceType) == "AWS::EC2::Instance" ||
			aws.ToString(resource.ResourceType) == "AWS::RDS::DBInstance" {
			ce.ResourceName = aws.ToString(resource.ResourceName)
		}
	}

	// CloudTrail SDK doesn't expose all fields directly
	// We'll need to parse them from CloudTrailEvent JSON if needed
	// For now, return what we have

	return ce
}

// IsCreationEvent checks if an event represents resource creation
func IsCreationEvent(eventName string) bool {
	creationEvents := map[string]bool{
		"RunInstances":       true,
		"CreateDBInstance":   true,
		"CreateBucket":       true,
		"CreateFunction":     true,
		"CreateLoadBalancer": true,
		"CreateCluster":      true,
	}

	return creationEvents[eventName]
}

// IsModificationEvent checks if an event represents resource modification
func IsModificationEvent(eventName string) bool {
	modificationEvents := map[string]bool{
		"ModifyDBInstance":        true,
		"ModifyInstanceAttribute": true,
		"CreateTags":              true,
		"DeleteTags":              true,
		"UpdateFunctionCode":      true,
		"ModifyLoadBalancer":      true,
	}

	return modificationEvents[eventName]
}
