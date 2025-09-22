package aws

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/autoscaling"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ecr"
	"github.com/aws/aws-sdk-go-v2/service/ecs"
	ecstypes "github.com/aws/aws-sdk-go-v2/service/ecs/types"
	"github.com/aws/aws-sdk-go-v2/service/eks"
	"github.com/aws/aws-sdk-go-v2/service/iam"
	"github.com/aws/aws-sdk-go-v2/service/kms"
	"github.com/aws/aws-sdk-go-v2/service/lambda"
	"github.com/aws/aws-sdk-go-v2/service/rds"
	"github.com/aws/aws-sdk-go-v2/service/route53"
	"github.com/aws/aws-sdk-go-v2/service/s3"

	"github.com/yairfalse/elava/types"
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
			tags = p.convertTagsToElava(tagsOutput.TagSet)
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
					tags = p.convertTagsToElava(tagsOutput.Tags)
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
			tags := p.convertTagsToElava(volume.Tags)

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
		tags := p.convertTagsToElava(address.Tags)

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
			tags := p.convertTagsToElava(natGw.Tags)

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
			tags := p.convertTagsToElava(snapshot.Tags)

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
		tags := p.convertTagsToElava(image.Tags)

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

// listSecurityGroups discovers Security Groups - accumulate the fastest
func (p *RealAWSProvider) listSecurityGroups(ctx context.Context, filter types.ResourceFilter) ([]types.Resource, error) {
	var resources []types.Resource

	input := &ec2.DescribeSecurityGroupsInput{}
	paginator := ec2.NewDescribeSecurityGroupsPaginator(p.ec2Client, input)

	for paginator.HasMorePages() {
		output, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to describe security groups: %w", err)
		}

		for _, sg := range output.SecurityGroups {
			tags := p.convertTagsToElava(sg.Tags)

			// Check if security group is overly permissive
			hasWideOpen := false
			ruleCount := len(sg.IpPermissions) + len(sg.IpPermissionsEgress)

			for _, rule := range sg.IpPermissions {
				for _, ipRange := range rule.IpRanges {
					if aws.ToString(ipRange.CidrIp) == "0.0.0.0/0" {
						hasWideOpen = true
						break
					}
				}
			}

			// Check if default security group (usually orphaned)
			isDefault := aws.ToString(sg.GroupName) == "default"

			resource := types.Resource{
				ID:         aws.ToString(sg.GroupId),
				Type:       "security_group",
				Provider:   "aws",
				Region:     p.region,
				AccountID:  p.accountID,
				Name:       aws.ToString(sg.GroupName),
				Status:     "active",
				Tags:       tags,
				CreatedAt:  time.Now(), // SGs don't have creation time
				LastSeenAt: time.Now(),
				IsOrphaned: p.isResourceOrphaned(tags) || isDefault,
				Metadata: map[string]interface{}{
					"description":   aws.ToString(sg.Description),
					"vpc_id":        aws.ToString(sg.VpcId),
					"rule_count":    ruleCount,
					"has_wide_open": hasWideOpen,
					"is_default":    isDefault,
					"owner_id":      aws.ToString(sg.OwnerId),
				},
			}

			// High priority if overly permissive
			if hasWideOpen {
				resource.Metadata["security_priority"] = "high"
			}

			resources = append(resources, resource)
		}
	}

	return resources, nil
}

// listEKSClusters scans for EKS clusters
func (p *RealAWSProvider) listEKSClusters(ctx context.Context, filter types.ResourceFilter) ([]types.Resource, error) {
	if filter.Type != "" && filter.Type != "eks" {
		return nil, nil
	}

	client := p.eksClient
	output, err := client.ListClusters(ctx, &eks.ListClustersInput{})
	if err != nil {
		return nil, fmt.Errorf("failed to list EKS clusters: %w", err)
	}

	var resources []types.Resource
	for _, clusterName := range output.Clusters {
		clusterDetails, err := client.DescribeCluster(ctx, &eks.DescribeClusterInput{
			Name: aws.String(clusterName),
		})
		if err != nil {
			continue
		}

		cluster := clusterDetails.Cluster
		tags := p.convertTagsToElava(cluster.Tags)

		resource := types.Resource{
			ID:         aws.ToString(cluster.Name),
			Type:       "eks",
			Region:     p.region,
			Status:     string(cluster.Status),
			Tags:       tags,
			CreatedAt:  aws.ToTime(cluster.CreatedAt),
			LastSeenAt: time.Now(),
			IsOrphaned: p.isResourceOrphaned(tags),
			Metadata: map[string]interface{}{
				"version":          aws.ToString(cluster.Version),
				"endpoint":         aws.ToString(cluster.Endpoint),
				"platform_version": aws.ToString(cluster.PlatformVersion),
				"role_arn":         aws.ToString(cluster.RoleArn),
			},
		}

		resources = append(resources, resource)
	}

	return resources, nil
}

// listECSClusters scans for ECS clusters
func (p *RealAWSProvider) listECSClusters(ctx context.Context, filter types.ResourceFilter) ([]types.Resource, error) {
	if filter.Type != "" && filter.Type != "ecs" {
		return nil, nil
	}

	client := p.ecsClient
	output, err := client.ListClusters(ctx, &ecs.ListClustersInput{})
	if err != nil {
		return nil, fmt.Errorf("failed to list ECS clusters: %w", err)
	}

	if len(output.ClusterArns) == 0 {
		return nil, nil
	}

	details, err := client.DescribeClusters(ctx, &ecs.DescribeClustersInput{
		Clusters: output.ClusterArns,
		Include:  []ecstypes.ClusterField{ecstypes.ClusterFieldTags},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to describe ECS clusters: %w", err)
	}

	var resources []types.Resource
	for _, cluster := range details.Clusters {
		tags := p.convertTagsToElava(cluster.Tags)

		resource := types.Resource{
			ID:         aws.ToString(cluster.ClusterName),
			Type:       "ecs",
			Region:     p.region,
			Status:     aws.ToString(cluster.Status),
			Tags:       tags,
			CreatedAt:  time.Now(),
			LastSeenAt: time.Now(),
			IsOrphaned: p.isResourceOrphaned(tags),
			Metadata: map[string]interface{}{
				"active_services":    cluster.ActiveServicesCount,
				"running_tasks":      cluster.RunningTasksCount,
				"pending_tasks":      cluster.PendingTasksCount,
				"capacity_providers": cluster.CapacityProviders,
			},
		}

		resources = append(resources, resource)
	}

	return resources, nil
}

// listAutoScalingGroups scans for Auto Scaling Groups
func (p *RealAWSProvider) listAutoScalingGroups(ctx context.Context, filter types.ResourceFilter) ([]types.Resource, error) {
	if filter.Type != "" && filter.Type != "asg" {
		return nil, nil
	}

	client := p.asgClient
	paginator := autoscaling.NewDescribeAutoScalingGroupsPaginator(client, &autoscaling.DescribeAutoScalingGroupsInput{})

	var resources []types.Resource
	for paginator.HasMorePages() {
		output, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to list Auto Scaling Groups: %w", err)
		}

		for _, asg := range output.AutoScalingGroups {
			tags := p.convertTagsToElava(asg.Tags)

			resource := types.Resource{
				ID:         aws.ToString(asg.AutoScalingGroupName),
				Type:       "asg",
				Region:     p.region,
				Status:     "active",
				Tags:       tags,
				CreatedAt:  aws.ToTime(asg.CreatedTime),
				LastSeenAt: time.Now(),
				IsOrphaned: p.isResourceOrphaned(tags),
				Metadata: map[string]interface{}{
					"min_size":         asg.MinSize,
					"max_size":         asg.MaxSize,
					"desired_capacity": asg.DesiredCapacity,
					"instances":        len(asg.Instances),
					"launch_template":  asg.LaunchTemplate,
				},
			}

			resources = append(resources, resource)
		}
	}

	return resources, nil
}

// listVPCEndpoints scans for VPC endpoints
func (p *RealAWSProvider) listVPCEndpoints(ctx context.Context, filter types.ResourceFilter) ([]types.Resource, error) {
	if filter.Type != "" && filter.Type != "vpc_endpoint" {
		return nil, nil
	}

	client := p.ec2Client
	output, err := client.DescribeVpcEndpoints(ctx, &ec2.DescribeVpcEndpointsInput{})
	if err != nil {
		return nil, fmt.Errorf("failed to list VPC endpoints: %w", err)
	}

	var resources []types.Resource
	for _, endpoint := range output.VpcEndpoints {
		tags := p.convertTagsToElava(endpoint.Tags)

		resource := types.Resource{
			ID:         aws.ToString(endpoint.VpcEndpointId),
			Type:       "vpc_endpoint",
			Region:     p.region,
			Status:     string(endpoint.State),
			Tags:       tags,
			CreatedAt:  aws.ToTime(endpoint.CreationTimestamp),
			LastSeenAt: time.Now(),
			IsOrphaned: p.isResourceOrphaned(tags),
			Metadata: map[string]interface{}{
				"service_name": aws.ToString(endpoint.ServiceName),
				"vpc_id":       aws.ToString(endpoint.VpcId),
				"type":         string(endpoint.VpcEndpointType),
			},
		}

		resources = append(resources, resource)
	}

	return resources, nil
}

// listRDSSnapshots scans for RDS snapshots
func (p *RealAWSProvider) listRDSSnapshots(ctx context.Context, filter types.ResourceFilter) ([]types.Resource, error) {
	if filter.Type != "" && filter.Type != "rds_snapshot" {
		return nil, nil
	}

	rdsClient := p.rdsClient
	output, err := rdsClient.DescribeDBSnapshots(ctx, &rds.DescribeDBSnapshotsInput{
		SnapshotType: aws.String("manual"),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to list RDS snapshots: %w", err)
	}

	var resources []types.Resource
	for _, snapshot := range output.DBSnapshots {
		// Get snapshot tags
		tagsOutput, err := rdsClient.ListTagsForResource(ctx, &rds.ListTagsForResourceInput{
			ResourceName: snapshot.DBSnapshotArn,
		})
		var tags types.Tags
		if err == nil && tagsOutput.TagList != nil {
			tags = p.convertTagsToElava(tagsOutput.TagList)
		} else {
			tags = types.Tags{}
		}

		resource := types.Resource{
			ID:         aws.ToString(snapshot.DBSnapshotIdentifier),
			Type:       "rds_snapshot",
			Region:     p.region,
			Status:     aws.ToString(snapshot.Status),
			Tags:       tags,
			CreatedAt:  aws.ToTime(snapshot.SnapshotCreateTime),
			LastSeenAt: time.Now(),
			IsOrphaned: p.isResourceOrphaned(tags),
			Metadata: map[string]interface{}{
				"db_instance_identifier": aws.ToString(snapshot.DBInstanceIdentifier),
				"engine":                 aws.ToString(snapshot.Engine),
				"allocated_storage":      snapshot.AllocatedStorage,
				"encrypted":              snapshot.Encrypted,
			},
		}

		resources = append(resources, resource)
	}

	return resources, nil
}

// listIAMRoles scans for IAM roles
func (p *RealAWSProvider) listIAMRoles(ctx context.Context, filter types.ResourceFilter) ([]types.Resource, error) {
	if filter.Type != "" && filter.Type != "iam_role" {
		return nil, nil
	}

	client := p.iamClient
	paginator := iam.NewListRolesPaginator(client, &iam.ListRolesInput{})

	var resources []types.Resource
	for paginator.HasMorePages() {
		output, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to list IAM roles: %w", err)
		}

		for _, role := range output.Roles {
			tagsOutput, err := client.ListRoleTags(ctx, &iam.ListRoleTagsInput{
				RoleName: role.RoleName,
			})
			var tags types.Tags
			if err == nil {
				tags = p.convertTagsToElava(tagsOutput.Tags)
			} else {
				tags = types.Tags{}
			}

			resource := types.Resource{
				ID:         aws.ToString(role.RoleName),
				Type:       "iam_role",
				Region:     "global",
				Status:     "active",
				Tags:       tags,
				CreatedAt:  aws.ToTime(role.CreateDate),
				LastSeenAt: time.Now(),
				IsOrphaned: p.isResourceOrphaned(tags),
				Metadata: map[string]interface{}{
					"arn":                    aws.ToString(role.Arn),
					"path":                   aws.ToString(role.Path),
					"max_session_duration":   role.MaxSessionDuration,
					"assume_role_policy_doc": aws.ToString(role.AssumeRolePolicyDocument),
				},
			}

			resources = append(resources, resource)
		}
	}

	return resources, nil
}

// listNetworkInterfaces scans for network interfaces
func (p *RealAWSProvider) listNetworkInterfaces(ctx context.Context, filter types.ResourceFilter) ([]types.Resource, error) {
	if filter.Type != "" && filter.Type != "network_interface" {
		return nil, nil
	}

	client := p.ec2Client
	output, err := client.DescribeNetworkInterfaces(ctx, &ec2.DescribeNetworkInterfacesInput{})
	if err != nil {
		return nil, fmt.Errorf("failed to list network interfaces: %w", err)
	}

	var resources []types.Resource
	for _, eni := range output.NetworkInterfaces {
		tags := p.convertTagsToElava(eni.TagSet)

		resource := types.Resource{
			ID:         aws.ToString(eni.NetworkInterfaceId),
			Type:       "network_interface",
			Region:     p.region,
			Status:     string(eni.Status),
			Tags:       tags,
			CreatedAt:  time.Now(),
			LastSeenAt: time.Now(),
			IsOrphaned: p.isResourceOrphaned(tags),
			Metadata: map[string]interface{}{
				"vpc_id":         aws.ToString(eni.VpcId),
				"subnet_id":      aws.ToString(eni.SubnetId),
				"private_ip":     aws.ToString(eni.PrivateIpAddress),
				"interface_type": string(eni.InterfaceType),
				"attachment":     eni.Attachment,
			},
		}

		resources = append(resources, resource)
	}

	return resources, nil
}

// listECRRepositories scans for ECR repositories
func (p *RealAWSProvider) listECRRepositories(ctx context.Context, filter types.ResourceFilter) ([]types.Resource, error) {
	if filter.Type != "" && filter.Type != "ecr" {
		return nil, nil
	}

	client := p.ecrClient
	output, err := client.DescribeRepositories(ctx, &ecr.DescribeRepositoriesInput{})
	if err != nil {
		return nil, fmt.Errorf("failed to list ECR repositories: %w", err)
	}

	var resources []types.Resource
	for _, repo := range output.Repositories {
		tagsOutput, err := client.ListTagsForResource(ctx, &ecr.ListTagsForResourceInput{
			ResourceArn: repo.RepositoryArn,
		})
		var tags types.Tags
		if err == nil {
			tags = p.convertTagsToElava(tagsOutput.Tags)
		} else {
			tags = types.Tags{}
		}

		resource := types.Resource{
			ID:         aws.ToString(repo.RepositoryName),
			Type:       "ecr",
			Region:     p.region,
			Status:     "active",
			Tags:       tags,
			CreatedAt:  aws.ToTime(repo.CreatedAt),
			LastSeenAt: time.Now(),
			IsOrphaned: p.isResourceOrphaned(tags),
			Metadata: map[string]interface{}{
				"registry_id":          aws.ToString(repo.RegistryId),
				"repository_uri":       aws.ToString(repo.RepositoryUri),
				"image_tag_mutability": string(repo.ImageTagMutability),
			},
		}

		resources = append(resources, resource)
	}

	return resources, nil
}

// listRoute53HostedZones scans for Route53 hosted zones
func (p *RealAWSProvider) listRoute53HostedZones(ctx context.Context, filter types.ResourceFilter) ([]types.Resource, error) {
	if filter.Type != "" && filter.Type != "route53" {
		return nil, nil
	}

	client := p.route53Client
	output, err := client.ListHostedZones(ctx, &route53.ListHostedZonesInput{})
	if err != nil {
		return nil, fmt.Errorf("failed to list Route53 hosted zones: %w", err)
	}

	var resources []types.Resource
	for _, zone := range output.HostedZones {
		tagsOutput, err := client.ListTagsForResource(ctx, &route53.ListTagsForResourceInput{
			ResourceType: "hostedzone",
			ResourceId:   zone.Id,
		})
		var tags types.Tags
		if err == nil {
			tags = p.convertTagsToElava(tagsOutput.ResourceTagSet.Tags)
		} else {
			tags = types.Tags{}
		}

		resource := types.Resource{
			ID:         aws.ToString(zone.Id),
			Type:       "route53",
			Region:     "global",
			Status:     "active",
			Tags:       tags,
			CreatedAt:  time.Now(),
			LastSeenAt: time.Now(),
			IsOrphaned: p.isResourceOrphaned(tags),
			Metadata: map[string]interface{}{
				"name":         aws.ToString(zone.Name),
				"record_count": zone.ResourceRecordSetCount,
				"private_zone": zone.Config.PrivateZone,
				"comment":      aws.ToString(zone.Config.Comment),
			},
		}

		resources = append(resources, resource)
	}

	return resources, nil
}

// listKMSKeys scans for KMS keys
func (p *RealAWSProvider) listKMSKeys(ctx context.Context, filter types.ResourceFilter) ([]types.Resource, error) {
	if filter.Type != "" && filter.Type != "kms" {
		return nil, nil
	}

	client := p.kmsClient
	output, err := client.ListKeys(ctx, &kms.ListKeysInput{})
	if err != nil {
		return nil, fmt.Errorf("failed to list KMS keys: %w", err)
	}

	var resources []types.Resource
	for _, keyEntry := range output.Keys {
		keyMetadata, err := client.DescribeKey(ctx, &kms.DescribeKeyInput{
			KeyId: keyEntry.KeyId,
		})
		if err != nil {
			continue
		}

		tagsOutput, err := client.ListResourceTags(ctx, &kms.ListResourceTagsInput{
			KeyId: keyEntry.KeyId,
		})
		var tags types.Tags
		if err == nil {
			tags = p.convertTagsToElava(tagsOutput.Tags)
		} else {
			tags = types.Tags{}
		}

		key := keyMetadata.KeyMetadata
		resource := types.Resource{
			ID:         aws.ToString(key.KeyId),
			Type:       "kms",
			Region:     p.region,
			Status:     string(key.KeyState),
			Tags:       tags,
			CreatedAt:  aws.ToTime(key.CreationDate),
			LastSeenAt: time.Now(),
			IsOrphaned: p.isResourceOrphaned(tags),
			Metadata: map[string]interface{}{
				"arn":         aws.ToString(key.Arn),
				"description": aws.ToString(key.Description),
				"usage":       string(key.KeyUsage),
				"enabled":     key.Enabled,
			},
		}

		resources = append(resources, resource)
	}

	return resources, nil
}
