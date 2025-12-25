package aws

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/acm"
	acmtypes "github.com/aws/aws-sdk-go-v2/service/acm/types"
	"github.com/aws/aws-sdk-go-v2/service/apigatewayv2"
	apigwtypes "github.com/aws/aws-sdk-go-v2/service/apigatewayv2/types"
	"github.com/aws/aws-sdk-go-v2/service/autoscaling"
	asgtypes "github.com/aws/aws-sdk-go-v2/service/autoscaling/types"
	"github.com/aws/aws-sdk-go-v2/service/cloudfront"
	cftypes "github.com/aws/aws-sdk-go-v2/service/cloudfront/types"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs"
	cwltypes "github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs/types"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	ddbtypes "github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/aws/aws-sdk-go-v2/service/ecs"
	ecstypes "github.com/aws/aws-sdk-go-v2/service/ecs/types"
	"github.com/aws/aws-sdk-go-v2/service/eks"
	ekstypes "github.com/aws/aws-sdk-go-v2/service/eks/types"
	"github.com/aws/aws-sdk-go-v2/service/elasticache"
	ectypes "github.com/aws/aws-sdk-go-v2/service/elasticache/types"
	"github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2"
	elbtypes "github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2/types"
	"github.com/aws/aws-sdk-go-v2/service/glue"
	gluetypes "github.com/aws/aws-sdk-go-v2/service/glue/types"
	"github.com/aws/aws-sdk-go-v2/service/iam"
	iamtypes "github.com/aws/aws-sdk-go-v2/service/iam/types"
	"github.com/aws/aws-sdk-go-v2/service/kinesis"
	kinesistypes "github.com/aws/aws-sdk-go-v2/service/kinesis/types"
	"github.com/aws/aws-sdk-go-v2/service/lambda"
	lambdatypes "github.com/aws/aws-sdk-go-v2/service/lambda/types"
	"github.com/aws/aws-sdk-go-v2/service/rds"
	rdstypes "github.com/aws/aws-sdk-go-v2/service/rds/types"
	"github.com/aws/aws-sdk-go-v2/service/redshift"
	redshifttypes "github.com/aws/aws-sdk-go-v2/service/redshift/types"
	"github.com/aws/aws-sdk-go-v2/service/route53"
	r53types "github.com/aws/aws-sdk-go-v2/service/route53/types"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	s3types "github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/aws/aws-sdk-go-v2/service/secretsmanager"
	smtypes "github.com/aws/aws-sdk-go-v2/service/secretsmanager/types"
	"github.com/aws/aws-sdk-go-v2/service/sfn"
	sfntypes "github.com/aws/aws-sdk-go-v2/service/sfn/types"
	"github.com/aws/aws-sdk-go-v2/service/sns"
	snstypes "github.com/aws/aws-sdk-go-v2/service/sns/types"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ══════════════════════════════════════════════════════════════════════════════
// RDS Tests
// ══════════════════════════════════════════════════════════════════════════════

type mockRDSClient struct {
	DescribeDBInstancesFunc func(ctx context.Context, params *rds.DescribeDBInstancesInput, optFns ...func(*rds.Options)) (*rds.DescribeDBInstancesOutput, error)
}

func (m *mockRDSClient) DescribeDBInstances(ctx context.Context, params *rds.DescribeDBInstancesInput, optFns ...func(*rds.Options)) (*rds.DescribeDBInstancesOutput, error) {
	return m.DescribeDBInstancesFunc(ctx, params, optFns...)
}

func TestScanRDS(t *testing.T) {
	mock := &mockRDSClient{
		DescribeDBInstancesFunc: func(_ context.Context, _ *rds.DescribeDBInstancesInput, _ ...func(*rds.Options)) (*rds.DescribeDBInstancesOutput, error) {
			return &rds.DescribeDBInstancesOutput{
				DBInstances: []rdstypes.DBInstance{
					{
						DBInstanceIdentifier: aws.String("my-db"),
						DBInstanceStatus:     aws.String("available"),
						Engine:               aws.String("postgres"),
						EngineVersion:        aws.String("14.5"),
						DBInstanceClass:      aws.String("db.t3.micro"),
						AllocatedStorage:     aws.Int32(20),
						MultiAZ:              aws.Bool(false),
						Endpoint:             &rdstypes.Endpoint{Address: aws.String("my-db.xyz.rds.amazonaws.com"), Port: aws.Int32(5432)},
					},
				},
			}, nil
		},
	}

	p := &Plugin{region: "us-east-1", accountID: "123456789012", rdsClient: func() RDSAPI { return mock }}
	resources, err := p.scanRDS(context.Background())

	require.NoError(t, err)
	require.Len(t, resources, 1)

	r := resources[0]
	assert.Equal(t, "my-db", r.ID)
	assert.Equal(t, "rds", r.Type)
	assert.Equal(t, "available", r.Status)
	assert.Equal(t, "postgres", r.Attrs["engine"])
	assert.Equal(t, "db.t3.micro", r.Attrs["instance_class"])
}

func TestScanRDS_Error(t *testing.T) {
	mock := &mockRDSClient{
		DescribeDBInstancesFunc: func(_ context.Context, _ *rds.DescribeDBInstancesInput, _ ...func(*rds.Options)) (*rds.DescribeDBInstancesOutput, error) {
			return nil, errors.New("access denied")
		},
	}

	p := &Plugin{region: "us-east-1", accountID: "123456789012", rdsClient: func() RDSAPI { return mock }}
	_, err := p.scanRDS(context.Background())

	require.Error(t, err)
	assert.Contains(t, err.Error(), "access denied")
}

// ══════════════════════════════════════════════════════════════════════════════
// S3 Tests
// ══════════════════════════════════════════════════════════════════════════════

type mockS3Client struct {
	ListBucketsFunc       func(ctx context.Context, params *s3.ListBucketsInput, optFns ...func(*s3.Options)) (*s3.ListBucketsOutput, error)
	GetBucketLocationFunc func(ctx context.Context, params *s3.GetBucketLocationInput, optFns ...func(*s3.Options)) (*s3.GetBucketLocationOutput, error)
}

func (m *mockS3Client) ListBuckets(ctx context.Context, params *s3.ListBucketsInput, optFns ...func(*s3.Options)) (*s3.ListBucketsOutput, error) {
	return m.ListBucketsFunc(ctx, params, optFns...)
}

func (m *mockS3Client) GetBucketLocation(ctx context.Context, params *s3.GetBucketLocationInput, optFns ...func(*s3.Options)) (*s3.GetBucketLocationOutput, error) {
	if m.GetBucketLocationFunc != nil {
		return m.GetBucketLocationFunc(ctx, params, optFns...)
	}
	// Default: return us-east-1 (empty string means us-east-1 in AWS)
	return &s3.GetBucketLocationOutput{}, nil
}

func TestScanS3(t *testing.T) {
	mock := &mockS3Client{
		ListBucketsFunc: func(_ context.Context, _ *s3.ListBucketsInput, _ ...func(*s3.Options)) (*s3.ListBucketsOutput, error) {
			return &s3.ListBucketsOutput{
				Buckets: []s3types.Bucket{
					{Name: aws.String("my-bucket-1")},
					{Name: aws.String("my-bucket-2")},
				},
			}, nil
		},
	}

	p := &Plugin{region: "us-east-1", accountID: "123456789012", s3Client: func() S3API { return mock }}
	resources, err := p.scanS3(context.Background())

	require.NoError(t, err)
	require.Len(t, resources, 2)

	assert.Equal(t, "my-bucket-1", resources[0].ID)
	assert.Equal(t, "s3", resources[0].Type)
	assert.Equal(t, "active", resources[0].Status)
}

// ══════════════════════════════════════════════════════════════════════════════
// EKS Tests
// ══════════════════════════════════════════════════════════════════════════════

type mockEKSClient struct {
	ListClustersFunc    func(ctx context.Context, params *eks.ListClustersInput, optFns ...func(*eks.Options)) (*eks.ListClustersOutput, error)
	DescribeClusterFunc func(ctx context.Context, params *eks.DescribeClusterInput, optFns ...func(*eks.Options)) (*eks.DescribeClusterOutput, error)
}

func (m *mockEKSClient) ListClusters(ctx context.Context, params *eks.ListClustersInput, optFns ...func(*eks.Options)) (*eks.ListClustersOutput, error) {
	return m.ListClustersFunc(ctx, params, optFns...)
}

func (m *mockEKSClient) DescribeCluster(ctx context.Context, params *eks.DescribeClusterInput, optFns ...func(*eks.Options)) (*eks.DescribeClusterOutput, error) {
	return m.DescribeClusterFunc(ctx, params, optFns...)
}

func TestScanEKS(t *testing.T) {
	mock := &mockEKSClient{
		ListClustersFunc: func(_ context.Context, _ *eks.ListClustersInput, _ ...func(*eks.Options)) (*eks.ListClustersOutput, error) {
			return &eks.ListClustersOutput{Clusters: []string{"prod-cluster"}}, nil
		},
		DescribeClusterFunc: func(_ context.Context, params *eks.DescribeClusterInput, _ ...func(*eks.Options)) (*eks.DescribeClusterOutput, error) {
			return &eks.DescribeClusterOutput{
				Cluster: &ekstypes.Cluster{
					Name:     params.Name,
					Arn:      aws.String("arn:aws:eks:us-east-1:123456789012:cluster/prod-cluster"),
					Status:   ekstypes.ClusterStatusActive,
					Version:  aws.String("1.28"),
					Endpoint: aws.String("https://xyz.eks.amazonaws.com"),
					Tags:     map[string]string{"env": "prod"},
				},
			}, nil
		},
	}

	p := &Plugin{region: "us-east-1", accountID: "123456789012", eksClient: func() EKSAPI { return mock }}
	resources, err := p.scanEKS(context.Background())

	require.NoError(t, err)
	require.Len(t, resources, 1)

	r := resources[0]
	assert.Contains(t, r.ID, "prod-cluster")
	assert.Equal(t, "eks", r.Type)
	assert.Equal(t, "ACTIVE", r.Status)
	assert.Equal(t, "1.28", r.Attrs["version"])
	assert.Equal(t, "prod", r.Labels["env"])
}

// ══════════════════════════════════════════════════════════════════════════════
// VPC Tests
// ══════════════════════════════════════════════════════════════════════════════

func TestScanVPC(t *testing.T) {
	mock := &mockEC2Client{
		DescribeInstancesFunc: func(_ context.Context, _ *ec2.DescribeInstancesInput, _ ...func(*ec2.Options)) (*ec2.DescribeInstancesOutput, error) {
			return &ec2.DescribeInstancesOutput{}, nil
		},
	}
	mock.describeVpcsFunc = func(_ context.Context, _ *ec2.DescribeVpcsInput, _ ...func(*ec2.Options)) (*ec2.DescribeVpcsOutput, error) {
		return &ec2.DescribeVpcsOutput{
			Vpcs: []ec2types.Vpc{
				{
					VpcId:     aws.String("vpc-123"),
					State:     ec2types.VpcStateAvailable,
					CidrBlock: aws.String("10.0.0.0/16"),
					IsDefault: aws.Bool(false),
					Tags:      []ec2types.Tag{{Key: aws.String("Name"), Value: aws.String("prod-vpc")}},
				},
			},
		}, nil
	}

	p := &Plugin{region: "us-east-1", accountID: "123456789012", ec2Client: func() EC2API { return mock }}
	resources, err := p.scanVPC(context.Background())

	require.NoError(t, err)
	require.Len(t, resources, 1)

	r := resources[0]
	assert.Equal(t, "vpc-123", r.ID)
	assert.Equal(t, "vpc", r.Type)
	assert.Equal(t, "available", r.Status)
	assert.Equal(t, "prod-vpc", r.Name)
	assert.Equal(t, "10.0.0.0/16", r.Attrs["cidr"])
}

// ══════════════════════════════════════════════════════════════════════════════
// DynamoDB Tests
// ══════════════════════════════════════════════════════════════════════════════

type mockDynamoDBClient struct {
	ListTablesFunc    func(ctx context.Context, params *dynamodb.ListTablesInput, optFns ...func(*dynamodb.Options)) (*dynamodb.ListTablesOutput, error)
	DescribeTableFunc func(ctx context.Context, params *dynamodb.DescribeTableInput, optFns ...func(*dynamodb.Options)) (*dynamodb.DescribeTableOutput, error)
}

func (m *mockDynamoDBClient) ListTables(ctx context.Context, params *dynamodb.ListTablesInput, optFns ...func(*dynamodb.Options)) (*dynamodb.ListTablesOutput, error) {
	return m.ListTablesFunc(ctx, params, optFns...)
}

func (m *mockDynamoDBClient) DescribeTable(ctx context.Context, params *dynamodb.DescribeTableInput, optFns ...func(*dynamodb.Options)) (*dynamodb.DescribeTableOutput, error) {
	return m.DescribeTableFunc(ctx, params, optFns...)
}

func TestScanDynamoDB(t *testing.T) {
	mock := &mockDynamoDBClient{
		ListTablesFunc: func(_ context.Context, _ *dynamodb.ListTablesInput, _ ...func(*dynamodb.Options)) (*dynamodb.ListTablesOutput, error) {
			return &dynamodb.ListTablesOutput{TableNames: []string{"users", "orders"}}, nil
		},
		DescribeTableFunc: func(_ context.Context, params *dynamodb.DescribeTableInput, _ ...func(*dynamodb.Options)) (*dynamodb.DescribeTableOutput, error) {
			return &dynamodb.DescribeTableOutput{
				Table: &ddbtypes.TableDescription{
					TableName:      params.TableName,
					TableArn:       aws.String("arn:aws:dynamodb:us-east-1:123456789012:table/" + *params.TableName),
					TableStatus:    ddbtypes.TableStatusActive,
					ItemCount:      aws.Int64(1000),
					TableSizeBytes: aws.Int64(50000),
				},
			}, nil
		},
	}

	p := &Plugin{region: "us-east-1", accountID: "123456789012", dynamodbClient: func() DynamoDBAPI { return mock }}
	resources, err := p.scanDynamoDB(context.Background())

	require.NoError(t, err)
	require.Len(t, resources, 2)

	assert.Equal(t, "dynamodb", resources[0].Type)
	assert.Equal(t, "ACTIVE", resources[0].Status)
	assert.Equal(t, "1000", resources[0].Attrs["items"])
}

// ══════════════════════════════════════════════════════════════════════════════
// Helper Tests
// ══════════════════════════════════════════════════════════════════════════════

func TestExtractNameTag(t *testing.T) {
	tests := []struct {
		name string
		tags []ec2types.Tag
		want string
	}{
		{"found", []ec2types.Tag{{Key: aws.String("Name"), Value: aws.String("my-instance")}}, "my-instance"},
		{"not found", []ec2types.Tag{{Key: aws.String("env"), Value: aws.String("prod")}}, ""},
		{"empty", []ec2types.Tag{}, ""},
		{"multiple tags", []ec2types.Tag{{Key: aws.String("env"), Value: aws.String("prod")}, {Key: aws.String("Name"), Value: aws.String("web-1")}}, "web-1"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractNameTag(tt.tags)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestExtractQueueName(t *testing.T) {
	tests := []struct {
		url  string
		want string
	}{
		{"https://sqs.us-east-1.amazonaws.com/123456789012/my-queue", "my-queue"},
		{"https://sqs.us-east-1.amazonaws.com/123456789012/orders-dlq", "orders-dlq"},
		{"no-slash", "no-slash"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			got := extractQueueName(tt.url)
			assert.Equal(t, tt.want, got)
		})
	}
}

// ══════════════════════════════════════════════════════════════════════════════
// Lambda Tests
// ══════════════════════════════════════════════════════════════════════════════

type mockLambdaClient struct {
	ListFunctionsFunc func(ctx context.Context, params *lambda.ListFunctionsInput, optFns ...func(*lambda.Options)) (*lambda.ListFunctionsOutput, error)
}

func (m *mockLambdaClient) ListFunctions(ctx context.Context, params *lambda.ListFunctionsInput, optFns ...func(*lambda.Options)) (*lambda.ListFunctionsOutput, error) {
	return m.ListFunctionsFunc(ctx, params, optFns...)
}

func TestScanLambda(t *testing.T) {
	mock := &mockLambdaClient{
		ListFunctionsFunc: func(_ context.Context, _ *lambda.ListFunctionsInput, _ ...func(*lambda.Options)) (*lambda.ListFunctionsOutput, error) {
			return &lambda.ListFunctionsOutput{
				Functions: []lambdatypes.FunctionConfiguration{
					{
						FunctionName: aws.String("my-function"),
						FunctionArn:  aws.String("arn:aws:lambda:us-east-1:123456789012:function:my-function"),
						Runtime:      lambdatypes.RuntimePython39,
						MemorySize:   aws.Int32(128),
						Timeout:      aws.Int32(30),
						State:        lambdatypes.StateActive,
					},
				},
			}, nil
		},
	}

	p := &Plugin{region: "us-east-1", accountID: "123456789012", lambdaClient: func() LambdaAPI { return mock }}
	resources, err := p.scanLambda(context.Background())

	require.NoError(t, err)
	require.Len(t, resources, 1)

	r := resources[0]
	assert.Contains(t, r.ID, "my-function")
	assert.Equal(t, "lambda", r.Type)
	assert.Equal(t, "Active", r.Status)
	assert.Equal(t, "python3.9", r.Attrs["runtime"])
	assert.Equal(t, "128", r.Attrs["memory_mb"])
}

// ══════════════════════════════════════════════════════════════════════════════
// ASG Tests
// ══════════════════════════════════════════════════════════════════════════════

type mockASGClient struct {
	DescribeAutoScalingGroupsFunc func(ctx context.Context, params *autoscaling.DescribeAutoScalingGroupsInput, optFns ...func(*autoscaling.Options)) (*autoscaling.DescribeAutoScalingGroupsOutput, error)
}

func (m *mockASGClient) DescribeAutoScalingGroups(ctx context.Context, params *autoscaling.DescribeAutoScalingGroupsInput, optFns ...func(*autoscaling.Options)) (*autoscaling.DescribeAutoScalingGroupsOutput, error) {
	return m.DescribeAutoScalingGroupsFunc(ctx, params, optFns...)
}

func TestScanASG(t *testing.T) {
	mock := &mockASGClient{
		DescribeAutoScalingGroupsFunc: func(_ context.Context, _ *autoscaling.DescribeAutoScalingGroupsInput, _ ...func(*autoscaling.Options)) (*autoscaling.DescribeAutoScalingGroupsOutput, error) {
			return &autoscaling.DescribeAutoScalingGroupsOutput{
				AutoScalingGroups: []asgtypes.AutoScalingGroup{
					{
						AutoScalingGroupName: aws.String("web-asg"),
						AutoScalingGroupARN:  aws.String("arn:aws:autoscaling:us-east-1:123456789012:autoScalingGroup:abc"),
						MinSize:              aws.Int32(1),
						MaxSize:              aws.Int32(5),
						DesiredCapacity:      aws.Int32(2),
						Instances:            []asgtypes.Instance{{}, {}},
					},
				},
			}, nil
		},
	}

	p := &Plugin{region: "us-east-1", accountID: "123456789012", asgClient: func() AutoScalingAPI { return mock }}
	resources, err := p.scanASG(context.Background())

	require.NoError(t, err)
	require.Len(t, resources, 1)

	r := resources[0]
	assert.Equal(t, "asg", r.Type)
	assert.Equal(t, "active", r.Status)
	assert.Equal(t, "web-asg", r.Name)
	assert.Equal(t, "2", r.Attrs["desired"])
	assert.Equal(t, "2", r.Attrs["instances"])
}

func TestScanASG_Stopped(t *testing.T) {
	mock := &mockASGClient{
		DescribeAutoScalingGroupsFunc: func(_ context.Context, _ *autoscaling.DescribeAutoScalingGroupsInput, _ ...func(*autoscaling.Options)) (*autoscaling.DescribeAutoScalingGroupsOutput, error) {
			return &autoscaling.DescribeAutoScalingGroupsOutput{
				AutoScalingGroups: []asgtypes.AutoScalingGroup{
					{
						AutoScalingGroupName: aws.String("stopped-asg"),
						AutoScalingGroupARN:  aws.String("arn:aws:autoscaling:us-east-1:123456789012:autoScalingGroup:xyz"),
						MinSize:              aws.Int32(0),
						MaxSize:              aws.Int32(5),
						DesiredCapacity:      aws.Int32(0),
						Instances:            []asgtypes.Instance{},
					},
				},
			}, nil
		},
	}

	p := &Plugin{region: "us-east-1", accountID: "123456789012", asgClient: func() AutoScalingAPI { return mock }}
	resources, err := p.scanASG(context.Background())

	require.NoError(t, err)
	require.Len(t, resources, 1)
	assert.Equal(t, "stopped", resources[0].Status)
}

// ══════════════════════════════════════════════════════════════════════════════
// SQS Tests
// ══════════════════════════════════════════════════════════════════════════════

type mockSQSClient struct {
	ListQueuesFunc func(ctx context.Context, params *sqs.ListQueuesInput, optFns ...func(*sqs.Options)) (*sqs.ListQueuesOutput, error)
}

func (m *mockSQSClient) ListQueues(ctx context.Context, params *sqs.ListQueuesInput, optFns ...func(*sqs.Options)) (*sqs.ListQueuesOutput, error) {
	return m.ListQueuesFunc(ctx, params, optFns...)
}

func TestScanSQS(t *testing.T) {
	mock := &mockSQSClient{
		ListQueuesFunc: func(_ context.Context, _ *sqs.ListQueuesInput, _ ...func(*sqs.Options)) (*sqs.ListQueuesOutput, error) {
			return &sqs.ListQueuesOutput{
				QueueUrls: []string{
					"https://sqs.us-east-1.amazonaws.com/123456789012/orders",
					"https://sqs.us-east-1.amazonaws.com/123456789012/orders-dlq",
				},
			}, nil
		},
	}

	p := &Plugin{region: "us-east-1", accountID: "123456789012", sqsClient: func() SQSAPI { return mock }}
	resources, err := p.scanSQS(context.Background())

	require.NoError(t, err)
	require.Len(t, resources, 2)

	assert.Equal(t, "orders", resources[0].Name)
	assert.Equal(t, "sqs", resources[0].Type)
	assert.Equal(t, "active", resources[0].Status)
	assert.Equal(t, "orders-dlq", resources[1].Name)
}

// ══════════════════════════════════════════════════════════════════════════════
// ELB Tests
// ══════════════════════════════════════════════════════════════════════════════

type mockELBClient struct {
	DescribeLoadBalancersFunc func(ctx context.Context, params *elasticloadbalancingv2.DescribeLoadBalancersInput, optFns ...func(*elasticloadbalancingv2.Options)) (*elasticloadbalancingv2.DescribeLoadBalancersOutput, error)
}

func (m *mockELBClient) DescribeLoadBalancers(ctx context.Context, params *elasticloadbalancingv2.DescribeLoadBalancersInput, optFns ...func(*elasticloadbalancingv2.Options)) (*elasticloadbalancingv2.DescribeLoadBalancersOutput, error) {
	return m.DescribeLoadBalancersFunc(ctx, params, optFns...)
}

func TestScanELB(t *testing.T) {
	mock := &mockELBClient{
		DescribeLoadBalancersFunc: func(_ context.Context, _ *elasticloadbalancingv2.DescribeLoadBalancersInput, _ ...func(*elasticloadbalancingv2.Options)) (*elasticloadbalancingv2.DescribeLoadBalancersOutput, error) {
			return &elasticloadbalancingv2.DescribeLoadBalancersOutput{
				LoadBalancers: []elbtypes.LoadBalancer{
					{
						LoadBalancerArn:  aws.String("arn:aws:elasticloadbalancing:us-east-1:123456789012:loadbalancer/app/my-alb/abc"),
						LoadBalancerName: aws.String("my-alb"),
						Type:             elbtypes.LoadBalancerTypeEnumApplication,
						Scheme:           elbtypes.LoadBalancerSchemeEnumInternetFacing,
						VpcId:            aws.String("vpc-123"),
						DNSName:          aws.String("my-alb-123.us-east-1.elb.amazonaws.com"),
						State:            &elbtypes.LoadBalancerState{Code: elbtypes.LoadBalancerStateEnumActive},
					},
				},
			}, nil
		},
	}

	p := &Plugin{region: "us-east-1", accountID: "123456789012", elbClient: func() ELBAPI { return mock }}
	resources, err := p.scanELB(context.Background())

	require.NoError(t, err)
	require.Len(t, resources, 1)

	r := resources[0]
	assert.Equal(t, "elb", r.Type)
	assert.Equal(t, "active", r.Status)
	assert.Equal(t, "my-alb", r.Name)
	assert.Equal(t, "application", r.Attrs["type"])
	assert.Equal(t, "internet-facing", r.Attrs["scheme"])
}

// ══════════════════════════════════════════════════════════════════════════════
// IAM Role Tests
// ══════════════════════════════════════════════════════════════════════════════

type mockIAMClient struct {
	ListRolesFunc func(ctx context.Context, params *iam.ListRolesInput, optFns ...func(*iam.Options)) (*iam.ListRolesOutput, error)
}

func (m *mockIAMClient) ListRoles(ctx context.Context, params *iam.ListRolesInput, optFns ...func(*iam.Options)) (*iam.ListRolesOutput, error) {
	return m.ListRolesFunc(ctx, params, optFns...)
}

func TestScanIAMRoles(t *testing.T) {
	mock := &mockIAMClient{
		ListRolesFunc: func(_ context.Context, _ *iam.ListRolesInput, _ ...func(*iam.Options)) (*iam.ListRolesOutput, error) {
			return &iam.ListRolesOutput{
				Roles: []iamtypes.Role{
					{
						RoleName:    aws.String("MyRole"),
						Arn:         aws.String("arn:aws:iam::123456789012:role/MyRole"),
						Path:        aws.String("/"),
						Description: aws.String("My test role"),
					},
				},
				IsTruncated: false,
			}, nil
		},
	}

	p := &Plugin{region: "us-east-1", accountID: "123456789012", iamClient: func() IAMAPI { return mock }}
	resources, err := p.scanIAMRoles(context.Background())

	require.NoError(t, err)
	require.Len(t, resources, 1)

	r := resources[0]
	assert.Equal(t, "iam_role", r.Type)
	assert.Equal(t, "active", r.Status)
	assert.Equal(t, "MyRole", r.Name)
	assert.Equal(t, "/", r.Attrs["path"])
	assert.Equal(t, "My test role", r.Attrs["description"])
}

// ══════════════════════════════════════════════════════════════════════════════
// ECS Tests
// ══════════════════════════════════════════════════════════════════════════════

type mockECSClient struct {
	ListClustersFunc     func(ctx context.Context, params *ecs.ListClustersInput, optFns ...func(*ecs.Options)) (*ecs.ListClustersOutput, error)
	DescribeClustersFunc func(ctx context.Context, params *ecs.DescribeClustersInput, optFns ...func(*ecs.Options)) (*ecs.DescribeClustersOutput, error)
}

func (m *mockECSClient) ListClusters(ctx context.Context, params *ecs.ListClustersInput, optFns ...func(*ecs.Options)) (*ecs.ListClustersOutput, error) {
	return m.ListClustersFunc(ctx, params, optFns...)
}

func (m *mockECSClient) DescribeClusters(ctx context.Context, params *ecs.DescribeClustersInput, optFns ...func(*ecs.Options)) (*ecs.DescribeClustersOutput, error) {
	return m.DescribeClustersFunc(ctx, params, optFns...)
}

func TestScanECS(t *testing.T) {
	mock := &mockECSClient{
		ListClustersFunc: func(_ context.Context, _ *ecs.ListClustersInput, _ ...func(*ecs.Options)) (*ecs.ListClustersOutput, error) {
			return &ecs.ListClustersOutput{
				ClusterArns: []string{"arn:aws:ecs:us-east-1:123456789012:cluster/prod"},
			}, nil
		},
		DescribeClustersFunc: func(_ context.Context, _ *ecs.DescribeClustersInput, _ ...func(*ecs.Options)) (*ecs.DescribeClustersOutput, error) {
			return &ecs.DescribeClustersOutput{
				Clusters: []ecstypes.Cluster{
					{
						ClusterArn:          aws.String("arn:aws:ecs:us-east-1:123456789012:cluster/prod"),
						ClusterName:         aws.String("prod"),
						Status:              aws.String("ACTIVE"),
						ActiveServicesCount: 3,
						RunningTasksCount:   10,
						PendingTasksCount:   0,
					},
				},
			}, nil
		},
	}

	p := &Plugin{region: "us-east-1", accountID: "123456789012", ecsClient: func() ECSAPI { return mock }}
	resources, err := p.scanECS(context.Background())

	require.NoError(t, err)
	require.Len(t, resources, 1)

	r := resources[0]
	assert.Equal(t, "ecs", r.Type)
	assert.Equal(t, "ACTIVE", r.Status)
	assert.Equal(t, "prod", r.Name)
	assert.Equal(t, "3", r.Attrs["services"])
	assert.Equal(t, "10", r.Attrs["tasks_running"])
}

func TestScanECS_Empty(t *testing.T) {
	mock := &mockECSClient{
		ListClustersFunc: func(_ context.Context, _ *ecs.ListClustersInput, _ ...func(*ecs.Options)) (*ecs.ListClustersOutput, error) {
			return &ecs.ListClustersOutput{ClusterArns: []string{}}, nil
		},
	}

	p := &Plugin{region: "us-east-1", accountID: "123456789012", ecsClient: func() ECSAPI { return mock }}
	resources, err := p.scanECS(context.Background())

	require.NoError(t, err)
	assert.Empty(t, resources)
}

func TestScanECS_Batching(t *testing.T) {
	// Generate 150 cluster ARNs to test batching (limit is 100 per call)
	var clusterArns []string
	for i := 0; i < 150; i++ {
		clusterArns = append(clusterArns, fmt.Sprintf("arn:aws:ecs:us-east-1:123456789012:cluster/cluster-%d", i))
	}

	describeCalls := 0
	mock := &mockECSClient{
		ListClustersFunc: func(_ context.Context, _ *ecs.ListClustersInput, _ ...func(*ecs.Options)) (*ecs.ListClustersOutput, error) {
			return &ecs.ListClustersOutput{ClusterArns: clusterArns}, nil
		},
		DescribeClustersFunc: func(_ context.Context, params *ecs.DescribeClustersInput, _ ...func(*ecs.Options)) (*ecs.DescribeClustersOutput, error) {
			describeCalls++
			// Verify batch size is <= 100
			require.LessOrEqual(t, len(params.Clusters), 100)

			var clusters []ecstypes.Cluster
			for _, arn := range params.Clusters {
				clusters = append(clusters, ecstypes.Cluster{
					ClusterArn:  aws.String(arn),
					ClusterName: aws.String("cluster"),
					Status:      aws.String("ACTIVE"),
				})
			}
			return &ecs.DescribeClustersOutput{Clusters: clusters}, nil
		},
	}

	p := &Plugin{region: "us-east-1", accountID: "123456789012", ecsClient: func() ECSAPI { return mock }}
	resources, err := p.scanECS(context.Background())

	require.NoError(t, err)
	assert.Len(t, resources, 150)
	assert.Equal(t, 2, describeCalls, "should make 2 DescribeClusters calls for 150 clusters")
}

// ══════════════════════════════════════════════════════════════════════════════
// Route53 Tests
// ══════════════════════════════════════════════════════════════════════════════

type mockRoute53Client struct {
	ListHostedZonesFunc func(ctx context.Context, params *route53.ListHostedZonesInput, optFns ...func(*route53.Options)) (*route53.ListHostedZonesOutput, error)
}

func (m *mockRoute53Client) ListHostedZones(ctx context.Context, params *route53.ListHostedZonesInput, optFns ...func(*route53.Options)) (*route53.ListHostedZonesOutput, error) {
	return m.ListHostedZonesFunc(ctx, params, optFns...)
}

func TestScanRoute53(t *testing.T) {
	mock := &mockRoute53Client{
		ListHostedZonesFunc: func(_ context.Context, _ *route53.ListHostedZonesInput, _ ...func(*route53.Options)) (*route53.ListHostedZonesOutput, error) {
			return &route53.ListHostedZonesOutput{
				HostedZones: []r53types.HostedZone{
					{
						Id:                     aws.String("/hostedzone/Z123"),
						Name:                   aws.String("example.com."),
						Config:                 &r53types.HostedZoneConfig{PrivateZone: false},
						ResourceRecordSetCount: aws.Int64(10),
					},
					{
						Id:                     aws.String("/hostedzone/Z456"),
						Name:                   aws.String("internal.com."),
						Config:                 &r53types.HostedZoneConfig{PrivateZone: true},
						ResourceRecordSetCount: aws.Int64(5),
					},
				},
				IsTruncated: false,
			}, nil
		},
	}

	p := &Plugin{region: "us-east-1", accountID: "123456789012", route53Client: func() Route53API { return mock }}
	resources, err := p.scanRoute53(context.Background())

	require.NoError(t, err)
	require.Len(t, resources, 2)

	assert.Equal(t, "route53", resources[0].Type)
	assert.Equal(t, "example.com.", resources[0].Name)
	assert.Equal(t, "public", resources[0].Attrs["type"])
	assert.Equal(t, "10", resources[0].Attrs["records"])

	assert.Equal(t, "private", resources[1].Attrs["type"])
}

// ══════════════════════════════════════════════════════════════════════════════
// CloudWatch Logs Tests
// ══════════════════════════════════════════════════════════════════════════════

type mockCWLogsClient struct {
	DescribeLogGroupsFunc func(ctx context.Context, params *cloudwatchlogs.DescribeLogGroupsInput, optFns ...func(*cloudwatchlogs.Options)) (*cloudwatchlogs.DescribeLogGroupsOutput, error)
}

func (m *mockCWLogsClient) DescribeLogGroups(ctx context.Context, params *cloudwatchlogs.DescribeLogGroupsInput, optFns ...func(*cloudwatchlogs.Options)) (*cloudwatchlogs.DescribeLogGroupsOutput, error) {
	return m.DescribeLogGroupsFunc(ctx, params, optFns...)
}

func TestScanCloudWatchLogs(t *testing.T) {
	mock := &mockCWLogsClient{
		DescribeLogGroupsFunc: func(_ context.Context, _ *cloudwatchlogs.DescribeLogGroupsInput, _ ...func(*cloudwatchlogs.Options)) (*cloudwatchlogs.DescribeLogGroupsOutput, error) {
			return &cloudwatchlogs.DescribeLogGroupsOutput{
				LogGroups: []cwltypes.LogGroup{
					{
						LogGroupName:    aws.String("/aws/lambda/my-function"),
						Arn:             aws.String("arn:aws:logs:us-east-1:123456789012:log-group:/aws/lambda/my-function"),
						StoredBytes:     aws.Int64(1024000),
						RetentionInDays: aws.Int32(14),
					},
				},
			}, nil
		},
	}

	p := &Plugin{region: "us-east-1", accountID: "123456789012", cwLogsClient: func() CloudWatchLogsAPI { return mock }}
	resources, err := p.scanCloudWatchLogs(context.Background())

	require.NoError(t, err)
	require.Len(t, resources, 1)

	r := resources[0]
	assert.Equal(t, "cloudwatch_logs", r.Type)
	assert.Equal(t, "active", r.Status)
	assert.Equal(t, "/aws/lambda/my-function", r.Name)
	assert.Equal(t, "1024000", r.Attrs["stored_bytes"])
	assert.Equal(t, "14", r.Attrs["retention_days"])
}

// ══════════════════════════════════════════════════════════════════════════════
// Subnet Tests
// ══════════════════════════════════════════════════════════════════════════════

func TestScanSubnets(t *testing.T) {
	mock := &mockEC2Client{}
	mock.describeSubnetsFunc = func(_ context.Context, _ *ec2.DescribeSubnetsInput, _ ...func(*ec2.Options)) (*ec2.DescribeSubnetsOutput, error) {
		return &ec2.DescribeSubnetsOutput{
			Subnets: []ec2types.Subnet{
				{
					SubnetId:            aws.String("subnet-123"),
					VpcId:               aws.String("vpc-456"),
					CidrBlock:           aws.String("10.0.1.0/24"),
					AvailabilityZone:    aws.String("us-east-1a"),
					State:               ec2types.SubnetStateAvailable,
					MapPublicIpOnLaunch: aws.Bool(true),
					Tags:                []ec2types.Tag{{Key: aws.String("Name"), Value: aws.String("public-1a")}},
				},
			},
		}, nil
	}

	p := &Plugin{region: "us-east-1", accountID: "123456789012", ec2Client: func() EC2API { return mock }}
	resources, err := p.scanSubnets(context.Background())

	require.NoError(t, err)
	require.Len(t, resources, 1)

	r := resources[0]
	assert.Equal(t, "subnet-123", r.ID)
	assert.Equal(t, "subnet", r.Type)
	assert.Equal(t, "available", r.Status)
	assert.Equal(t, "public-1a", r.Name)
	assert.Equal(t, "10.0.1.0/24", r.Attrs["cidr"])
	assert.Equal(t, "true", r.Attrs["public"])
}

// ══════════════════════════════════════════════════════════════════════════════
// Security Group Tests
// ══════════════════════════════════════════════════════════════════════════════

func TestScanSecurityGroups(t *testing.T) {
	mock := &mockEC2Client{}
	mock.describeSecurityGroupsFunc = func(_ context.Context, _ *ec2.DescribeSecurityGroupsInput, _ ...func(*ec2.Options)) (*ec2.DescribeSecurityGroupsOutput, error) {
		return &ec2.DescribeSecurityGroupsOutput{
			SecurityGroups: []ec2types.SecurityGroup{
				{
					GroupId:             aws.String("sg-123"),
					GroupName:           aws.String("web-sg"),
					VpcId:               aws.String("vpc-456"),
					Description:         aws.String("Web server security group"),
					IpPermissions:       []ec2types.IpPermission{{}, {}},
					IpPermissionsEgress: []ec2types.IpPermission{{}},
				},
			},
		}, nil
	}

	p := &Plugin{region: "us-east-1", accountID: "123456789012", ec2Client: func() EC2API { return mock }}
	resources, err := p.scanSecurityGroups(context.Background())

	require.NoError(t, err)
	require.Len(t, resources, 1)

	r := resources[0]
	assert.Equal(t, "sg-123", r.ID)
	assert.Equal(t, "security_group", r.Type)
	assert.Equal(t, "active", r.Status)
	assert.Equal(t, "web-sg", r.Name)
	assert.Equal(t, "2", r.Attrs["inbound_rules"])
	assert.Equal(t, "1", r.Attrs["outbound_rules"])
}

// ══════════════════════════════════════════════════════════════════════════════
// EBS Volume Tests
// ══════════════════════════════════════════════════════════════════════════════

func TestScanEBSVolumes(t *testing.T) {
	mock := &mockEC2Client{}
	mock.describeVolumesFunc = func(_ context.Context, _ *ec2.DescribeVolumesInput, _ ...func(*ec2.Options)) (*ec2.DescribeVolumesOutput, error) {
		return &ec2.DescribeVolumesOutput{
			Volumes: []ec2types.Volume{
				{
					VolumeId:         aws.String("vol-123"),
					Size:             aws.Int32(100),
					VolumeType:       ec2types.VolumeTypeGp3,
					State:            ec2types.VolumeStateInUse,
					AvailabilityZone: aws.String("us-east-1a"),
					Encrypted:        aws.Bool(true),
					Attachments:      []ec2types.VolumeAttachment{{}},
					Tags:             []ec2types.Tag{{Key: aws.String("Name"), Value: aws.String("data-vol")}},
				},
			},
		}, nil
	}

	p := &Plugin{region: "us-east-1", accountID: "123456789012", ec2Client: func() EC2API { return mock }}
	resources, err := p.scanEBSVolumes(context.Background())

	require.NoError(t, err)
	require.Len(t, resources, 1)

	r := resources[0]
	assert.Equal(t, "vol-123", r.ID)
	assert.Equal(t, "ebs", r.Type)
	assert.Equal(t, "in-use", r.Status)
	assert.Equal(t, "data-vol", r.Name)
	assert.Equal(t, "100", r.Attrs["size_gb"])
	assert.Equal(t, "gp3", r.Attrs["type"])
	assert.Equal(t, "true", r.Attrs["encrypted"])
	assert.Equal(t, "true", r.Attrs["attached"])
}

// ══════════════════════════════════════════════════════════════════════════════
// Elastic IP Tests
// ══════════════════════════════════════════════════════════════════════════════

func TestScanElasticIPs(t *testing.T) {
	mock := &mockEC2Client{}
	mock.describeAddressesFunc = func(_ context.Context, _ *ec2.DescribeAddressesInput, _ ...func(*ec2.Options)) (*ec2.DescribeAddressesOutput, error) {
		return &ec2.DescribeAddressesOutput{
			Addresses: []ec2types.Address{
				{
					AllocationId:     aws.String("eipalloc-123"),
					PublicIp:         aws.String("54.1.2.3"),
					PrivateIpAddress: aws.String("10.0.0.1"),
					InstanceId:       aws.String("i-abc123"),
					AssociationId:    aws.String("eipassoc-xyz"),
				},
				{
					AllocationId: aws.String("eipalloc-456"),
					PublicIp:     aws.String("54.4.5.6"),
				},
			},
		}, nil
	}

	p := &Plugin{region: "us-east-1", accountID: "123456789012", ec2Client: func() EC2API { return mock }}
	resources, err := p.scanElasticIPs(context.Background())

	require.NoError(t, err)
	require.Len(t, resources, 2)

	assert.Equal(t, "eipalloc-123", resources[0].ID)
	assert.Equal(t, "eip", resources[0].Type)
	assert.Equal(t, "attached", resources[0].Status)
	assert.Equal(t, "54.1.2.3", resources[0].Attrs["public_ip"])

	assert.Equal(t, "unattached", resources[1].Status)
}

// ══════════════════════════════════════════════════════════════════════════════
// NAT Gateway Tests
// ══════════════════════════════════════════════════════════════════════════════

func TestScanNATGateways(t *testing.T) {
	mock := &mockEC2Client{}
	mock.describeNatGatewaysFunc = func(_ context.Context, _ *ec2.DescribeNatGatewaysInput, _ ...func(*ec2.Options)) (*ec2.DescribeNatGatewaysOutput, error) {
		return &ec2.DescribeNatGatewaysOutput{
			NatGateways: []ec2types.NatGateway{
				{
					NatGatewayId: aws.String("nat-123"),
					VpcId:        aws.String("vpc-456"),
					SubnetId:     aws.String("subnet-789"),
					State:        ec2types.NatGatewayStateAvailable,
					NatGatewayAddresses: []ec2types.NatGatewayAddress{
						{PublicIp: aws.String("54.1.2.3")},
					},
					Tags: []ec2types.Tag{{Key: aws.String("Name"), Value: aws.String("public-nat")}},
				},
			},
		}, nil
	}

	p := &Plugin{region: "us-east-1", accountID: "123456789012", ec2Client: func() EC2API { return mock }}
	resources, err := p.scanNATGateways(context.Background())

	require.NoError(t, err)
	require.Len(t, resources, 1)

	r := resources[0]
	assert.Equal(t, "nat-123", r.ID)
	assert.Equal(t, "nat_gateway", r.Type)
	assert.Equal(t, "available", r.Status)
	assert.Equal(t, "public-nat", r.Name)
	assert.Equal(t, "54.1.2.3", r.Attrs["public_ip"])
}

// ══════════════════════════════════════════════════════════════════════════════
// SNS Tests
// ══════════════════════════════════════════════════════════════════════════════

type mockSNSClient struct {
	ListTopicsFunc func(ctx context.Context, params *sns.ListTopicsInput, optFns ...func(*sns.Options)) (*sns.ListTopicsOutput, error)
}

func (m *mockSNSClient) ListTopics(ctx context.Context, params *sns.ListTopicsInput, optFns ...func(*sns.Options)) (*sns.ListTopicsOutput, error) {
	return m.ListTopicsFunc(ctx, params, optFns...)
}

func TestScanSNS(t *testing.T) {
	mock := &mockSNSClient{
		ListTopicsFunc: func(_ context.Context, _ *sns.ListTopicsInput, _ ...func(*sns.Options)) (*sns.ListTopicsOutput, error) {
			return &sns.ListTopicsOutput{
				Topics: []snstypes.Topic{
					{TopicArn: aws.String("arn:aws:sns:us-east-1:123456789012:my-topic")},
					{TopicArn: aws.String("arn:aws:sns:us-east-1:123456789012:alerts")},
				},
			}, nil
		},
	}

	p := &Plugin{region: "us-east-1", accountID: "123456789012", snsClient: func() SNSAPI { return mock }}
	resources, err := p.scanSNS(context.Background())

	require.NoError(t, err)
	require.Len(t, resources, 2)

	assert.Equal(t, "sns", resources[0].Type)
	assert.Equal(t, "my-topic", resources[0].Name)
	assert.Equal(t, "active", resources[0].Status)
}

// ══════════════════════════════════════════════════════════════════════════════
// CloudFront Tests
// ══════════════════════════════════════════════════════════════════════════════

type mockCloudFrontClient struct {
	ListDistributionsFunc func(ctx context.Context, params *cloudfront.ListDistributionsInput, optFns ...func(*cloudfront.Options)) (*cloudfront.ListDistributionsOutput, error)
}

func (m *mockCloudFrontClient) ListDistributions(ctx context.Context, params *cloudfront.ListDistributionsInput, optFns ...func(*cloudfront.Options)) (*cloudfront.ListDistributionsOutput, error) {
	return m.ListDistributionsFunc(ctx, params, optFns...)
}

func TestScanCloudFront(t *testing.T) {
	mock := &mockCloudFrontClient{
		ListDistributionsFunc: func(_ context.Context, _ *cloudfront.ListDistributionsInput, _ ...func(*cloudfront.Options)) (*cloudfront.ListDistributionsOutput, error) {
			return &cloudfront.ListDistributionsOutput{
				DistributionList: &cftypes.DistributionList{
					Items: []cftypes.DistributionSummary{
						{
							Id:         aws.String("E123ABC"),
							DomainName: aws.String("d123.cloudfront.net"),
							Status:     aws.String("Deployed"),
							Enabled:    aws.Bool(true),
							Origins: &cftypes.Origins{
								Items: []cftypes.Origin{
									{DomainName: aws.String("mybucket.s3.amazonaws.com")},
								},
							},
						},
					},
					IsTruncated: aws.Bool(false),
				},
			}, nil
		},
	}

	p := &Plugin{region: "us-east-1", accountID: "123456789012", cloudfrontClient: func() CloudFrontAPI { return mock }}
	resources, err := p.scanCloudFront(context.Background())

	require.NoError(t, err)
	require.Len(t, resources, 1)

	r := resources[0]
	assert.Equal(t, "E123ABC", r.ID)
	assert.Equal(t, "cloudfront", r.Type)
	assert.Equal(t, "Deployed", r.Status)
	assert.Equal(t, "d123.cloudfront.net", r.Attrs["domain"])
	assert.Equal(t, "mybucket.s3.amazonaws.com", r.Attrs["origin"])
}

// ══════════════════════════════════════════════════════════════════════════════
// ElastiCache Tests
// ══════════════════════════════════════════════════════════════════════════════

type mockElastiCacheClient struct {
	DescribeCacheClustersFunc func(ctx context.Context, params *elasticache.DescribeCacheClustersInput, optFns ...func(*elasticache.Options)) (*elasticache.DescribeCacheClustersOutput, error)
}

func (m *mockElastiCacheClient) DescribeCacheClusters(ctx context.Context, params *elasticache.DescribeCacheClustersInput, optFns ...func(*elasticache.Options)) (*elasticache.DescribeCacheClustersOutput, error) {
	return m.DescribeCacheClustersFunc(ctx, params, optFns...)
}

func TestScanElastiCache(t *testing.T) {
	mock := &mockElastiCacheClient{
		DescribeCacheClustersFunc: func(_ context.Context, _ *elasticache.DescribeCacheClustersInput, _ ...func(*elasticache.Options)) (*elasticache.DescribeCacheClustersOutput, error) {
			return &elasticache.DescribeCacheClustersOutput{
				CacheClusters: []ectypes.CacheCluster{
					{
						CacheClusterId:     aws.String("my-redis"),
						CacheClusterStatus: aws.String("available"),
						Engine:             aws.String("redis"),
						EngineVersion:      aws.String("7.0"),
						CacheNodeType:      aws.String("cache.t3.micro"),
						NumCacheNodes:      aws.Int32(1),
					},
				},
			}, nil
		},
	}

	p := &Plugin{region: "us-east-1", accountID: "123456789012", elasticacheClient: func() ElastiCacheAPI { return mock }}
	resources, err := p.scanElastiCache(context.Background())

	require.NoError(t, err)
	require.Len(t, resources, 1)

	r := resources[0]
	assert.Equal(t, "my-redis", r.ID)
	assert.Equal(t, "elasticache", r.Type)
	assert.Equal(t, "available", r.Status)
	assert.Equal(t, "redis", r.Attrs["engine"])
	assert.Equal(t, "cache.t3.micro", r.Attrs["node_type"])
}

// ══════════════════════════════════════════════════════════════════════════════
// Secrets Manager Tests
// ══════════════════════════════════════════════════════════════════════════════

type mockSecretsManagerClient struct {
	ListSecretsFunc func(ctx context.Context, params *secretsmanager.ListSecretsInput, optFns ...func(*secretsmanager.Options)) (*secretsmanager.ListSecretsOutput, error)
}

func (m *mockSecretsManagerClient) ListSecrets(ctx context.Context, params *secretsmanager.ListSecretsInput, optFns ...func(*secretsmanager.Options)) (*secretsmanager.ListSecretsOutput, error) {
	return m.ListSecretsFunc(ctx, params, optFns...)
}

func TestScanSecretsManager(t *testing.T) {
	mock := &mockSecretsManagerClient{
		ListSecretsFunc: func(_ context.Context, _ *secretsmanager.ListSecretsInput, _ ...func(*secretsmanager.Options)) (*secretsmanager.ListSecretsOutput, error) {
			return &secretsmanager.ListSecretsOutput{
				SecretList: []smtypes.SecretListEntry{
					{
						ARN:         aws.String("arn:aws:secretsmanager:us-east-1:123456789012:secret:db-password-abc123"),
						Name:        aws.String("db-password"),
						Description: aws.String("Database password"),
					},
				},
			}, nil
		},
	}

	p := &Plugin{region: "us-east-1", accountID: "123456789012", secretsmanagerClient: func() SecretsManagerAPI { return mock }}
	resources, err := p.scanSecretsManager(context.Background())

	require.NoError(t, err)
	require.Len(t, resources, 1)

	r := resources[0]
	assert.Equal(t, "secretsmanager", r.Type)
	assert.Equal(t, "db-password", r.Name)
	assert.Equal(t, "active", r.Status)
	assert.Equal(t, "Database password", r.Attrs["description"])
}

// ══════════════════════════════════════════════════════════════════════════════
// extractTopicName Tests
// ══════════════════════════════════════════════════════════════════════════════

func TestExtractTopicName(t *testing.T) {
	tests := []struct {
		arn  string
		want string
	}{
		{"arn:aws:sns:us-east-1:123456789012:my-topic", "my-topic"},
		{"arn:aws:sns:eu-west-1:987654321:alerts", "alerts"},
		{"simple-name", "simple-name"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			got := extractTopicName(tt.arn)
			assert.Equal(t, tt.want, got)
		})
	}
}

// ══════════════════════════════════════════════════════════════════════════════
// ACM Tests
// ══════════════════════════════════════════════════════════════════════════════

type mockACMClient struct {
	ListCertificatesFunc func(ctx context.Context, params *acm.ListCertificatesInput, optFns ...func(*acm.Options)) (*acm.ListCertificatesOutput, error)
}

func (m *mockACMClient) ListCertificates(ctx context.Context, params *acm.ListCertificatesInput, optFns ...func(*acm.Options)) (*acm.ListCertificatesOutput, error) {
	return m.ListCertificatesFunc(ctx, params, optFns...)
}

func TestScanACM(t *testing.T) {
	mock := &mockACMClient{
		ListCertificatesFunc: func(_ context.Context, _ *acm.ListCertificatesInput, _ ...func(*acm.Options)) (*acm.ListCertificatesOutput, error) {
			return &acm.ListCertificatesOutput{
				CertificateSummaryList: []acmtypes.CertificateSummary{
					{
						CertificateArn: aws.String("arn:aws:acm:us-east-1:123456789012:certificate/abc-123"),
						DomainName:     aws.String("example.com"),
						Status:         acmtypes.CertificateStatusIssued,
						Type:           acmtypes.CertificateTypeAmazonIssued,
					},
				},
			}, nil
		},
	}

	p := &Plugin{region: "us-east-1", accountID: "123456789012", acmClient: func() ACMAPI { return mock }}
	resources, err := p.scanACM(context.Background())

	require.NoError(t, err)
	require.Len(t, resources, 1)

	r := resources[0]
	assert.Equal(t, "acm", r.Type)
	assert.Equal(t, "example.com", r.Name)
	assert.Equal(t, "ISSUED", r.Status)
	assert.Equal(t, "AMAZON_ISSUED", r.Attrs["type"])
}

// ══════════════════════════════════════════════════════════════════════════════
// API Gateway Tests
// ══════════════════════════════════════════════════════════════════════════════

type mockAPIGatewayClient struct {
	GetApisFunc func(ctx context.Context, params *apigatewayv2.GetApisInput, optFns ...func(*apigatewayv2.Options)) (*apigatewayv2.GetApisOutput, error)
}

func (m *mockAPIGatewayClient) GetApis(ctx context.Context, params *apigatewayv2.GetApisInput, optFns ...func(*apigatewayv2.Options)) (*apigatewayv2.GetApisOutput, error) {
	return m.GetApisFunc(ctx, params, optFns...)
}

func TestScanAPIGateway(t *testing.T) {
	mock := &mockAPIGatewayClient{
		GetApisFunc: func(_ context.Context, _ *apigatewayv2.GetApisInput, _ ...func(*apigatewayv2.Options)) (*apigatewayv2.GetApisOutput, error) {
			return &apigatewayv2.GetApisOutput{
				Items: []apigwtypes.Api{
					{
						ApiId:        aws.String("abc123"),
						Name:         aws.String("my-api"),
						ProtocolType: apigwtypes.ProtocolTypeHttp,
						ApiEndpoint:  aws.String("https://abc123.execute-api.us-east-1.amazonaws.com"),
					},
				},
			}, nil
		},
	}

	p := &Plugin{region: "us-east-1", accountID: "123456789012", apigatewayClient: func() APIGatewayAPI { return mock }}
	resources, err := p.scanAPIGateway(context.Background())

	require.NoError(t, err)
	require.Len(t, resources, 1)

	r := resources[0]
	assert.Equal(t, "apigateway", r.Type)
	assert.Equal(t, "my-api", r.Name)
	assert.Equal(t, "active", r.Status)
	assert.Equal(t, "HTTP", r.Attrs["protocol"])
}

// ══════════════════════════════════════════════════════════════════════════════
// Kinesis Tests
// ══════════════════════════════════════════════════════════════════════════════

type mockKinesisClient struct {
	ListStreamsFunc func(ctx context.Context, params *kinesis.ListStreamsInput, optFns ...func(*kinesis.Options)) (*kinesis.ListStreamsOutput, error)
}

func (m *mockKinesisClient) ListStreams(ctx context.Context, params *kinesis.ListStreamsInput, optFns ...func(*kinesis.Options)) (*kinesis.ListStreamsOutput, error) {
	return m.ListStreamsFunc(ctx, params, optFns...)
}

func TestScanKinesis(t *testing.T) {
	mock := &mockKinesisClient{
		ListStreamsFunc: func(_ context.Context, _ *kinesis.ListStreamsInput, _ ...func(*kinesis.Options)) (*kinesis.ListStreamsOutput, error) {
			return &kinesis.ListStreamsOutput{
				StreamSummaries: []kinesistypes.StreamSummary{
					{
						StreamName:   aws.String("my-stream"),
						StreamARN:    aws.String("arn:aws:kinesis:us-east-1:123456789012:stream/my-stream"),
						StreamStatus: kinesistypes.StreamStatusActive,
					},
				},
			}, nil
		},
	}

	p := &Plugin{region: "us-east-1", accountID: "123456789012", kinesisClient: func() KinesisAPI { return mock }}
	resources, err := p.scanKinesis(context.Background())

	require.NoError(t, err)
	require.Len(t, resources, 1)

	r := resources[0]
	assert.Equal(t, "kinesis", r.Type)
	assert.Equal(t, "my-stream", r.Name)
	assert.Equal(t, "ACTIVE", r.Status)
}

// ══════════════════════════════════════════════════════════════════════════════
// Redshift Tests
// ══════════════════════════════════════════════════════════════════════════════

type mockRedshiftClient struct {
	DescribeClustersFunc func(ctx context.Context, params *redshift.DescribeClustersInput, optFns ...func(*redshift.Options)) (*redshift.DescribeClustersOutput, error)
}

func (m *mockRedshiftClient) DescribeClusters(ctx context.Context, params *redshift.DescribeClustersInput, optFns ...func(*redshift.Options)) (*redshift.DescribeClustersOutput, error) {
	return m.DescribeClustersFunc(ctx, params, optFns...)
}

func TestScanRedshift(t *testing.T) {
	mock := &mockRedshiftClient{
		DescribeClustersFunc: func(_ context.Context, _ *redshift.DescribeClustersInput, _ ...func(*redshift.Options)) (*redshift.DescribeClustersOutput, error) {
			return &redshift.DescribeClustersOutput{
				Clusters: []redshifttypes.Cluster{
					{
						ClusterIdentifier: aws.String("my-cluster"),
						ClusterStatus:     aws.String("available"),
						NodeType:          aws.String("dc2.large"),
						NumberOfNodes:     aws.Int32(2),
						DBName:            aws.String("mydb"),
					},
				},
			}, nil
		},
	}

	p := &Plugin{region: "us-east-1", accountID: "123456789012", redshiftClient: func() RedshiftAPI { return mock }}
	resources, err := p.scanRedshift(context.Background())

	require.NoError(t, err)
	require.Len(t, resources, 1)

	r := resources[0]
	assert.Equal(t, "redshift", r.Type)
	assert.Equal(t, "my-cluster", r.ID)
	assert.Equal(t, "available", r.Status)
	assert.Equal(t, "dc2.large", r.Attrs["node_type"])
	assert.Equal(t, "2", r.Attrs["node_count"])
}

// ══════════════════════════════════════════════════════════════════════════════
// Step Functions Tests
// ══════════════════════════════════════════════════════════════════════════════

type mockSFNClient struct {
	ListStateMachinesFunc func(ctx context.Context, params *sfn.ListStateMachinesInput, optFns ...func(*sfn.Options)) (*sfn.ListStateMachinesOutput, error)
}

func (m *mockSFNClient) ListStateMachines(ctx context.Context, params *sfn.ListStateMachinesInput, optFns ...func(*sfn.Options)) (*sfn.ListStateMachinesOutput, error) {
	return m.ListStateMachinesFunc(ctx, params, optFns...)
}

func TestScanStepFunctions(t *testing.T) {
	mock := &mockSFNClient{
		ListStateMachinesFunc: func(_ context.Context, _ *sfn.ListStateMachinesInput, _ ...func(*sfn.Options)) (*sfn.ListStateMachinesOutput, error) {
			return &sfn.ListStateMachinesOutput{
				StateMachines: []sfntypes.StateMachineListItem{
					{
						StateMachineArn: aws.String("arn:aws:states:us-east-1:123456789012:stateMachine:my-workflow"),
						Name:            aws.String("my-workflow"),
						Type:            sfntypes.StateMachineTypeStandard,
					},
				},
			}, nil
		},
	}

	p := &Plugin{region: "us-east-1", accountID: "123456789012", sfnClient: func() StepFunctionsAPI { return mock }}
	resources, err := p.scanStepFunctions(context.Background())

	require.NoError(t, err)
	require.Len(t, resources, 1)

	r := resources[0]
	assert.Equal(t, "stepfunctions", r.Type)
	assert.Equal(t, "my-workflow", r.Name)
	assert.Equal(t, "active", r.Status)
	assert.Equal(t, "STANDARD", r.Attrs["type"])
}

// ══════════════════════════════════════════════════════════════════════════════
// Glue Tests
// ══════════════════════════════════════════════════════════════════════════════

type mockGlueClient struct {
	GetDatabasesFunc func(ctx context.Context, params *glue.GetDatabasesInput, optFns ...func(*glue.Options)) (*glue.GetDatabasesOutput, error)
}

func (m *mockGlueClient) GetDatabases(ctx context.Context, params *glue.GetDatabasesInput, optFns ...func(*glue.Options)) (*glue.GetDatabasesOutput, error) {
	return m.GetDatabasesFunc(ctx, params, optFns...)
}

func TestScanGlue(t *testing.T) {
	mock := &mockGlueClient{
		GetDatabasesFunc: func(_ context.Context, _ *glue.GetDatabasesInput, _ ...func(*glue.Options)) (*glue.GetDatabasesOutput, error) {
			return &glue.GetDatabasesOutput{
				DatabaseList: []gluetypes.Database{
					{
						Name:        aws.String("my-database"),
						Description: aws.String("Analytics database"),
						CatalogId:   aws.String("123456789012"),
					},
				},
			}, nil
		},
	}

	p := &Plugin{region: "us-east-1", accountID: "123456789012", glueClient: func() GlueAPI { return mock }}
	resources, err := p.scanGlue(context.Background())

	require.NoError(t, err)
	require.Len(t, resources, 1)

	r := resources[0]
	assert.Equal(t, "glue_database", r.Type)
	assert.Equal(t, "my-database", r.Name)
	assert.Equal(t, "active", r.Status)
	assert.Equal(t, "Analytics database", r.Attrs["description"])
}
