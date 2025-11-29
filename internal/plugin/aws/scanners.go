package aws

import (
	"context"
	"fmt"
	"strconv"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/autoscaling"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/aws/aws-sdk-go-v2/service/ecs"
	"github.com/aws/aws-sdk-go-v2/service/eks"
	"github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2"
	"github.com/aws/aws-sdk-go-v2/service/iam"
	"github.com/aws/aws-sdk-go-v2/service/lambda"
	"github.com/aws/aws-sdk-go-v2/service/rds"
	"github.com/aws/aws-sdk-go-v2/service/route53"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/sqs"

	"github.com/yairfalse/elava/pkg/resource"
)

// scanEC2 scans EC2 instances.
func (p *Plugin) scanEC2(ctx context.Context) ([]resource.Resource, error) {
	var resources []resource.Resource

	paginator := ec2.NewDescribeInstancesPaginator(p.ec2Client, &ec2.DescribeInstancesInput{})

	for paginator.HasMorePages() {
		output, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, fmt.Errorf("describe instances: %w", err)
		}

		for _, reservation := range output.Reservations {
			for _, instance := range reservation.Instances {
				r := p.newResource(
					aws.ToString(instance.InstanceId),
					"ec2",
					string(instance.State.Name),
					extractNameTag(instance.Tags),
				)

				// Add labels from tags
				for _, tag := range instance.Tags {
					r.Labels[aws.ToString(tag.Key)] = aws.ToString(tag.Value)
				}

				// Add attributes
				r.Attrs["instance_type"] = string(instance.InstanceType)
				r.Attrs["az"] = aws.ToString(instance.Placement.AvailabilityZone)
				r.Attrs["vpc_id"] = aws.ToString(instance.VpcId)
				r.Attrs["subnet_id"] = aws.ToString(instance.SubnetId)
				r.Attrs["private_ip"] = aws.ToString(instance.PrivateIpAddress)
				if instance.PublicIpAddress != nil {
					r.Attrs["public_ip"] = aws.ToString(instance.PublicIpAddress)
				}

				resources = append(resources, r)
			}
		}
	}

	return resources, nil
}

// scanRDS scans RDS instances.
func (p *Plugin) scanRDS(ctx context.Context) ([]resource.Resource, error) {
	var resources []resource.Resource

	paginator := rds.NewDescribeDBInstancesPaginator(p.rdsClient, &rds.DescribeDBInstancesInput{})

	for paginator.HasMorePages() {
		output, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, fmt.Errorf("describe db instances: %w", err)
		}

		for _, instance := range output.DBInstances {
			r := p.newResource(
				aws.ToString(instance.DBInstanceIdentifier),
				"rds",
				aws.ToString(instance.DBInstanceStatus),
				aws.ToString(instance.DBInstanceIdentifier),
			)

			// Add labels from tags
			for _, tag := range instance.TagList {
				r.Labels[aws.ToString(tag.Key)] = aws.ToString(tag.Value)
			}

			// Add attributes
			r.Attrs["engine"] = aws.ToString(instance.Engine)
			r.Attrs["engine_version"] = aws.ToString(instance.EngineVersion)
			r.Attrs["instance_class"] = aws.ToString(instance.DBInstanceClass)
			r.Attrs["storage_gb"] = strconv.Itoa(int(aws.ToInt32(instance.AllocatedStorage)))
			r.Attrs["multi_az"] = strconv.FormatBool(aws.ToBool(instance.MultiAZ))
			if instance.Endpoint != nil {
				r.Attrs["endpoint"] = aws.ToString(instance.Endpoint.Address)
				r.Attrs["port"] = strconv.Itoa(int(aws.ToInt32(instance.Endpoint.Port)))
			}

			resources = append(resources, r)
		}
	}

	return resources, nil
}

// scanELB scans Elastic Load Balancers.
func (p *Plugin) scanELB(ctx context.Context) ([]resource.Resource, error) {
	var resources []resource.Resource

	paginator := elasticloadbalancingv2.NewDescribeLoadBalancersPaginator(p.elbClient, &elasticloadbalancingv2.DescribeLoadBalancersInput{})

	for paginator.HasMorePages() {
		output, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, fmt.Errorf("describe load balancers: %w", err)
		}

		for _, lb := range output.LoadBalancers {
			status := "unknown"
			if lb.State != nil {
				status = string(lb.State.Code)
			}

			r := p.newResource(
				aws.ToString(lb.LoadBalancerArn),
				"elb",
				status,
				aws.ToString(lb.LoadBalancerName),
			)

			// Add attributes
			r.Attrs["type"] = string(lb.Type)
			r.Attrs["scheme"] = string(lb.Scheme)
			r.Attrs["vpc_id"] = aws.ToString(lb.VpcId)
			r.Attrs["dns_name"] = aws.ToString(lb.DNSName)

			resources = append(resources, r)
		}
	}

	return resources, nil
}

// scanS3 scans S3 buckets.
func (p *Plugin) scanS3(ctx context.Context) ([]resource.Resource, error) {
	var resources []resource.Resource

	output, err := p.s3Client.ListBuckets(ctx, &s3.ListBucketsInput{})
	if err != nil {
		return nil, fmt.Errorf("list buckets: %w", err)
	}

	for _, bucket := range output.Buckets {
		r := p.newResource(
			aws.ToString(bucket.Name),
			"s3",
			"active",
			aws.ToString(bucket.Name),
		)

		if bucket.CreationDate != nil {
			r.Attrs["created"] = bucket.CreationDate.Format("2006-01-02")
		}

		resources = append(resources, r)
	}

	return resources, nil
}

// scanEKS scans EKS clusters.
func (p *Plugin) scanEKS(ctx context.Context) ([]resource.Resource, error) {
	var resources []resource.Resource

	listOutput, err := p.eksClient.ListClusters(ctx, &eks.ListClustersInput{})
	if err != nil {
		return nil, fmt.Errorf("list clusters: %w", err)
	}

	for _, clusterName := range listOutput.Clusters {
		descOutput, err := p.eksClient.DescribeCluster(ctx, &eks.DescribeClusterInput{
			Name: aws.String(clusterName),
		})
		if err != nil {
			continue
		}

		cluster := descOutput.Cluster
		r := p.newResource(
			aws.ToString(cluster.Arn),
			"eks",
			string(cluster.Status),
			aws.ToString(cluster.Name),
		)

		// Add labels from tags
		for k, v := range cluster.Tags {
			r.Labels[k] = v
		}

		// Add attributes
		r.Attrs["version"] = aws.ToString(cluster.Version)
		r.Attrs["endpoint"] = aws.ToString(cluster.Endpoint)

		resources = append(resources, r)
	}

	return resources, nil
}

// scanASG scans Auto Scaling Groups.
func (p *Plugin) scanASG(ctx context.Context) ([]resource.Resource, error) {
	var resources []resource.Resource

	paginator := autoscaling.NewDescribeAutoScalingGroupsPaginator(p.asgClient, &autoscaling.DescribeAutoScalingGroupsInput{})

	for paginator.HasMorePages() {
		output, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, fmt.Errorf("describe auto scaling groups: %w", err)
		}

		for _, asg := range output.AutoScalingGroups {
			status := "active"
			if aws.ToInt32(asg.DesiredCapacity) == 0 {
				status = "stopped"
			}

			r := p.newResource(
				aws.ToString(asg.AutoScalingGroupARN),
				"asg",
				status,
				aws.ToString(asg.AutoScalingGroupName),
			)

			// Add labels from tags
			for _, tag := range asg.Tags {
				r.Labels[aws.ToString(tag.Key)] = aws.ToString(tag.Value)
			}

			// Add attributes
			r.Attrs["min_size"] = strconv.Itoa(int(aws.ToInt32(asg.MinSize)))
			r.Attrs["max_size"] = strconv.Itoa(int(aws.ToInt32(asg.MaxSize)))
			r.Attrs["desired"] = strconv.Itoa(int(aws.ToInt32(asg.DesiredCapacity)))
			r.Attrs["instances"] = strconv.Itoa(len(asg.Instances))

			resources = append(resources, r)
		}
	}

	return resources, nil
}

// scanLambda scans Lambda functions.
func (p *Plugin) scanLambda(ctx context.Context) ([]resource.Resource, error) {
	var resources []resource.Resource

	paginator := lambda.NewListFunctionsPaginator(p.lambdaClient, &lambda.ListFunctionsInput{})

	for paginator.HasMorePages() {
		output, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, fmt.Errorf("list functions: %w", err)
		}

		for _, fn := range output.Functions {
			r := p.newResource(
				aws.ToString(fn.FunctionArn),
				"lambda",
				string(fn.State),
				aws.ToString(fn.FunctionName),
			)

			// Add attributes
			r.Attrs["runtime"] = string(fn.Runtime)
			r.Attrs["memory_mb"] = strconv.Itoa(int(aws.ToInt32(fn.MemorySize)))
			r.Attrs["timeout_sec"] = strconv.Itoa(int(aws.ToInt32(fn.Timeout)))

			resources = append(resources, r)
		}
	}

	return resources, nil
}

// extractNameTag extracts the Name tag from EC2 tags.
func extractNameTag(tags []ec2types.Tag) string {
	for _, tag := range tags {
		if aws.ToString(tag.Key) == "Name" {
			return aws.ToString(tag.Value)
		}
	}
	return ""
}

// scanVPC scans VPCs.
func (p *Plugin) scanVPC(ctx context.Context) ([]resource.Resource, error) {
	var resources []resource.Resource

	paginator := ec2.NewDescribeVpcsPaginator(p.ec2Client, &ec2.DescribeVpcsInput{})

	for paginator.HasMorePages() {
		output, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, fmt.Errorf("describe vpcs: %w", err)
		}

		for _, vpc := range output.Vpcs {
			r := p.newResource(
				aws.ToString(vpc.VpcId),
				"vpc",
				string(vpc.State),
				extractNameTag(vpc.Tags),
			)

			for _, tag := range vpc.Tags {
				r.Labels[aws.ToString(tag.Key)] = aws.ToString(tag.Value)
			}

			r.Attrs["cidr"] = aws.ToString(vpc.CidrBlock)
			r.Attrs["is_default"] = strconv.FormatBool(aws.ToBool(vpc.IsDefault))

			resources = append(resources, r)
		}
	}

	return resources, nil
}

// scanSubnets scans VPC subnets.
func (p *Plugin) scanSubnets(ctx context.Context) ([]resource.Resource, error) {
	var resources []resource.Resource

	paginator := ec2.NewDescribeSubnetsPaginator(p.ec2Client, &ec2.DescribeSubnetsInput{})

	for paginator.HasMorePages() {
		output, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, fmt.Errorf("describe subnets: %w", err)
		}

		for _, subnet := range output.Subnets {
			r := p.newResource(
				aws.ToString(subnet.SubnetId),
				"subnet",
				string(subnet.State),
				extractNameTag(subnet.Tags),
			)

			for _, tag := range subnet.Tags {
				r.Labels[aws.ToString(tag.Key)] = aws.ToString(tag.Value)
			}

			r.Attrs["vpc_id"] = aws.ToString(subnet.VpcId)
			r.Attrs["cidr"] = aws.ToString(subnet.CidrBlock)
			r.Attrs["az"] = aws.ToString(subnet.AvailabilityZone)
			r.Attrs["public"] = strconv.FormatBool(aws.ToBool(subnet.MapPublicIpOnLaunch))

			resources = append(resources, r)
		}
	}

	return resources, nil
}

// scanSecurityGroups scans security groups.
func (p *Plugin) scanSecurityGroups(ctx context.Context) ([]resource.Resource, error) {
	var resources []resource.Resource

	paginator := ec2.NewDescribeSecurityGroupsPaginator(p.ec2Client, &ec2.DescribeSecurityGroupsInput{})

	for paginator.HasMorePages() {
		output, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, fmt.Errorf("describe security groups: %w", err)
		}

		for _, sg := range output.SecurityGroups {
			r := p.newResource(
				aws.ToString(sg.GroupId),
				"security_group",
				"active",
				aws.ToString(sg.GroupName),
			)

			for _, tag := range sg.Tags {
				r.Labels[aws.ToString(tag.Key)] = aws.ToString(tag.Value)
			}

			r.Attrs["vpc_id"] = aws.ToString(sg.VpcId)
			r.Attrs["description"] = aws.ToString(sg.Description)
			r.Attrs["inbound_rules"] = strconv.Itoa(len(sg.IpPermissions))
			r.Attrs["outbound_rules"] = strconv.Itoa(len(sg.IpPermissionsEgress))

			resources = append(resources, r)
		}
	}

	return resources, nil
}

// scanDynamoDB scans DynamoDB tables.
func (p *Plugin) scanDynamoDB(ctx context.Context) ([]resource.Resource, error) {
	var resources []resource.Resource

	paginator := dynamodb.NewListTablesPaginator(p.dynamodbClient, &dynamodb.ListTablesInput{})

	for paginator.HasMorePages() {
		output, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, fmt.Errorf("list tables: %w", err)
		}

		for _, tableName := range output.TableNames {
			desc, err := p.dynamodbClient.DescribeTable(ctx, &dynamodb.DescribeTableInput{
				TableName: aws.String(tableName),
			})
			if err != nil {
				continue
			}

			table := desc.Table
			r := p.newResource(
				aws.ToString(table.TableArn),
				"dynamodb",
				string(table.TableStatus),
				tableName,
			)

			r.Attrs["items"] = strconv.FormatInt(aws.ToInt64(table.ItemCount), 10)
			r.Attrs["size_bytes"] = strconv.FormatInt(aws.ToInt64(table.TableSizeBytes), 10)
			if table.BillingModeSummary != nil {
				r.Attrs["billing_mode"] = string(table.BillingModeSummary.BillingMode)
			}

			resources = append(resources, r)
		}
	}

	return resources, nil
}

// scanSQS scans SQS queues.
func (p *Plugin) scanSQS(ctx context.Context) ([]resource.Resource, error) {
	var resources []resource.Resource

	paginator := sqs.NewListQueuesPaginator(p.sqsClient, &sqs.ListQueuesInput{})

	for paginator.HasMorePages() {
		output, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, fmt.Errorf("list queues: %w", err)
		}

		for _, queueURL := range output.QueueUrls {
			r := p.newResource(
				queueURL,
				"sqs",
				"active",
				extractQueueName(queueURL),
			)

			r.Attrs["url"] = queueURL

			resources = append(resources, r)
		}
	}

	return resources, nil
}

// scanEBSVolumes scans EBS volumes.
func (p *Plugin) scanEBSVolumes(ctx context.Context) ([]resource.Resource, error) {
	var resources []resource.Resource

	paginator := ec2.NewDescribeVolumesPaginator(p.ec2Client, &ec2.DescribeVolumesInput{})

	for paginator.HasMorePages() {
		output, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, fmt.Errorf("describe volumes: %w", err)
		}

		for _, vol := range output.Volumes {
			r := p.newResource(
				aws.ToString(vol.VolumeId),
				"ebs",
				string(vol.State),
				extractNameTag(vol.Tags),
			)

			for _, tag := range vol.Tags {
				r.Labels[aws.ToString(tag.Key)] = aws.ToString(tag.Value)
			}

			r.Attrs["size_gb"] = strconv.Itoa(int(aws.ToInt32(vol.Size)))
			r.Attrs["type"] = string(vol.VolumeType)
			r.Attrs["az"] = aws.ToString(vol.AvailabilityZone)
			r.Attrs["encrypted"] = strconv.FormatBool(aws.ToBool(vol.Encrypted))
			r.Attrs["attached"] = strconv.FormatBool(len(vol.Attachments) > 0)

			resources = append(resources, r)
		}
	}

	return resources, nil
}

// scanElasticIPs scans Elastic IPs.
func (p *Plugin) scanElasticIPs(ctx context.Context) ([]resource.Resource, error) {
	var resources []resource.Resource

	output, err := p.ec2Client.DescribeAddresses(ctx, &ec2.DescribeAddressesInput{})
	if err != nil {
		return nil, fmt.Errorf("describe addresses: %w", err)
	}

	for _, addr := range output.Addresses {
		status := "unattached"
		if addr.AssociationId != nil {
			status = "attached"
		}

		r := p.newResource(
			aws.ToString(addr.AllocationId),
			"eip",
			status,
			aws.ToString(addr.PublicIp),
		)

		for _, tag := range addr.Tags {
			r.Labels[aws.ToString(tag.Key)] = aws.ToString(tag.Value)
		}

		r.Attrs["public_ip"] = aws.ToString(addr.PublicIp)
		r.Attrs["private_ip"] = aws.ToString(addr.PrivateIpAddress)
		r.Attrs["instance_id"] = aws.ToString(addr.InstanceId)

		resources = append(resources, r)
	}

	return resources, nil
}

// scanNATGateways scans NAT Gateways.
func (p *Plugin) scanNATGateways(ctx context.Context) ([]resource.Resource, error) {
	var resources []resource.Resource

	paginator := ec2.NewDescribeNatGatewaysPaginator(p.ec2Client, &ec2.DescribeNatGatewaysInput{})

	for paginator.HasMorePages() {
		output, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, fmt.Errorf("describe nat gateways: %w", err)
		}

		for _, nat := range output.NatGateways {
			r := p.newResource(
				aws.ToString(nat.NatGatewayId),
				"nat_gateway",
				string(nat.State),
				extractNameTag(nat.Tags),
			)

			for _, tag := range nat.Tags {
				r.Labels[aws.ToString(tag.Key)] = aws.ToString(tag.Value)
			}

			r.Attrs["vpc_id"] = aws.ToString(nat.VpcId)
			r.Attrs["subnet_id"] = aws.ToString(nat.SubnetId)
			if len(nat.NatGatewayAddresses) > 0 {
				r.Attrs["public_ip"] = aws.ToString(nat.NatGatewayAddresses[0].PublicIp)
			}

			resources = append(resources, r)
		}
	}

	return resources, nil
}

// scanIAMRoles scans IAM roles.
func (p *Plugin) scanIAMRoles(ctx context.Context) ([]resource.Resource, error) {
	var resources []resource.Resource

	paginator := iam.NewListRolesPaginator(p.iamClient, &iam.ListRolesInput{})

	for paginator.HasMorePages() {
		output, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, fmt.Errorf("list roles: %w", err)
		}

		for _, role := range output.Roles {
			r := p.newResource(
				aws.ToString(role.Arn),
				"iam_role",
				"active",
				aws.ToString(role.RoleName),
			)

			r.Attrs["path"] = aws.ToString(role.Path)
			if role.Description != nil {
				r.Attrs["description"] = aws.ToString(role.Description)
			}

			resources = append(resources, r)
		}
	}

	return resources, nil
}

// scanECS scans ECS clusters and services.
func (p *Plugin) scanECS(ctx context.Context) ([]resource.Resource, error) {
	var resources []resource.Resource

	listOutput, err := p.ecsClient.ListClusters(ctx, &ecs.ListClustersInput{})
	if err != nil {
		return nil, fmt.Errorf("list clusters: %w", err)
	}

	if len(listOutput.ClusterArns) == 0 {
		return resources, nil
	}

	descOutput, err := p.ecsClient.DescribeClusters(ctx, &ecs.DescribeClustersInput{
		Clusters: listOutput.ClusterArns,
	})
	if err != nil {
		return nil, fmt.Errorf("describe clusters: %w", err)
	}

	for _, cluster := range descOutput.Clusters {
		r := p.newResource(
			aws.ToString(cluster.ClusterArn),
			"ecs",
			aws.ToString(cluster.Status),
			aws.ToString(cluster.ClusterName),
		)

		r.Attrs["services"] = strconv.Itoa(int(cluster.ActiveServicesCount))
		r.Attrs["tasks_running"] = strconv.Itoa(int(cluster.RunningTasksCount))
		r.Attrs["tasks_pending"] = strconv.Itoa(int(cluster.PendingTasksCount))

		resources = append(resources, r)
	}

	return resources, nil
}

// scanRoute53 scans Route53 hosted zones.
func (p *Plugin) scanRoute53(ctx context.Context) ([]resource.Resource, error) {
	var resources []resource.Resource

	paginator := route53.NewListHostedZonesPaginator(p.route53Client, &route53.ListHostedZonesInput{})

	for paginator.HasMorePages() {
		output, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, fmt.Errorf("list hosted zones: %w", err)
		}

		for _, zone := range output.HostedZones {
			zoneType := "public"
			if zone.Config != nil && zone.Config.PrivateZone {
				zoneType = "private"
			}

			r := p.newResource(
				aws.ToString(zone.Id),
				"route53",
				"active",
				aws.ToString(zone.Name),
			)

			r.Attrs["type"] = zoneType
			r.Attrs["records"] = strconv.FormatInt(aws.ToInt64(zone.ResourceRecordSetCount), 10)

			resources = append(resources, r)
		}
	}

	return resources, nil
}

// scanCloudWatchLogs scans CloudWatch Log Groups.
func (p *Plugin) scanCloudWatchLogs(ctx context.Context) ([]resource.Resource, error) {
	var resources []resource.Resource

	paginator := cloudwatchlogs.NewDescribeLogGroupsPaginator(p.cwLogsClient, &cloudwatchlogs.DescribeLogGroupsInput{})

	for paginator.HasMorePages() {
		output, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, fmt.Errorf("describe log groups: %w", err)
		}

		for _, lg := range output.LogGroups {
			r := p.newResource(
				aws.ToString(lg.Arn),
				"cloudwatch_logs",
				"active",
				aws.ToString(lg.LogGroupName),
			)

			r.Attrs["stored_bytes"] = strconv.FormatInt(aws.ToInt64(lg.StoredBytes), 10)
			if lg.RetentionInDays != nil {
				r.Attrs["retention_days"] = strconv.Itoa(int(aws.ToInt32(lg.RetentionInDays)))
			}

			resources = append(resources, r)
		}
	}

	return resources, nil
}

// extractQueueName extracts queue name from SQS URL.
func extractQueueName(queueURL string) string {
	// URL format: https://sqs.region.amazonaws.com/account/queue-name
	parts := splitLast(queueURL, "/")
	if len(parts) > 0 {
		return parts[len(parts)-1]
	}
	return queueURL
}

func splitLast(s, sep string) []string {
	var result []string
	for s != "" {
		idx := len(s)
		for i := len(s) - 1; i >= 0; i-- {
			if s[i] == sep[0] {
				idx = i
				break
			}
		}
		if idx == len(s) {
			result = append([]string{s}, result...)
			break
		}
		result = append([]string{s[idx+1:]}, result...)
		s = s[:idx]
	}
	return result
}
