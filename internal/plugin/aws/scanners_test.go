package aws

import (
	"context"
	"errors"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	ddbtypes "github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/aws/aws-sdk-go-v2/service/eks"
	ekstypes "github.com/aws/aws-sdk-go-v2/service/eks/types"
	"github.com/aws/aws-sdk-go-v2/service/rds"
	rdstypes "github.com/aws/aws-sdk-go-v2/service/rds/types"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	s3types "github.com/aws/aws-sdk-go-v2/service/s3/types"
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
