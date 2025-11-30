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
	DescribeInstancesFunc      func(ctx context.Context, params *ec2.DescribeInstancesInput, optFns ...func(*ec2.Options)) (*ec2.DescribeInstancesOutput, error)
	describeVpcsFunc           func(ctx context.Context, params *ec2.DescribeVpcsInput, optFns ...func(*ec2.Options)) (*ec2.DescribeVpcsOutput, error)
	describeSubnetsFunc        func(ctx context.Context, params *ec2.DescribeSubnetsInput, optFns ...func(*ec2.Options)) (*ec2.DescribeSubnetsOutput, error)
	describeSecurityGroupsFunc func(ctx context.Context, params *ec2.DescribeSecurityGroupsInput, optFns ...func(*ec2.Options)) (*ec2.DescribeSecurityGroupsOutput, error)
	describeVolumesFunc        func(ctx context.Context, params *ec2.DescribeVolumesInput, optFns ...func(*ec2.Options)) (*ec2.DescribeVolumesOutput, error)
	describeAddressesFunc      func(ctx context.Context, params *ec2.DescribeAddressesInput, optFns ...func(*ec2.Options)) (*ec2.DescribeAddressesOutput, error)
	describeNatGatewaysFunc    func(ctx context.Context, params *ec2.DescribeNatGatewaysInput, optFns ...func(*ec2.Options)) (*ec2.DescribeNatGatewaysOutput, error)
	describeAccountAttrsFunc   func(ctx context.Context, params *ec2.DescribeAccountAttributesInput, optFns ...func(*ec2.Options)) (*ec2.DescribeAccountAttributesOutput, error)
}

func (m *mockEC2Client) DescribeInstances(ctx context.Context, params *ec2.DescribeInstancesInput, optFns ...func(*ec2.Options)) (*ec2.DescribeInstancesOutput, error) {
	if m.DescribeInstancesFunc != nil {
		return m.DescribeInstancesFunc(ctx, params, optFns...)
	}
	return &ec2.DescribeInstancesOutput{}, nil
}

func (m *mockEC2Client) DescribeVpcs(ctx context.Context, params *ec2.DescribeVpcsInput, optFns ...func(*ec2.Options)) (*ec2.DescribeVpcsOutput, error) {
	if m.describeVpcsFunc != nil {
		return m.describeVpcsFunc(ctx, params, optFns...)
	}
	return &ec2.DescribeVpcsOutput{}, nil
}

func (m *mockEC2Client) DescribeSubnets(ctx context.Context, params *ec2.DescribeSubnetsInput, optFns ...func(*ec2.Options)) (*ec2.DescribeSubnetsOutput, error) {
	if m.describeSubnetsFunc != nil {
		return m.describeSubnetsFunc(ctx, params, optFns...)
	}
	return &ec2.DescribeSubnetsOutput{}, nil
}

func (m *mockEC2Client) DescribeSecurityGroups(ctx context.Context, params *ec2.DescribeSecurityGroupsInput, optFns ...func(*ec2.Options)) (*ec2.DescribeSecurityGroupsOutput, error) {
	if m.describeSecurityGroupsFunc != nil {
		return m.describeSecurityGroupsFunc(ctx, params, optFns...)
	}
	return &ec2.DescribeSecurityGroupsOutput{}, nil
}

func (m *mockEC2Client) DescribeVolumes(ctx context.Context, params *ec2.DescribeVolumesInput, optFns ...func(*ec2.Options)) (*ec2.DescribeVolumesOutput, error) {
	if m.describeVolumesFunc != nil {
		return m.describeVolumesFunc(ctx, params, optFns...)
	}
	return &ec2.DescribeVolumesOutput{}, nil
}

func (m *mockEC2Client) DescribeAddresses(ctx context.Context, params *ec2.DescribeAddressesInput, optFns ...func(*ec2.Options)) (*ec2.DescribeAddressesOutput, error) {
	if m.describeAddressesFunc != nil {
		return m.describeAddressesFunc(ctx, params, optFns...)
	}
	return &ec2.DescribeAddressesOutput{}, nil
}

func (m *mockEC2Client) DescribeNatGateways(ctx context.Context, params *ec2.DescribeNatGatewaysInput, optFns ...func(*ec2.Options)) (*ec2.DescribeNatGatewaysOutput, error) {
	if m.describeNatGatewaysFunc != nil {
		return m.describeNatGatewaysFunc(ctx, params, optFns...)
	}
	return &ec2.DescribeNatGatewaysOutput{}, nil
}

func (m *mockEC2Client) DescribeAccountAttributes(ctx context.Context, params *ec2.DescribeAccountAttributesInput, optFns ...func(*ec2.Options)) (*ec2.DescribeAccountAttributesOutput, error) {
	if m.describeAccountAttrsFunc != nil {
		return m.describeAccountAttrsFunc(ctx, params, optFns...)
	}
	return &ec2.DescribeAccountAttributesOutput{}, nil
}

func newTestInstance() types.Instance {
	return types.Instance{
		InstanceId:       aws.String("i-abc123"),
		InstanceType:     types.InstanceTypeT2Micro,
		State:            &types.InstanceState{Name: types.InstanceStateNameRunning},
		Placement:        &types.Placement{AvailabilityZone: aws.String("us-east-1a")},
		VpcId:            aws.String("vpc-123"),
		SubnetId:         aws.String("subnet-456"),
		PrivateIpAddress: aws.String("10.0.0.1"),
		Tags: []types.Tag{
			{Key: aws.String("Name"), Value: aws.String("test-instance")},
			{Key: aws.String("env"), Value: aws.String("prod")},
		},
	}
}

func TestScanEC2(t *testing.T) {
	mock := &mockEC2Client{
		DescribeInstancesFunc: func(_ context.Context, _ *ec2.DescribeInstancesInput, _ ...func(*ec2.Options)) (*ec2.DescribeInstancesOutput, error) {
			return &ec2.DescribeInstancesOutput{
				Reservations: []types.Reservation{{Instances: []types.Instance{newTestInstance()}}},
			}, nil
		},
	}

	p := &Plugin{region: "us-east-1", accountID: "123456789012", ec2Client: mock}
	resources, err := p.scanEC2(context.Background())

	require.NoError(t, err)
	require.Len(t, resources, 1)

	r := resources[0]
	assert.Equal(t, "i-abc123", r.ID)
	assert.Equal(t, "ec2", r.Type)
	assert.Equal(t, "running", r.Status)
	assert.Equal(t, "test-instance", r.Name)
	assert.Equal(t, "prod", r.Labels["env"])
	assert.Equal(t, "t2.micro", r.Attrs["instance_type"])
}

func TestScanEC2_Empty(t *testing.T) {
	mock := &mockEC2Client{
		DescribeInstancesFunc: func(_ context.Context, _ *ec2.DescribeInstancesInput, _ ...func(*ec2.Options)) (*ec2.DescribeInstancesOutput, error) {
			return &ec2.DescribeInstancesOutput{}, nil
		},
	}

	p := &Plugin{region: "us-east-1", accountID: "123456789012", ec2Client: mock}
	resources, err := p.scanEC2(context.Background())

	require.NoError(t, err)
	assert.Empty(t, resources)
}

func TestScanEC2_Pagination(t *testing.T) {
	callCount := 0
	mock := &mockEC2Client{
		DescribeInstancesFunc: func(_ context.Context, params *ec2.DescribeInstancesInput, _ ...func(*ec2.Options)) (*ec2.DescribeInstancesOutput, error) {
			callCount++
			if callCount == 1 {
				return &ec2.DescribeInstancesOutput{
					Reservations: []types.Reservation{{Instances: []types.Instance{{InstanceId: aws.String("i-1"), State: &types.InstanceState{Name: types.InstanceStateNameRunning}}}}},
					NextToken:    aws.String("token"),
				}, nil
			}
			return &ec2.DescribeInstancesOutput{
				Reservations: []types.Reservation{{Instances: []types.Instance{{InstanceId: aws.String("i-2"), State: &types.InstanceState{Name: types.InstanceStateNameRunning}}}}},
			}, nil
		},
	}

	p := &Plugin{region: "us-east-1", accountID: "123456789012", ec2Client: mock}
	resources, err := p.scanEC2(context.Background())

	require.NoError(t, err)
	assert.Len(t, resources, 2)
	assert.Equal(t, 2, callCount)
}
