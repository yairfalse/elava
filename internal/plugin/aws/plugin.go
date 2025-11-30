// Package aws implements the AWS scanner plugin for Elava.
package aws

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/autoscaling"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ecs"
	"github.com/aws/aws-sdk-go-v2/service/eks"
	"github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2"
	"github.com/aws/aws-sdk-go-v2/service/iam"
	"github.com/aws/aws-sdk-go-v2/service/lambda"
	"github.com/aws/aws-sdk-go-v2/service/rds"
	"github.com/aws/aws-sdk-go-v2/service/route53"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
	"github.com/rs/zerolog/log"

	"github.com/yairfalse/elava/pkg/resource"
)

// Plugin implements the AWS scanner.
type Plugin struct {
	region    string
	accountID string

	// AWS clients (interfaces for testability)
	ec2Client      EC2API
	rdsClient      RDSAPI
	elbClient      ELBAPI
	s3Client       S3API
	eksClient      EKSAPI
	asgClient      AutoScalingAPI
	lambdaClient   LambdaAPI
	dynamodbClient DynamoDBAPI
	sqsClient      SQSAPI
	iamClient      IAMAPI
	ecsClient      ECSAPI
	route53Client  Route53API
	cwLogsClient   CloudWatchLogsAPI
}

// Config holds AWS plugin configuration.
type Config struct {
	Region string
}

// New creates a new AWS plugin.
func New(ctx context.Context, cfg Config) (*Plugin, error) {
	awsCfg, err := config.LoadDefaultConfig(ctx, config.WithRegion(cfg.Region))
	if err != nil {
		return nil, fmt.Errorf("load aws config: %w", err)
	}

	ec2Client := ec2.NewFromConfig(awsCfg)

	// Get account ID
	accountID, err := getAccountID(ctx, ec2Client)
	if err != nil {
		return nil, fmt.Errorf("get account id: %w", err)
	}

	return &Plugin{
		region:         cfg.Region,
		accountID:      accountID,
		ec2Client:      ec2Client,
		rdsClient:      rds.NewFromConfig(awsCfg),
		elbClient:      elasticloadbalancingv2.NewFromConfig(awsCfg),
		s3Client:       s3.NewFromConfig(awsCfg),
		eksClient:      eks.NewFromConfig(awsCfg),
		asgClient:      autoscaling.NewFromConfig(awsCfg),
		lambdaClient:   lambda.NewFromConfig(awsCfg),
		dynamodbClient: dynamodb.NewFromConfig(awsCfg),
		sqsClient:      sqs.NewFromConfig(awsCfg),
		iamClient:      iam.NewFromConfig(awsCfg),
		ecsClient:      ecs.NewFromConfig(awsCfg),
		route53Client:  route53.NewFromConfig(awsCfg),
		cwLogsClient:   cloudwatchlogs.NewFromConfig(awsCfg),
	}, nil
}

func getAccountID(ctx context.Context, client *ec2.Client) (string, error) {
	output, err := client.DescribeAccountAttributes(ctx, &ec2.DescribeAccountAttributesInput{})
	if err != nil {
		return "", err
	}

	for _, attr := range output.AccountAttributes {
		if aws.ToString(attr.AttributeName) == "account-id" && len(attr.AttributeValues) > 0 {
			return aws.ToString(attr.AttributeValues[0].AttributeValue), nil
		}
	}

	return "unknown", nil
}

// Name returns the plugin identifier.
func (p *Plugin) Name() string {
	return "aws"
}

type scanner struct {
	name string
	fn   func(context.Context) ([]resource.Resource, error)
}

func (p *Plugin) scanners() []scanner {
	return []scanner{
		{"ec2", p.scanEC2},
		{"rds", p.scanRDS},
		{"elb", p.scanELB},
		{"s3", p.scanS3},
		{"eks", p.scanEKS},
		{"asg", p.scanASG},
		{"lambda", p.scanLambda},
		{"vpc", p.scanVPC},
		{"subnet", p.scanSubnets},
		{"security_group", p.scanSecurityGroups},
		{"dynamodb", p.scanDynamoDB},
		{"sqs", p.scanSQS},
		{"ebs", p.scanEBSVolumes},
		{"eip", p.scanElasticIPs},
		{"nat_gateway", p.scanNATGateways},
		{"iam_role", p.scanIAMRoles},
		{"ecs", p.scanECS},
		{"route53", p.scanRoute53},
		{"cloudwatch_logs", p.scanCloudWatchLogs},
	}
}

// Scan scans all AWS resources and returns them in unified format.
func (p *Plugin) Scan(ctx context.Context) ([]resource.Resource, error) {
	var (
		mu        sync.Mutex
		resources []resource.Resource
		wg        sync.WaitGroup
	)

	for _, s := range p.scanners() {
		wg.Add(1)
		go func(s scanner) {
			defer wg.Done()
			result, err := s.fn(ctx)
			if err != nil {
				log.Warn().Err(err).Str("scanner", s.name).Msg("scan failed")
				return
			}
			mu.Lock()
			resources = append(resources, result...)
			mu.Unlock()
			log.Debug().Str("scanner", s.name).Int("count", len(result)).Msg("scan complete")
		}(s)
	}

	wg.Wait()
	return resources, nil
}

// helper to create resource with common fields
func (p *Plugin) newResource(id, typ, status, name string) resource.Resource {
	return resource.Resource{
		ID:        id,
		Type:      typ,
		Provider:  "aws",
		Region:    p.region,
		Account:   p.accountID,
		Name:      name,
		Status:    status,
		Labels:    make(map[string]string),
		Attrs:     make(map[string]string),
		ScannedAt: time.Now(),
	}
}
