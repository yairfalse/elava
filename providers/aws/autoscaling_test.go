package aws

import (
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	autoscalingtypes "github.com/aws/aws-sdk-go-v2/service/autoscaling/types"
	"github.com/stretchr/testify/assert"
)

func TestBuildASGResource(t *testing.T) {
	t.Run("basic ASG with instances", func(t *testing.T) {
		asg := autoscalingtypes.AutoScalingGroup{
			AutoScalingGroupName: aws.String("web-asg-prod"),
			MinSize:              aws.Int32(2),
			MaxSize:              aws.Int32(10),
			DesiredCapacity:      aws.Int32(5),
			Instances: []autoscalingtypes.Instance{
				{InstanceId: aws.String("i-abc123")},
				{InstanceId: aws.String("i-def456")},
				{InstanceId: aws.String("i-ghi789")},
			},
			Tags: []autoscalingtypes.TagDescription{
				{Key: aws.String("Environment"), Value: aws.String("production")},
			},
		}

		resource := buildASGResource(asg, "us-east-1", "123456789")

		assert.Equal(t, "autoscaling_group", resource.Type)
		assert.Equal(t, "web-asg-prod", resource.ID)
		assert.Equal(t, int32(2), resource.Metadata.MinSize)
		assert.Equal(t, int32(10), resource.Metadata.MaxSize)
		assert.Equal(t, int32(5), resource.Metadata.DesiredCapacity)
		assert.Equal(t, int32(3), resource.Metadata.CurrentSize)
		assert.Equal(t, "i-abc123,i-def456,i-ghi789", resource.Metadata.InstanceIDs)
	})

	t.Run("ASG with launch template", func(t *testing.T) {
		asg := autoscalingtypes.AutoScalingGroup{
			AutoScalingGroupName: aws.String("api-asg"),
			MinSize:              aws.Int32(1),
			MaxSize:              aws.Int32(5),
			DesiredCapacity:      aws.Int32(2),
			LaunchTemplate: &autoscalingtypes.LaunchTemplateSpecification{
				LaunchTemplateId:   aws.String("lt-abc123"),
				LaunchTemplateName: aws.String("api-template"),
			},
		}

		resource := buildASGResource(asg, "us-east-1", "123456789")

		assert.Equal(t, "api-template", resource.Metadata.LaunchTemplate)
	})

	t.Run("ASG with target groups", func(t *testing.T) {
		asg := autoscalingtypes.AutoScalingGroup{
			AutoScalingGroupName: aws.String("web-asg"),
			MinSize:              aws.Int32(2),
			MaxSize:              aws.Int32(10),
			DesiredCapacity:      aws.Int32(5),
			TargetGroupARNs: []string{
				"arn:aws:elasticloadbalancing:us-east-1:123:targetgroup/web-tg/abc",
				"arn:aws:elasticloadbalancing:us-east-1:123:targetgroup/api-tg/def",
			},
		}

		resource := buildASGResource(asg, "us-east-1", "123456789")

		expected := "arn:aws:elasticloadbalancing:us-east-1:123:targetgroup/web-tg/abc,arn:aws:elasticloadbalancing:us-east-1:123:targetgroup/api-tg/def"
		assert.Equal(t, expected, resource.Metadata.TargetGroupARNs)
	})

	t.Run("ASG with VPC zone identifiers", func(t *testing.T) {
		asg := autoscalingtypes.AutoScalingGroup{
			AutoScalingGroupName: aws.String("db-asg"),
			MinSize:              aws.Int32(1),
			MaxSize:              aws.Int32(3),
			DesiredCapacity:      aws.Int32(2),
			VPCZoneIdentifier:    aws.String("subnet-111,subnet-222,subnet-333"),
		}

		resource := buildASGResource(asg, "us-east-1", "123456789")

		assert.Equal(t, "subnet-111,subnet-222,subnet-333", resource.Metadata.VPCZoneIdentifiers)
	})

	t.Run("empty ASG", func(t *testing.T) {
		asg := autoscalingtypes.AutoScalingGroup{
			AutoScalingGroupName: aws.String("empty-asg"),
			MinSize:              aws.Int32(0),
			MaxSize:              aws.Int32(0),
			DesiredCapacity:      aws.Int32(0),
		}

		resource := buildASGResource(asg, "us-east-1", "123456789")

		assert.Equal(t, int32(0), resource.Metadata.MinSize)
		assert.Equal(t, int32(0), resource.Metadata.MaxSize)
		assert.Equal(t, int32(0), resource.Metadata.DesiredCapacity)
		assert.Equal(t, int32(0), resource.Metadata.CurrentSize)
		assert.Equal(t, "", resource.Metadata.InstanceIDs)
	})
}

func TestConvertASGTags(t *testing.T) {
	tags := []autoscalingtypes.TagDescription{
		{
			Key:   aws.String("Environment"),
			Value: aws.String("production"),
		},
		{
			Key:   aws.String("Team"),
			Value: aws.String("platform"),
		},
		{
			Key:   aws.String("Name"),
			Value: aws.String("web-asg-prod"),
		},
	}

	result := convertASGTags(tags)

	assert.Equal(t, "production", result.Environment)
	assert.Equal(t, "platform", result.Team)
	assert.Equal(t, "web-asg-prod", result.Name)
}

func TestExtractLaunchTemplateName(t *testing.T) {
	tests := []struct {
		name     string
		template *autoscalingtypes.LaunchTemplateSpecification
		want     string
	}{
		{
			name: "with name",
			template: &autoscalingtypes.LaunchTemplateSpecification{
				LaunchTemplateName: aws.String("api-template"),
			},
			want: "api-template",
		},
		{
			name: "with ID only",
			template: &autoscalingtypes.LaunchTemplateSpecification{
				LaunchTemplateId: aws.String("lt-abc123"),
			},
			want: "lt-abc123",
		},
		{
			name: "with both name and ID",
			template: &autoscalingtypes.LaunchTemplateSpecification{
				LaunchTemplateName: aws.String("api-template"),
				LaunchTemplateId:   aws.String("lt-abc123"),
			},
			want: "api-template",
		},
		{
			name:     "nil template",
			template: nil,
			want:     "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractLaunchTemplateName(tt.template)
			assert.Equal(t, tt.want, result)
		})
	}
}
