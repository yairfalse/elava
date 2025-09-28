package aws

import (
	"context"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	s3types "github.com/aws/aws-sdk-go-v2/service/s3/types"
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
		resource := p.processS3Bucket(ctx, bucket)
		if resource != nil {
			resources = append(resources, *resource)
		}
	}

	return resources, nil
}

// processS3Bucket processes a single S3 bucket
func (p *RealAWSProvider) processS3Bucket(ctx context.Context, bucket s3types.Bucket) *types.Resource {
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
		return nil
	}

	// Get bucket tags
	tags := p.getBucketTags(ctx, bucket.Name)

	// Check if bucket is empty
	isEmpty := p.isBucketEmpty(ctx, bucket.Name)

	return &types.Resource{
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
}

// getBucketTags retrieves tags for an S3 bucket
func (p *RealAWSProvider) getBucketTags(ctx context.Context, bucketName *string) types.Tags {
	tags := types.Tags{}
	tagsOutput, err := p.s3Client.GetBucketTagging(ctx, &s3.GetBucketTaggingInput{
		Bucket: bucketName,
	})
	if err == nil && tagsOutput.TagSet != nil {
		tags = p.convertTagsToElava(tagsOutput.TagSet)
	}
	return tags
}

// isBucketEmpty checks if an S3 bucket is empty
func (p *RealAWSProvider) isBucketEmpty(ctx context.Context, bucketName *string) bool {
	objectsOutput, _ := p.s3Client.ListObjectsV2(ctx, &s3.ListObjectsV2Input{
		Bucket:  bucketName,
		MaxKeys: aws.Int32(1),
	})
	return objectsOutput == nil || len(objectsOutput.Contents) == 0
}
