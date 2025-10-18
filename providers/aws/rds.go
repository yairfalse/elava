package aws

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/rds"
	rdstypes "github.com/aws/aws-sdk-go-v2/service/rds/types"

	"github.com/yairfalse/elava/types"
)

// ListRDSInstances scans all RDS instances
func (p *RealAWSProvider) ListRDSInstances(ctx context.Context) ([]types.Resource, error) {
	paginator := rds.NewDescribeDBInstancesPaginator(p.rdsClient, &rds.DescribeDBInstancesInput{})

	var resources []types.Resource
	for paginator.HasMorePages() {
		output, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to describe RDS instances: %w", err)
		}

		for _, instance := range output.DBInstances {
			resource := buildRDSInstanceResource(instance, p.region, p.accountID)
			resources = append(resources, resource)
		}
	}

	return resources, nil
}

// buildRDSInstanceResource converts RDS instance to types.Resource
func buildRDSInstanceResource(instance rdstypes.DBInstance, region, accountID string) types.Resource {
	var endpoint string
	var port int32
	if instance.Endpoint != nil {
		endpoint = aws.ToString(instance.Endpoint.Address)
		port = aws.ToInt32(instance.Endpoint.Port)
	}

	var subnetGroupName string
	if instance.DBSubnetGroup != nil {
		subnetGroupName = aws.ToString(instance.DBSubnetGroup.DBSubnetGroupName)
	}

	return types.Resource{
		ID:         aws.ToString(instance.DBInstanceIdentifier),
		Type:       "rds_instance",
		Provider:   "aws",
		Region:     region,
		AccountID:  accountID,
		Name:       aws.ToString(instance.DBInstanceIdentifier),
		Status:     aws.ToString(instance.DBInstanceStatus),
		Tags:       convertRDSTags(instance.TagList),
		CreatedAt:  aws.ToTime(instance.InstanceCreateTime),
		LastSeenAt: time.Now(),
		Metadata: types.ResourceMetadata{
			Engine:                      aws.ToString(instance.Engine),
			EngineVersion:               aws.ToString(instance.EngineVersion),
			InstanceClass:               aws.ToString(instance.DBInstanceClass),
			AllocatedStorage:            aws.ToInt32(instance.AllocatedStorage),
			Encrypted:                   aws.ToBool(instance.StorageEncrypted),
			MultiAZ:                     aws.ToBool(instance.MultiAZ),
			PubliclyAccessible:          aws.ToBool(instance.PubliclyAccessible),
			Endpoint:                    endpoint,
			Port:                        port,
			AvailabilityZone:            aws.ToString(instance.AvailabilityZone),
			SecondaryAvailabilityZone:   aws.ToString(instance.SecondaryAvailabilityZone),
			DBSubnetGroupName:           subnetGroupName,
			SecurityGroupIDs:            extractVPCSecurityGroupIDs(instance.VpcSecurityGroups),
			ReadReplicaIdentifiers:      extractReadReplicaIdentifiers(instance.ReadReplicaDBInstanceIdentifiers),
			ReadReplicaSourceIdentifier: aws.ToString(instance.ReadReplicaSourceDBInstanceIdentifier),
			BackupRetentionPeriod:       int(aws.ToInt32(instance.BackupRetentionPeriod)),
		},
	}
}

// ListDBSubnetGroups scans DB subnet groups
func (p *RealAWSProvider) ListDBSubnetGroups(ctx context.Context) ([]types.Resource, error) {
	paginator := rds.NewDescribeDBSubnetGroupsPaginator(p.rdsClient, &rds.DescribeDBSubnetGroupsInput{})

	var resources []types.Resource
	for paginator.HasMorePages() {
		output, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to describe DB subnet groups: %w", err)
		}

		for _, sg := range output.DBSubnetGroups {
			resource := buildDBSubnetGroupResource(sg, p.region, p.accountID)
			resources = append(resources, resource)
		}
	}

	return resources, nil
}

// buildDBSubnetGroupResource converts DB subnet group to types.Resource
func buildDBSubnetGroupResource(sg rdstypes.DBSubnetGroup, region, accountID string) types.Resource {
	subnetIDs, azs := extractSubnetDetails(sg.Subnets)

	return types.Resource{
		ID:         aws.ToString(sg.DBSubnetGroupName),
		Type:       "db_subnet_group",
		Provider:   "aws",
		Region:     region,
		AccountID:  accountID,
		Name:       aws.ToString(sg.DBSubnetGroupName),
		Status:     aws.ToString(sg.SubnetGroupStatus),
		Tags:       types.Tags{}, // DB subnet groups don't have tags
		CreatedAt:  time.Now(),   // API doesn't provide creation time
		LastSeenAt: time.Now(),
		Metadata: types.ResourceMetadata{
			VpcID:             aws.ToString(sg.VpcId),
			SubnetIDs:         subnetIDs,
			AvailabilityZones: azs,
			Comment:           aws.ToString(sg.DBSubnetGroupDescription),
		},
	}
}

// extractReadReplicaIdentifiers converts read replica slice to comma-separated string
func extractReadReplicaIdentifiers(identifiers []string) string {
	if len(identifiers) == 0 {
		return ""
	}
	return strings.Join(identifiers, ",")
}

// extractVPCSecurityGroupIDs converts VPC security groups to comma-separated string
func extractVPCSecurityGroupIDs(groups []rdstypes.VpcSecurityGroupMembership) string {
	if len(groups) == 0 {
		return ""
	}

	sgIDs := make([]string, len(groups))
	for i, group := range groups {
		sgIDs[i] = aws.ToString(group.VpcSecurityGroupId)
	}
	return strings.Join(sgIDs, ",")
}

// extractSubnetDetails extracts subnet IDs and availability zones
func extractSubnetDetails(subnets []rdstypes.Subnet) (string, string) {
	if len(subnets) == 0 {
		return "", ""
	}

	subnetIDs := make([]string, len(subnets))
	azs := make([]string, len(subnets))

	for i, subnet := range subnets {
		subnetIDs[i] = aws.ToString(subnet.SubnetIdentifier)
		if subnet.SubnetAvailabilityZone != nil {
			azs[i] = aws.ToString(subnet.SubnetAvailabilityZone.Name)
		}
	}

	return strings.Join(subnetIDs, ","), strings.Join(azs, ",")
}

// convertRDSTags converts RDS tags to Elava tags
func convertRDSTags(tags []rdstypes.Tag) types.Tags {
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
