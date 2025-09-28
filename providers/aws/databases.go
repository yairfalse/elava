package aws

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	dynamodbtypes "github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/aws/aws-sdk-go-v2/service/memorydb"
	memorydbtypes "github.com/aws/aws-sdk-go-v2/service/memorydb/types"
	"github.com/aws/aws-sdk-go-v2/service/rds"
	"github.com/aws/aws-sdk-go-v2/service/redshift"
	redshifttypes "github.com/aws/aws-sdk-go-v2/service/redshift/types"

	"github.com/yairfalse/elava/types"
)

// listAuroraClusters discovers Aurora DB clusters
func (p *RealAWSProvider) listAuroraClusters(ctx context.Context, filter types.ResourceFilter) ([]types.Resource, error) {
	var resources []types.Resource

	paginator := rds.NewDescribeDBClustersPaginator(p.rdsClient, &rds.DescribeDBClustersInput{})

	for paginator.HasMorePages() {
		output, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to describe Aurora clusters: %w", err)
		}

		for _, cluster := range output.DBClusters {
			resource := p.convertAuroraCluster(cluster)
			resources = append(resources, resource)
		}
	}

	return resources, nil
}

// convertAuroraCluster converts an Aurora cluster to Resource
func (p *RealAWSProvider) convertAuroraCluster(cluster rdstypes.DBCluster) types.Resource {
	tags := p.convertTagsToElava(cluster.TagList)

	instanceCount := len(cluster.DBClusterMembers)
	isServerless := strings.Contains(aws.ToString(cluster.EngineMode), "serverless")
	isIdle := cluster.AllocatedStorage == nil || aws.ToInt32(cluster.AllocatedStorage) == 0

	return types.Resource{
		ID:         aws.ToString(cluster.DBClusterIdentifier),
		Type:       "aurora",
		Provider:   "aws",
		Region:     p.region,
		AccountID:  p.accountID,
		Name:       aws.ToString(cluster.DBClusterIdentifier),
		Status:     aws.ToString(cluster.Status),
		Tags:       tags,
		CreatedAt:  p.safeTimeValue(cluster.ClusterCreateTime),
		LastSeenAt: time.Now(),
		IsOrphaned: p.isResourceOrphaned(tags) || isIdle,
		Metadata: types.ResourceMetadata{
			Engine:           aws.ToString(cluster.Engine),
			EngineVersion:    aws.ToString(cluster.EngineVersion),
			NodeCount:        instanceCount,
			MultiAZ:          aws.ToBool(cluster.MultiAZ),
			BackupWindow:     aws.ToString(cluster.PreferredBackupWindow),
			ClusterID:        aws.ToString(cluster.DBClusterIdentifier),
			IsIdle:           isIdle,
			State:            aws.ToString(cluster.Status),
			AllocatedStorage: aws.ToInt32(cluster.AllocatedStorage),
		},
	}
}

// listRedshiftClusters discovers Redshift data warehouse clusters
func (p *RealAWSProvider) listRedshiftClusters(ctx context.Context, filter types.ResourceFilter) ([]types.Resource, error) {
	var resources []types.Resource

	output, err := p.redshiftClient.DescribeClusters(ctx, &redshift.DescribeClustersInput{})
	if err != nil {
		return nil, fmt.Errorf("failed to describe Redshift clusters: %w", err)
	}

	for _, cluster := range output.Clusters {
		tags := p.convertRedshiftTags(cluster.Tags)

		// Check cluster utilization
		nodeCount := aws.ToInt32(cluster.NumberOfNodes)
		isPaused := cluster.ClusterStatus != nil && *cluster.ClusterStatus == "paused"

		resource := types.Resource{
			ID:         aws.ToString(cluster.ClusterIdentifier),
			Type:       "redshift",
			Provider:   "aws",
			Region:     p.region,
			AccountID:  p.accountID,
			Name:       aws.ToString(cluster.ClusterIdentifier),
			Status:     aws.ToString(cluster.ClusterStatus),
			Tags:       tags,
			CreatedAt:  p.safeTimeValue(cluster.ClusterCreateTime),
			LastSeenAt: time.Now(),
			IsOrphaned: p.isResourceOrphaned(tags) || isPaused,
			Metadata: types.ResourceMetadata{
				NodeCount:    nodeCount,
				DBName:       aws.ToString(cluster.DBName),
				Endpoint:     aws.ToString(cluster.Endpoint.Address),
				Port:         aws.ToInt32(cluster.Endpoint.Port),
				Encrypted:    aws.ToBool(cluster.Encrypted),
				IsPaused:     isPaused,
				BackupWindow: aws.ToString(cluster.PreferredMaintenanceWindow),
				State:        aws.ToString(cluster.ClusterStatus),
			},
		}

		resources = append(resources, resource)
	}

	return resources, nil
}

// listRedshiftSnapshots discovers Redshift snapshots
func (p *RealAWSProvider) listRedshiftSnapshots(ctx context.Context, filter types.ResourceFilter) ([]types.Resource, error) {
	var resources []types.Resource

	output, err := p.redshiftClient.DescribeClusterSnapshots(ctx, &redshift.DescribeClusterSnapshotsInput{
		SnapshotType: aws.String("manual"),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to describe Redshift snapshots: %w", err)
	}

	for _, snapshot := range output.Snapshots {
		tags := p.convertRedshiftTags(snapshot.Tags)

		// Calculate age
		age := time.Since(p.safeTimeValue(snapshot.SnapshotCreateTime))
		ageInDays := int(age.Hours() / 24)
		isOld := ageInDays > 30

		resource := types.Resource{
			ID:         aws.ToString(snapshot.SnapshotIdentifier),
			Type:       "redshift_snapshot",
			Provider:   "aws",
			Region:     p.region,
			AccountID:  p.accountID,
			Name:       aws.ToString(snapshot.SnapshotIdentifier),
			Status:     aws.ToString(snapshot.Status),
			Tags:       tags,
			CreatedAt:  p.safeTimeValue(snapshot.SnapshotCreateTime),
			LastSeenAt: time.Now(),
			IsOrphaned: p.isResourceOrphaned(tags) || isOld,
			Metadata: map[string]interface{}{
				"cluster_identifier": aws.ToString(snapshot.ClusterIdentifier),
				"snapshot_type":      aws.ToString(snapshot.SnapshotType),
				"node_count":         aws.ToInt32(snapshot.NumberOfNodes),
				"backup_size_mb":     aws.ToFloat64(snapshot.TotalBackupSizeInMegaBytes),
				"actual_size_mb":     aws.ToFloat64(snapshot.ActualIncrementalBackupSizeInMegaBytes),
				"age_days":           ageInDays,
				"is_old":             isOld,
				"encrypted":          aws.ToBool(snapshot.Encrypted),
			},
		}

		resources = append(resources, resource)
	}

	return resources, nil
}

// listMemoryDBClusters discovers MemoryDB Redis clusters
func (p *RealAWSProvider) listMemoryDBClusters(ctx context.Context, filter types.ResourceFilter) ([]types.Resource, error) {
	var resources []types.Resource

	output, err := p.memorydbClient.DescribeClusters(ctx, &memorydb.DescribeClustersInput{})
	if err != nil {
		return nil, fmt.Errorf("failed to describe MemoryDB clusters: %w", err)
	}

	for _, cluster := range output.Clusters {
		// MemoryDB requires separate API call for tags
		tagsOutput, err := p.memorydbClient.ListTags(ctx, &memorydb.ListTagsInput{
			ResourceArn: cluster.ARN,
		})

		tags := types.Tags{}
		if err == nil && tagsOutput != nil {
			tags = p.convertMemoryDBTagList(tagsOutput.TagList)
		}

		// Count nodes across all shards
		totalNodes := 0
		for _, shard := range cluster.Shards {
			totalNodes += len(shard.Nodes)
		}

		resource := types.Resource{
			ID:         aws.ToString(cluster.Name),
			Type:       "memorydb",
			Provider:   "aws",
			Region:     p.region,
			AccountID:  p.accountID,
			Name:       aws.ToString(cluster.Name),
			Status:     aws.ToString(cluster.Status),
			Tags:       tags,
			CreatedAt:  time.Now(), // MemoryDB doesn't provide creation time
			LastSeenAt: time.Now(),
			IsOrphaned: p.isResourceOrphaned(tags),
			Metadata: map[string]interface{}{
				"node_type":          aws.ToString(cluster.NodeType),
				"shard_count":        len(cluster.Shards),
				"total_nodes":        totalNodes,
				"engine_version":     aws.ToString(cluster.EngineVersion),
				"tls_enabled":        aws.ToBool(cluster.TLSEnabled),
				"snapshot_retention": aws.ToInt32(cluster.SnapshotRetentionLimit),
				"maintenance_window": aws.ToString(cluster.MaintenanceWindow),
				"parameter_group":    aws.ToString(cluster.ParameterGroupName),
			},
		}

		// MemoryDB is expensive for in-memory workloads
		resource.Metadata["cost_priority"] = "high"
		resource.Metadata["monthly_cost_estimate"] = float64(totalNodes) * 120.0

		resources = append(resources, resource)
	}

	return resources, nil
}

// listDynamoDBTables discovers DynamoDB tables
func (p *RealAWSProvider) listDynamoDBTables(ctx context.Context, filter types.ResourceFilter) ([]types.Resource, error) {
	var resources []types.Resource

	paginator := dynamodb.NewListTablesPaginator(p.dynamodbClient, &dynamodb.ListTablesInput{})

	var tableNames []string
	for paginator.HasMorePages() {
		output, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to list DynamoDB tables: %w", err)
		}
		tableNames = append(tableNames, output.TableNames...)
	}

	// Describe each table to get details
	for _, tableName := range tableNames {
		tableOutput, err := p.dynamodbClient.DescribeTable(ctx, &dynamodb.DescribeTableInput{
			TableName: aws.String(tableName),
		})
		if err != nil {
			continue
		}

		table := tableOutput.Table

		// DynamoDB requires separate API call for tags
		tagsOutput, err := p.dynamodbClient.ListTagsOfResource(ctx, &dynamodb.ListTagsOfResourceInput{
			ResourceArn: table.TableArn,
		})

		tags := types.Tags{}
		if err == nil && tagsOutput != nil {
			tags = p.convertDynamoDBTagList(tagsOutput.Tags)
		}

		// Check billing mode
		isOnDemand := table.BillingModeSummary != nil &&
			table.BillingModeSummary.BillingMode == dynamodbtypes.BillingModePayPerRequest

		// Calculate provisioned capacity costs
		var readCapacity, writeCapacity int64
		if table.ProvisionedThroughput != nil {
			readCapacity = aws.ToInt64(table.ProvisionedThroughput.ReadCapacityUnits)
			writeCapacity = aws.ToInt64(table.ProvisionedThroughput.WriteCapacityUnits)
		}

		resource := types.Resource{
			ID:         aws.ToString(table.TableName),
			Type:       "dynamodb",
			Provider:   "aws",
			Region:     p.region,
			AccountID:  p.accountID,
			Name:       aws.ToString(table.TableName),
			Status:     string(table.TableStatus),
			Tags:       tags,
			CreatedAt:  p.safeTimeValue(table.CreationDateTime),
			LastSeenAt: time.Now(),
			IsOrphaned: p.isResourceOrphaned(tags),
			Metadata: map[string]interface{}{
				"table_size_bytes":         aws.ToInt64(table.TableSizeBytes),
				"item_count":               aws.ToInt64(table.ItemCount),
				"is_on_demand":             isOnDemand,
				"read_capacity":            readCapacity,
				"write_capacity":           writeCapacity,
				"global_secondary_indexes": len(table.GlobalSecondaryIndexes),
				"local_secondary_indexes":  len(table.LocalSecondaryIndexes),
				"stream_enabled":           table.StreamSpecification != nil && aws.ToBool(table.StreamSpecification.StreamEnabled),
				"encryption_type":          string(table.SSEDescription.SSEType),
			},
		}

		// Calculate estimated monthly cost
		if isOnDemand {
			resource.Metadata["billing_mode"] = "on_demand"
		} else {
			resource.Metadata["billing_mode"] = "provisioned"
			// Rough estimate: $0.00065 per RCU hour, $0.00325 per WCU hour
			monthlyCost := float64(readCapacity)*0.47 + float64(writeCapacity)*2.35
			resource.Metadata["monthly_cost_estimate"] = monthlyCost
		}

		resources = append(resources, resource)
	}

	return resources, nil
}

// listDynamoDBBackups discovers DynamoDB backups
func (p *RealAWSProvider) listDynamoDBBackups(ctx context.Context, filter types.ResourceFilter) ([]types.Resource, error) {
	var resources []types.Resource

	output, err := p.dynamodbClient.ListBackups(ctx, &dynamodb.ListBackupsInput{
		BackupType: dynamodbtypes.BackupTypeFilterUser,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to list DynamoDB backups: %w", err)
	}

	for _, backup := range output.BackupSummaries {
		// Get backup details
		backupDetails, err := p.dynamodbClient.DescribeBackup(ctx, &dynamodb.DescribeBackupInput{
			BackupArn: backup.BackupArn,
		})
		if err != nil {
			continue
		}

		desc := backupDetails.BackupDescription
		tags := types.Tags{} // DynamoDB backups don't have tags directly

		// Calculate age
		age := time.Since(p.safeTimeValue(backup.BackupCreationDateTime))
		ageInDays := int(age.Hours() / 24)
		isOld := ageInDays > 30

		resource := types.Resource{
			ID:         aws.ToString(backup.BackupName),
			Type:       "dynamodb_backup",
			Provider:   "aws",
			Region:     p.region,
			AccountID:  p.accountID,
			Name:       aws.ToString(backup.BackupName),
			Status:     string(backup.BackupStatus),
			Tags:       tags,
			CreatedAt:  p.safeTimeValue(backup.BackupCreationDateTime),
			LastSeenAt: time.Now(),
			IsOrphaned: isOld, // Backups without tags are considered orphaned if old
			Metadata: map[string]interface{}{
				"table_name":        aws.ToString(backup.TableName),
				"backup_size_bytes": aws.ToInt64(backup.BackupSizeBytes),
				"backup_type":       string(backup.BackupType),
				"age_days":          ageInDays,
				"is_old":            isOld,
				"expires_at":        p.safeTimeValue(backup.BackupExpiryDateTime),
			},
		}

		// Add source table info if available
		if desc.SourceTableDetails != nil {
			resource.Metadata["source_table_size"] = aws.ToInt64(desc.SourceTableDetails.TableSizeBytes)
			resource.Metadata["source_item_count"] = aws.ToInt64(desc.SourceTableDetails.ItemCount)
		}

		resources = append(resources, resource)
	}

	return resources, nil
}

// Helper functions for tag conversion

func (p *RealAWSProvider) convertRedshiftTags(tags []redshifttypes.Tag) types.Tags {
	result := types.Tags{}
	for _, tag := range tags {
		key := aws.ToString(tag.Key)
		value := aws.ToString(tag.Value)
		switch key {
		case "elava:owner", "Owner", "owner":
			result.ElavaOwner = value
		case "elava:managed":
			result.ElavaManaged = value == "true"
		case "elava:blessed":
			result.ElavaBlessed = value == "true"
		case "Environment", "environment", "env":
			result.Environment = value
		case "Team", "team":
			result.Team = value
		case "Name", "name":
			result.Name = value
		case "Project", "project":
			result.Project = value
		case "CostCenter", "cost-center", "costcenter":
			result.CostCenter = value
		}
	}
	return result
}

func (p *RealAWSProvider) convertMemoryDBTagList(tags []memorydbtypes.Tag) types.Tags {
	result := types.Tags{}
	for _, tag := range tags {
		key := aws.ToString(tag.Key)
		value := aws.ToString(tag.Value)
		switch key {
		case "elava:owner", "Owner", "owner":
			result.ElavaOwner = value
		case "elava:managed":
			result.ElavaManaged = value == "true"
		case "elava:blessed":
			result.ElavaBlessed = value == "true"
		case "Environment", "environment", "env":
			result.Environment = value
		case "Team", "team":
			result.Team = value
		case "Name", "name":
			result.Name = value
		case "Project", "project":
			result.Project = value
		case "CostCenter", "cost-center", "costcenter":
			result.CostCenter = value
		}
	}
	return result
}

func (p *RealAWSProvider) convertDynamoDBTagList(tags []dynamodbtypes.Tag) types.Tags {
	result := types.Tags{}
	for _, tag := range tags {
		key := aws.ToString(tag.Key)
		value := aws.ToString(tag.Value)
		switch key {
		case "elava:owner", "Owner", "owner":
			result.ElavaOwner = value
		case "elava:managed":
			result.ElavaManaged = value == "true"
		case "elava:blessed":
			result.ElavaBlessed = value == "true"
		case "Environment", "environment", "env":
			result.Environment = value
		case "Team", "team":
			result.Team = value
		case "Name", "name":
			result.Name = value
		case "Project", "project":
			result.Project = value
		case "CostCenter", "cost-center", "costcenter":
			result.CostCenter = value
		}
	}
	return result
}
