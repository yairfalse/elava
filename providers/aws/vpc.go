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

// ListSubnets scans all VPC subnets
func (p *RealAWSProvider) ListSubnets(ctx context.Context) ([]types.Resource, error) {
	paginator := ec2.NewDescribeSubnetsPaginator(p.ec2Client, &ec2.DescribeSubnetsInput{})

	var resources []types.Resource
	for paginator.HasMorePages() {
		output, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to describe subnets: %w", err)
		}

		for _, subnet := range output.Subnets {
			resource := buildSubnetResource(subnet, p.region, p.accountID)
			resources = append(resources, resource)
		}
	}

	return resources, nil
}

// buildSubnetResource converts EC2 subnet to types.Resource
func buildSubnetResource(subnet ec2types.Subnet, region, accountID string) types.Resource {
	return types.Resource{
		ID:         aws.ToString(subnet.SubnetId),
		Type:       "subnet",
		Provider:   "aws",
		Region:     region,
		AccountID:  accountID,
		Name:       extractNameTag(subnet.Tags),
		Status:     string(subnet.State),
		Tags:       convertEC2Tags(subnet.Tags),
		CreatedAt:  time.Now(), // EC2 subnets don't provide creation time
		LastSeenAt: time.Now(),
		Metadata: types.ResourceMetadata{
			VpcID:               aws.ToString(subnet.VpcId),
			CIDRBlock:           aws.ToString(subnet.CidrBlock),
			AvailabilityZone:    aws.ToString(subnet.AvailabilityZone),
			MapPublicIPOnLaunch: aws.ToBool(subnet.MapPublicIpOnLaunch),
		},
	}
}

// ListRouteTables scans all VPC route tables
func (p *RealAWSProvider) ListRouteTables(ctx context.Context) ([]types.Resource, error) {
	paginator := ec2.NewDescribeRouteTablesPaginator(p.ec2Client, &ec2.DescribeRouteTablesInput{})

	var resources []types.Resource
	for paginator.HasMorePages() {
		output, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to describe route tables: %w", err)
		}

		for _, routeTable := range output.RouteTables {
			resource := buildRouteTableResource(routeTable, p.region, p.accountID)
			resources = append(resources, resource)
		}
	}

	return resources, nil
}

// buildRouteTableResource converts EC2 route table to types.Resource
func buildRouteTableResource(routeTable ec2types.RouteTable, region, accountID string) types.Resource {
	return types.Resource{
		ID:         aws.ToString(routeTable.RouteTableId),
		Type:       "route_table",
		Provider:   "aws",
		Region:     region,
		AccountID:  accountID,
		Name:       extractNameTag(routeTable.Tags),
		Status:     "active",
		Tags:       convertEC2Tags(routeTable.Tags),
		CreatedAt:  time.Now(),
		LastSeenAt: time.Now(),
		Metadata: types.ResourceMetadata{
			VpcID:               aws.ToString(routeTable.VpcId),
			IsMainRouteTable:    isMainRouteTable(routeTable.Associations),
			AssociatedSubnetIDs: extractAssociatedSubnetIDs(routeTable.Associations),
			Routes:              formatRoutes(routeTable.Routes),
		},
	}
}

// ListInternetGateways scans all Internet Gateways
func (p *RealAWSProvider) ListInternetGateways(ctx context.Context) ([]types.Resource, error) {
	paginator := ec2.NewDescribeInternetGatewaysPaginator(p.ec2Client, &ec2.DescribeInternetGatewaysInput{})

	var resources []types.Resource
	for paginator.HasMorePages() {
		output, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to describe internet gateways: %w", err)
		}

		for _, igw := range output.InternetGateways {
			resource := buildInternetGatewayResource(igw, p.region, p.accountID)
			resources = append(resources, resource)
		}
	}

	return resources, nil
}

// buildInternetGatewayResource converts EC2 internet gateway to types.Resource
func buildInternetGatewayResource(igw ec2types.InternetGateway, region, accountID string) types.Resource {
	var vpcID, attachmentState string

	if len(igw.Attachments) > 0 {
		vpcID = aws.ToString(igw.Attachments[0].VpcId)
		attachmentState = string(igw.Attachments[0].State)
	} else {
		attachmentState = "detached"
	}

	return types.Resource{
		ID:         aws.ToString(igw.InternetGatewayId),
		Type:       "internet_gateway",
		Provider:   "aws",
		Region:     region,
		AccountID:  accountID,
		Name:       extractNameTag(igw.Tags),
		Status:     attachmentState,
		Tags:       convertEC2Tags(igw.Tags),
		CreatedAt:  time.Now(),
		LastSeenAt: time.Now(),
		Metadata: types.ResourceMetadata{
			VpcID:           vpcID,
			AttachmentState: attachmentState,
		},
	}
}

// ListNATGateways scans all NAT Gateways
func (p *RealAWSProvider) ListNATGateways(ctx context.Context) ([]types.Resource, error) {
	paginator := ec2.NewDescribeNatGatewaysPaginator(p.ec2Client, &ec2.DescribeNatGatewaysInput{})

	var resources []types.Resource
	for paginator.HasMorePages() {
		output, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to describe NAT gateways: %w", err)
		}

		for _, natGw := range output.NatGateways {
			resource := buildNATGatewayResource(natGw, p.region, p.accountID)
			resources = append(resources, resource)
		}
	}

	return resources, nil
}

// buildNATGatewayResource converts EC2 NAT gateway to types.Resource
func buildNATGatewayResource(natGw ec2types.NatGateway, region, accountID string) types.Resource {
	var elasticIPAllocationID, publicIP, networkInterfaceID string

	if len(natGw.NatGatewayAddresses) > 0 {
		addr := natGw.NatGatewayAddresses[0]
		elasticIPAllocationID = aws.ToString(addr.AllocationId)
		publicIP = aws.ToString(addr.PublicIp)
		networkInterfaceID = aws.ToString(addr.NetworkInterfaceId)
	}

	return types.Resource{
		ID:         aws.ToString(natGw.NatGatewayId),
		Type:       "nat_gateway",
		Provider:   "aws",
		Region:     region,
		AccountID:  accountID,
		Name:       extractNameTag(natGw.Tags),
		Status:     string(natGw.State),
		Tags:       convertEC2Tags(natGw.Tags),
		CreatedAt:  aws.ToTime(natGw.CreateTime),
		LastSeenAt: time.Now(),
		Metadata: types.ResourceMetadata{
			VpcID:                 aws.ToString(natGw.VpcId),
			SubnetID:              aws.ToString(natGw.SubnetId),
			ElasticIPAllocationID: elasticIPAllocationID,
			PublicIP:              publicIP,
			NetworkInterfaceID:    networkInterfaceID,
		},
	}
}

// ListVPCPeeringConnections scans all VPC peering connections
func (p *RealAWSProvider) ListVPCPeeringConnections(ctx context.Context) ([]types.Resource, error) {
	paginator := ec2.NewDescribeVpcPeeringConnectionsPaginator(p.ec2Client, &ec2.DescribeVpcPeeringConnectionsInput{})

	var resources []types.Resource
	for paginator.HasMorePages() {
		output, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to describe VPC peering connections: %w", err)
		}

		for _, peering := range output.VpcPeeringConnections {
			resource := buildVPCPeeringConnectionResource(peering, p.region, p.accountID)
			resources = append(resources, resource)
		}
	}

	return resources, nil
}

// buildVPCPeeringConnectionResource converts EC2 VPC peering connection to types.Resource
func buildVPCPeeringConnectionResource(peering ec2types.VpcPeeringConnection, region, accountID string) types.Resource {
	var status string
	if peering.Status != nil {
		status = string(peering.Status.Code)
	}

	var requesterVpcID, requesterCIDR, accepterVpcID, accepterCIDR, peerRegion string

	if peering.RequesterVpcInfo != nil {
		requesterVpcID = aws.ToString(peering.RequesterVpcInfo.VpcId)
		requesterCIDR = aws.ToString(peering.RequesterVpcInfo.CidrBlock)
	}

	if peering.AccepterVpcInfo != nil {
		accepterVpcID = aws.ToString(peering.AccepterVpcInfo.VpcId)
		accepterCIDR = aws.ToString(peering.AccepterVpcInfo.CidrBlock)
		peerRegion = aws.ToString(peering.AccepterVpcInfo.Region)
	}

	return types.Resource{
		ID:         aws.ToString(peering.VpcPeeringConnectionId),
		Type:       "vpc_peering_connection",
		Provider:   "aws",
		Region:     region,
		AccountID:  accountID,
		Name:       extractNameTag(peering.Tags),
		Status:     status,
		Tags:       convertEC2Tags(peering.Tags),
		CreatedAt:  time.Now(),
		LastSeenAt: time.Now(),
		Metadata: types.ResourceMetadata{
			RequesterVpcID:     requesterVpcID,
			AccepterVpcID:      accepterVpcID,
			RequesterCIDRBlock: requesterCIDR,
			AccepterCIDRBlock:  accepterCIDR,
			PeerRegion:         peerRegion,
		},
	}
}

// Helper functions

// formatRoutes formats route table routes into readable string
func formatRoutes(routes []ec2types.Route) string {
	if len(routes) == 0 {
		return ""
	}

	formatted := make([]string, 0, len(routes))
	for _, route := range routes {
		dest := aws.ToString(route.DestinationCidrBlock)
		if dest == "" {
			dest = aws.ToString(route.DestinationIpv6CidrBlock)
		}
		if dest == "" {
			dest = aws.ToString(route.DestinationPrefixListId)
		}

		var target string
		if route.GatewayId != nil {
			target = aws.ToString(route.GatewayId)
		} else if route.NatGatewayId != nil {
			target = aws.ToString(route.NatGatewayId)
		} else if route.VpcPeeringConnectionId != nil {
			target = aws.ToString(route.VpcPeeringConnectionId)
		} else if route.NetworkInterfaceId != nil {
			target = aws.ToString(route.NetworkInterfaceId)
		} else if route.TransitGatewayId != nil {
			target = aws.ToString(route.TransitGatewayId)
		} else {
			target = "unknown"
		}

		state := string(route.State)
		formatted = append(formatted, fmt.Sprintf("%s → %s (%s)", dest, target, state))
	}

	return strings.Join(formatted, "; ")
}

// extractAssociatedSubnetIDs extracts subnet IDs from route table associations
func extractAssociatedSubnetIDs(associations []ec2types.RouteTableAssociation) string {
	if len(associations) == 0 {
		return ""
	}

	subnetIDs := make([]string, 0, len(associations))
	for _, assoc := range associations {
		if assoc.SubnetId != nil {
			subnetIDs = append(subnetIDs, aws.ToString(assoc.SubnetId))
		}
	}

	return strings.Join(subnetIDs, ",")
}

// isMainRouteTable checks if route table is the main route table
func isMainRouteTable(associations []ec2types.RouteTableAssociation) bool {
	for _, assoc := range associations {
		if aws.ToBool(assoc.Main) {
			return true
		}
	}
	return false
}

// extractNameTag extracts the Name tag from EC2 tags
func extractNameTag(tags []ec2types.Tag) string {
	for _, tag := range tags {
		if aws.ToString(tag.Key) == "Name" {
			return aws.ToString(tag.Value)
		}
	}
	return ""
}

// convertEC2Tags converts EC2 tags to Elava tags
func convertEC2Tags(tags []ec2types.Tag) types.Tags {
	result := types.Tags{}
	for _, tag := range tags {
		key := aws.ToString(tag.Key)
		value := aws.ToString(tag.Value)

		switch key {
		case "Name":
			result.Name = value
		case "Environment":
			result.Environment = value
		case "Team":
			result.Team = value
		case "Project":
			result.Project = value
		case "Application":
			result.Application = value
		case "Owner":
			result.Owner = value
		case "Contact":
			result.Contact = value
		case "CostCenter", "cost-center":
			result.CostCenter = value
		case "CreatedBy", "created-by":
			result.CreatedBy = value
		case "CreatedDate", "created-date":
			result.CreatedDate = value
		case "elava:owner":
			result.ElavaOwner = value
		case "elava:managed":
			result.ElavaManaged = value == "true"
		case "elava:blessed":
			result.ElavaBlessed = value == "true"
		case "elava:generation":
			result.ElavaGeneration = value
		case "elava:claimed_at":
			result.ElavaClaimedAt = value
		}
	}
	return result
}
