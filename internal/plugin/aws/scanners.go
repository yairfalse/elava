package aws

import (
	"context"
	"fmt"
	"strconv"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/autoscaling"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/aws/aws-sdk-go-v2/service/eks"
	"github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2"
	"github.com/aws/aws-sdk-go-v2/service/lambda"
	"github.com/aws/aws-sdk-go-v2/service/rds"
	"github.com/aws/aws-sdk-go-v2/service/s3"

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
