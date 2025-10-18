package aws

import (
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	rdstypes "github.com/aws/aws-sdk-go-v2/service/rds/types"
	"github.com/stretchr/testify/assert"
)

func TestBuildRDSInstanceResource(t *testing.T) {
	t.Run("basic RDS instance", func(t *testing.T) {
		instance := rdstypes.DBInstance{
			DBInstanceIdentifier:  aws.String("prod-db-primary"),
			DBInstanceClass:       aws.String("db.r5.large"),
			Engine:                aws.String("postgres"),
			EngineVersion:         aws.String("14.7"),
			DBInstanceStatus:      aws.String("available"),
			MultiAZ:               aws.Bool(false),
			PubliclyAccessible:    aws.Bool(false),
			AllocatedStorage:      aws.Int32(100),
			StorageEncrypted:      aws.Bool(true),
			BackupRetentionPeriod: aws.Int32(7),
			Endpoint: &rdstypes.Endpoint{
				Address: aws.String("prod-db.abc123.us-east-1.rds.amazonaws.com"),
				Port:    aws.Int32(5432),
			},
			AvailabilityZone: aws.String("us-east-1a"),
			DBSubnetGroup: &rdstypes.DBSubnetGroup{
				DBSubnetGroupName: aws.String("default-vpc-subnet-group"),
			},
			TagList: []rdstypes.Tag{
				{Key: aws.String("Environment"), Value: aws.String("production")},
				{Key: aws.String("Team"), Value: aws.String("platform")},
			},
		}

		resource := buildRDSInstanceResource(instance, "us-east-1", "123456789")

		assert.Equal(t, "rds_instance", resource.Type)
		assert.Equal(t, "prod-db-primary", resource.ID)
		assert.Equal(t, "prod-db-primary", resource.Name)
		assert.Equal(t, "postgres", resource.Metadata.Engine)
		assert.Equal(t, "14.7", resource.Metadata.EngineVersion)
		assert.Equal(t, "db.r5.large", resource.Metadata.InstanceClass)
		assert.Equal(t, false, resource.Metadata.MultiAZ)
		assert.Equal(t, false, resource.Metadata.PubliclyAccessible)
		assert.Equal(t, int32(100), resource.Metadata.AllocatedStorage)
		assert.Equal(t, true, resource.Metadata.Encrypted)
		assert.Equal(t, 7, resource.Metadata.BackupRetentionPeriod)
		assert.Equal(t, "prod-db.abc123.us-east-1.rds.amazonaws.com", resource.Metadata.Endpoint)
		assert.Equal(t, int32(5432), resource.Metadata.Port)
		assert.Equal(t, "us-east-1a", resource.Metadata.AvailabilityZone)
		assert.Equal(t, "default-vpc-subnet-group", resource.Metadata.DBSubnetGroupName)
	})

	t.Run("multi-AZ RDS instance", func(t *testing.T) {
		instance := rdstypes.DBInstance{
			DBInstanceIdentifier:      aws.String("prod-db-ha"),
			DBInstanceClass:           aws.String("db.r5.xlarge"),
			Engine:                    aws.String("mysql"),
			EngineVersion:             aws.String("8.0.32"),
			MultiAZ:                   aws.Bool(true),
			AvailabilityZone:          aws.String("us-east-1a"),
			SecondaryAvailabilityZone: aws.String("us-east-1b"),
		}

		resource := buildRDSInstanceResource(instance, "us-east-1", "123456789")

		assert.Equal(t, true, resource.Metadata.MultiAZ)
		assert.Equal(t, "us-east-1a", resource.Metadata.AvailabilityZone)
		assert.Equal(t, "us-east-1b", resource.Metadata.SecondaryAvailabilityZone)
	})

	t.Run("RDS primary with read replicas", func(t *testing.T) {
		instance := rdstypes.DBInstance{
			DBInstanceIdentifier: aws.String("prod-db-primary"),
			Engine:               aws.String("postgres"),
			ReadReplicaDBInstanceIdentifiers: []string{
				"prod-db-replica-1",
				"prod-db-replica-2",
				"prod-db-replica-3",
			},
		}

		resource := buildRDSInstanceResource(instance, "us-east-1", "123456789")

		expected := "prod-db-replica-1,prod-db-replica-2,prod-db-replica-3"
		assert.Equal(t, expected, resource.Metadata.ReadReplicaIdentifiers)
	})

	t.Run("RDS read replica", func(t *testing.T) {
		instance := rdstypes.DBInstance{
			DBInstanceIdentifier:                  aws.String("prod-db-replica-1"),
			Engine:                                aws.String("postgres"),
			ReadReplicaSourceDBInstanceIdentifier: aws.String("prod-db-primary"),
		}

		resource := buildRDSInstanceResource(instance, "us-east-1", "123456789")

		assert.Equal(t, "prod-db-primary", resource.Metadata.ReadReplicaSourceIdentifier)
	})

	t.Run("RDS with VPC security groups", func(t *testing.T) {
		instance := rdstypes.DBInstance{
			DBInstanceIdentifier: aws.String("prod-db"),
			Engine:               aws.String("postgres"),
			VpcSecurityGroups: []rdstypes.VpcSecurityGroupMembership{
				{VpcSecurityGroupId: aws.String("sg-001")},
				{VpcSecurityGroupId: aws.String("sg-002")},
				{VpcSecurityGroupId: aws.String("sg-003")},
			},
		}

		resource := buildRDSInstanceResource(instance, "us-east-1", "123456789")

		assert.Equal(t, "sg-001,sg-002,sg-003", resource.Metadata.SecurityGroupIDs)
	})
}

func TestBuildDBSubnetGroupResource(t *testing.T) {
	t.Run("basic subnet group", func(t *testing.T) {
		subnetGroup := rdstypes.DBSubnetGroup{
			DBSubnetGroupName:        aws.String("default-vpc-subnet-group"),
			DBSubnetGroupDescription: aws.String("Default subnet group for VPC"),
			VpcId:                    aws.String("vpc-abc123"),
			Subnets: []rdstypes.Subnet{
				{
					SubnetIdentifier: aws.String("subnet-111"),
					SubnetAvailabilityZone: &rdstypes.AvailabilityZone{
						Name: aws.String("us-east-1a"),
					},
				},
				{
					SubnetIdentifier: aws.String("subnet-222"),
					SubnetAvailabilityZone: &rdstypes.AvailabilityZone{
						Name: aws.String("us-east-1b"),
					},
				},
				{
					SubnetIdentifier: aws.String("subnet-333"),
					SubnetAvailabilityZone: &rdstypes.AvailabilityZone{
						Name: aws.String("us-east-1c"),
					},
				},
			},
		}

		resource := buildDBSubnetGroupResource(subnetGroup, "us-east-1", "123456789")

		assert.Equal(t, "db_subnet_group", resource.Type)
		assert.Equal(t, "default-vpc-subnet-group", resource.ID)
		assert.Equal(t, "default-vpc-subnet-group", resource.Name)
		assert.Equal(t, "vpc-abc123", resource.Metadata.VpcID)
		assert.Equal(t, "subnet-111,subnet-222,subnet-333", resource.Metadata.SubnetIDs)
		assert.Equal(t, "us-east-1a,us-east-1b,us-east-1c", resource.Metadata.AvailabilityZones)
		assert.Equal(t, "Default subnet group for VPC", resource.Metadata.Comment)
	})

	t.Run("empty subnet group", func(t *testing.T) {
		subnetGroup := rdstypes.DBSubnetGroup{
			DBSubnetGroupName: aws.String("empty-group"),
			Subnets:           []rdstypes.Subnet{},
		}

		resource := buildDBSubnetGroupResource(subnetGroup, "us-east-1", "123456789")

		assert.Equal(t, "", resource.Metadata.SubnetIDs)
		assert.Equal(t, "", resource.Metadata.AvailabilityZones)
	})
}

func TestExtractReadReplicaIdentifiers(t *testing.T) {
	t.Run("multiple read replicas", func(t *testing.T) {
		identifiers := []string{"replica-1", "replica-2", "replica-3"}
		result := extractReadReplicaIdentifiers(identifiers)
		assert.Equal(t, "replica-1,replica-2,replica-3", result)
	})

	t.Run("no read replicas", func(t *testing.T) {
		result := extractReadReplicaIdentifiers([]string{})
		assert.Equal(t, "", result)
	})
}

func TestExtractVPCSecurityGroupIDs(t *testing.T) {
	t.Run("multiple security groups", func(t *testing.T) {
		groups := []rdstypes.VpcSecurityGroupMembership{
			{VpcSecurityGroupId: aws.String("sg-001")},
			{VpcSecurityGroupId: aws.String("sg-002")},
		}
		result := extractVPCSecurityGroupIDs(groups)
		assert.Equal(t, "sg-001,sg-002", result)
	})

	t.Run("empty security groups", func(t *testing.T) {
		result := extractVPCSecurityGroupIDs([]rdstypes.VpcSecurityGroupMembership{})
		assert.Equal(t, "", result)
	})
}

func TestExtractSubnetDetails(t *testing.T) {
	t.Run("multiple subnets", func(t *testing.T) {
		subnets := []rdstypes.Subnet{
			{
				SubnetIdentifier: aws.String("subnet-111"),
				SubnetAvailabilityZone: &rdstypes.AvailabilityZone{
					Name: aws.String("us-east-1a"),
				},
			},
			{
				SubnetIdentifier: aws.String("subnet-222"),
				SubnetAvailabilityZone: &rdstypes.AvailabilityZone{
					Name: aws.String("us-east-1b"),
				},
			},
		}

		subnetIDs, azs := extractSubnetDetails(subnets)
		assert.Equal(t, "subnet-111,subnet-222", subnetIDs)
		assert.Equal(t, "us-east-1a,us-east-1b", azs)
	})

	t.Run("empty subnets", func(t *testing.T) {
		subnetIDs, azs := extractSubnetDetails([]rdstypes.Subnet{})
		assert.Equal(t, "", subnetIDs)
		assert.Equal(t, "", azs)
	})
}

func TestConvertRDSTags(t *testing.T) {
	tags := []rdstypes.Tag{
		{Key: aws.String("Environment"), Value: aws.String("production")},
		{Key: aws.String("Team"), Value: aws.String("platform")},
		{Key: aws.String("Name"), Value: aws.String("prod-db")},
	}

	result := convertRDSTags(tags)

	assert.Equal(t, "production", result.Environment)
	assert.Equal(t, "platform", result.Team)
	assert.Equal(t, "prod-db", result.Name)
}
