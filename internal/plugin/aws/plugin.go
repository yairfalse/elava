// Package aws implements the AWS scanner plugin for Elava.
package aws

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/acm"
	"github.com/aws/aws-sdk-go-v2/service/apigatewayv2"
	"github.com/aws/aws-sdk-go-v2/service/autoscaling"
	"github.com/aws/aws-sdk-go-v2/service/cloudfront"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ecs"
	"github.com/aws/aws-sdk-go-v2/service/eks"
	"github.com/aws/aws-sdk-go-v2/service/elasticache"
	"github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2"
	"github.com/aws/aws-sdk-go-v2/service/glue"
	"github.com/aws/aws-sdk-go-v2/service/iam"
	"github.com/aws/aws-sdk-go-v2/service/kafka"
	"github.com/aws/aws-sdk-go-v2/service/kinesis"
	"github.com/aws/aws-sdk-go-v2/service/lambda"
	"github.com/aws/aws-sdk-go-v2/service/opensearch"
	"github.com/aws/aws-sdk-go-v2/service/rds"
	"github.com/aws/aws-sdk-go-v2/service/redshift"
	"github.com/aws/aws-sdk-go-v2/service/route53"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/secretsmanager"
	"github.com/aws/aws-sdk-go-v2/service/sfn"
	"github.com/aws/aws-sdk-go-v2/service/sns"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
	"github.com/aws/aws-sdk-go-v2/service/sts"
	"github.com/rs/zerolog/log"
	"golang.org/x/sync/semaphore"

	"github.com/yairfalse/elava/internal/filter"
	"github.com/yairfalse/elava/pkg/resource"
)

// Plugin implements the AWS scanner.
type Plugin struct {
	region          string
	accountID       string
	maxConcurrency  int64
	filter          *filter.Filter
	scanGlobalTypes bool // true = scan global types (IAM, Route53, CloudFront, S3)

	// AWS clients - lazy initialized via sync.OnceValue for efficiency
	// Only clients that are actually used get created
	ec2Client            func() EC2API
	rdsClient            func() RDSAPI
	elbClient            func() ELBAPI
	s3Client             func() S3API
	eksClient            func() EKSAPI
	asgClient            func() AutoScalingAPI
	lambdaClient         func() LambdaAPI
	dynamodbClient       func() DynamoDBAPI
	sqsClient            func() SQSAPI
	iamClient            func() IAMAPI
	ecsClient            func() ECSAPI
	route53Client        func() Route53API
	cwLogsClient         func() CloudWatchLogsAPI
	snsClient            func() SNSAPI
	cloudfrontClient     func() CloudFrontAPI
	elasticacheClient    func() ElastiCacheAPI
	secretsmanagerClient func() SecretsManagerAPI
	acmClient            func() ACMAPI
	apigatewayClient     func() APIGatewayAPI
	kinesisClient        func() KinesisAPI
	redshiftClient       func() RedshiftAPI
	sfnClient            func() StepFunctionsAPI
	glueClient           func() GlueAPI
	opensearchClient     func() OpenSearchAPI
	mskClient            func() MSKAPI
}

// Config holds AWS plugin configuration.
type Config struct {
	Region          string
	MaxConcurrency  int
	Filter          *filter.Filter
	ScanGlobalTypes bool // true = scan global types (set for first region only)
}

// New creates a new AWS plugin.
func New(ctx context.Context, cfg Config) (*Plugin, error) {
	awsCfg, err := config.LoadDefaultConfig(ctx, config.WithRegion(cfg.Region))
	if err != nil {
		return nil, fmt.Errorf("load aws config: %w", err)
	}

	// Get account ID using STS
	accountID, err := getAccountID(ctx, awsCfg)
	if err != nil {
		return nil, fmt.Errorf("get account id: %w", err)
	}

	maxConcurrency := int64(cfg.MaxConcurrency)
	if maxConcurrency <= 0 {
		maxConcurrency = 5 // default
	}

	return &Plugin{
		region:               cfg.Region,
		accountID:            accountID,
		maxConcurrency:       maxConcurrency,
		filter:               cfg.Filter,
		scanGlobalTypes:      cfg.ScanGlobalTypes,
		ec2Client:            sync.OnceValue(func() EC2API { return ec2.NewFromConfig(awsCfg) }),
		rdsClient:            sync.OnceValue(func() RDSAPI { return rds.NewFromConfig(awsCfg) }),
		elbClient:            sync.OnceValue(func() ELBAPI { return elasticloadbalancingv2.NewFromConfig(awsCfg) }),
		s3Client:             sync.OnceValue(func() S3API { return s3.NewFromConfig(awsCfg) }),
		eksClient:            sync.OnceValue(func() EKSAPI { return eks.NewFromConfig(awsCfg) }),
		asgClient:            sync.OnceValue(func() AutoScalingAPI { return autoscaling.NewFromConfig(awsCfg) }),
		lambdaClient:         sync.OnceValue(func() LambdaAPI { return lambda.NewFromConfig(awsCfg) }),
		dynamodbClient:       sync.OnceValue(func() DynamoDBAPI { return dynamodb.NewFromConfig(awsCfg) }),
		sqsClient:            sync.OnceValue(func() SQSAPI { return sqs.NewFromConfig(awsCfg) }),
		iamClient:            sync.OnceValue(func() IAMAPI { return iam.NewFromConfig(awsCfg) }),
		ecsClient:            sync.OnceValue(func() ECSAPI { return ecs.NewFromConfig(awsCfg) }),
		route53Client:        sync.OnceValue(func() Route53API { return route53.NewFromConfig(awsCfg) }),
		cwLogsClient:         sync.OnceValue(func() CloudWatchLogsAPI { return cloudwatchlogs.NewFromConfig(awsCfg) }),
		snsClient:            sync.OnceValue(func() SNSAPI { return sns.NewFromConfig(awsCfg) }),
		cloudfrontClient:     sync.OnceValue(func() CloudFrontAPI { return cloudfront.NewFromConfig(awsCfg) }),
		elasticacheClient:    sync.OnceValue(func() ElastiCacheAPI { return elasticache.NewFromConfig(awsCfg) }),
		secretsmanagerClient: sync.OnceValue(func() SecretsManagerAPI { return secretsmanager.NewFromConfig(awsCfg) }),
		acmClient:            sync.OnceValue(func() ACMAPI { return acm.NewFromConfig(awsCfg) }),
		apigatewayClient:     sync.OnceValue(func() APIGatewayAPI { return apigatewayv2.NewFromConfig(awsCfg) }),
		kinesisClient:        sync.OnceValue(func() KinesisAPI { return kinesis.NewFromConfig(awsCfg) }),
		redshiftClient:       sync.OnceValue(func() RedshiftAPI { return redshift.NewFromConfig(awsCfg) }),
		sfnClient:            sync.OnceValue(func() StepFunctionsAPI { return sfn.NewFromConfig(awsCfg) }),
		glueClient:           sync.OnceValue(func() GlueAPI { return glue.NewFromConfig(awsCfg) }),
		opensearchClient:     sync.OnceValue(func() OpenSearchAPI { return opensearch.NewFromConfig(awsCfg) }),
		mskClient:            sync.OnceValue(func() MSKAPI { return kafka.NewFromConfig(awsCfg) }),
	}, nil
}

func getAccountID(ctx context.Context, awsCfg aws.Config) (string, error) {
	stsClient := sts.NewFromConfig(awsCfg)
	output, err := stsClient.GetCallerIdentity(ctx, &sts.GetCallerIdentityInput{})
	if err != nil {
		return "", fmt.Errorf("get caller identity: %w", err)
	}
	return aws.ToString(output.Account), nil
}

// Name returns the plugin identifier.
func (p *Plugin) Name() string {
	return "aws"
}

type scanner struct {
	name   string
	fn     func(context.Context) ([]resource.Resource, error)
	global bool // true = run only once per account (IAM, Route53, CloudFront, S3)
}

func (p *Plugin) scanners() []scanner {
	return []scanner{
		// Regional scanners
		{"ec2", p.scanEC2, false},
		{"rds", p.scanRDS, false},
		{"elb", p.scanELB, false},
		{"eks", p.scanEKS, false},
		{"asg", p.scanASG, false},
		{"lambda", p.scanLambda, false},
		{"vpc", p.scanVPC, false},
		{"subnet", p.scanSubnets, false},
		{"security_group", p.scanSecurityGroups, false},
		{"dynamodb", p.scanDynamoDB, false},
		{"sqs", p.scanSQS, false},
		{"ebs", p.scanEBSVolumes, false},
		{"eip", p.scanElasticIPs, false},
		{"nat_gateway", p.scanNATGateways, false},
		{"ecs", p.scanECS, false},
		{"cloudwatch_logs", p.scanCloudWatchLogs, false},
		{"sns", p.scanSNS, false},
		{"elasticache", p.scanElastiCache, false},
		{"secretsmanager", p.scanSecretsManager, false},
		{"acm", p.scanACM, false},
		{"apigateway", p.scanAPIGateway, false},
		{"kinesis", p.scanKinesis, false},
		{"redshift", p.scanRedshift, false},
		{"stepfunctions", p.scanStepFunctions, false},
		{"glue", p.scanGlue, false},
		{"opensearch", p.scanOpenSearch, false},
		{"msk", p.scanMSK, false},

		// Global scanners - run only once per account
		{"s3", p.scanS3, true},
		{"iam_role", p.scanIAMRoles, true},
		{"route53", p.scanRoute53, true},
		{"cloudfront", p.scanCloudFront, true},
	}
}

// Scan scans all AWS resources and returns them in unified format.
func (p *Plugin) Scan(ctx context.Context) ([]resource.Resource, error) {
	var (
		mu        sync.Mutex
		resources []resource.Resource
		wg        sync.WaitGroup
		scanErr   error
	)

	sem := semaphore.NewWeighted(p.maxConcurrency)

	for _, s := range p.scanners() {
		// Skip global scanners if not designated as the global scanner region
		if s.global && !p.scanGlobalTypes {
			log.Debug().Str("scanner", s.name).Msg("skipped global scanner (not first region)")
			continue
		}

		// Skip scanner if type is excluded
		if p.filter != nil && !p.filter.ShouldScanType(s.name) {
			log.Debug().Str("scanner", s.name).Msg("skipped by filter")
			continue
		}

		if err := sem.Acquire(ctx, 1); err != nil {
			scanErr = fmt.Errorf("acquire semaphore: %w", err)
			break
		}
		wg.Add(1)
		go func(s scanner) {
			defer sem.Release(1)
			defer wg.Done()
			result, err := s.fn(ctx)
			if err != nil {
				log.Warn().Err(err).Str("scanner", s.name).Msg("scan failed")
				return
			}

			// Filter resources by tags
			if p.filter != nil {
				originalCount := len(result)
				result = p.filter.FilterResources(result)
				if originalCount != len(result) {
					log.Debug().Str("scanner", s.name).Int("original", originalCount).Int("filtered", len(result)).Msg("resources filtered by tags")
				}
			}

			mu.Lock()
			resources = append(resources, result...)
			mu.Unlock()
			log.Debug().Str("scanner", s.name).Int("count", len(result)).Msg("scan complete")
		}(s)
	}

	wg.Wait()
	return resources, scanErr
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

// helper to create global resource (IAM, Route53, CloudFront)
func (p *Plugin) newGlobalResource(id, typ, status, name string) resource.Resource {
	return resource.Resource{
		ID:        id,
		Type:      typ,
		Provider:  "aws",
		Region:    "global",
		Account:   p.accountID,
		Name:      name,
		Status:    status,
		Labels:    make(map[string]string),
		Attrs:     make(map[string]string),
		ScannedAt: time.Now(),
	}
}
