package aws

import (
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs/types"
)

// Test detectWastefulLogGroup - Single responsibility tests
func TestDetectWastefulLogGroup_NoRetentionPolicy(t *testing.T) {
	logGroup := &types.LogGroup{
		LogGroupName: stringPtr("/aws/lambda/my-function"),
		// RetentionInDays is nil = no retention policy
	}

	isWasteful := detectWastefulLogGroup(logGroup)

	if !isWasteful {
		t.Error("Expected log group with no retention policy to be wasteful")
	}
}

func TestDetectWastefulLogGroup_HasRetentionPolicy(t *testing.T) {
	logGroup := &types.LogGroup{
		LogGroupName:    stringPtr("/aws/lambda/my-function"),
		RetentionInDays: int32Ptr(14), // Has retention policy
	}

	isWasteful := detectWastefulLogGroup(logGroup)

	if isWasteful {
		t.Error("Expected log group with retention policy to not be wasteful")
	}
}

func TestDetectWastefulLogGroup_TestLogGroup(t *testing.T) {
	logGroup := &types.LogGroup{
		LogGroupName:    stringPtr("/aws/lambda/test-function"),
		RetentionInDays: int32Ptr(7),
	}

	isWasteful := detectWastefulLogGroup(logGroup)

	if !isWasteful {
		t.Error("Expected test log group to be considered wasteful")
	}
}

func TestDetectWastefulLogGroup_TempLogGroup(t *testing.T) {
	logGroup := &types.LogGroup{
		LogGroupName:    stringPtr("/aws/lambda/temp-processing"),
		RetentionInDays: int32Ptr(30),
	}

	isWasteful := detectWastefulLogGroup(logGroup)

	if !isWasteful {
		t.Error("Expected temp log group to be considered wasteful")
	}
}

// Test hasRecentActivity
func TestHasRecentActivity_HasStoredBytes(t *testing.T) {
	storedBytes := int64(1024 * 1024) // 1MB
	logGroup := &types.LogGroup{
		LogGroupName: stringPtr("/aws/lambda/active-function"),
		StoredBytes:  &storedBytes,
	}

	hasActivity := hasRecentActivity(logGroup)

	if !hasActivity {
		t.Error("Expected log group with stored bytes to have activity")
	}
}

func TestHasRecentActivity_NoStoredBytes(t *testing.T) {
	storedBytes := int64(0)
	logGroup := &types.LogGroup{
		LogGroupName: stringPtr("/aws/lambda/empty-function"),
		StoredBytes:  &storedBytes,
	}

	hasActivity := hasRecentActivity(logGroup)

	if hasActivity {
		t.Error("Expected log group with no stored bytes to not have activity")
	}
}

func TestHasRecentActivity_NilStoredBytes(t *testing.T) {
	logGroup := &types.LogGroup{
		LogGroupName: stringPtr("/aws/lambda/empty-function"),
		// No StoredBytes field
	}

	hasActivity := hasRecentActivity(logGroup)

	if hasActivity {
		t.Error("Expected log group with nil stored bytes to not have activity")
	}
}

// Test resource conversion
func TestConvertLogGroupToResource_WastefulGroup(t *testing.T) {
	logGroup := &types.LogGroup{
		LogGroupName: stringPtr("/aws/lambda/test-function"),
		Arn:          stringPtr("arn:aws:logs:us-east-1:123456789012:log-group:/aws/lambda/test-function"),
		CreationTime: int64Ptr(time.Now().Add(-48 * time.Hour).UnixMilli()), // 2 days old
		StoredBytes:  int64Ptr(1024 * 1024),                                 // 1MB
		// No RetentionInDays = infinite retention
	}

	resource := convertLogGroupToResource(logGroup, "us-east-1")

	// Verify basic conversion
	if resource.ID != "/aws/lambda/test-function" {
		t.Errorf("Expected resource ID to be log group name, got %s", resource.ID)
	}

	if resource.Type != "cloudwatch_logs" {
		t.Errorf("Expected resource type to be cloudwatch_logs, got %s", resource.Type)
	}

	if resource.Status != "no_retention" {
		t.Errorf("Expected status to be no_retention, got %s", resource.Status)
	}

	if resource.Region != "us-east-1" {
		t.Errorf("Expected region to be us-east-1, got %s", resource.Region)
	}
}

func TestConvertLogGroupToResource_HealthyGroup(t *testing.T) {
	storedBytes := int64(1024 * 1024) // 1MB
	logGroup := &types.LogGroup{
		LogGroupName:    stringPtr("/aws/lambda/production-function"),
		RetentionInDays: int32Ptr(30),
		StoredBytes:     &storedBytes,
	}

	resource := convertLogGroupToResource(logGroup, "us-east-1")

	if resource.Status != "active" {
		t.Errorf("Expected status to be active for healthy group, got %s", resource.Status)
	}
}

// Table-driven test for multiple scenarios
func TestLogGroupWastefulnessScenarios(t *testing.T) {
	tests := []struct {
		name         string
		logGroup     *types.LogGroup
		wantWasteful bool
	}{
		{
			name: "no retention, production name",
			logGroup: &types.LogGroup{
				LogGroupName: stringPtr("/aws/lambda/prod-api"),
				// No RetentionInDays
			},
			wantWasteful: true,
		},
		{
			name: "has retention, production name",
			logGroup: &types.LogGroup{
				LogGroupName:    stringPtr("/aws/lambda/prod-api"),
				RetentionInDays: int32Ptr(90),
			},
			wantWasteful: false,
		},
		{
			name: "test name, has retention",
			logGroup: &types.LogGroup{
				LogGroupName:    stringPtr("/aws/lambda/test-service"),
				RetentionInDays: int32Ptr(7),
			},
			wantWasteful: true,
		},
		{
			name: "temp name, has retention",
			logGroup: &types.LogGroup{
				LogGroupName:    stringPtr("/aws/lambda/temp-migration"),
				RetentionInDays: int32Ptr(14),
			},
			wantWasteful: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := detectWastefulLogGroup(tt.logGroup)
			if got != tt.wantWasteful {
				t.Errorf("detectWastefulLogGroup() = %v, want %v", got, tt.wantWasteful)
			}
		})
	}
}

// Helper functions for test data
func stringPtr(s string) *string {
	return &s
}

func int32Ptr(i int32) *int32 {
	return &i
}

func int64Ptr(i int64) *int64 {
	return &i
}
