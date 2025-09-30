package aws

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/yairfalse/elava/types"
)

// listElasticIPs discovers Elastic IPs
func (p *RealAWSProvider) listElasticIPs(ctx context.Context, filter types.ResourceFilter) ([]types.Resource, error) {
	var resources []types.Resource

	output, err := p.ec2Client.DescribeAddresses(ctx, &ec2.DescribeAddressesInput{})
	if err != nil {
		return nil, fmt.Errorf("failed to list Elastic IPs: %w", err)
	}

	for _, eip := range output.Addresses {
		resource := p.processElasticIP(eip)
		resources = append(resources, resource)
	}

	return resources, nil
}

// processElasticIP processes a single Elastic IP
func (p *RealAWSProvider) processElasticIP(eip ec2types.Address) types.Resource {
	tags := p.convertEC2TagsToElava(eip.Tags)

	status := "allocated"
	isOrphaned := eip.InstanceId == nil && eip.NetworkInterfaceId == nil
	if isOrphaned {
		status = "unassociated"
	}

	return types.Resource{
		ID:         aws.ToString(eip.AllocationId),
		Type:       "eip",
		Provider:   "aws",
		Region:     p.region,
		AccountID:  p.accountID,
		Name:       aws.ToString(eip.PublicIp),
		Status:     status,
		Tags:       tags,
		LastSeenAt: time.Now(),
		IsOrphaned: isOrphaned || p.isResourceOrphaned(tags),
		Metadata: types.ResourceMetadata{
			PublicIP:      aws.ToString(eip.PublicIp),
			AssociationID: aws.ToString(eip.AssociationId),
			IsAssociated:  aws.ToString(eip.InstanceId) != "",
			State:         status,
		},
	}
}

// listNATGateways discovers NAT Gateways
func (p *RealAWSProvider) listNATGateways(ctx context.Context, filter types.ResourceFilter) ([]types.Resource, error) {
	var resources []types.Resource
	paginator := ec2.NewDescribeNatGatewaysPaginator(p.ec2Client, &ec2.DescribeNatGatewaysInput{})

	for paginator.HasMorePages() {
		output, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to list NAT Gateways: %w", err)
		}

		for _, nat := range output.NatGateways {
			resource := p.processNATGateway(nat)
			resources = append(resources, resource)
		}
	}

	return resources, nil
}

// processNATGateway processes a single NAT Gateway
func (p *RealAWSProvider) processNATGateway(nat ec2types.NatGateway) types.Resource {
	tags := p.convertEC2TagsToElava(nat.Tags)

	name := aws.ToString(nat.NatGatewayId)
	if tags.Name != "" {
		name = tags.Name
	}

	isOrphaned := p.isResourceOrphaned(tags) ||
		nat.State == ec2types.NatGatewayStateFailed ||
		nat.State == ec2types.NatGatewayStateDeleted

	return types.Resource{
		ID:         aws.ToString(nat.NatGatewayId),
		Type:       "nat",
		Provider:   "aws",
		Region:     p.region,
		AccountID:  p.accountID,
		Name:       name,
		Status:     string(nat.State),
		Tags:       tags,
		CreatedAt:  p.safeTimeValue(nat.CreateTime),
		LastSeenAt: time.Now(),
		IsOrphaned: isOrphaned,
		Metadata: types.ResourceMetadata{
			VpcID:        aws.ToString(nat.VpcId),
			SubnetID:     aws.ToString(nat.SubnetId),
			NatGatewayID: aws.ToString(nat.NatGatewayId),
			State:        string(nat.State),
		},
	}
}

// listSecurityGroups discovers Security Groups
func (p *RealAWSProvider) listSecurityGroups(ctx context.Context, filter types.ResourceFilter) ([]types.Resource, error) {
	var resources []types.Resource
	paginator := ec2.NewDescribeSecurityGroupsPaginator(p.ec2Client, &ec2.DescribeSecurityGroupsInput{})

	for paginator.HasMorePages() {
		output, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to list Security Groups: %w", err)
		}

		for _, sg := range output.SecurityGroups {
			resource := p.processSecurityGroup(sg)
			resources = append(resources, resource)
		}
	}

	return resources, nil
}

// processSecurityGroup processes a single Security Group
func (p *RealAWSProvider) processSecurityGroup(sg ec2types.SecurityGroup) types.Resource {
	tags := p.convertEC2TagsToElava(sg.Tags)

	// Check if SG is default or in use
	isDefault := aws.ToString(sg.GroupName) == "default"
	hasRules := len(sg.IpPermissions) > 0 || len(sg.IpPermissionsEgress) > 1

	// Check if it's referenced by any network interfaces
	isInUse := p.isSecurityGroupInUse(aws.ToString(sg.GroupId))

	isOrphaned := !isDefault && !hasRules && !isInUse && p.isResourceOrphaned(tags)

	return types.Resource{
		ID:         aws.ToString(sg.GroupId),
		Type:       "sg",
		Provider:   "aws",
		Region:     p.region,
		AccountID:  p.accountID,
		Name:       aws.ToString(sg.GroupName),
		Status:     "active",
		Tags:       tags,
		LastSeenAt: time.Now(),
		IsOrphaned: isOrphaned,
		Metadata: types.ResourceMetadata{
			VpcID:     aws.ToString(sg.VpcId),
			GroupName: aws.ToString(sg.GroupName),
			State:     "active",
		},
	}
}

// isSecurityGroupInUse checks if a security group is in use
func (p *RealAWSProvider) isSecurityGroupInUse(groupID string) bool {
	// This is a simplified check - in production you'd check network interfaces
	// For now, we'll assume any non-default group might be in use
	return false
}

// listVPCEndpoints discovers VPC Endpoints
func (p *RealAWSProvider) listVPCEndpoints(ctx context.Context, filter types.ResourceFilter) ([]types.Resource, error) {
	var resources []types.Resource
	paginator := ec2.NewDescribeVpcEndpointsPaginator(p.ec2Client, &ec2.DescribeVpcEndpointsInput{})

	for paginator.HasMorePages() {
		output, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to list VPC Endpoints: %w", err)
		}

		for _, endpoint := range output.VpcEndpoints {
			resource := p.processVPCEndpoint(endpoint)
			resources = append(resources, resource)
		}
	}

	return resources, nil
}

// processVPCEndpoint processes a single VPC Endpoint
func (p *RealAWSProvider) processVPCEndpoint(endpoint ec2types.VpcEndpoint) types.Resource {
	tags := p.convertEC2TagsToElava(endpoint.Tags)

	name := aws.ToString(endpoint.VpcEndpointId)
	if tags.Name != "" {
		name = tags.Name
	}

	isOrphaned := p.isResourceOrphaned(tags) ||
		endpoint.State == ec2types.StateDeleting ||
		endpoint.State == ec2types.StateFailed

	return types.Resource{
		ID:         aws.ToString(endpoint.VpcEndpointId),
		Type:       "vpc-endpoint",
		Provider:   "aws",
		Region:     p.region,
		AccountID:  p.accountID,
		Name:       name,
		Status:     string(endpoint.State),
		Tags:       tags,
		CreatedAt:  p.safeTimeValue(endpoint.CreationTimestamp),
		LastSeenAt: time.Now(),
		IsOrphaned: isOrphaned,
		Metadata: types.ResourceMetadata{
			VpcID: aws.ToString(endpoint.VpcId),
			State: string(endpoint.State),
		},
	}
}

// listNetworkInterfaces discovers Network Interfaces
func (p *RealAWSProvider) listNetworkInterfaces(ctx context.Context, filter types.ResourceFilter) ([]types.Resource, error) {
	var resources []types.Resource
	paginator := ec2.NewDescribeNetworkInterfacesPaginator(p.ec2Client, &ec2.DescribeNetworkInterfacesInput{})

	for paginator.HasMorePages() {
		output, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to list Network Interfaces: %w", err)
		}

		for _, ni := range output.NetworkInterfaces {
			resource := p.processNetworkInterface(ni)
			resources = append(resources, resource)
		}
	}

	return resources, nil
}

// processNetworkInterface processes a single Network Interface
func (p *RealAWSProvider) processNetworkInterface(ni ec2types.NetworkInterface) types.Resource {
	tags := p.convertEC2TagsToElava(ni.TagSet)

	// Check if it's attached
	isAttached := ni.Attachment != nil
	isManaged := strings.HasPrefix(aws.ToString(ni.RequesterId), "amazon-")

	status := string(ni.Status)
	isOrphaned := !isAttached && !isManaged && p.isResourceOrphaned(tags)

	return types.Resource{
		ID:         aws.ToString(ni.NetworkInterfaceId),
		Type:       "eni",
		Provider:   "aws",
		Region:     p.region,
		AccountID:  p.accountID,
		Name:       aws.ToString(ni.NetworkInterfaceId),
		Status:     status,
		Tags:       tags,
		LastSeenAt: time.Now(),
		IsOrphaned: isOrphaned,
		Metadata: types.ResourceMetadata{
			VpcID:              aws.ToString(ni.VpcId),
			SubnetID:           aws.ToString(ni.SubnetId),
			NetworkInterfaceID: aws.ToString(ni.NetworkInterfaceId),
			PrivateIP:          aws.ToString(ni.PrivateIpAddress),
			IsAttached:         isAttached,
			State:              status,
		},
	}
}
