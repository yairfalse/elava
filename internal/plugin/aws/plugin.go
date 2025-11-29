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
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/eks"
	"github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2"
	"github.com/aws/aws-sdk-go-v2/service/lambda"
	"github.com/aws/aws-sdk-go-v2/service/rds"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/rs/zerolog/log"

	"github.com/yairfalse/elava/pkg/resource"
)

// Plugin implements the AWS scanner.
type Plugin struct {
	region    string
	accountID string

	// AWS clients
	ec2Client    *ec2.Client
	rdsClient    *rds.Client
	elbClient    *elasticloadbalancingv2.Client
	s3Client     *s3.Client
	eksClient    *eks.Client
	asgClient    *autoscaling.Client
	lambdaClient *lambda.Client
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
		region:       cfg.Region,
		accountID:    accountID,
		ec2Client:    ec2Client,
		rdsClient:    rds.NewFromConfig(awsCfg),
		elbClient:    elasticloadbalancingv2.NewFromConfig(awsCfg),
		s3Client:     s3.NewFromConfig(awsCfg),
		eksClient:    eks.NewFromConfig(awsCfg),
		asgClient:    autoscaling.NewFromConfig(awsCfg),
		lambdaClient: lambda.NewFromConfig(awsCfg),
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

// Scan scans all AWS resources and returns them in unified format.
func (p *Plugin) Scan(ctx context.Context) ([]resource.Resource, error) {
	var (
		mu        sync.Mutex
		resources []resource.Resource
		wg        sync.WaitGroup
	)

	// Define scanners
	scanners := []struct {
		name string
		fn   func(context.Context) ([]resource.Resource, error)
	}{
		{"ec2", p.scanEC2},
		{"rds", p.scanRDS},
		{"elb", p.scanELB},
		{"s3", p.scanS3},
		{"eks", p.scanEKS},
		{"asg", p.scanASG},
		{"lambda", p.scanLambda},
	}

	// Run scanners concurrently
	for _, scanner := range scanners {
		wg.Add(1)
		go func(name string, fn func(context.Context) ([]resource.Resource, error)) {
			defer wg.Done()

			result, err := fn(ctx)
			if err != nil {
				log.Warn().Err(err).Str("scanner", name).Msg("scan failed")
				return
			}

			mu.Lock()
			resources = append(resources, result...)
			mu.Unlock()

			log.Debug().Str("scanner", name).Int("count", len(result)).Msg("scan complete")
		}(scanner.name, scanner.fn)
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
