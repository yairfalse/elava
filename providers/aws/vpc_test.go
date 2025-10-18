package aws

import (
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/stretchr/testify/assert"
)

func TestBuildSubnetResource(t *testing.T) {
	t.Run("public subnet", func(t *testing.T) {
		subnet := ec2types.Subnet{
			SubnetId:            aws.String("subnet-public-1a"),
			VpcId:               aws.String("vpc-abc123"),
			CidrBlock:           aws.String("10.0.1.0/24"),
			AvailabilityZone:    aws.String("us-east-1a"),
			MapPublicIpOnLaunch: aws.Bool(true),
			Tags: []ec2types.Tag{
				{Key: aws.String("Name"), Value: aws.String("public-subnet-1a")},
				{Key: aws.String("Type"), Value: aws.String("public")},
			},
		}

		resource := buildSubnetResource(subnet, "us-east-1", "123456789")

		assert.Equal(t, "subnet", resource.Type)
		assert.Equal(t, "subnet-public-1a", resource.ID)
		assert.Equal(t, "public-subnet-1a", resource.Name)
		assert.Equal(t, "vpc-abc123", resource.Metadata.VpcID)
		assert.Equal(t, "10.0.1.0/24", resource.Metadata.CIDRBlock)
		assert.Equal(t, "us-east-1a", resource.Metadata.AvailabilityZone)
		assert.Equal(t, true, resource.Metadata.MapPublicIPOnLaunch)
	})

	t.Run("private subnet", func(t *testing.T) {
		subnet := ec2types.Subnet{
			SubnetId:            aws.String("subnet-private-1a"),
			VpcId:               aws.String("vpc-abc123"),
			CidrBlock:           aws.String("10.0.10.0/24"),
			AvailabilityZone:    aws.String("us-east-1a"),
			MapPublicIpOnLaunch: aws.Bool(false),
		}

		resource := buildSubnetResource(subnet, "us-east-1", "123456789")

		assert.Equal(t, false, resource.Metadata.MapPublicIPOnLaunch)
	})
}

func TestBuildRouteTableResource(t *testing.T) {
	t.Run("main route table with routes", func(t *testing.T) {
		routeTable := ec2types.RouteTable{
			RouteTableId: aws.String("rtb-main"),
			VpcId:        aws.String("vpc-abc123"),
			Associations: []ec2types.RouteTableAssociation{
				{
					Main:     aws.Bool(true),
					SubnetId: aws.String("subnet-111"),
				},
				{
					SubnetId: aws.String("subnet-222"),
				},
			},
			Routes: []ec2types.Route{
				{
					DestinationCidrBlock: aws.String("10.0.0.0/16"),
					GatewayId:            aws.String("local"),
					State:                ec2types.RouteStateActive,
				},
				{
					DestinationCidrBlock: aws.String("0.0.0.0/0"),
					GatewayId:            aws.String("igw-xyz789"),
					State:                ec2types.RouteStateActive,
				},
			},
			Tags: []ec2types.Tag{
				{Key: aws.String("Name"), Value: aws.String("main-route-table")},
			},
		}

		resource := buildRouteTableResource(routeTable, "us-east-1", "123456789")

		assert.Equal(t, "route_table", resource.Type)
		assert.Equal(t, "rtb-main", resource.ID)
		assert.Equal(t, "main-route-table", resource.Name)
		assert.Equal(t, "vpc-abc123", resource.Metadata.VpcID)
		assert.Equal(t, true, resource.Metadata.IsMainRouteTable)
		assert.Equal(t, "subnet-111,subnet-222", resource.Metadata.AssociatedSubnetIDs)
		assert.Contains(t, resource.Metadata.Routes, "10.0.0.0/16 → local (active)")
		assert.Contains(t, resource.Metadata.Routes, "0.0.0.0/0 → igw-xyz789 (active)")
	})

	t.Run("custom route table with NAT", func(t *testing.T) {
		routeTable := ec2types.RouteTable{
			RouteTableId: aws.String("rtb-private"),
			VpcId:        aws.String("vpc-abc123"),
			Routes: []ec2types.Route{
				{
					DestinationCidrBlock: aws.String("0.0.0.0/0"),
					NatGatewayId:         aws.String("nat-abc123"),
					State:                ec2types.RouteStateActive,
				},
			},
		}

		resource := buildRouteTableResource(routeTable, "us-east-1", "123456789")

		assert.Equal(t, false, resource.Metadata.IsMainRouteTable)
		assert.Contains(t, resource.Metadata.Routes, "0.0.0.0/0 → nat-abc123 (active)")
	})

	t.Run("route table with blackhole route", func(t *testing.T) {
		routeTable := ec2types.RouteTable{
			RouteTableId: aws.String("rtb-broken"),
			VpcId:        aws.String("vpc-abc123"),
			Routes: []ec2types.Route{
				{
					DestinationCidrBlock: aws.String("0.0.0.0/0"),
					NatGatewayId:         aws.String("nat-deleted"),
					State:                ec2types.RouteStateBlackhole,
				},
			},
		}

		resource := buildRouteTableResource(routeTable, "us-east-1", "123456789")

		assert.Contains(t, resource.Metadata.Routes, "0.0.0.0/0 → nat-deleted (blackhole)")
	})
}

func TestBuildInternetGatewayResource(t *testing.T) {
	t.Run("attached IGW", func(t *testing.T) {
		igw := ec2types.InternetGateway{
			InternetGatewayId: aws.String("igw-abc123"),
			Attachments: []ec2types.InternetGatewayAttachment{
				{
					VpcId: aws.String("vpc-abc123"),
					State: ec2types.AttachmentStatusAttached,
				},
			},
			Tags: []ec2types.Tag{
				{Key: aws.String("Name"), Value: aws.String("main-igw")},
			},
		}

		resource := buildInternetGatewayResource(igw, "us-east-1", "123456789")

		assert.Equal(t, "internet_gateway", resource.Type)
		assert.Equal(t, "igw-abc123", resource.ID)
		assert.Equal(t, "main-igw", resource.Name)
		assert.Equal(t, "vpc-abc123", resource.Metadata.VpcID)
		assert.Equal(t, "attached", resource.Metadata.AttachmentState)
	})

	t.Run("detached IGW", func(t *testing.T) {
		igw := ec2types.InternetGateway{
			InternetGatewayId: aws.String("igw-orphaned"),
			Attachments:       []ec2types.InternetGatewayAttachment{},
		}

		resource := buildInternetGatewayResource(igw, "us-east-1", "123456789")

		assert.Equal(t, "", resource.Metadata.VpcID)
		assert.Equal(t, "detached", resource.Metadata.AttachmentState)
	})
}

func TestBuildNATGatewayResource(t *testing.T) {
	t.Run("active NAT gateway", func(t *testing.T) {
		natGw := ec2types.NatGateway{
			NatGatewayId: aws.String("nat-abc123"),
			VpcId:        aws.String("vpc-abc123"),
			SubnetId:     aws.String("subnet-public-1a"),
			State:        ec2types.NatGatewayStateAvailable,
			NatGatewayAddresses: []ec2types.NatGatewayAddress{
				{
					AllocationId:       aws.String("eipalloc-xyz789"),
					PublicIp:           aws.String("54.123.45.67"),
					NetworkInterfaceId: aws.String("eni-nat123"),
				},
			},
			Tags: []ec2types.Tag{
				{Key: aws.String("Name"), Value: aws.String("nat-public-1a")},
			},
		}

		resource := buildNATGatewayResource(natGw, "us-east-1", "123456789")

		assert.Equal(t, "nat_gateway", resource.Type)
		assert.Equal(t, "nat-abc123", resource.ID)
		assert.Equal(t, "nat-public-1a", resource.Name)
		assert.Equal(t, "vpc-abc123", resource.Metadata.VpcID)
		assert.Equal(t, "subnet-public-1a", resource.Metadata.SubnetID)
		assert.Equal(t, "available", resource.Status)
		assert.Equal(t, "eipalloc-xyz789", resource.Metadata.ElasticIPAllocationID)
		assert.Equal(t, "54.123.45.67", resource.Metadata.PublicIP)
		assert.Equal(t, "eni-nat123", resource.Metadata.NetworkInterfaceID)
	})

	t.Run("failed NAT gateway", func(t *testing.T) {
		natGw := ec2types.NatGateway{
			NatGatewayId: aws.String("nat-failed"),
			State:        ec2types.NatGatewayStateFailed,
		}

		resource := buildNATGatewayResource(natGw, "us-east-1", "123456789")

		assert.Equal(t, "failed", resource.Status)
	})
}

func TestBuildVPCPeeringConnectionResource(t *testing.T) {
	t.Run("active peering connection", func(t *testing.T) {
		peering := ec2types.VpcPeeringConnection{
			VpcPeeringConnectionId: aws.String("pcx-abc123"),
			RequesterVpcInfo: &ec2types.VpcPeeringConnectionVpcInfo{
				VpcId:     aws.String("vpc-requester"),
				CidrBlock: aws.String("10.0.0.0/16"),
				Region:    aws.String("us-east-1"),
			},
			AccepterVpcInfo: &ec2types.VpcPeeringConnectionVpcInfo{
				VpcId:     aws.String("vpc-accepter"),
				CidrBlock: aws.String("10.1.0.0/16"),
				Region:    aws.String("us-west-2"),
			},
			Status: &ec2types.VpcPeeringConnectionStateReason{
				Code:    ec2types.VpcPeeringConnectionStateReasonCodeActive,
				Message: aws.String("Active"),
			},
			Tags: []ec2types.Tag{
				{Key: aws.String("Name"), Value: aws.String("east-to-west-peering")},
			},
		}

		resource := buildVPCPeeringConnectionResource(peering, "us-east-1", "123456789")

		assert.Equal(t, "vpc_peering_connection", resource.Type)
		assert.Equal(t, "pcx-abc123", resource.ID)
		assert.Equal(t, "east-to-west-peering", resource.Name)
		assert.Equal(t, "active", resource.Status)
		assert.Equal(t, "vpc-requester", resource.Metadata.RequesterVpcID)
		assert.Equal(t, "vpc-accepter", resource.Metadata.AccepterVpcID)
		assert.Equal(t, "10.0.0.0/16", resource.Metadata.RequesterCIDRBlock)
		assert.Equal(t, "10.1.0.0/16", resource.Metadata.AccepterCIDRBlock)
		assert.Equal(t, "us-west-2", resource.Metadata.PeerRegion)
	})

	t.Run("pending peering connection", func(t *testing.T) {
		peering := ec2types.VpcPeeringConnection{
			VpcPeeringConnectionId: aws.String("pcx-pending"),
			Status: &ec2types.VpcPeeringConnectionStateReason{
				Code: ec2types.VpcPeeringConnectionStateReasonCodePendingAcceptance,
			},
		}

		resource := buildVPCPeeringConnectionResource(peering, "us-east-1", "123456789")

		assert.Equal(t, "pending-acceptance", resource.Status)
	})
}

func TestFormatRoutes(t *testing.T) {
	routes := []ec2types.Route{
		{
			DestinationCidrBlock: aws.String("10.0.0.0/16"),
			GatewayId:            aws.String("local"),
			State:                ec2types.RouteStateActive,
		},
		{
			DestinationCidrBlock: aws.String("0.0.0.0/0"),
			GatewayId:            aws.String("igw-abc123"),
			State:                ec2types.RouteStateActive,
		},
		{
			DestinationCidrBlock: aws.String("192.168.0.0/16"),
			NatGatewayId:         aws.String("nat-xyz789"),
			State:                ec2types.RouteStateActive,
		},
		{
			DestinationCidrBlock:   aws.String("172.16.0.0/12"),
			VpcPeeringConnectionId: aws.String("pcx-peering"),
			State:                  ec2types.RouteStateActive,
		},
	}

	result := formatRoutes(routes)

	assert.Contains(t, result, "10.0.0.0/16 → local (active)")
	assert.Contains(t, result, "0.0.0.0/0 → igw-abc123 (active)")
	assert.Contains(t, result, "192.168.0.0/16 → nat-xyz789 (active)")
	assert.Contains(t, result, "172.16.0.0/12 → pcx-peering (active)")
}

func TestExtractAssociatedSubnetIDs(t *testing.T) {
	t.Run("multiple subnets", func(t *testing.T) {
		associations := []ec2types.RouteTableAssociation{
			{SubnetId: aws.String("subnet-111")},
			{SubnetId: aws.String("subnet-222")},
			{SubnetId: aws.String("subnet-333")},
		}

		result := extractAssociatedSubnetIDs(associations)
		assert.Equal(t, "subnet-111,subnet-222,subnet-333", result)
	})

	t.Run("no subnets", func(t *testing.T) {
		result := extractAssociatedSubnetIDs([]ec2types.RouteTableAssociation{})
		assert.Equal(t, "", result)
	})
}

func TestIsMainRouteTable(t *testing.T) {
	t.Run("main route table", func(t *testing.T) {
		associations := []ec2types.RouteTableAssociation{
			{Main: aws.Bool(true)},
		}
		assert.True(t, isMainRouteTable(associations))
	})

	t.Run("custom route table", func(t *testing.T) {
		associations := []ec2types.RouteTableAssociation{
			{Main: aws.Bool(false)},
		}
		assert.False(t, isMainRouteTable(associations))
	})

	t.Run("no associations", func(t *testing.T) {
		assert.False(t, isMainRouteTable([]ec2types.RouteTableAssociation{}))
	})
}
