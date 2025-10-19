# Design Session: AWS Provider (Read-Only Scanner)

## What problem are we solving?
Need an AWS provider that scans cloud resources (EC2, RDS, S3, VPC, etc.) and feeds observations into Elava's MVCC storage. **Read-only scanning only** - no infrastructure modifications.

## What's the simplest solution?
- AWS provider struct with EC2/RDS/S3 clients
- Scan resources using AWS Describe* APIs
- Convert AWS types to Elava Resource types
- Return observations for storage

## Can we break it into smaller functions?
Yes - one scanner per resource type:
- `ListEC2Instances()` - Scan EC2 instances (max 30 lines)
- `ListRDSInstances()` - Scan RDS databases (max 30 lines)
- `ListS3Buckets()` - Scan S3 buckets (max 30 lines)
- `ListVPCs()` - Scan VPCs (max 30 lines)
- `ListSubnets()` - Scan subnets (max 30 lines)
- `buildEC2Resource()` - Convert AWS EC2 to Resource (max 30 lines)
- `buildRDSResource()` - Convert AWS RDS to Resource (max 30 lines)

## What interfaces do we need?
```go
// AWS-specific clients (read-only)
type EC2Client interface {
    DescribeInstances(ctx, input) (output, error)
    DescribeVolumes(ctx, input) (output, error)
    DescribeSnapshots(ctx, input) (output, error)
}

type RDSClient interface {
    DescribeDBInstances(ctx, input) (output, error)
    DescribeDBClusters(ctx, input) (output, error)
}

type S3Client interface {
    ListBuckets(ctx, input) (output, error)
    GetBucketTagging(ctx, input) (output, error)
}

// Provider implements read-only scanning
type AWSProvider struct {
    ec2Client EC2Client
    rdsClient RDSClient
    s3Client  S3Client
    region    string
    accountID string
}
```

## What can go wrong?
- AWS credentials missing/invalid
- Rate limiting from AWS API
- Insufficient read permissions (need DescribeOnly)
- Network connectivity issues
- Partial scan failures (handle gracefully)

## Flow diagram
```
Scanner → AWSProvider → AWS Describe APIs
              ↓
       buildResource()
              ↓
    Resource[] → MVCC Storage
```

## Smallest testable units:
1. Mock AWS clients for testing
2. Scan EC2 instances
3. Scan RDS instances
4. Convert AWS instance to Resource struct
5. Handle pagination correctly
6. Handle API errors gracefully