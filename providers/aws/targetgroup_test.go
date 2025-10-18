package aws

import (
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	elbv2types "github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2/types"
	"github.com/stretchr/testify/assert"
)

func TestBuildTargetGroupResource(t *testing.T) {
	t.Run("instance target group with ALB", func(t *testing.T) {
		tg := elbv2types.TargetGroup{
			TargetGroupArn:             aws.String("arn:aws:elasticloadbalancing:us-east-1:123456789:targetgroup/web-tg/abc123"),
			TargetGroupName:            aws.String("web-tg"),
			TargetType:                 elbv2types.TargetTypeEnumInstance,
			Protocol:                   elbv2types.ProtocolEnumHttp,
			Port:                       aws.Int32(80),
			VpcId:                      aws.String("vpc-abc123"),
			HealthCheckProtocol:        elbv2types.ProtocolEnumHttp,
			HealthCheckPort:            aws.String("traffic-port"),
			HealthCheckPath:            aws.String("/health"),
			HealthCheckIntervalSeconds: aws.Int32(30),
			HealthCheckTimeoutSeconds:  aws.Int32(5),
			HealthyThresholdCount:      aws.Int32(2),
			UnhealthyThresholdCount:    aws.Int32(3),
			LoadBalancerArns: []string{
				"arn:aws:elasticloadbalancing:us-east-1:123456789:loadbalancer/app/web-alb/xyz789",
			},
		}

		resource := buildTargetGroupResource(tg, "us-east-1", "123456789")

		assert.Equal(t, "target_group", resource.Type)
		assert.Equal(t, "arn:aws:elasticloadbalancing:us-east-1:123456789:targetgroup/web-tg/abc123", resource.ID)
		assert.Equal(t, "web-tg", resource.Name)
		assert.Equal(t, "vpc-abc123", resource.Metadata.VpcID)
		assert.Equal(t, "instance", resource.Metadata.TargetType)
		assert.Equal(t, "HTTP", resource.Metadata.Protocol)
		assert.Equal(t, int32(80), resource.Metadata.Port)
		assert.Equal(t, "HTTP", resource.Metadata.HealthCheckProtocol)
		assert.Equal(t, "traffic-port", resource.Metadata.HealthCheckPort)
		assert.Equal(t, "/health", resource.Metadata.HealthCheckPath)
		assert.Equal(t, int32(30), resource.Metadata.HealthCheckIntervalSeconds)
		assert.Equal(t, int32(5), resource.Metadata.HealthCheckTimeoutSeconds)
		assert.Equal(t, int32(2), resource.Metadata.HealthyThresholdCount)
		assert.Equal(t, int32(3), resource.Metadata.UnhealthyThresholdCount)
		assert.Equal(t, "arn:aws:elasticloadbalancing:us-east-1:123456789:loadbalancer/app/web-alb/xyz789", resource.Metadata.LoadBalancerARNs)
	})

	t.Run("IP target group with NLB", func(t *testing.T) {
		tg := elbv2types.TargetGroup{
			TargetGroupArn:  aws.String("arn:aws:elasticloadbalancing:us-east-1:123456789:targetgroup/api-tg/def456"),
			TargetGroupName: aws.String("api-tg"),
			TargetType:      elbv2types.TargetTypeEnumIp,
			Protocol:        elbv2types.ProtocolEnumTcp,
			Port:            aws.Int32(443),
			VpcId:           aws.String("vpc-xyz789"),
			LoadBalancerArns: []string{
				"arn:aws:elasticloadbalancing:us-east-1:123456789:loadbalancer/net/api-nlb/nlb123",
			},
		}

		resource := buildTargetGroupResource(tg, "us-east-1", "123456789")

		assert.Equal(t, "ip", resource.Metadata.TargetType)
		assert.Equal(t, "TCP", resource.Metadata.Protocol)
		assert.Equal(t, int32(443), resource.Metadata.Port)
	})

	t.Run("lambda target group", func(t *testing.T) {
		tg := elbv2types.TargetGroup{
			TargetGroupArn:  aws.String("arn:aws:elasticloadbalancing:us-east-1:123456789:targetgroup/lambda-tg/ghi789"),
			TargetGroupName: aws.String("lambda-tg"),
			TargetType:      elbv2types.TargetTypeEnumLambda,
		}

		resource := buildTargetGroupResource(tg, "us-east-1", "123456789")

		assert.Equal(t, "lambda", resource.Metadata.TargetType)
	})

	t.Run("target group with multiple load balancers", func(t *testing.T) {
		tg := elbv2types.TargetGroup{
			TargetGroupArn:  aws.String("arn:aws:elasticloadbalancing:us-east-1:123456789:targetgroup/shared-tg/jkl012"),
			TargetGroupName: aws.String("shared-tg"),
			TargetType:      elbv2types.TargetTypeEnumInstance,
			Protocol:        elbv2types.ProtocolEnumHttps,
			Port:            aws.Int32(443),
			LoadBalancerArns: []string{
				"arn:aws:elasticloadbalancing:us-east-1:123456789:loadbalancer/app/alb-1/aaa111",
				"arn:aws:elasticloadbalancing:us-east-1:123456789:loadbalancer/app/alb-2/bbb222",
				"arn:aws:elasticloadbalancing:us-east-1:123456789:loadbalancer/net/nlb-1/ccc333",
			},
		}

		resource := buildTargetGroupResource(tg, "us-east-1", "123456789")

		expected := "arn:aws:elasticloadbalancing:us-east-1:123456789:loadbalancer/app/alb-1/aaa111,arn:aws:elasticloadbalancing:us-east-1:123456789:loadbalancer/app/alb-2/bbb222,arn:aws:elasticloadbalancing:us-east-1:123456789:loadbalancer/net/nlb-1/ccc333"
		assert.Equal(t, expected, resource.Metadata.LoadBalancerARNs)
	})

	t.Run("orphaned target group (no load balancers)", func(t *testing.T) {
		tg := elbv2types.TargetGroup{
			TargetGroupArn:   aws.String("arn:aws:elasticloadbalancing:us-east-1:123456789:targetgroup/orphan-tg/mno345"),
			TargetGroupName:  aws.String("orphan-tg"),
			TargetType:       elbv2types.TargetTypeEnumInstance,
			LoadBalancerArns: []string{},
		}

		resource := buildTargetGroupResource(tg, "us-east-1", "123456789")

		assert.Equal(t, "", resource.Metadata.LoadBalancerARNs)
	})
}

func TestExtractLoadBalancerARNs(t *testing.T) {
	t.Run("multiple load balancers", func(t *testing.T) {
		arns := []string{
			"arn:aws:elasticloadbalancing:us-east-1:123456789:loadbalancer/app/alb-1/aaa",
			"arn:aws:elasticloadbalancing:us-east-1:123456789:loadbalancer/net/nlb-1/bbb",
		}
		result := extractLoadBalancerARNs(arns)
		expected := "arn:aws:elasticloadbalancing:us-east-1:123456789:loadbalancer/app/alb-1/aaa,arn:aws:elasticloadbalancing:us-east-1:123456789:loadbalancer/net/nlb-1/bbb"
		assert.Equal(t, expected, result)
	})

	t.Run("no load balancers", func(t *testing.T) {
		result := extractLoadBalancerARNs([]string{})
		assert.Equal(t, "", result)
	})
}

func TestConvertELBv2Tags(t *testing.T) {
	tags := []elbv2types.Tag{
		{Key: aws.String("Name"), Value: aws.String("web-target-group")},
		{Key: aws.String("Environment"), Value: aws.String("production")},
		{Key: aws.String("Team"), Value: aws.String("platform")},
	}

	result := convertELBv2Tags(tags)

	assert.Equal(t, "web-target-group", result.Name)
	assert.Equal(t, "production", result.Environment)
	assert.Equal(t, "platform", result.Team)
}
