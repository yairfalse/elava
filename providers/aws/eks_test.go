package aws

import (
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	ekstypes "github.com/aws/aws-sdk-go-v2/service/eks/types"
	"github.com/stretchr/testify/assert"
)

func TestBuildEKSClusterResource(t *testing.T) {
	t.Run("basic EKS cluster", func(t *testing.T) {
		cluster := ekstypes.Cluster{
			Name:     aws.String("prod-cluster"),
			Version:  aws.String("1.28"),
			Status:   ekstypes.ClusterStatusActive,
			Endpoint: aws.String("https://ABCD1234.gr7.us-east-1.eks.amazonaws.com"),
			ResourcesVpcConfig: &ekstypes.VpcConfigResponse{
				VpcId:            aws.String("vpc-xyz789"),
				SubnetIds:        []string{"subnet-111", "subnet-222", "subnet-333"},
				SecurityGroupIds: []string{"sg-cluster-abc"},
			},
			RoleArn: aws.String("arn:aws:iam::123456789:role/eks-cluster-role"),
			Tags: map[string]string{
				"Environment": "production",
				"Team":        "platform",
			},
		}

		resource := buildEKSClusterResource(cluster, "us-east-1", "123456789")

		assert.Equal(t, "eks_cluster", resource.Type)
		assert.Equal(t, "prod-cluster", resource.ID)
		assert.Equal(t, "prod-cluster", resource.Name)
		assert.Equal(t, "1.28", resource.Metadata.ClusterVersion)
		assert.Equal(t, "vpc-xyz789", resource.Metadata.VpcID)
		assert.Equal(t, "subnet-111,subnet-222,subnet-333", resource.Metadata.SubnetIDs)
		assert.Equal(t, "sg-cluster-abc", resource.Metadata.SecurityGroupIDs)
		assert.Equal(t, "https://ABCD1234.gr7.us-east-1.eks.amazonaws.com", resource.Metadata.Endpoint)
		assert.Equal(t, "arn:aws:iam::123456789:role/eks-cluster-role", resource.Metadata.RoleArn)
	})

	t.Run("EKS cluster with multiple security groups", func(t *testing.T) {
		cluster := ekstypes.Cluster{
			Name:    aws.String("dev-cluster"),
			Version: aws.String("1.27"),
			Status:  ekstypes.ClusterStatusActive,
			ResourcesVpcConfig: &ekstypes.VpcConfigResponse{
				VpcId:            aws.String("vpc-dev"),
				SubnetIds:        []string{"subnet-aaa"},
				SecurityGroupIds: []string{"sg-001", "sg-002", "sg-003"},
			},
		}

		resource := buildEKSClusterResource(cluster, "us-west-2", "987654321")

		assert.Equal(t, "sg-001,sg-002,sg-003", resource.Metadata.SecurityGroupIDs)
	})
}

func TestBuildEKSNodeGroupResource(t *testing.T) {
	t.Run("basic node group", func(t *testing.T) {
		nodeGroup := ekstypes.Nodegroup{
			ClusterName:   aws.String("prod-cluster"),
			NodegroupName: aws.String("prod-workers"),
			Status:        ekstypes.NodegroupStatusActive,
			ScalingConfig: &ekstypes.NodegroupScalingConfig{
				DesiredSize: aws.Int32(5),
				MinSize:     aws.Int32(2),
				MaxSize:     aws.Int32(10),
			},
			InstanceTypes: []string{"t3.large", "t3.xlarge"},
			Subnets:       []string{"subnet-111", "subnet-222"},
			Labels: map[string]string{
				"node-role.kubernetes.io/worker": "true",
				"workload-type":                  "general",
			},
			Tags: map[string]string{
				"Environment": "production",
			},
		}

		resource := buildEKSNodeGroupResource(nodeGroup, "us-east-1", "123456789")

		assert.Equal(t, "eks_node_group", resource.Type)
		assert.Equal(t, "prod-cluster/prod-workers", resource.ID)
		assert.Equal(t, "prod-workers", resource.Name)
		assert.Equal(t, "prod-cluster", resource.Metadata.ClusterName)
		assert.Equal(t, int32(5), resource.Metadata.DesiredCapacity)
		assert.Equal(t, int32(2), resource.Metadata.MinSize)
		assert.Equal(t, int32(10), resource.Metadata.MaxSize)
		assert.Equal(t, "t3.large,t3.xlarge", resource.Metadata.InstanceTypes)
		assert.Equal(t, "subnet-111,subnet-222", resource.Metadata.SubnetIDs)
		assert.Contains(t, resource.Metadata.NodeLabels, "node-role.kubernetes.io/worker")
		assert.Equal(t, "true", resource.Metadata.NodeLabels["node-role.kubernetes.io/worker"])
	})

	t.Run("node group with ASG reference", func(t *testing.T) {
		nodeGroup := ekstypes.Nodegroup{
			ClusterName:   aws.String("prod-cluster"),
			NodegroupName: aws.String("prod-workers"),
			Status:        ekstypes.NodegroupStatusActive,
			Resources: &ekstypes.NodegroupResources{
				AutoScalingGroups: []ekstypes.AutoScalingGroup{
					{Name: aws.String("eks-prod-workers-asg-abc123")},
				},
			},
			ScalingConfig: &ekstypes.NodegroupScalingConfig{
				DesiredSize: aws.Int32(3),
				MinSize:     aws.Int32(1),
				MaxSize:     aws.Int32(5),
			},
		}

		resource := buildEKSNodeGroupResource(nodeGroup, "us-east-1", "123456789")

		// Critical for Tapio → AWS bridge!
		assert.Equal(t, "eks-prod-workers-asg-abc123", resource.Metadata.AutoScalingGroupName)
	})

	t.Run("node group with taints", func(t *testing.T) {
		nodeGroup := ekstypes.Nodegroup{
			ClusterName:   aws.String("prod-cluster"),
			NodegroupName: aws.String("gpu-workers"),
			Status:        ekstypes.NodegroupStatusActive,
			Taints: []ekstypes.Taint{
				{
					Key:    aws.String("nvidia.com/gpu"),
					Value:  aws.String("true"),
					Effect: ekstypes.TaintEffectNoSchedule,
				},
			},
			ScalingConfig: &ekstypes.NodegroupScalingConfig{
				DesiredSize: aws.Int32(2),
			},
		}

		resource := buildEKSNodeGroupResource(nodeGroup, "us-east-1", "123456789")

		assert.Contains(t, resource.Metadata.NodeTaints, "nvidia.com/gpu=true:NoSchedule")
	})
}

func TestConvertEKSTags(t *testing.T) {
	tags := map[string]string{
		"Environment": "production",
		"Team":        "platform",
		"Name":        "prod-cluster",
	}

	result := convertEKSTags(tags)

	assert.Equal(t, "production", result.Environment)
	assert.Equal(t, "platform", result.Team)
	assert.Equal(t, "prod-cluster", result.Name)
}

func TestExtractSubnetIDs(t *testing.T) {
	t.Run("multiple subnets", func(t *testing.T) {
		subnets := []string{"subnet-111", "subnet-222", "subnet-333"}
		result := extractSubnetIDs(subnets)
		assert.Equal(t, "subnet-111,subnet-222,subnet-333", result)
	})

	t.Run("empty subnets", func(t *testing.T) {
		result := extractSubnetIDs([]string{})
		assert.Equal(t, "", result)
	})
}

func TestExtractSecurityGroupIDs(t *testing.T) {
	t.Run("multiple security groups", func(t *testing.T) {
		sgs := []string{"sg-001", "sg-002"}
		result := extractSecurityGroupIDs(sgs)
		assert.Equal(t, "sg-001,sg-002", result)
	})

	t.Run("empty security groups", func(t *testing.T) {
		result := extractSecurityGroupIDs([]string{})
		assert.Equal(t, "", result)
	})
}

func TestExtractInstanceTypes(t *testing.T) {
	t.Run("multiple instance types", func(t *testing.T) {
		types := []string{"t3.large", "t3.xlarge", "t3.2xlarge"}
		result := extractInstanceTypes(types)
		assert.Equal(t, "t3.large,t3.xlarge,t3.2xlarge", result)
	})

	t.Run("single instance type", func(t *testing.T) {
		types := []string{"t3.large"}
		result := extractInstanceTypes(types)
		assert.Equal(t, "t3.large", result)
	})
}

func TestExtractASGName(t *testing.T) {
	t.Run("with ASG", func(t *testing.T) {
		resources := &ekstypes.NodegroupResources{
			AutoScalingGroups: []ekstypes.AutoScalingGroup{
				{Name: aws.String("eks-workers-asg")},
			},
		}
		result := extractASGName(resources)
		assert.Equal(t, "eks-workers-asg", result)
	})

	t.Run("nil resources", func(t *testing.T) {
		result := extractASGName(nil)
		assert.Equal(t, "", result)
	})

	t.Run("empty ASG list", func(t *testing.T) {
		resources := &ekstypes.NodegroupResources{
			AutoScalingGroups: []ekstypes.AutoScalingGroup{},
		}
		result := extractASGName(resources)
		assert.Equal(t, "", result)
	})
}

func TestFormatNodeTaints(t *testing.T) {
	t.Run("multiple taints", func(t *testing.T) {
		taints := []ekstypes.Taint{
			{
				Key:    aws.String("nvidia.com/gpu"),
				Value:  aws.String("true"),
				Effect: ekstypes.TaintEffectNoSchedule,
			},
			{
				Key:    aws.String("dedicated"),
				Value:  aws.String("ml"),
				Effect: ekstypes.TaintEffectNoExecute,
			},
		}
		result := formatNodeTaints(taints)
		assert.Equal(t, "nvidia.com/gpu=true:NoSchedule,dedicated=ml:NoExecute", result)
	})

	t.Run("empty taints", func(t *testing.T) {
		result := formatNodeTaints([]ekstypes.Taint{})
		assert.Equal(t, "", result)
	})
}
