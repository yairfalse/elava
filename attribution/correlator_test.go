package attribution

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/yairfalse/elava/analyzer"
	"github.com/yairfalse/elava/providers/aws"
)

func TestCorrelationEngine_CalculateTimeScore(t *testing.T) {
	engine := NewCorrelationEngine()
	baseTime := time.Now()

	tests := []struct {
		name      string
		driftTime time.Time
		eventTime time.Time
		wantScore float64
	}{
		{
			name:      "exact match",
			driftTime: baseTime,
			eventTime: baseTime,
			wantScore: 1.0,
		},
		{
			name:      "30 seconds apart",
			driftTime: baseTime,
			eventTime: baseTime.Add(-30 * time.Second),
			wantScore: 0.9,
		},
		{
			name:      "2 minutes apart",
			driftTime: baseTime,
			eventTime: baseTime.Add(-2 * time.Minute),
			wantScore: 0.6,
		},
		{
			name:      "5 minutes apart",
			driftTime: baseTime,
			eventTime: baseTime.Add(-5 * time.Minute),
			wantScore: 0.0,
		},
		{
			name:      "beyond window",
			driftTime: baseTime,
			eventTime: baseTime.Add(-10 * time.Minute),
			wantScore: 0.0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			score := engine.calculateTimeScore(tt.driftTime, tt.eventTime)
			assert.InDelta(t, tt.wantScore, score, 0.01)
		})
	}
}

func TestCorrelationEngine_ResourceMatches(t *testing.T) {
	engine := NewCorrelationEngine()

	tests := []struct {
		name       string
		resourceID string
		event      *aws.CloudTrailEvent
		want       bool
	}{
		{
			name:       "direct match",
			resourceID: "i-1234567890",
			event: &aws.CloudTrailEvent{
				ResourceID: "i-1234567890",
			},
			want: true,
		},
		{
			name:       "resource name match",
			resourceID: "arn:aws:ec2:us-east-1:123456789012:instance/i-1234567890",
			event: &aws.CloudTrailEvent{
				ResourceName: "i-1234567890",
			},
			want: true,
		},
		{
			name:       "no match",
			resourceID: "i-1234567890",
			event: &aws.CloudTrailEvent{
				ResourceID: "i-0987654321",
			},
			want: false,
		},
		{
			name:       "empty event",
			resourceID: "i-1234567890",
			event:      &aws.CloudTrailEvent{},
			want:       false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := engine.resourceMatches(tt.resourceID, tt.event)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestCorrelationEngine_IsRelevantAPICall(t *testing.T) {
	engine := NewCorrelationEngine()

	tests := []struct {
		name       string
		eventName  string
		changeType analyzer.ChangeType
		want       bool
	}{
		{
			name:       "creation event matches created change",
			eventName:  "RunInstances",
			changeType: analyzer.ChangeCreated,
			want:       true,
		},
		{
			name:       "modify event matches modified change",
			eventName:  "ModifyDBInstance",
			changeType: analyzer.ChangeModified,
			want:       true,
		},
		{
			name:       "tag event matches tag change",
			eventName:  "CreateTags",
			changeType: analyzer.ChangeTagsChanged,
			want:       true,
		},
		{
			name:       "deletion event matches disappeared change",
			eventName:  "TerminateInstances",
			changeType: analyzer.ChangeDisappeared,
			want:       true,
		},
		{
			name:       "mismatched event and change",
			eventName:  "RunInstances",
			changeType: analyzer.ChangeDisappeared,
			want:       false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := engine.isRelevantAPICall(tt.eventName, tt.changeType)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestCorrelationEngine_Correlate(t *testing.T) {
	engine := NewCorrelationEngine()
	baseTime := time.Now()

	drift := analyzer.DriftEvent{
		ResourceID: "i-1234567890",
		Timestamp:  baseTime,
		Type:       string(analyzer.ChangeCreated),
	}

	tests := []struct {
		name          string
		events        []aws.CloudTrailEvent
		wantActor     string
		wantConfident bool
	}{
		{
			name: "perfect match",
			events: []aws.CloudTrailEvent{
				{
					EventName:  "RunInstances",
					EventTime:  baseTime.Add(-30 * time.Second),
					ResourceID: "i-1234567890",
					Username:   "john.doe",
				},
			},
			wantActor:     "john.doe",
			wantConfident: true,
		},
		{
			name: "multiple events - best match wins",
			events: []aws.CloudTrailEvent{
				{
					EventName:  "RunInstances",
					EventTime:  baseTime.Add(-5 * time.Minute),
					ResourceID: "i-1234567890",
					Username:   "old.user",
				},
				{
					EventName:  "RunInstances",
					EventTime:  baseTime.Add(-10 * time.Second),
					ResourceID: "i-1234567890",
					Username:   "recent.user",
				},
			},
			wantActor:     "recent.user",
			wantConfident: true,
		},
		{
			name:          "no events",
			events:        []aws.CloudTrailEvent{},
			wantActor:     "",
			wantConfident: false,
		},
		{
			name: "low confidence match",
			events: []aws.CloudTrailEvent{
				{
					EventName:  "DescribeInstances", // Wrong API
					EventTime:  baseTime.Add(-4 * time.Minute),
					ResourceID: "i-1234567890",
					Username:   "viewer.user",
				},
			},
			wantActor:     "",
			wantConfident: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			attr, err := engine.Correlate(drift, tt.events)
			assert.NoError(t, err)

			if tt.wantConfident {
				assert.NotNil(t, attr)
				assert.Equal(t, tt.wantActor, attr.Actor)
				assert.Greater(t, attr.Confidence, 0.5)
			} else {
				if attr != nil {
					assert.LessOrEqual(t, attr.Confidence, 0.5)
				}
			}
		})
	}
}

func TestCorrelationEngine_DetermineActorType(t *testing.T) {
	engine := NewCorrelationEngine()

	tests := []struct {
		name  string
		event *aws.CloudTrailEvent
		want  string
	}{
		{
			name: "terraform automation",
			event: &aws.CloudTrailEvent{
				UserAgent: "terraform/1.0.0",
			},
			want: string(ActorTypeAutomation),
		},
		{
			name: "cloudformation automation",
			event: &aws.CloudTrailEvent{
				UserAgent: "CloudFormation",
			},
			want: string(ActorTypeAutomation),
		},
		{
			name: "assumed role service",
			event: &aws.CloudTrailEvent{
				UserType: "AssumedRole",
			},
			want: string(ActorTypeService),
		},
		{
			name: "IAM user human",
			event: &aws.CloudTrailEvent{
				UserType: "IAMUser",
			},
			want: string(ActorTypeHuman),
		},
		{
			name:  "unknown",
			event: &aws.CloudTrailEvent{},
			want:  string(ActorTypeUnknown),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := engine.determineActorType(tt.event)
			assert.Equal(t, tt.want, got)
		})
	}
}
