package aws

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ecr"
	ecrtypes "github.com/aws/aws-sdk-go-v2/service/ecr/types"
	"github.com/aws/aws-sdk-go-v2/service/iam"
	iamtypes "github.com/aws/aws-sdk-go-v2/service/iam/types"
	"github.com/aws/aws-sdk-go-v2/service/kms"
	kmstypes "github.com/aws/aws-sdk-go-v2/service/kms/types"
	"github.com/aws/aws-sdk-go-v2/service/route53"
	route53types "github.com/aws/aws-sdk-go-v2/service/route53/types"
	"github.com/yairfalse/elava/types"
)

// listIAMRoles discovers IAM roles
func (p *RealAWSProvider) listIAMRoles(ctx context.Context, filter types.ResourceFilter) ([]types.Resource, error) {
	var resources []types.Resource
	paginator := iam.NewListRolesPaginator(p.iamClient, &iam.ListRolesInput{})

	for paginator.HasMorePages() {
		output, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to list IAM roles: %w", err)
		}

		for _, role := range output.Roles {
			resource := p.processIAMRole(ctx, role)
			resources = append(resources, resource)
		}
	}

	return resources, nil
}

// processIAMRole processes a single IAM role
func (p *RealAWSProvider) processIAMRole(ctx context.Context, role iamtypes.Role) types.Resource {
	tags := p.getIAMRoleTags(ctx, role.RoleName)

	// Check if it's a service-linked role
	isServiceLinked := strings.HasPrefix(aws.ToString(role.Path), "/aws-service-role/")

	// Check last used
	lastUsedTime := p.getIAMRoleLastUsed(ctx, role.RoleName)
	isUnused := p.isIAMRoleUnused(lastUsedTime)

	isOrphaned := !isServiceLinked && (p.isResourceOrphaned(tags) || isUnused)

	return types.Resource{
		ID:         aws.ToString(role.Arn),
		Type:       "iam-role",
		Provider:   "aws",
		Region:     "global", // IAM is global
		AccountID:  p.accountID,
		Name:       aws.ToString(role.RoleName),
		Status:     "active",
		Tags:       tags,
		CreatedAt:  p.safeTimeValue(role.CreateDate),
		LastSeenAt: time.Now(),
		IsOrphaned: isOrphaned,
		Metadata: map[string]interface{}{
			"path":              aws.ToString(role.Path),
			"is_service_linked": isServiceLinked,
			"last_used":         lastUsedTime,
			"is_unused":         isUnused,
		},
	}
}

// getIAMRoleTags retrieves tags for an IAM role
func (p *RealAWSProvider) getIAMRoleTags(ctx context.Context, roleName *string) types.Tags {
	tags := types.Tags{}
	output, err := p.iamClient.ListRoleTags(ctx, &iam.ListRoleTagsInput{
		RoleName: roleName,
	})
	if err == nil && output.Tags != nil {
		tags = p.convertTagsToElava(output.Tags)
	}
	return tags
}

// getIAMRoleLastUsed gets when a role was last used
func (p *RealAWSProvider) getIAMRoleLastUsed(ctx context.Context, roleName *string) *time.Time {
	output, err := p.iamClient.GetRole(ctx, &iam.GetRoleInput{
		RoleName: roleName,
	})
	if err == nil && output.Role.RoleLastUsed != nil {
		return output.Role.RoleLastUsed.LastUsedDate
	}
	return nil
}

// isIAMRoleUnused checks if a role hasn't been used recently
func (p *RealAWSProvider) isIAMRoleUnused(lastUsed *time.Time) bool {
	if lastUsed == nil {
		return true // Never used
	}
	// Consider unused if not used in 90 days
	return time.Since(*lastUsed) > 90*24*time.Hour
}

// listECRRepositories discovers ECR repositories
func (p *RealAWSProvider) listECRRepositories(ctx context.Context, filter types.ResourceFilter) ([]types.Resource, error) {
	var resources []types.Resource
	paginator := ecr.NewDescribeRepositoriesPaginator(p.ecrClient, &ecr.DescribeRepositoriesInput{})

	for paginator.HasMorePages() {
		output, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to list ECR repositories: %w", err)
		}

		for _, repo := range output.Repositories {
			resource := p.processECRRepository(ctx, repo)
			resources = append(resources, resource)
		}
	}

	return resources, nil
}

// processECRRepository processes a single ECR repository
func (p *RealAWSProvider) processECRRepository(ctx context.Context, repo ecrtypes.Repository) types.Resource {
	// ECR doesn't have tags on repos directly, using repository URI as identifier
	tags := types.Tags{}

	// Check if repository is empty
	imageCount := p.getECRImageCount(ctx, repo.RepositoryName)
	isEmpty := imageCount == 0

	isOrphaned := isEmpty || p.isResourceOrphaned(tags)

	return types.Resource{
		ID:         aws.ToString(repo.RepositoryArn),
		Type:       "ecr",
		Provider:   "aws",
		Region:     p.region,
		AccountID:  p.accountID,
		Name:       aws.ToString(repo.RepositoryName),
		Status:     "active",
		Tags:       tags,
		CreatedAt:  p.safeTimeValue(repo.CreatedAt),
		LastSeenAt: time.Now(),
		IsOrphaned: isOrphaned,
		Metadata: map[string]interface{}{
			"repository_uri": aws.ToString(repo.RepositoryUri),
			"image_count":    imageCount,
			"is_empty":       isEmpty,
		},
	}
}

// getECRImageCount gets the number of images in a repository
func (p *RealAWSProvider) getECRImageCount(ctx context.Context, repoName *string) int {
	output, err := p.ecrClient.DescribeImages(ctx, &ecr.DescribeImagesInput{
		RepositoryName: repoName,
		MaxResults:     aws.Int32(100), // Just to get a count
	})
	if err != nil {
		return 0
	}
	return len(output.ImageDetails)
}

// listRoute53HostedZones discovers Route 53 hosted zones
func (p *RealAWSProvider) listRoute53HostedZones(ctx context.Context, filter types.ResourceFilter) ([]types.Resource, error) {
	var resources []types.Resource
	paginator := route53.NewListHostedZonesPaginator(p.route53Client, &route53.ListHostedZonesInput{})

	for paginator.HasMorePages() {
		output, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to list Route 53 hosted zones: %w", err)
		}

		for _, zone := range output.HostedZones {
			resource := p.processRoute53Zone(ctx, zone)
			resources = append(resources, resource)
		}
	}

	return resources, nil
}

// processRoute53Zone processes a single Route 53 hosted zone
func (p *RealAWSProvider) processRoute53Zone(ctx context.Context, zone route53types.HostedZone) types.Resource {
	tags := p.getRoute53Tags(ctx, zone.Id)

	// Extract zone ID without the /hostedzone/ prefix
	zoneID := strings.TrimPrefix(aws.ToString(zone.Id), "/hostedzone/")

	// Check record count
	recordCount := p.getRoute53RecordCount(ctx, zone.Id)
	// Zone with only SOA and NS records is likely empty
	isEmpty := recordCount <= 2

	isOrphaned := isEmpty || p.isResourceOrphaned(tags)

	return types.Resource{
		ID:         zoneID,
		Type:       "route53",
		Provider:   "aws",
		Region:     "global", // Route 53 is global
		AccountID:  p.accountID,
		Name:       aws.ToString(zone.Name),
		Status:     "active",
		Tags:       tags,
		LastSeenAt: time.Now(),
		IsOrphaned: isOrphaned,
		Metadata: map[string]interface{}{
			"private_zone": zone.Config.PrivateZone,
			"record_count": recordCount,
			"comment":      aws.ToString(zone.Config.Comment),
			"is_empty":     isEmpty,
		},
	}
}

// getRoute53Tags retrieves tags for a hosted zone
func (p *RealAWSProvider) getRoute53Tags(ctx context.Context, zoneID *string) types.Tags {
	tags := types.Tags{}
	output, err := p.route53Client.ListTagsForResource(ctx, &route53.ListTagsForResourceInput{
		ResourceType: route53types.TagResourceTypeHostedzone,
		ResourceId:   zoneID,
	})
	if err == nil && output.ResourceTagSet != nil && output.ResourceTagSet.Tags != nil {
		tags = p.convertTagsToElava(output.ResourceTagSet.Tags)
	}
	return tags
}

// getRoute53RecordCount gets the number of records in a zone
func (p *RealAWSProvider) getRoute53RecordCount(ctx context.Context, zoneID *string) int {
	output, err := p.route53Client.ListResourceRecordSets(ctx, &route53.ListResourceRecordSetsInput{
		HostedZoneId: zoneID,
		MaxItems:     aws.Int32(100),
	})
	if err != nil {
		return 0
	}
	return len(output.ResourceRecordSets)
}

// listKMSKeys discovers KMS keys
func (p *RealAWSProvider) listKMSKeys(ctx context.Context, filter types.ResourceFilter) ([]types.Resource, error) {
	var resources []types.Resource
	paginator := kms.NewListKeysPaginator(p.kmsClient, &kms.ListKeysInput{})

	for paginator.HasMorePages() {
		output, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to list KMS keys: %w", err)
		}

		for _, key := range output.Keys {
			resource := p.processKMSKey(ctx, key)
			if resource != nil {
				resources = append(resources, *resource)
			}
		}
	}

	return resources, nil
}

// processKMSKey processes a single KMS key
func (p *RealAWSProvider) processKMSKey(ctx context.Context, key kmstypes.KeyListEntry) *types.Resource {
	// Describe the key to get more details
	keyInfo, err := p.kmsClient.DescribeKey(ctx, &kms.DescribeKeyInput{
		KeyId: key.KeyId,
	})
	if err != nil {
		return nil
	}

	metadata := keyInfo.KeyMetadata

	// Skip AWS-managed keys
	if metadata.KeyManager == kmstypes.KeyManagerTypeAws {
		return nil
	}

	tags := p.getKMSTags(ctx, key.KeyId)

	// Check if key is pending deletion
	isPendingDeletion := metadata.KeyState == kmstypes.KeyStatePendingDeletion
	isDisabled := metadata.KeyState == kmstypes.KeyStateDisabled

	isOrphaned := isPendingDeletion || isDisabled || p.isResourceOrphaned(tags)

	// Get key alias for a better name
	keyName := p.getKMSKeyAlias(ctx, key.KeyId)
	if keyName == "" {
		keyName = aws.ToString(key.KeyId)
	}

	return &types.Resource{
		ID:         aws.ToString(key.KeyArn),
		Type:       "kms",
		Provider:   "aws",
		Region:     p.region,
		AccountID:  p.accountID,
		Name:       keyName,
		Status:     string(metadata.KeyState),
		Tags:       tags,
		CreatedAt:  p.safeTimeValue(metadata.CreationDate),
		LastSeenAt: time.Now(),
		IsOrphaned: isOrphaned,
		Metadata: map[string]interface{}{
			"key_usage":           string(metadata.KeyUsage),
			"key_spec":            string(metadata.KeySpec),
			"is_pending_deletion": isPendingDeletion,
			"is_disabled":         isDisabled,
		},
	}
}

// getKMSTags retrieves tags for a KMS key
func (p *RealAWSProvider) getKMSTags(ctx context.Context, keyID *string) types.Tags {
	tags := types.Tags{}
	output, err := p.kmsClient.ListResourceTags(ctx, &kms.ListResourceTagsInput{
		KeyId: keyID,
	})
	if err == nil && output.Tags != nil {
		tags = p.convertTagsToElava(output.Tags)
	}
	return tags
}

// getKMSKeyAlias gets the primary alias for a KMS key
func (p *RealAWSProvider) getKMSKeyAlias(ctx context.Context, keyID *string) string {
	output, err := p.kmsClient.ListAliases(ctx, &kms.ListAliasesInput{
		KeyId: keyID,
	})
	if err == nil && len(output.Aliases) > 0 {
		return aws.ToString(output.Aliases[0].AliasName)
	}
	return ""
}
