package aws

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/lambda"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	s3types "github.com/aws/aws-sdk-go-v2/service/s3/types"

	"github.com/yairfalse/ovi/types"
)

// listS3Buckets discovers S3 buckets
func (p *RealAWSProvider) listS3Buckets(ctx context.Context, filter types.ResourceFilter) ([]types.Resource, error) {
	var resources []types.Resource

	// S3 is global, but we list buckets once
	output, err := p.s3Client.ListBuckets(ctx, &s3.ListBucketsInput{})
	if err != nil {
		return nil, fmt.Errorf("failed to list S3 buckets: %w", err)
	}

	for _, bucket := range output.Buckets {
		// Get bucket location to determine region
		locationOutput, err := p.s3Client.GetBucketLocation(ctx, &s3.GetBucketLocationInput{
			Bucket: bucket.Name,
		})

		bucketRegion := "us-east-1"
		if err == nil && locationOutput.LocationConstraint != "" {
			bucketRegion = string(locationOutput.LocationConstraint)
		}

		// Only include buckets from our region
		if bucketRegion != p.region && p.region != "us-east-1" {
			continue
		}

		// Get bucket tags
		tags := types.Tags{}
		tagsOutput, err := p.s3Client.GetBucketTagging(ctx, &s3.GetBucketTaggingInput{
			Bucket: bucket.Name,
		})
		if err == nil && tagsOutput.TagSet != nil {
			tags = p.convertS3Tags(tagsOutput.TagSet)
		}

		// Check if bucket is empty (good indicator of unused)
		objectsOutput, _ := p.s3Client.ListObjectsV2(ctx, &s3.ListObjectsV2Input{
			Bucket:  bucket.Name,
			MaxKeys: aws.Int32(1),
		})
		isEmpty := objectsOutput == nil || len(objectsOutput.Contents) == 0

		resource := types.Resource{
			ID:         aws.ToString(bucket.Name),
			Type:       "s3",
			Provider:   "aws",
			Region:     bucketRegion,
			AccountID:  p.accountID,
			Name:       aws.ToString(bucket.Name),
			Status:     "active",
			Tags:       tags,
			CreatedAt:  p.safeTimeValue(bucket.CreationDate),
			LastSeenAt: time.Now(),
			IsOrphaned: p.isResourceOrphaned(tags) || isEmpty,
			Metadata: map[string]interface{}{
				"is_empty":      isEmpty,
				"creation_date": bucket.CreationDate,
			},
		}

		resources = append(resources, resource)
	}

	return resources, nil
}

// listLambdaFunctions discovers Lambda functions
func (p *RealAWSProvider) listLambdaFunctions(ctx context.Context, filter types.ResourceFilter) ([]types.Resource, error) {
	var resources []types.Resource

	paginator := lambda.NewListFunctionsPaginator(p.lambdaClient, &lambda.ListFunctionsInput{})

	for paginator.HasMorePages() {
		output, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to list Lambda functions: %w", err)
		}

		for _, function := range output.Functions {
			// Get function tags
			tags := types.Tags{}
			if function.FunctionArn != nil {
				tagsOutput, err := p.lambdaClient.ListTags(ctx, &lambda.ListTagsInput{
					Resource: function.FunctionArn,
				})
				if err == nil && tagsOutput.Tags != nil {
					tags = p.convertLambdaTags(tagsOutput.Tags)
				}
			}

			// Parse last modified time
			lastModified := time.Now()
			if function.LastModified != nil {
				if t, err := time.Parse(time.RFC3339, *function.LastModified); err == nil {
					lastModified = t
				}
			}

			// Check if function has been invoked recently
			daysSinceModified := int(time.Since(lastModified).Hours() / 24)
			isStale := daysSinceModified > 30

			resource := types.Resource{
				ID:         aws.ToString(function.FunctionName),
				Type:       "lambda",
				Provider:   "aws",
				Region:     p.region,
				AccountID:  p.accountID,
				Name:       aws.ToString(function.FunctionName),
				Status:     string(function.State),
				Tags:       tags,
				CreatedAt:  lastModified,
				LastSeenAt: time.Now(),
				IsOrphaned: p.isResourceOrphaned(tags) || isStale,
				Metadata: map[string]interface{}{
					"runtime":             string(function.Runtime),
					"memory_size":         aws.ToInt32(function.MemorySize),
					"timeout":             aws.ToInt32(function.Timeout),
					"code_size":           function.CodeSize,
					"last_modified":       function.LastModified,
					"days_since_modified": daysSinceModified,
					"is_stale":            isStale,
				},
			}

			resources = append(resources, resource)
		}
	}

	return resources, nil
}

// listEBSVolumes discovers EBS volumes
func (p *RealAWSProvider) listEBSVolumes(ctx context.Context, filter types.ResourceFilter) ([]types.Resource, error) {
	var resources []types.Resource

	input := &ec2.DescribeVolumesInput{}
	paginator := ec2.NewDescribeVolumesPaginator(p.ec2Client, input)

	for paginator.HasMorePages() {
		output, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to describe EBS volumes: %w", err)
		}

		for _, volume := range output.Volumes {
			tags := p.convertEC2Tags(volume.Tags)

			// Check if volume is attached
			isAttached := len(volume.Attachments) > 0
			status := string(volume.State)
			if !isAttached {
				status = "unattached"
			}

			resource := types.Resource{
				ID:         aws.ToString(volume.VolumeId),
				Type:       "ebs",
				Provider:   "aws",
				Region:     p.region,
				AccountID:  p.accountID,
				Name:       tags.Name,
				Status:     status,
				Tags:       tags,
				CreatedAt:  p.safeTimeValue(volume.CreateTime),
				LastSeenAt: time.Now(),
				IsOrphaned: !isAttached || p.isResourceOrphaned(tags),
				Metadata: map[string]interface{}{
					"volume_type":       string(volume.VolumeType),
					"size_gb":           aws.ToInt32(volume.Size),
					"iops":              aws.ToInt32(volume.Iops),
					"availability_zone": aws.ToString(volume.AvailabilityZone),
					"is_attached":       isAttached,
					"state":             string(volume.State),
				},
			}

			// Add attachment info if attached
			if isAttached && len(volume.Attachments) > 0 {
				resource.Metadata["attached_to"] = aws.ToString(volume.Attachments[0].InstanceId)
				resource.Metadata["device"] = aws.ToString(volume.Attachments[0].Device)
			}

			resources = append(resources, resource)
		}
	}

	return resources, nil
}

// listElasticIPs discovers Elastic IPs
func (p *RealAWSProvider) listElasticIPs(ctx context.Context, filter types.ResourceFilter) ([]types.Resource, error) {
	var resources []types.Resource

	output, err := p.ec2Client.DescribeAddresses(ctx, &ec2.DescribeAddressesInput{})
	if err != nil {
		return nil, fmt.Errorf("failed to describe Elastic IPs: %w", err)
	}

	for _, address := range output.Addresses {
		tags := p.convertEC2Tags(address.Tags)

		// Check if EIP is associated
		isAssociated := address.AssociationId != nil
		status := "allocated"
		if !isAssociated {
			status = "unassociated"
		}

		resource := types.Resource{
			ID:         aws.ToString(address.AllocationId),
			Type:       "elastic_ip",
			Provider:   "aws",
			Region:     p.region,
			AccountID:  p.accountID,
			Name:       tags.Name,
			Status:     status,
			Tags:       tags,
			CreatedAt:  time.Now(), // EIPs don't have creation time
			LastSeenAt: time.Now(),
			IsOrphaned: !isAssociated || p.isResourceOrphaned(tags),
			Metadata: map[string]interface{}{
				"public_ip":     aws.ToString(address.PublicIp),
				"is_associated": isAssociated,
				"domain":        string(address.Domain),
			},
		}

		// Add association info if associated
		if isAssociated {
			if address.InstanceId != nil {
				resource.Metadata["instance_id"] = aws.ToString(address.InstanceId)
			}
			if address.NetworkInterfaceId != nil {
				resource.Metadata["network_interface_id"] = aws.ToString(address.NetworkInterfaceId)
			}
		}

		resources = append(resources, resource)
	}

	return resources, nil
}

// listNATGateways discovers NAT Gateways
func (p *RealAWSProvider) listNATGateways(ctx context.Context, filter types.ResourceFilter) ([]types.Resource, error) {
	var resources []types.Resource

	paginator := ec2.NewDescribeNatGatewaysPaginator(p.ec2Client, &ec2.DescribeNatGatewaysInput{})

	for paginator.HasMorePages() {
		output, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to describe NAT gateways: %w", err)
		}

		for _, natGw := range output.NatGateways {
			tags := p.convertEC2Tags(natGw.Tags)

			resource := types.Resource{
				ID:         aws.ToString(natGw.NatGatewayId),
				Type:       "nat_gateway",
				Provider:   "aws",
				Region:     p.region,
				AccountID:  p.accountID,
				Name:       tags.Name,
				Status:     string(natGw.State),
				Tags:       tags,
				CreatedAt:  p.safeTimeValue(natGw.CreateTime),
				LastSeenAt: time.Now(),
				IsOrphaned: p.isResourceOrphaned(tags),
				Metadata: map[string]interface{}{
					"vpc_id":    aws.ToString(natGw.VpcId),
					"subnet_id": aws.ToString(natGw.SubnetId),
					"state":     string(natGw.State),
				},
			}

			// NAT Gateways are expensive - mark as high priority if orphaned
			if resource.IsOrphaned {
				resource.Metadata["cost_priority"] = "high"
				resource.Metadata["monthly_cost_estimate"] = 45.0 // ~$45/month per NAT Gateway
			}

			resources = append(resources, resource)
		}
	}

	return resources, nil
}

// convertS3Tags converts S3 tags to Ovi tags
func (p *RealAWSProvider) convertS3Tags(s3Tags []s3types.Tag) types.Tags {
	tags := types.Tags{}

	for _, tag := range s3Tags {
		key := aws.ToString(tag.Key)
		value := aws.ToString(tag.Value)

		switch key {
		case "ovi:owner", "Owner", "owner":
			tags.OviOwner = value
		case "ovi:managed":
			tags.OviManaged = value == "true"
		case "ovi:blessed":
			tags.OviBlessed = value == "true"
		case "Environment", "environment", "env":
			tags.Environment = value
		case "Team", "team":
			tags.Team = value
		case "Name", "name":
			tags.Name = value
		case "Project", "project":
			tags.Project = value
		case "CostCenter", "cost-center", "costcenter":
			tags.CostCenter = value
		}
	}

	return tags
}

// convertLambdaTags converts Lambda tags to Ovi tags
func (p *RealAWSProvider) convertLambdaTags(lambdaTags map[string]string) types.Tags {
	tags := types.Tags{}

	for key, value := range lambdaTags {
		switch key {
		case "ovi:owner", "Owner", "owner":
			tags.OviOwner = value
		case "ovi:managed":
			tags.OviManaged = value == "true"
		case "ovi:blessed":
			tags.OviBlessed = value == "true"
		case "Environment", "environment", "env":
			tags.Environment = value
		case "Team", "team":
			tags.Team = value
		case "Name", "name":
			tags.Name = value
		case "Project", "project":
			tags.Project = value
		case "CostCenter", "cost-center", "costcenter":
			tags.CostCenter = value
		}
	}

	return tags
}

// listSnapshots discovers EBS snapshots
func (p *RealAWSProvider) listSnapshots(ctx context.Context, filter types.ResourceFilter) ([]types.Resource, error) {
	var resources []types.Resource

	// Only list snapshots owned by this account
	input := &ec2.DescribeSnapshotsInput{
		OwnerIds: []string{"self"},
	}

	paginator := ec2.NewDescribeSnapshotsPaginator(p.ec2Client, input)

	for paginator.HasMorePages() {
		output, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to describe snapshots: %w", err)
		}

		for _, snapshot := range output.Snapshots {
			tags := p.convertEC2Tags(snapshot.Tags)

			// Calculate age
			age := time.Since(p.safeTimeValue(snapshot.StartTime))
			ageInDays := int(age.Hours() / 24)

			// Check if snapshot is old (potential waste)
			isOld := ageInDays > 30

			// Look for patterns that indicate temporary snapshots
			description := aws.ToString(snapshot.Description)
			isTemp := strings.Contains(strings.ToLower(description), "temp") ||
				strings.Contains(strings.ToLower(description), "test") ||
				strings.Contains(strings.ToLower(description), "backup") ||
				strings.Contains(strings.ToLower(description), "before")

			resource := types.Resource{
				ID:         aws.ToString(snapshot.SnapshotId),
				Type:       "snapshot",
				Provider:   "aws",
				Region:     p.region,
				AccountID:  p.accountID,
				Name:       tags.Name,
				Status:     string(snapshot.State),
				Tags:       tags,
				CreatedAt:  p.safeTimeValue(snapshot.StartTime),
				LastSeenAt: time.Now(),
				IsOrphaned: p.isResourceOrphaned(tags) || (isOld && isTemp),
				Metadata: map[string]interface{}{
					"volume_size_gb": aws.ToInt32(snapshot.VolumeSize),
					"description":    description,
					"age_days":       ageInDays,
					"is_old":         isOld,
					"is_temp":        isTemp,
					"volume_id":      aws.ToString(snapshot.VolumeId),
					"encrypted":      aws.ToBool(snapshot.Encrypted),
				},
			}

			resources = append(resources, resource)
		}
	}

	return resources, nil
}

// listAMIs discovers custom AMIs
func (p *RealAWSProvider) listAMIs(ctx context.Context, filter types.ResourceFilter) ([]types.Resource, error) {
	var resources []types.Resource

	// Only list AMIs owned by this account
	input := &ec2.DescribeImagesInput{
		Owners: []string{"self"},
	}

	output, err := p.ec2Client.DescribeImages(ctx, input)
	if err != nil {
		return nil, fmt.Errorf("failed to describe AMIs: %w", err)
	}

	for _, image := range output.Images {
		tags := p.convertEC2Tags(image.Tags)

		// Parse creation date
		creationTime := time.Now()
		if image.CreationDate != nil {
			if t, err := time.Parse(time.RFC3339, *image.CreationDate); err == nil {
				creationTime = t
			}
		}

		// Calculate age
		age := time.Since(creationTime)
		ageInDays := int(age.Hours() / 24)

		// Check if AMI is old
		isOld := ageInDays > 90 // AMIs older than 3 months

		// Look for patterns in name/description
		name := aws.ToString(image.Name)
		description := aws.ToString(image.Description)
		nameAndDesc := strings.ToLower(name + " " + description)

		isTemp := strings.Contains(nameAndDesc, "temp") ||
			strings.Contains(nameAndDesc, "test") ||
			strings.Contains(nameAndDesc, "old") ||
			strings.Contains(nameAndDesc, "backup")

		// Count associated snapshots (AMIs can have multiple snapshots)
		snapshotCount := len(image.BlockDeviceMappings)

		resource := types.Resource{
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
			IsOrphaned: p.isResourceOrphaned(tags) || (isOld && isTemp),
			Metadata: map[string]interface{}{
				"description":    description,
				"age_days":       ageInDays,
				"is_old":         isOld,
				"is_temp":        isTemp,
				"architecture":   string(image.Architecture),
				"virtualization": string(image.VirtualizationType),
				"snapshot_count": snapshotCount,
				"root_device":    string(image.RootDeviceType),
				"public":         aws.ToBool(image.Public),
			},
		}

		// Add snapshot IDs to metadata
		var snapshotIds []string
		for _, mapping := range image.BlockDeviceMappings {
			if mapping.Ebs != nil && mapping.Ebs.SnapshotId != nil {
				snapshotIds = append(snapshotIds, aws.ToString(mapping.Ebs.SnapshotId))
			}
		}
		if len(snapshotIds) > 0 {
			resource.Metadata["snapshot_ids"] = snapshotIds
		}

		resources = append(resources, resource)
	}

	return resources, nil
}
