package aws

import (
	"context"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/aws/aws-sdk-go-v2/service/rds"
	rdstypes "github.com/aws/aws-sdk-go-v2/service/rds/types"
	"github.com/yairfalse/elava/types"
)

// listEBSVolumes discovers EBS volumes
func (p *RealAWSProvider) listEBSVolumes(ctx context.Context, filter types.ResourceFilter) ([]types.Resource, error) {
	var resources []types.Resource
	paginator := ec2.NewDescribeVolumesPaginator(p.ec2Client, &ec2.DescribeVolumesInput{})

	for paginator.HasMorePages() {
		output, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to list EBS volumes: %w", err)
		}

		for _, volume := range output.Volumes {
			resource := p.processEBSVolume(volume)
			resources = append(resources, resource)
		}
	}

	return resources, nil
}

// processEBSVolume processes a single EBS volume
func (p *RealAWSProvider) processEBSVolume(volume ec2types.Volume) types.Resource {
	tags := p.convertEC2TagsToElava(volume.Tags)

	name := aws.ToString(volume.VolumeId)
	if tags.Name != "" {
		name = tags.Name
	}

	// Check if orphaned - unattached volumes
	isOrphaned := volume.State == ec2types.VolumeStateAvailable ||
		len(volume.Attachments) == 0 ||
		p.isResourceOrphaned(tags)

	return types.Resource{
		ID:         aws.ToString(volume.VolumeId),
		Type:       "ebs",
		Provider:   "aws",
		Region:     p.region,
		AccountID:  p.accountID,
		Name:       name,
		Status:     string(volume.State),
		Tags:       tags,
		CreatedAt:  p.safeTimeValue(volume.CreateTime),
		LastSeenAt: time.Now(),
		IsOrphaned: isOrphaned,
		Metadata: map[string]interface{}{
			"size_gb":     aws.ToInt32(volume.Size),
			"volume_type": string(volume.VolumeType),
			"iops":        aws.ToInt32(volume.Iops),
			"encrypted":   aws.ToBool(volume.Encrypted),
			"attached_to": p.getVolumeAttachments(volume.Attachments),
		},
	}
}

// getVolumeAttachments extracts attachment info
func (p *RealAWSProvider) getVolumeAttachments(attachments []ec2types.VolumeAttachment) []string {
	var instances []string
	for _, att := range attachments {
		instances = append(instances, aws.ToString(att.InstanceId))
	}
	return instances
}

// listSnapshots discovers EBS snapshots
func (p *RealAWSProvider) listSnapshots(ctx context.Context, filter types.ResourceFilter) ([]types.Resource, error) {
	var resources []types.Resource

	// Only list snapshots owned by this account
	paginator := ec2.NewDescribeSnapshotsPaginator(p.ec2Client, &ec2.DescribeSnapshotsInput{
		OwnerIds: []string{"self"},
	})

	for paginator.HasMorePages() {
		output, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to list snapshots: %w", err)
		}

		for _, snapshot := range output.Snapshots {
			resource := p.processSnapshot(snapshot)
			resources = append(resources, resource)
		}
	}

	return resources, nil
}

// processSnapshot processes a single EBS snapshot
func (p *RealAWSProvider) processSnapshot(snapshot ec2types.Snapshot) types.Resource {
	tags := p.convertEC2TagsToElava(snapshot.Tags)

	name := aws.ToString(snapshot.SnapshotId)
	if tags.Name != "" {
		name = tags.Name
	}

	// Check age for orphan detection
	isOld := time.Since(p.safeTimeValue(snapshot.StartTime)) > 90*24*time.Hour
	isOrphaned := p.isResourceOrphaned(tags) || isOld

	return types.Resource{
		ID:         aws.ToString(snapshot.SnapshotId),
		Type:       "snapshot",
		Provider:   "aws",
		Region:     p.region,
		AccountID:  p.accountID,
		Name:       name,
		Status:     string(snapshot.State),
		Tags:       tags,
		CreatedAt:  p.safeTimeValue(snapshot.StartTime),
		LastSeenAt: time.Now(),
		IsOrphaned: isOrphaned,
		Metadata: map[string]interface{}{
			"volume_id":   aws.ToString(snapshot.VolumeId),
			"size_gb":     aws.ToInt32(snapshot.VolumeSize),
			"encrypted":   aws.ToBool(snapshot.Encrypted),
			"description": aws.ToString(snapshot.Description),
			"progress":    aws.ToString(snapshot.Progress),
			"is_old":      isOld,
		},
	}
}

// listAMIs discovers Amazon Machine Images
func (p *RealAWSProvider) listAMIs(ctx context.Context, filter types.ResourceFilter) ([]types.Resource, error) {
	var resources []types.Resource

	// Only list AMIs owned by this account
	output, err := p.ec2Client.DescribeImages(ctx, &ec2.DescribeImagesInput{
		Owners: []string{"self"},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to list AMIs: %w", err)
	}

	for _, image := range output.Images {
		resource := p.processAMI(image)
		resources = append(resources, resource)
	}

	return resources, nil
}

// processAMI processes a single AMI
func (p *RealAWSProvider) processAMI(image ec2types.Image) types.Resource {
	tags := p.convertEC2TagsToElava(image.Tags)

	name := aws.ToString(image.Name)
	if name == "" {
		name = aws.ToString(image.ImageId)
	}

	// Check if AMI is old (>90 days) or deprecated
	creationTime := p.parseImageCreationDate(aws.ToString(image.CreationDate))
	isOld := time.Since(creationTime) > 90*24*time.Hour
	isDeprecated := image.DeprecationTime != nil

	isOrphaned := p.isResourceOrphaned(tags) || isOld || isDeprecated

	return types.Resource{
		ID:         aws.ToString(image.ImageId),
		Type:       "ami",
		Provider:   "aws",
		Region:     p.region,
		AccountID:  p.accountID,
		Name:       name,
		Status:     string(image.State),
		Tags:       tags,
		CreatedAt:  creationTime,
		LastSeenAt: time.Now(),
		IsOrphaned: isOrphaned,
		Metadata: map[string]interface{}{
			"architecture":   string(image.Architecture),
			"platform":       string(image.Platform),
			"virtualization": string(image.VirtualizationType),
			"root_device":    string(image.RootDeviceType),
			"is_public":      aws.ToBool(image.Public),
			"is_old":         isOld,
			"is_deprecated":  isDeprecated,
		},
	}
}

// parseImageCreationDate parses AMI creation date
func (p *RealAWSProvider) parseImageCreationDate(dateStr string) time.Time {
	if dateStr == "" {
		return time.Now()
	}
	t, err := time.Parse(time.RFC3339, dateStr)
	if err != nil {
		return time.Now()
	}
	return t
}

// listRDSSnapshots discovers RDS snapshots
func (p *RealAWSProvider) listRDSSnapshots(ctx context.Context, filter types.ResourceFilter) ([]types.Resource, error) {
	var resources []types.Resource

	// List DB snapshots
	dbPaginator := rds.NewDescribeDBSnapshotsPaginator(p.rdsClient, &rds.DescribeDBSnapshotsInput{})
	for dbPaginator.HasMorePages() {
		output, err := dbPaginator.NextPage(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to list RDS DB snapshots: %w", err)
		}

		for _, snapshot := range output.DBSnapshots {
			resource := p.processRDSSnapshot(snapshot)
			resources = append(resources, resource)
		}
	}

	// List DB cluster snapshots
	clusterPaginator := rds.NewDescribeDBClusterSnapshotsPaginator(p.rdsClient, &rds.DescribeDBClusterSnapshotsInput{})
	for clusterPaginator.HasMorePages() {
		output, err := clusterPaginator.NextPage(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to list RDS cluster snapshots: %w", err)
		}

		for _, snapshot := range output.DBClusterSnapshots {
			resource := p.processRDSClusterSnapshot(snapshot)
			resources = append(resources, resource)
		}
	}

	return resources, nil
}

// processRDSSnapshot processes a single RDS DB snapshot
func (p *RealAWSProvider) processRDSSnapshot(snapshot rdstypes.DBSnapshot) types.Resource {
	tags := p.convertRDSTagsToElava(snapshot.TagList)

	// Check age for orphan detection
	isOld := time.Since(p.safeTimeValue(snapshot.SnapshotCreateTime)) > 30*24*time.Hour
	isOrphaned := p.isResourceOrphaned(tags) || isOld

	return types.Resource{
		ID:         aws.ToString(snapshot.DBSnapshotArn),
		Type:       "rds-snapshot",
		Provider:   "aws",
		Region:     p.region,
		AccountID:  p.accountID,
		Name:       aws.ToString(snapshot.DBSnapshotIdentifier),
		Status:     aws.ToString(snapshot.Status),
		Tags:       tags,
		CreatedAt:  p.safeTimeValue(snapshot.SnapshotCreateTime),
		LastSeenAt: time.Now(),
		IsOrphaned: isOrphaned,
		Metadata: map[string]interface{}{
			"engine":         aws.ToString(snapshot.Engine),
			"engine_version": aws.ToString(snapshot.EngineVersion),
			"storage_gb":     aws.ToInt32(snapshot.AllocatedStorage),
			"encrypted":      aws.ToBool(snapshot.Encrypted),
			"is_old":         isOld,
		},
	}
}

// processRDSClusterSnapshot processes a single RDS cluster snapshot
func (p *RealAWSProvider) processRDSClusterSnapshot(snapshot rdstypes.DBClusterSnapshot) types.Resource {
	tags := p.convertRDSTagsToElava(snapshot.TagList)

	// Check age for orphan detection
	isOld := time.Since(p.safeTimeValue(snapshot.SnapshotCreateTime)) > 30*24*time.Hour
	isOrphaned := p.isResourceOrphaned(tags) || isOld

	return types.Resource{
		ID:         aws.ToString(snapshot.DBClusterSnapshotArn),
		Type:       "rds-cluster-snapshot",
		Provider:   "aws",
		Region:     p.region,
		AccountID:  p.accountID,
		Name:       aws.ToString(snapshot.DBClusterSnapshotIdentifier),
		Status:     aws.ToString(snapshot.Status),
		Tags:       tags,
		CreatedAt:  p.safeTimeValue(snapshot.SnapshotCreateTime),
		LastSeenAt: time.Now(),
		IsOrphaned: isOrphaned,
		Metadata: map[string]interface{}{
			"engine":         aws.ToString(snapshot.Engine),
			"engine_version": aws.ToString(snapshot.EngineVersion),
			"storage_gb":     aws.ToInt32(snapshot.AllocatedStorage),
			"encrypted":      aws.ToBool(snapshot.StorageEncrypted),
			"is_old":         isOld,
		},
	}
}
