package aws

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/acm"
	acmtypes "github.com/aws/aws-sdk-go-v2/service/acm/types"
	"github.com/aws/aws-sdk-go-v2/service/apigatewayv2"
	apigwtypes "github.com/aws/aws-sdk-go-v2/service/apigatewayv2/types"
	"github.com/aws/aws-sdk-go-v2/service/autoscaling"
	asgtypes "github.com/aws/aws-sdk-go-v2/service/autoscaling/types"
	"github.com/aws/aws-sdk-go-v2/service/cloudfront"
	cftypes "github.com/aws/aws-sdk-go-v2/service/cloudfront/types"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs"
	cwltypes "github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs/types"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	ddbtypes "github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/aws/aws-sdk-go-v2/service/ecs"
	ecstypes "github.com/aws/aws-sdk-go-v2/service/ecs/types"
	"github.com/aws/aws-sdk-go-v2/service/eks"
	ekstypes "github.com/aws/aws-sdk-go-v2/service/eks/types"
	"github.com/aws/aws-sdk-go-v2/service/elasticache"
	ectypes "github.com/aws/aws-sdk-go-v2/service/elasticache/types"
	"github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2"
	elbtypes "github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2/types"
	"github.com/aws/aws-sdk-go-v2/service/glue"
	gluetypes "github.com/aws/aws-sdk-go-v2/service/glue/types"
	"github.com/aws/aws-sdk-go-v2/service/iam"
	iamtypes "github.com/aws/aws-sdk-go-v2/service/iam/types"
	"github.com/aws/aws-sdk-go-v2/service/kinesis"
	kinesistypes "github.com/aws/aws-sdk-go-v2/service/kinesis/types"
	"github.com/aws/aws-sdk-go-v2/service/lambda"
	lambdatypes "github.com/aws/aws-sdk-go-v2/service/lambda/types"
	"github.com/aws/aws-sdk-go-v2/service/rds"
	rdstypes "github.com/aws/aws-sdk-go-v2/service/rds/types"
	"github.com/aws/aws-sdk-go-v2/service/redshift"
	redshifttypes "github.com/aws/aws-sdk-go-v2/service/redshift/types"
	"github.com/aws/aws-sdk-go-v2/service/route53"
	r53types "github.com/aws/aws-sdk-go-v2/service/route53/types"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/secretsmanager"
	smtypes "github.com/aws/aws-sdk-go-v2/service/secretsmanager/types"
	"github.com/aws/aws-sdk-go-v2/service/sfn"
	sfntypes "github.com/aws/aws-sdk-go-v2/service/sfn/types"
	"github.com/aws/aws-sdk-go-v2/service/sns"
	snstypes "github.com/aws/aws-sdk-go-v2/service/sns/types"
	"github.com/aws/aws-sdk-go-v2/service/sqs"

	"github.com/yairfalse/elava/pkg/resource"
)

// scanEC2 scans EC2 instances.
func (p *Plugin) scanEC2(ctx context.Context) ([]resource.Resource, error) {
	var resources []resource.Resource
	var nextToken *string

	for {
		output, err := p.ec2Client.DescribeInstances(ctx, &ec2.DescribeInstancesInput{NextToken: nextToken})
		if err != nil {
			return nil, fmt.Errorf("describe instances: %w", err)
		}

		for _, reservation := range output.Reservations {
			for _, instance := range reservation.Instances {
				resources = append(resources, p.convertEC2Instance(instance))
			}
		}

		if output.NextToken == nil {
			break
		}
		nextToken = output.NextToken
	}

	return resources, nil
}

func (p *Plugin) convertEC2Instance(instance ec2types.Instance) resource.Resource {
	r := p.newResource(aws.ToString(instance.InstanceId), "ec2", string(instance.State.Name), extractNameTag(instance.Tags))
	for _, tag := range instance.Tags {
		r.Labels[aws.ToString(tag.Key)] = aws.ToString(tag.Value)
	}
	r.Attrs["instance_type"] = string(instance.InstanceType)
	if instance.Placement != nil {
		r.Attrs["az"] = aws.ToString(instance.Placement.AvailabilityZone)
	}
	r.Attrs["vpc_id"] = aws.ToString(instance.VpcId)
	r.Attrs["subnet_id"] = aws.ToString(instance.SubnetId)
	r.Attrs["private_ip"] = aws.ToString(instance.PrivateIpAddress)
	if instance.PublicIpAddress != nil {
		r.Attrs["public_ip"] = aws.ToString(instance.PublicIpAddress)
	}
	return r
}

// scanRDS scans RDS instances.
func (p *Plugin) scanRDS(ctx context.Context) ([]resource.Resource, error) {
	var resources []resource.Resource
	var marker *string

	for {
		output, err := p.rdsClient.DescribeDBInstances(ctx, &rds.DescribeDBInstancesInput{Marker: marker})
		if err != nil {
			return nil, fmt.Errorf("describe db instances: %w", err)
		}

		for _, instance := range output.DBInstances {
			resources = append(resources, p.convertRDSInstance(instance))
		}

		if output.Marker == nil {
			break
		}
		marker = output.Marker
	}

	return resources, nil
}

func (p *Plugin) convertRDSInstance(instance rdstypes.DBInstance) resource.Resource {
	r := p.newResource(aws.ToString(instance.DBInstanceIdentifier), "rds", aws.ToString(instance.DBInstanceStatus), aws.ToString(instance.DBInstanceIdentifier))
	for _, tag := range instance.TagList {
		r.Labels[aws.ToString(tag.Key)] = aws.ToString(tag.Value)
	}
	r.Attrs["engine"] = aws.ToString(instance.Engine)
	r.Attrs["engine_version"] = aws.ToString(instance.EngineVersion)
	r.Attrs["instance_class"] = aws.ToString(instance.DBInstanceClass)
	r.Attrs["storage_gb"] = strconv.Itoa(int(aws.ToInt32(instance.AllocatedStorage)))
	r.Attrs["multi_az"] = strconv.FormatBool(aws.ToBool(instance.MultiAZ))
	if instance.Endpoint != nil {
		r.Attrs["endpoint"] = aws.ToString(instance.Endpoint.Address)
		r.Attrs["port"] = strconv.Itoa(int(aws.ToInt32(instance.Endpoint.Port)))
	}
	return r
}

// scanELB scans Elastic Load Balancers.
func (p *Plugin) scanELB(ctx context.Context) ([]resource.Resource, error) {
	var resources []resource.Resource
	var marker *string

	for {
		output, err := p.elbClient.DescribeLoadBalancers(ctx, &elasticloadbalancingv2.DescribeLoadBalancersInput{Marker: marker})
		if err != nil {
			return nil, fmt.Errorf("describe load balancers: %w", err)
		}

		for _, lb := range output.LoadBalancers {
			resources = append(resources, p.convertELB(lb))
		}

		if output.NextMarker == nil {
			break
		}
		marker = output.NextMarker
	}

	return resources, nil
}

func (p *Plugin) convertELB(lb elbtypes.LoadBalancer) resource.Resource {
	status := "unknown"
	if lb.State != nil {
		status = string(lb.State.Code)
	}
	r := p.newResource(aws.ToString(lb.LoadBalancerArn), "elb", status, aws.ToString(lb.LoadBalancerName))
	r.Attrs["type"] = string(lb.Type)
	r.Attrs["scheme"] = string(lb.Scheme)
	r.Attrs["vpc_id"] = aws.ToString(lb.VpcId)
	r.Attrs["dns_name"] = aws.ToString(lb.DNSName)
	return r
}

// scanS3 scans S3 buckets (no pagination needed).
func (p *Plugin) scanS3(ctx context.Context) ([]resource.Resource, error) {
	output, err := p.s3Client.ListBuckets(ctx, &s3.ListBucketsInput{})
	if err != nil {
		return nil, fmt.Errorf("list buckets: %w", err)
	}

	var resources []resource.Resource
	for _, bucket := range output.Buckets {
		r := p.newResource(aws.ToString(bucket.Name), "s3", "active", aws.ToString(bucket.Name))
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
	var nextToken *string

	for {
		listOutput, err := p.eksClient.ListClusters(ctx, &eks.ListClustersInput{NextToken: nextToken})
		if err != nil {
			return nil, fmt.Errorf("list clusters: %w", err)
		}

		for _, clusterName := range listOutput.Clusters {
			descOutput, err := p.eksClient.DescribeCluster(ctx, &eks.DescribeClusterInput{Name: aws.String(clusterName)})
			if err != nil {
				continue
			}
			resources = append(resources, p.convertEKSCluster(descOutput.Cluster))
		}

		if listOutput.NextToken == nil {
			break
		}
		nextToken = listOutput.NextToken
	}

	return resources, nil
}

func (p *Plugin) convertEKSCluster(cluster *ekstypes.Cluster) resource.Resource {
	r := p.newResource(aws.ToString(cluster.Arn), "eks", string(cluster.Status), aws.ToString(cluster.Name))
	for k, v := range cluster.Tags {
		r.Labels[k] = v
	}
	r.Attrs["version"] = aws.ToString(cluster.Version)
	r.Attrs["endpoint"] = aws.ToString(cluster.Endpoint)
	return r
}

// scanASG scans Auto Scaling Groups.
func (p *Plugin) scanASG(ctx context.Context) ([]resource.Resource, error) {
	var resources []resource.Resource
	var nextToken *string

	for {
		output, err := p.asgClient.DescribeAutoScalingGroups(ctx, &autoscaling.DescribeAutoScalingGroupsInput{NextToken: nextToken})
		if err != nil {
			return nil, fmt.Errorf("describe auto scaling groups: %w", err)
		}

		for _, asg := range output.AutoScalingGroups {
			resources = append(resources, p.convertASG(asg))
		}

		if output.NextToken == nil {
			break
		}
		nextToken = output.NextToken
	}

	return resources, nil
}

func (p *Plugin) convertASG(asg asgtypes.AutoScalingGroup) resource.Resource {
	status := "active"
	if aws.ToInt32(asg.DesiredCapacity) == 0 {
		status = "stopped"
	}
	r := p.newResource(aws.ToString(asg.AutoScalingGroupARN), "asg", status, aws.ToString(asg.AutoScalingGroupName))
	for _, tag := range asg.Tags {
		r.Labels[aws.ToString(tag.Key)] = aws.ToString(tag.Value)
	}
	r.Attrs["min_size"] = strconv.Itoa(int(aws.ToInt32(asg.MinSize)))
	r.Attrs["max_size"] = strconv.Itoa(int(aws.ToInt32(asg.MaxSize)))
	r.Attrs["desired"] = strconv.Itoa(int(aws.ToInt32(asg.DesiredCapacity)))
	r.Attrs["instances"] = strconv.Itoa(len(asg.Instances))
	return r
}

// scanLambda scans Lambda functions.
func (p *Plugin) scanLambda(ctx context.Context) ([]resource.Resource, error) {
	var resources []resource.Resource
	var marker *string

	for {
		output, err := p.lambdaClient.ListFunctions(ctx, &lambda.ListFunctionsInput{Marker: marker})
		if err != nil {
			return nil, fmt.Errorf("list functions: %w", err)
		}

		for _, fn := range output.Functions {
			resources = append(resources, p.convertLambda(fn))
		}

		if output.NextMarker == nil {
			break
		}
		marker = output.NextMarker
	}

	return resources, nil
}

func (p *Plugin) convertLambda(fn lambdatypes.FunctionConfiguration) resource.Resource {
	r := p.newResource(aws.ToString(fn.FunctionArn), "lambda", string(fn.State), aws.ToString(fn.FunctionName))
	r.Attrs["runtime"] = string(fn.Runtime)
	r.Attrs["memory_mb"] = strconv.Itoa(int(aws.ToInt32(fn.MemorySize)))
	r.Attrs["timeout_sec"] = strconv.Itoa(int(aws.ToInt32(fn.Timeout)))
	return r
}

// scanVPC scans VPCs.
func (p *Plugin) scanVPC(ctx context.Context) ([]resource.Resource, error) {
	var resources []resource.Resource
	var nextToken *string

	for {
		output, err := p.ec2Client.DescribeVpcs(ctx, &ec2.DescribeVpcsInput{NextToken: nextToken})
		if err != nil {
			return nil, fmt.Errorf("describe vpcs: %w", err)
		}

		for _, vpc := range output.Vpcs {
			resources = append(resources, p.convertVPC(vpc))
		}

		if output.NextToken == nil {
			break
		}
		nextToken = output.NextToken
	}

	return resources, nil
}

func (p *Plugin) convertVPC(vpc ec2types.Vpc) resource.Resource {
	r := p.newResource(aws.ToString(vpc.VpcId), "vpc", string(vpc.State), extractNameTag(vpc.Tags))
	for _, tag := range vpc.Tags {
		r.Labels[aws.ToString(tag.Key)] = aws.ToString(tag.Value)
	}
	r.Attrs["cidr"] = aws.ToString(vpc.CidrBlock)
	r.Attrs["is_default"] = strconv.FormatBool(aws.ToBool(vpc.IsDefault))
	return r
}

// scanSubnets scans VPC subnets.
func (p *Plugin) scanSubnets(ctx context.Context) ([]resource.Resource, error) {
	var resources []resource.Resource
	var nextToken *string

	for {
		output, err := p.ec2Client.DescribeSubnets(ctx, &ec2.DescribeSubnetsInput{NextToken: nextToken})
		if err != nil {
			return nil, fmt.Errorf("describe subnets: %w", err)
		}

		for _, subnet := range output.Subnets {
			resources = append(resources, p.convertSubnet(subnet))
		}

		if output.NextToken == nil {
			break
		}
		nextToken = output.NextToken
	}

	return resources, nil
}

func (p *Plugin) convertSubnet(subnet ec2types.Subnet) resource.Resource {
	r := p.newResource(aws.ToString(subnet.SubnetId), "subnet", string(subnet.State), extractNameTag(subnet.Tags))
	for _, tag := range subnet.Tags {
		r.Labels[aws.ToString(tag.Key)] = aws.ToString(tag.Value)
	}
	r.Attrs["vpc_id"] = aws.ToString(subnet.VpcId)
	r.Attrs["cidr"] = aws.ToString(subnet.CidrBlock)
	r.Attrs["az"] = aws.ToString(subnet.AvailabilityZone)
	r.Attrs["public"] = strconv.FormatBool(aws.ToBool(subnet.MapPublicIpOnLaunch))
	return r
}

// scanSecurityGroups scans security groups.
func (p *Plugin) scanSecurityGroups(ctx context.Context) ([]resource.Resource, error) {
	var resources []resource.Resource
	var nextToken *string

	for {
		output, err := p.ec2Client.DescribeSecurityGroups(ctx, &ec2.DescribeSecurityGroupsInput{NextToken: nextToken})
		if err != nil {
			return nil, fmt.Errorf("describe security groups: %w", err)
		}

		for _, sg := range output.SecurityGroups {
			resources = append(resources, p.convertSecurityGroup(sg))
		}

		if output.NextToken == nil {
			break
		}
		nextToken = output.NextToken
	}

	return resources, nil
}

func (p *Plugin) convertSecurityGroup(sg ec2types.SecurityGroup) resource.Resource {
	r := p.newResource(aws.ToString(sg.GroupId), "security_group", "active", aws.ToString(sg.GroupName))
	for _, tag := range sg.Tags {
		r.Labels[aws.ToString(tag.Key)] = aws.ToString(tag.Value)
	}
	r.Attrs["vpc_id"] = aws.ToString(sg.VpcId)
	r.Attrs["description"] = aws.ToString(sg.Description)
	r.Attrs["inbound_rules"] = strconv.Itoa(len(sg.IpPermissions))
	r.Attrs["outbound_rules"] = strconv.Itoa(len(sg.IpPermissionsEgress))
	return r
}

// scanDynamoDB scans DynamoDB tables.
func (p *Plugin) scanDynamoDB(ctx context.Context) ([]resource.Resource, error) {
	var resources []resource.Resource
	var lastKey *string

	for {
		output, err := p.dynamodbClient.ListTables(ctx, &dynamodb.ListTablesInput{ExclusiveStartTableName: lastKey})
		if err != nil {
			return nil, fmt.Errorf("list tables: %w", err)
		}

		for _, tableName := range output.TableNames {
			desc, err := p.dynamodbClient.DescribeTable(ctx, &dynamodb.DescribeTableInput{TableName: aws.String(tableName)})
			if err != nil {
				continue
			}
			resources = append(resources, p.convertDynamoDBTable(desc.Table))
		}

		if output.LastEvaluatedTableName == nil {
			break
		}
		lastKey = output.LastEvaluatedTableName
	}

	return resources, nil
}

func (p *Plugin) convertDynamoDBTable(table *ddbtypes.TableDescription) resource.Resource {
	r := p.newResource(aws.ToString(table.TableArn), "dynamodb", string(table.TableStatus), aws.ToString(table.TableName))
	r.Attrs["items"] = strconv.FormatInt(aws.ToInt64(table.ItemCount), 10)
	r.Attrs["size_bytes"] = strconv.FormatInt(aws.ToInt64(table.TableSizeBytes), 10)
	if table.BillingModeSummary != nil {
		r.Attrs["billing_mode"] = string(table.BillingModeSummary.BillingMode)
	}
	return r
}

// scanSQS scans SQS queues.
func (p *Plugin) scanSQS(ctx context.Context) ([]resource.Resource, error) {
	var resources []resource.Resource
	var nextToken *string

	for {
		output, err := p.sqsClient.ListQueues(ctx, &sqs.ListQueuesInput{NextToken: nextToken})
		if err != nil {
			return nil, fmt.Errorf("list queues: %w", err)
		}

		for _, queueURL := range output.QueueUrls {
			r := p.newResource(queueURL, "sqs", "active", extractQueueName(queueURL))
			r.Attrs["url"] = queueURL
			resources = append(resources, r)
		}

		if output.NextToken == nil {
			break
		}
		nextToken = output.NextToken
	}

	return resources, nil
}

// scanEBSVolumes scans EBS volumes.
func (p *Plugin) scanEBSVolumes(ctx context.Context) ([]resource.Resource, error) {
	var resources []resource.Resource
	var nextToken *string

	for {
		output, err := p.ec2Client.DescribeVolumes(ctx, &ec2.DescribeVolumesInput{NextToken: nextToken})
		if err != nil {
			return nil, fmt.Errorf("describe volumes: %w", err)
		}

		for _, vol := range output.Volumes {
			resources = append(resources, p.convertEBSVolume(vol))
		}

		if output.NextToken == nil {
			break
		}
		nextToken = output.NextToken
	}

	return resources, nil
}

func (p *Plugin) convertEBSVolume(vol ec2types.Volume) resource.Resource {
	r := p.newResource(aws.ToString(vol.VolumeId), "ebs", string(vol.State), extractNameTag(vol.Tags))
	for _, tag := range vol.Tags {
		r.Labels[aws.ToString(tag.Key)] = aws.ToString(tag.Value)
	}
	r.Attrs["size_gb"] = strconv.Itoa(int(aws.ToInt32(vol.Size)))
	r.Attrs["type"] = string(vol.VolumeType)
	r.Attrs["az"] = aws.ToString(vol.AvailabilityZone)
	r.Attrs["encrypted"] = strconv.FormatBool(aws.ToBool(vol.Encrypted))
	r.Attrs["attached"] = strconv.FormatBool(len(vol.Attachments) > 0)
	return r
}

// scanElasticIPs scans Elastic IPs (no pagination needed).
func (p *Plugin) scanElasticIPs(ctx context.Context) ([]resource.Resource, error) {
	output, err := p.ec2Client.DescribeAddresses(ctx, &ec2.DescribeAddressesInput{})
	if err != nil {
		return nil, fmt.Errorf("describe addresses: %w", err)
	}

	var resources []resource.Resource
	for _, addr := range output.Addresses {
		resources = append(resources, p.convertElasticIP(addr))
	}

	return resources, nil
}

func (p *Plugin) convertElasticIP(addr ec2types.Address) resource.Resource {
	status := "unattached"
	if addr.AssociationId != nil {
		status = "attached"
	}
	r := p.newResource(aws.ToString(addr.AllocationId), "eip", status, aws.ToString(addr.PublicIp))
	for _, tag := range addr.Tags {
		r.Labels[aws.ToString(tag.Key)] = aws.ToString(tag.Value)
	}
	r.Attrs["public_ip"] = aws.ToString(addr.PublicIp)
	r.Attrs["private_ip"] = aws.ToString(addr.PrivateIpAddress)
	r.Attrs["instance_id"] = aws.ToString(addr.InstanceId)
	return r
}

// scanNATGateways scans NAT Gateways.
func (p *Plugin) scanNATGateways(ctx context.Context) ([]resource.Resource, error) {
	var resources []resource.Resource
	var nextToken *string

	for {
		output, err := p.ec2Client.DescribeNatGateways(ctx, &ec2.DescribeNatGatewaysInput{NextToken: nextToken})
		if err != nil {
			return nil, fmt.Errorf("describe nat gateways: %w", err)
		}

		for _, nat := range output.NatGateways {
			resources = append(resources, p.convertNATGateway(nat))
		}

		if output.NextToken == nil {
			break
		}
		nextToken = output.NextToken
	}

	return resources, nil
}

func (p *Plugin) convertNATGateway(nat ec2types.NatGateway) resource.Resource {
	r := p.newResource(aws.ToString(nat.NatGatewayId), "nat_gateway", string(nat.State), extractNameTag(nat.Tags))
	for _, tag := range nat.Tags {
		r.Labels[aws.ToString(tag.Key)] = aws.ToString(tag.Value)
	}
	r.Attrs["vpc_id"] = aws.ToString(nat.VpcId)
	r.Attrs["subnet_id"] = aws.ToString(nat.SubnetId)
	if len(nat.NatGatewayAddresses) > 0 {
		r.Attrs["public_ip"] = aws.ToString(nat.NatGatewayAddresses[0].PublicIp)
	}
	return r
}

// scanIAMRoles scans IAM roles.
func (p *Plugin) scanIAMRoles(ctx context.Context) ([]resource.Resource, error) {
	var resources []resource.Resource
	var marker *string

	for {
		output, err := p.iamClient.ListRoles(ctx, &iam.ListRolesInput{Marker: marker})
		if err != nil {
			return nil, fmt.Errorf("list roles: %w", err)
		}

		for _, role := range output.Roles {
			resources = append(resources, p.convertIAMRole(role))
		}

		if !output.IsTruncated {
			break
		}
		marker = output.Marker
	}

	return resources, nil
}

func (p *Plugin) convertIAMRole(role iamtypes.Role) resource.Resource {
	r := p.newResource(aws.ToString(role.Arn), "iam_role", "active", aws.ToString(role.RoleName))
	r.Attrs["path"] = aws.ToString(role.Path)
	if role.Description != nil {
		r.Attrs["description"] = aws.ToString(role.Description)
	}
	return r
}

// scanECS scans ECS clusters.
func (p *Plugin) scanECS(ctx context.Context) ([]resource.Resource, error) {
	var clusterArns []string
	var nextToken *string

	for {
		listOutput, err := p.ecsClient.ListClusters(ctx, &ecs.ListClustersInput{NextToken: nextToken})
		if err != nil {
			return nil, fmt.Errorf("list clusters: %w", err)
		}
		clusterArns = append(clusterArns, listOutput.ClusterArns...)

		if listOutput.NextToken == nil {
			break
		}
		nextToken = listOutput.NextToken
	}

	if len(clusterArns) == 0 {
		return nil, nil
	}

	// DescribeClusters has a limit of 100 clusters per call
	var resources []resource.Resource
	const batchSize = 100
	for i := 0; i < len(clusterArns); i += batchSize {
		end := i + batchSize
		if end > len(clusterArns) {
			end = len(clusterArns)
		}
		batch := clusterArns[i:end]

		descOutput, err := p.ecsClient.DescribeClusters(ctx, &ecs.DescribeClustersInput{Clusters: batch})
		if err != nil {
			return nil, fmt.Errorf("describe clusters: %w", err)
		}

		for _, cluster := range descOutput.Clusters {
			resources = append(resources, p.convertECSCluster(cluster))
		}
	}

	return resources, nil
}

func (p *Plugin) convertECSCluster(cluster ecstypes.Cluster) resource.Resource {
	r := p.newResource(aws.ToString(cluster.ClusterArn), "ecs", aws.ToString(cluster.Status), aws.ToString(cluster.ClusterName))
	r.Attrs["services"] = strconv.Itoa(int(cluster.ActiveServicesCount))
	r.Attrs["tasks_running"] = strconv.Itoa(int(cluster.RunningTasksCount))
	r.Attrs["tasks_pending"] = strconv.Itoa(int(cluster.PendingTasksCount))
	return r
}

// scanRoute53 scans Route53 hosted zones.
func (p *Plugin) scanRoute53(ctx context.Context) ([]resource.Resource, error) {
	var resources []resource.Resource
	var marker *string

	for {
		output, err := p.route53Client.ListHostedZones(ctx, &route53.ListHostedZonesInput{Marker: marker})
		if err != nil {
			return nil, fmt.Errorf("list hosted zones: %w", err)
		}

		for _, zone := range output.HostedZones {
			resources = append(resources, p.convertRoute53Zone(zone))
		}

		if !output.IsTruncated {
			break
		}
		marker = output.NextMarker
	}

	return resources, nil
}

func (p *Plugin) convertRoute53Zone(zone r53types.HostedZone) resource.Resource {
	zoneType := "public"
	if zone.Config != nil && zone.Config.PrivateZone {
		zoneType = "private"
	}
	r := p.newResource(aws.ToString(zone.Id), "route53", "active", aws.ToString(zone.Name))
	r.Attrs["type"] = zoneType
	r.Attrs["records"] = strconv.FormatInt(aws.ToInt64(zone.ResourceRecordSetCount), 10)
	return r
}

// scanCloudWatchLogs scans CloudWatch Log Groups.
func (p *Plugin) scanCloudWatchLogs(ctx context.Context) ([]resource.Resource, error) {
	var resources []resource.Resource
	var nextToken *string

	for {
		output, err := p.cwLogsClient.DescribeLogGroups(ctx, &cloudwatchlogs.DescribeLogGroupsInput{NextToken: nextToken})
		if err != nil {
			return nil, fmt.Errorf("describe log groups: %w", err)
		}

		for _, lg := range output.LogGroups {
			resources = append(resources, p.convertLogGroup(lg))
		}

		if output.NextToken == nil {
			break
		}
		nextToken = output.NextToken
	}

	return resources, nil
}

func (p *Plugin) convertLogGroup(lg cwltypes.LogGroup) resource.Resource {
	r := p.newResource(aws.ToString(lg.Arn), "cloudwatch_logs", "active", aws.ToString(lg.LogGroupName))
	r.Attrs["stored_bytes"] = strconv.FormatInt(aws.ToInt64(lg.StoredBytes), 10)
	if lg.RetentionInDays != nil {
		r.Attrs["retention_days"] = strconv.Itoa(int(aws.ToInt32(lg.RetentionInDays)))
	}
	return r
}

// scanSNS scans SNS topics.
func (p *Plugin) scanSNS(ctx context.Context) ([]resource.Resource, error) {
	var resources []resource.Resource
	var nextToken *string

	for {
		output, err := p.snsClient.ListTopics(ctx, &sns.ListTopicsInput{NextToken: nextToken})
		if err != nil {
			return nil, fmt.Errorf("list topics: %w", err)
		}

		for _, topic := range output.Topics {
			resources = append(resources, p.convertSNSTopic(topic))
		}

		if output.NextToken == nil {
			break
		}
		nextToken = output.NextToken
	}

	return resources, nil
}

func (p *Plugin) convertSNSTopic(topic snstypes.Topic) resource.Resource {
	arn := aws.ToString(topic.TopicArn)
	name := extractTopicName(arn)
	return p.newResource(arn, "sns", "active", name)
}

// scanCloudFront scans CloudFront distributions.
func (p *Plugin) scanCloudFront(ctx context.Context) ([]resource.Resource, error) {
	var resources []resource.Resource
	var marker *string

	for {
		output, err := p.cloudfrontClient.ListDistributions(ctx, &cloudfront.ListDistributionsInput{Marker: marker})
		if err != nil {
			return nil, fmt.Errorf("list distributions: %w", err)
		}

		if output.DistributionList != nil {
			for _, dist := range output.DistributionList.Items {
				resources = append(resources, p.convertCloudFrontDistribution(dist))
			}

			if !aws.ToBool(output.DistributionList.IsTruncated) {
				break
			}
			marker = output.DistributionList.NextMarker
		} else {
			break
		}
	}

	return resources, nil
}

func (p *Plugin) convertCloudFrontDistribution(dist cftypes.DistributionSummary) resource.Resource {
	r := p.newResource(aws.ToString(dist.Id), "cloudfront", aws.ToString(dist.Status), aws.ToString(dist.DomainName))
	r.Attrs["domain"] = aws.ToString(dist.DomainName)
	r.Attrs["enabled"] = strconv.FormatBool(aws.ToBool(dist.Enabled))
	if dist.Origins != nil && len(dist.Origins.Items) > 0 {
		r.Attrs["origin"] = aws.ToString(dist.Origins.Items[0].DomainName)
	}
	return r
}

// scanElastiCache scans ElastiCache clusters.
func (p *Plugin) scanElastiCache(ctx context.Context) ([]resource.Resource, error) {
	var resources []resource.Resource
	var marker *string

	for {
		output, err := p.elasticacheClient.DescribeCacheClusters(ctx, &elasticache.DescribeCacheClustersInput{Marker: marker})
		if err != nil {
			return nil, fmt.Errorf("describe cache clusters: %w", err)
		}

		for _, cluster := range output.CacheClusters {
			resources = append(resources, p.convertElastiCacheCluster(cluster))
		}

		if output.Marker == nil {
			break
		}
		marker = output.Marker
	}

	return resources, nil
}

func (p *Plugin) convertElastiCacheCluster(cluster ectypes.CacheCluster) resource.Resource {
	r := p.newResource(aws.ToString(cluster.CacheClusterId), "elasticache", aws.ToString(cluster.CacheClusterStatus), aws.ToString(cluster.CacheClusterId))
	r.Attrs["engine"] = aws.ToString(cluster.Engine)
	r.Attrs["engine_version"] = aws.ToString(cluster.EngineVersion)
	r.Attrs["node_type"] = aws.ToString(cluster.CacheNodeType)
	r.Attrs["num_nodes"] = strconv.Itoa(int(aws.ToInt32(cluster.NumCacheNodes)))
	return r
}

// scanSecretsManager scans Secrets Manager secrets.
func (p *Plugin) scanSecretsManager(ctx context.Context) ([]resource.Resource, error) {
	var resources []resource.Resource
	var nextToken *string

	for {
		output, err := p.secretsmanagerClient.ListSecrets(ctx, &secretsmanager.ListSecretsInput{NextToken: nextToken})
		if err != nil {
			return nil, fmt.Errorf("list secrets: %w", err)
		}

		for _, secret := range output.SecretList {
			resources = append(resources, p.convertSecret(secret))
		}

		if output.NextToken == nil {
			break
		}
		nextToken = output.NextToken
	}

	return resources, nil
}

func (p *Plugin) convertSecret(secret smtypes.SecretListEntry) resource.Resource {
	r := p.newResource(aws.ToString(secret.ARN), "secretsmanager", "active", aws.ToString(secret.Name))
	if secret.Description != nil {
		r.Attrs["description"] = aws.ToString(secret.Description)
	}
	if secret.LastRotatedDate != nil {
		r.Attrs["last_rotated"] = secret.LastRotatedDate.Format("2006-01-02")
	}
	return r
}

// extractTopicName extracts topic name from SNS ARN.
func extractTopicName(arn string) string {
	parts := strings.Split(arn, ":")
	return parts[len(parts)-1]
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

// extractQueueName extracts queue name from SQS URL.
func extractQueueName(queueURL string) string {
	for i := len(queueURL) - 1; i >= 0; i-- {
		if queueURL[i] == '/' {
			return queueURL[i+1:]
		}
	}
	return queueURL
}

// scanACM scans ACM certificates.
func (p *Plugin) scanACM(ctx context.Context) ([]resource.Resource, error) {
	var resources []resource.Resource
	var nextToken *string

	for {
		output, err := p.acmClient.ListCertificates(ctx, &acm.ListCertificatesInput{NextToken: nextToken})
		if err != nil {
			return nil, fmt.Errorf("list certificates: %w", err)
		}

		for _, cert := range output.CertificateSummaryList {
			resources = append(resources, p.convertACMCert(cert))
		}

		if output.NextToken == nil {
			break
		}
		nextToken = output.NextToken
	}

	return resources, nil
}

func (p *Plugin) convertACMCert(cert acmtypes.CertificateSummary) resource.Resource {
	r := p.newResource(aws.ToString(cert.CertificateArn), "acm", string(cert.Status), aws.ToString(cert.DomainName))
	r.Attrs["type"] = string(cert.Type)
	return r
}

// scanAPIGateway scans API Gateway v2 APIs.
func (p *Plugin) scanAPIGateway(ctx context.Context) ([]resource.Resource, error) {
	var resources []resource.Resource
	var nextToken *string

	for {
		output, err := p.apigatewayClient.GetApis(ctx, &apigatewayv2.GetApisInput{NextToken: nextToken})
		if err != nil {
			return nil, fmt.Errorf("get apis: %w", err)
		}

		for _, api := range output.Items {
			resources = append(resources, p.convertAPIGateway(api))
		}

		if output.NextToken == nil {
			break
		}
		nextToken = output.NextToken
	}

	return resources, nil
}

func (p *Plugin) convertAPIGateway(api apigwtypes.Api) resource.Resource {
	r := p.newResource(aws.ToString(api.ApiId), "apigateway", "active", aws.ToString(api.Name))
	r.Attrs["protocol"] = string(api.ProtocolType)
	if api.ApiEndpoint != nil {
		r.Attrs["endpoint"] = aws.ToString(api.ApiEndpoint)
	}
	return r
}

// scanKinesis scans Kinesis streams.
func (p *Plugin) scanKinesis(ctx context.Context) ([]resource.Resource, error) {
	var resources []resource.Resource
	var nextToken *string

	for {
		output, err := p.kinesisClient.ListStreams(ctx, &kinesis.ListStreamsInput{NextToken: nextToken})
		if err != nil {
			return nil, fmt.Errorf("list streams: %w", err)
		}

		for _, stream := range output.StreamSummaries {
			resources = append(resources, p.convertKinesisStream(stream))
		}

		if output.NextToken == nil {
			break
		}
		nextToken = output.NextToken
	}

	return resources, nil
}

func (p *Plugin) convertKinesisStream(stream kinesistypes.StreamSummary) resource.Resource {
	r := p.newResource(aws.ToString(stream.StreamARN), "kinesis", string(stream.StreamStatus), aws.ToString(stream.StreamName))
	return r
}

// scanRedshift scans Redshift clusters.
func (p *Plugin) scanRedshift(ctx context.Context) ([]resource.Resource, error) {
	var resources []resource.Resource
	var marker *string

	for {
		output, err := p.redshiftClient.DescribeClusters(ctx, &redshift.DescribeClustersInput{Marker: marker})
		if err != nil {
			return nil, fmt.Errorf("describe clusters: %w", err)
		}

		for _, cluster := range output.Clusters {
			resources = append(resources, p.convertRedshiftCluster(cluster))
		}

		if output.Marker == nil {
			break
		}
		marker = output.Marker
	}

	return resources, nil
}

func (p *Plugin) convertRedshiftCluster(cluster redshifttypes.Cluster) resource.Resource {
	r := p.newResource(aws.ToString(cluster.ClusterIdentifier), "redshift", aws.ToString(cluster.ClusterStatus), aws.ToString(cluster.ClusterIdentifier))
	r.Attrs["node_type"] = aws.ToString(cluster.NodeType)
	r.Attrs["node_count"] = strconv.Itoa(int(aws.ToInt32(cluster.NumberOfNodes)))
	if cluster.DBName != nil {
		r.Attrs["db_name"] = aws.ToString(cluster.DBName)
	}
	return r
}

// scanStepFunctions scans Step Functions state machines.
func (p *Plugin) scanStepFunctions(ctx context.Context) ([]resource.Resource, error) {
	var resources []resource.Resource
	var nextToken *string

	for {
		output, err := p.sfnClient.ListStateMachines(ctx, &sfn.ListStateMachinesInput{NextToken: nextToken})
		if err != nil {
			return nil, fmt.Errorf("list state machines: %w", err)
		}

		for _, sm := range output.StateMachines {
			resources = append(resources, p.convertStateMachine(sm))
		}

		if output.NextToken == nil {
			break
		}
		nextToken = output.NextToken
	}

	return resources, nil
}

func (p *Plugin) convertStateMachine(sm sfntypes.StateMachineListItem) resource.Resource {
	r := p.newResource(aws.ToString(sm.StateMachineArn), "stepfunctions", "active", aws.ToString(sm.Name))
	r.Attrs["type"] = string(sm.Type)
	return r
}

// scanGlue scans Glue databases.
func (p *Plugin) scanGlue(ctx context.Context) ([]resource.Resource, error) {
	var resources []resource.Resource
	var nextToken *string

	for {
		output, err := p.glueClient.GetDatabases(ctx, &glue.GetDatabasesInput{NextToken: nextToken})
		if err != nil {
			return nil, fmt.Errorf("get databases: %w", err)
		}

		for _, db := range output.DatabaseList {
			resources = append(resources, p.convertGlueDatabase(db))
		}

		if output.NextToken == nil {
			break
		}
		nextToken = output.NextToken
	}

	return resources, nil
}

func (p *Plugin) convertGlueDatabase(db gluetypes.Database) resource.Resource {
	r := p.newResource(aws.ToString(db.Name), "glue_database", "active", aws.ToString(db.Name))
	if db.Description != nil {
		r.Attrs["description"] = aws.ToString(db.Description)
	}
	return r
}
