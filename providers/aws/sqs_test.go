package aws

import (
	"testing"
	"time"

	"github.com/yairfalse/elava/types"
)

func TestExtractQueueName(t *testing.T) {
	tests := []struct {
		name     string
		queueURL string
		want     string
	}{
		{
			name:     "standard queue URL",
			queueURL: "https://sqs.us-east-1.amazonaws.com/123456789012/test-queue",
			want:     "test-queue",
		},
		{
			name:     "FIFO queue URL",
			queueURL: "https://sqs.us-east-1.amazonaws.com/123456789012/test-queue.fifo",
			want:     "test-queue.fifo",
		},
		{
			name:     "queue with hyphens",
			queueURL: "https://sqs.us-west-2.amazonaws.com/987654321098/my-test-queue-prod",
			want:     "my-test-queue-prod",
		},
		{
			name:     "single part",
			queueURL: "simple-queue",
			want:     "simple-queue",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractQueueName(tt.queueURL)
			if got != tt.want {
				t.Errorf("extractQueueName() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestParseQueueCreatedTime(t *testing.T) {
	tests := []struct {
		name  string
		attrs map[string]string
		want  time.Time
	}{
		{
			name: "valid timestamp",
			attrs: map[string]string{
				"CreatedTimestamp": "1609459200", // 2021-01-01 00:00:00 UTC
			},
			want: time.Unix(1609459200, 0),
		},
		{
			name: "invalid timestamp",
			attrs: map[string]string{
				"CreatedTimestamp": "invalid",
			},
			want: time.Time{},
		},
		{
			name:  "missing timestamp",
			attrs: map[string]string{},
			want:  time.Time{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseQueueCreatedTime(tt.attrs)
			if !got.Equal(tt.want) {
				t.Errorf("parseQueueCreatedTime() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestBuildSQSMetadata(t *testing.T) {
	tests := []struct {
		name      string
		attrs     map[string]string
		queueName string
		checks    func(t *testing.T, metadata types.ResourceMetadata)
	}{
		{
			name: "queue with messages",
			attrs: map[string]string{
				"ApproximateNumberOfMessages": "100",
				"VisibilityTimeout":           "30",
				"MessageRetentionPeriod":      "1209600",
				"LastModifiedTimestamp":       "1609459200",
			},
			queueName: "test-queue",
			checks: func(t *testing.T, metadata types.ResourceMetadata) {
				if metadata.ItemCount != 100 {
					t.Errorf("Expected ItemCount 100, got %d", metadata.ItemCount)
				}
				if metadata.IsEmpty {
					t.Error("Expected IsEmpty to be false")
				}
				if metadata.Timeout != 30 {
					t.Errorf("Expected Timeout 30, got %d", metadata.Timeout)
				}
				if metadata.BackupRetentionPeriod != 1209600 {
					t.Errorf("Expected BackupRetentionPeriod 1209600, got %d", metadata.BackupRetentionPeriod)
				}
			},
		},
		{
			name: "empty queue",
			attrs: map[string]string{
				"ApproximateNumberOfMessages": "0",
			},
			queueName: "empty-queue",
			checks: func(t *testing.T, metadata types.ResourceMetadata) {
				if metadata.ItemCount != 0 {
					t.Errorf("Expected ItemCount 0, got %d", metadata.ItemCount)
				}
				if !metadata.IsEmpty {
					t.Error("Expected IsEmpty to be true")
				}
			},
		},
		{
			name: "queue with old modification time",
			attrs: map[string]string{
				"LastModifiedTimestamp": "1577836800", // 2020-01-01 00:00:00 UTC
			},
			queueName: "old-queue",
			checks: func(t *testing.T, metadata types.ResourceMetadata) {
				if metadata.DaysSinceModified <= 0 {
					t.Errorf("Expected DaysSinceModified > 0, got %d", metadata.DaysSinceModified)
				}
				expectedTime := time.Unix(1577836800, 0)
				if !metadata.ModifiedTime.Equal(expectedTime) {
					t.Errorf("Expected ModifiedTime %v, got %v", expectedTime, metadata.ModifiedTime)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			metadata := buildSQSMetadata(tt.attrs, tt.queueName)
			tt.checks(t, metadata)
		})
	}
}

func TestConvertSQSTagsToElava(t *testing.T) {
	tests := []struct {
		name    string
		sqsTags map[string]string
		want    types.Tags
	}{
		{
			name: "complete tags",
			sqsTags: map[string]string{
				"elava:owner":   "team-web",
				"elava:managed": "true",
				"Environment":   "production",
				"Project":       "ecommerce",
				"Team":          "backend",
				"Name":          "order-processing",
				"CostCenter":    "engineering",
				"Owner":         "john.doe",
			},
			want: types.Tags{
				ElavaOwner:   "team-web",
				ElavaManaged: true,
				Environment:  "production",
				Project:      "ecommerce",
				Team:         "backend",
				Name:         "order-processing",
				CostCenter:   "engineering",
				Owner:        "john.doe",
			},
		},
		{
			name: "minimal tags",
			sqsTags: map[string]string{
				"Environment": "development",
			},
			want: types.Tags{
				ElavaOwner:   "",
				ElavaManaged: false,
				Environment:  "development",
				Project:      "",
				Team:         "",
				Name:         "",
				CostCenter:   "",
				Owner:        "",
			},
		},
		{
			name:    "empty tags",
			sqsTags: map[string]string{},
			want: types.Tags{
				ElavaOwner:   "",
				ElavaManaged: false,
				Environment:  "",
				Project:      "",
				Team:         "",
				Name:         "",
				CostCenter:   "",
				Owner:        "",
			},
		},
		{
			name: "elava managed false",
			sqsTags: map[string]string{
				"elava:managed": "false",
				"Team":          "ops",
			},
			want: types.Tags{
				ElavaOwner:   "",
				ElavaManaged: false,
				Environment:  "",
				Project:      "",
				Team:         "ops",
				Name:         "",
				CostCenter:   "",
				Owner:        "",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := convertSQSTagsToElava(tt.sqsTags)
			if got != tt.want {
				t.Errorf("convertSQSTagsToElava() = %+v, want %+v", got, tt.want)
			}
		})
	}
}

func TestSQSLister_Name(t *testing.T) {
	lister := &SQSLister{}
	expected := "SQS queues"
	if got := lister.Name(); got != expected {
		t.Errorf("SQSLister.Name() = %v, want %v", got, expected)
	}
}

func TestSQSLister_IsCritical(t *testing.T) {
	lister := &SQSLister{}
	if lister.IsCritical() {
		t.Error("SQSLister.IsCritical() should return false")
	}
}

// Integration test helper to validate SQS resource structure
func TestSQSResourceStructure(t *testing.T) {
	// This test validates that our SQS resource structure
	// matches the expected Elava resource format
	queueURL := "https://sqs.us-east-1.amazonaws.com/123456789012/test-queue"
	queueName := "test-queue"

	attrs := map[string]string{
		"CreatedTimestamp":            "1609459200",
		"ApproximateNumberOfMessages": "50",
		"VisibilityTimeout":           "30",
		"MessageRetentionPeriod":      "1209600",
		"LastModifiedTimestamp":       "1609459200",
	}

	sqsTags := map[string]string{
		"elava:owner": "team-web",
		"Environment": "production",
		"Project":     "ecommerce",
	}

	tags := convertSQSTagsToElava(sqsTags)
	metadata := buildSQSMetadata(attrs, queueName)
	createdAt := parseQueueCreatedTime(attrs)

	// Validate resource structure components
	if extractQueueName(queueURL) != queueName {
		t.Error("Queue name extraction failed")
	}

	if tags.ElavaOwner != "team-web" {
		t.Error("Tag conversion failed")
	}

	if metadata.ItemCount != 50 {
		t.Error("Metadata building failed")
	}

	if createdAt.IsZero() {
		t.Error("Time parsing failed")
	}

	// Validate that metadata has expected SQS-specific fields
	if metadata.Timeout != 30 {
		t.Error("Visibility timeout not properly set")
	}

	if metadata.BackupRetentionPeriod != 1209600 {
		t.Error("Message retention period not properly set")
	}

	if !metadata.IsEmpty && metadata.ItemCount == 0 {
		t.Error("IsEmpty flag inconsistent with ItemCount")
	}
}
