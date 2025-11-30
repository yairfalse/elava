package aws

import (
	"context"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockEC2Client implements EC2API for testing.
type mockEC2Client struct {
	DescribeInstancesFunc func(ctx context.Context, params *ec2.DescribeInstancesInput, optFns ...func(*ec2.Options)) (*ec2.DescribeInstancesOutput, error)
}

func (m *mockEC2Client) DescribeInstances(ctx context.Context, params *ec2.DescribeInstancesInput, optFns ...func(*ec2.Options)) (*ec2.DescribeInstancesOutput, error) {
	return m.DescribeInstancesFunc(ctx, params, optFns...)
}

func (m *mockEC2Client) DescribeVpcs(context.Context, *ec2.DescribeVpcsInput, ...func(*ec2.Options)) (*ec2.DescribeVpcsOutput, error) {
	return nil, nil
}

func (m *mockEC2Client) DescribeSubnets(context.Context, *ec2.DescribeSubnetsInput, ...func(*ec2.Options)) (*ec2.DescribeSubnetsOutput, error) {
	return nil, nil
}

func (m *mockEC2Client) DescribeSecurityGroups(context.Context, *ec2.DescribeSecurityGroupsInput, ...func(*ec2.Options)) (*ec2.DescribeSecurityGroupsOutput, error) {
	return nil, nil
}

func (m *mockEC2Client) DescribeVolumes(context.Context, *ec2.DescribeVolumesInput, ...func(*ec2.Options)) (*ec2.DescribeVolumesOutput, error) {
	return nil, nil
}

func (m *mockEC2Client) DescribeAddresses(context.Context, *ec2.DescribeAddressesInput, ...func(*ec2.Options)) (*ec2.DescribeAddressesOutput, error) {
	return nil, nil
}

func (m *mockEC2Client) DescribeNatGateways(context.Context, *ec2.DescribeNatGatewaysInput, ...func(*ec2.Options)) (*ec2.DescribeNatGatewaysOutput, error) {
	return nil, nil
}

func (m *mockEC2Client) DescribeAccountAttributes(context.Context, *ec2.DescribeAccountAttributesInput, ...func(*ec2.Options)) (*ec2.DescribeAccountAttributesOutput, error) {
	return nil, nil
}

func TestScanEC2(t *testing.T) {
	mock := &mockEC2Client{
		DescribeInstancesFunc: func(ctx context.Context, params *ec2.DescribeInstancesInput, optFns ...func(*ec2.Options)) (*ec2.DescribeInstancesOutput, error) {
			return &ec2.DescribeInstancesOutput{
				Reservations: []types.Reservation{
					{
						Instances: []types.Instance{
							{
								InstanceId:   aws.String("i-abc123"),
								InstanceType: types.InstanceTypeT2Micro,
								State: &types.InstanceState{
									Name: types.InstanceStateNameRunning,
								},
								Placement: &types.Placement{
									AvailabilityZone: aws.String("us-east-1a"),
								},
								VpcId:            aws.String("vpc-123"),
								SubnetId:         aws.String("subnet-456"),
								PrivateIpAddress: aws.String("10.0.0.1"),
								Tags: []types.Tag{
									{Key: aws.String("Name"), Value: aws.String("test-instance")},
									{Key: aws.String("env"), Value: aws.String("prod")},
								},
							},
						},
					},
				},
			}, nil
		},
	}

	p := &Plugin{
		region:    "us-east-1",
		accountID: "123456789012",
		ec2Client: mock, // This will fail - ec2Client expects *ec2.Client, not interface
	}

	resources, err := p.scanEC2(context.Background())

	require.NoError(t, err)
	require.Len(t, resources, 1)

	r := resources[0]
	assert.Equal(t, "i-abc123", r.ID)
	assert.Equal(t, "ec2", r.Type)
	assert.Equal(t, "aws", r.Provider)
	assert.Equal(t, "us-east-1", r.Region)
	assert.Equal(t, "running", r.Status)
	assert.Equal(t, "test-instance", r.Name)
	assert.Equal(t, "prod", r.Labels["env"])
	assert.Equal(t, "t2.micro", r.Attrs["instance_type"])
}

func TestScanEC2_Empty(t *testing.T) {
	mock := &mockEC2Client{
		DescribeInstancesFunc: func(ctx context.Context, params *ec2.DescribeInstancesInput, optFns ...func(*ec2.Options)) (*ec2.DescribeInstancesOutput, error) {
			return &ec2.DescribeInstancesOutput{}, nil
		},
	}

	p := &Plugin{
		region:    "us-east-1",
		accountID: "123456789012",
		ec2Client: mock,
	}

	resources, err := p.scanEC2(context.Background())

	require.NoError(t, err)
	assert.Empty(t, resources)
}
