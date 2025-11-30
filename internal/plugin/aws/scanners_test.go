package aws

import (
	"context"
	"errors"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/autoscaling"
	asgtypes "github.com/aws/aws-sdk-go-v2/service/autoscaling/types"
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
	"github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2"
	elbtypes "github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2/types"
	"github.com/aws/aws-sdk-go-v2/service/iam"
	iamtypes "github.com/aws/aws-sdk-go-v2/service/iam/types"
	"github.com/aws/aws-sdk-go-v2/service/lambda"
	lambdatypes "github.com/aws/aws-sdk-go-v2/service/lambda/types"
	"github.com/aws/aws-sdk-go-v2/service/rds"
	rdstypes "github.com/aws/aws-sdk-go-v2/service/rds/types"
	"github.com/aws/aws-sdk-go-v2/service/route53"
	r53types "github.com/aws/aws-sdk-go-v2/service/route53/types"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	s3types "github.com/aws/aws-sdk-go-v2/service/s3/types"
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

	p := &Plugin{region: "us-east-1", accountID: "123456789012", rdsClient: mock}
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

	p := &Plugin{region: "us-east-1", accountID: "123456789012", rdsClient: mock}
	_, err := p.scanRDS(context.Background())

	require.Error(t, err)
	assert.Contains(t, err.Error(), "access denied")
}

// ══════════════════════════════════════════════════════════════════════════════
// S3 Tests
// ══════════════════════════════════════════════════════════════════════════════

type mockS3Client struct {
	ListBucketsFunc func(ctx context.Context, params *s3.ListBucketsInput, optFns ...func(*s3.Options)) (*s3.ListBucketsOutput, error)
}

func (m *mockS3Client) ListBuckets(ctx context.Context, params *s3.ListBucketsInput, optFns ...func(*s3.Options)) (*s3.ListBucketsOutput, error) {
	return m.ListBucketsFunc(ctx, params, optFns...)
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

	p := &Plugin{region: "us-east-1", accountID: "123456789012", s3Client: mock}
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

	p := &Plugin{region: "us-east-1", accountID: "123456789012", eksClient: mock}
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

	p := &Plugin{region: "us-east-1", accountID: "123456789012", ec2Client: mock}
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

	p := &Plugin{region: "us-east-1", accountID: "123456789012", dynamodbClient: mock}
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

	p := &Plugin{region: "us-east-1", accountID: "123456789012", lambdaClient: mock}
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

	p := &Plugin{region: "us-east-1", accountID: "123456789012", asgClient: mock}
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

	p := &Plugin{region: "us-east-1", accountID: "123456789012", asgClient: mock}
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

	p := &Plugin{region: "us-east-1", accountID: "123456789012", sqsClient: mock}
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

	p := &Plugin{region: "us-east-1", accountID: "123456789012", elbClient: mock}
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

	p := &Plugin{region: "us-east-1", accountID: "123456789012", iamClient: mock}
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

	p := &Plugin{region: "us-east-1", accountID: "123456789012", ecsClient: mock}
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

	p := &Plugin{region: "us-east-1", accountID: "123456789012", ecsClient: mock}
	resources, err := p.scanECS(context.Background())

	require.NoError(t, err)
	assert.Empty(t, resources)
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

	p := &Plugin{region: "us-east-1", accountID: "123456789012", route53Client: mock}
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

	p := &Plugin{region: "us-east-1", accountID: "123456789012", cwLogsClient: mock}
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

	p := &Plugin{region: "us-east-1", accountID: "123456789012", ec2Client: mock}
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

	p := &Plugin{region: "us-east-1", accountID: "123456789012", ec2Client: mock}
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

	p := &Plugin{region: "us-east-1", accountID: "123456789012", ec2Client: mock}
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

	p := &Plugin{region: "us-east-1", accountID: "123456789012", ec2Client: mock}
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

	p := &Plugin{region: "us-east-1", accountID: "123456789012", ec2Client: mock}
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
