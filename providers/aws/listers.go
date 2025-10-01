package aws

import (
	"context"

	"github.com/yairfalse/elava/types"
)

// EC2Lister lists EC2 instances
type EC2Lister struct{}

func (l *EC2Lister) Name() string     { return "EC2 instances" }
func (l *EC2Lister) IsCritical() bool { return true }
func (l *EC2Lister) List(ctx context.Context, p *RealAWSProvider, filter types.ResourceFilter) ([]types.Resource, error) {
	return p.listEC2Instances(ctx, filter)
}

// RDSLister lists RDS instances
type RDSLister struct{}

func (l *RDSLister) Name() string     { return "RDS instances" }
func (l *RDSLister) IsCritical() bool { return true }
func (l *RDSLister) List(ctx context.Context, p *RealAWSProvider, filter types.ResourceFilter) ([]types.Resource, error) {
	return p.listRDSInstances(ctx, filter)
}

// ELBLister lists load balancers
type ELBLister struct{}

func (l *ELBLister) Name() string     { return "load balancers" }
func (l *ELBLister) IsCritical() bool { return true }
func (l *ELBLister) List(ctx context.Context, p *RealAWSProvider, filter types.ResourceFilter) ([]types.Resource, error) {
	return p.listLoadBalancers(ctx, filter)
}

// S3Lister lists S3 buckets
type S3Lister struct{}

func (l *S3Lister) Name() string     { return "S3 buckets" }
func (l *S3Lister) IsCritical() bool { return false }
func (l *S3Lister) List(ctx context.Context, p *RealAWSProvider, filter types.ResourceFilter) ([]types.Resource, error) {
	return p.listS3Buckets(ctx, filter)
}

// LambdaLister lists Lambda functions
type LambdaLister struct{}

func (l *LambdaLister) Name() string     { return "Lambda functions" }
func (l *LambdaLister) IsCritical() bool { return false }
func (l *LambdaLister) List(ctx context.Context, p *RealAWSProvider, filter types.ResourceFilter) ([]types.Resource, error) {
	return p.listLambdaFunctions(ctx, filter)
}

// EBSVolumeLister lists EBS volumes
type EBSVolumeLister struct{}

func (l *EBSVolumeLister) Name() string     { return "EBS volumes" }
func (l *EBSVolumeLister) IsCritical() bool { return false }
func (l *EBSVolumeLister) List(ctx context.Context, p *RealAWSProvider, filter types.ResourceFilter) ([]types.Resource, error) {
	return p.listEBSVolumes(ctx, filter)
}

// ElasticIPLister lists Elastic IPs
type ElasticIPLister struct{}

func (l *ElasticIPLister) Name() string     { return "Elastic IPs" }
func (l *ElasticIPLister) IsCritical() bool { return false }
func (l *ElasticIPLister) List(ctx context.Context, p *RealAWSProvider, filter types.ResourceFilter) ([]types.Resource, error) {
	return p.listElasticIPs(ctx, filter)
}

// NATGatewayLister lists NAT Gateways
type NATGatewayLister struct{}

func (l *NATGatewayLister) Name() string     { return "NAT Gateways" }
func (l *NATGatewayLister) IsCritical() bool { return false }
func (l *NATGatewayLister) List(ctx context.Context, p *RealAWSProvider, filter types.ResourceFilter) ([]types.Resource, error) {
	return p.listNATGateways(ctx, filter)
}

// SnapshotLister lists EBS snapshots
type SnapshotLister struct{}

func (l *SnapshotLister) Name() string     { return "EBS snapshots" }
func (l *SnapshotLister) IsCritical() bool { return false }
func (l *SnapshotLister) List(ctx context.Context, p *RealAWSProvider, filter types.ResourceFilter) ([]types.Resource, error) {
	return p.listSnapshots(ctx, filter)
}

// AMILister lists AMIs
type AMILister struct{}

func (l *AMILister) Name() string     { return "AMIs" }
func (l *AMILister) IsCritical() bool { return false }
func (l *AMILister) List(ctx context.Context, p *RealAWSProvider, filter types.ResourceFilter) ([]types.Resource, error) {
	return p.listAMIs(ctx, filter)
}

// CloudWatchLogsLister lists CloudWatch log groups
type CloudWatchLogsLister struct{}

func (l *CloudWatchLogsLister) Name() string     { return "CloudWatch log groups" }
func (l *CloudWatchLogsLister) IsCritical() bool { return false }
func (l *CloudWatchLogsLister) List(ctx context.Context, p *RealAWSProvider, filter types.ResourceFilter) ([]types.Resource, error) {
	return p.listCloudWatchLogs(ctx, filter)
}

// SecurityGroupLister lists security groups
type SecurityGroupLister struct{}

func (l *SecurityGroupLister) Name() string     { return "security groups" }
func (l *SecurityGroupLister) IsCritical() bool { return false }
func (l *SecurityGroupLister) List(ctx context.Context, p *RealAWSProvider, filter types.ResourceFilter) ([]types.Resource, error) {
	return p.listSecurityGroups(ctx, filter)
}

// EKSLister lists EKS clusters
type EKSLister struct{}

func (l *EKSLister) Name() string     { return "EKS clusters" }
func (l *EKSLister) IsCritical() bool { return false }
func (l *EKSLister) List(ctx context.Context, p *RealAWSProvider, filter types.ResourceFilter) ([]types.Resource, error) {
	return p.listEKSClusters(ctx, filter)
}

// ECSLister lists ECS clusters
type ECSLister struct{}

func (l *ECSLister) Name() string     { return "ECS clusters" }
func (l *ECSLister) IsCritical() bool { return false }
func (l *ECSLister) List(ctx context.Context, p *RealAWSProvider, filter types.ResourceFilter) ([]types.Resource, error) {
	return p.listECSClusters(ctx, filter)
}

// AutoScalingGroupLister lists Auto Scaling Groups
type AutoScalingGroupLister struct{}

func (l *AutoScalingGroupLister) Name() string     { return "Auto Scaling Groups" }
func (l *AutoScalingGroupLister) IsCritical() bool { return false }
func (l *AutoScalingGroupLister) List(ctx context.Context, p *RealAWSProvider, filter types.ResourceFilter) ([]types.Resource, error) {
	return p.listAutoScalingGroups(ctx, filter)
}

// VPCEndpointLister lists VPC endpoints
type VPCEndpointLister struct{}

func (l *VPCEndpointLister) Name() string     { return "VPC endpoints" }
func (l *VPCEndpointLister) IsCritical() bool { return false }
func (l *VPCEndpointLister) List(ctx context.Context, p *RealAWSProvider, filter types.ResourceFilter) ([]types.Resource, error) {
	return p.listVPCEndpoints(ctx, filter)
}

// RDSSnapshotLister lists RDS snapshots
type RDSSnapshotLister struct{}

func (l *RDSSnapshotLister) Name() string     { return "RDS snapshots" }
func (l *RDSSnapshotLister) IsCritical() bool { return false }
func (l *RDSSnapshotLister) List(ctx context.Context, p *RealAWSProvider, filter types.ResourceFilter) ([]types.Resource, error) {
	return p.listRDSSnapshots(ctx, filter)
}

// IAMRoleLister lists IAM roles
type IAMRoleLister struct{}

func (l *IAMRoleLister) Name() string     { return "IAM roles" }
func (l *IAMRoleLister) IsCritical() bool { return false }
func (l *IAMRoleLister) List(ctx context.Context, p *RealAWSProvider, filter types.ResourceFilter) ([]types.Resource, error) {
	return p.listIAMRoles(ctx, filter)
}

// NetworkInterfaceLister lists network interfaces
type NetworkInterfaceLister struct{}

func (l *NetworkInterfaceLister) Name() string     { return "network interfaces" }
func (l *NetworkInterfaceLister) IsCritical() bool { return false }
func (l *NetworkInterfaceLister) List(ctx context.Context, p *RealAWSProvider, filter types.ResourceFilter) ([]types.Resource, error) {
	return p.listNetworkInterfaces(ctx, filter)
}

// ECRLister lists ECR repositories
type ECRLister struct{}

func (l *ECRLister) Name() string     { return "ECR repositories" }
func (l *ECRLister) IsCritical() bool { return false }
func (l *ECRLister) List(ctx context.Context, p *RealAWSProvider, filter types.ResourceFilter) ([]types.Resource, error) {
	return p.listECRRepositories(ctx, filter)
}

// Route53Lister lists Route53 hosted zones
type Route53Lister struct{}

func (l *Route53Lister) Name() string     { return "Route53 hosted zones" }
func (l *Route53Lister) IsCritical() bool { return false }
func (l *Route53Lister) List(ctx context.Context, p *RealAWSProvider, filter types.ResourceFilter) ([]types.Resource, error) {
	return p.listRoute53HostedZones(ctx, filter)
}

// KMSLister lists KMS keys
type KMSLister struct{}

func (l *KMSLister) Name() string     { return "KMS keys" }
func (l *KMSLister) IsCritical() bool { return false }
func (l *KMSLister) List(ctx context.Context, p *RealAWSProvider, filter types.ResourceFilter) ([]types.Resource, error) {
	return p.listKMSKeys(ctx, filter)
}

// AuroraLister lists Aurora clusters
type AuroraLister struct{}

func (l *AuroraLister) Name() string     { return "Aurora clusters" }
func (l *AuroraLister) IsCritical() bool { return false }
func (l *AuroraLister) List(ctx context.Context, p *RealAWSProvider, filter types.ResourceFilter) ([]types.Resource, error) {
	return p.listAuroraClusters(ctx, filter)
}

// RedshiftLister lists Redshift clusters
type RedshiftLister struct{}

func (l *RedshiftLister) Name() string     { return "Redshift clusters" }
func (l *RedshiftLister) IsCritical() bool { return false }
func (l *RedshiftLister) List(ctx context.Context, p *RealAWSProvider, filter types.ResourceFilter) ([]types.Resource, error) {
	return p.listRedshiftClusters(ctx, filter)
}

// RedshiftSnapshotLister lists Redshift snapshots
type RedshiftSnapshotLister struct{}

func (l *RedshiftSnapshotLister) Name() string     { return "Redshift snapshots" }
func (l *RedshiftSnapshotLister) IsCritical() bool { return false }
func (l *RedshiftSnapshotLister) List(ctx context.Context, p *RealAWSProvider, filter types.ResourceFilter) ([]types.Resource, error) {
	return p.listRedshiftSnapshots(ctx, filter)
}

// MemoryDBLister lists MemoryDB clusters
type MemoryDBLister struct{}

func (l *MemoryDBLister) Name() string     { return "MemoryDB clusters" }
func (l *MemoryDBLister) IsCritical() bool { return false }
func (l *MemoryDBLister) List(ctx context.Context, p *RealAWSProvider, filter types.ResourceFilter) ([]types.Resource, error) {
	return p.listMemoryDBClusters(ctx, filter)
}

// DynamoDBLister lists DynamoDB tables
type DynamoDBLister struct{}

func (l *DynamoDBLister) Name() string     { return "DynamoDB tables" }
func (l *DynamoDBLister) IsCritical() bool { return false }
func (l *DynamoDBLister) List(ctx context.Context, p *RealAWSProvider, filter types.ResourceFilter) ([]types.Resource, error) {
	return p.listDynamoDBTables(ctx, filter)
}

// DynamoDBBackupLister lists DynamoDB backups
type DynamoDBBackupLister struct{}

func (l *DynamoDBBackupLister) Name() string     { return "DynamoDB backups" }
func (l *DynamoDBBackupLister) IsCritical() bool { return false }
func (l *DynamoDBBackupLister) List(ctx context.Context, p *RealAWSProvider, filter types.ResourceFilter) ([]types.Resource, error) {
	return p.listDynamoDBBackups(ctx, filter)
}

// SQSLister lists SQS queues
type SQSLister struct{}

func (l *SQSLister) Name() string     { return "SQS queues" }
func (l *SQSLister) IsCritical() bool { return false }
func (l *SQSLister) List(ctx context.Context, p *RealAWSProvider, filter types.ResourceFilter) ([]types.Resource, error) {
	return p.listSQSQueues(ctx, filter)
}
