package aws

import (
	"context"
	"fmt"

	"github.com/yairfalse/elava/types"
)

// ResourceLister lists a specific type of AWS resource
type ResourceLister interface {
	List(ctx context.Context, p *RealAWSProvider, filter types.ResourceFilter) ([]types.Resource, error)
	Name() string
	IsCritical() bool // Critical resources fail hard, optional resources log warnings
}

// ResourceListerRegistry holds all resource listers
type ResourceListerRegistry struct {
	listers []ResourceLister
}

// NewResourceListerRegistry creates a new registry with all listers
func NewResourceListerRegistry() *ResourceListerRegistry {
	return &ResourceListerRegistry{
		listers: []ResourceLister{
			// Critical resources - must succeed
			&EC2Lister{},
			&RDSLister{},
			&ELBLister{},

			// Optional resources - log warnings on failure
			&S3Lister{},
			&LambdaLister{},
			&EBSVolumeLister{},
			&ElasticIPLister{},
			&NATGatewayLister{},
			&SnapshotLister{},
			&AMILister{},
			&CloudWatchLogsLister{},
			&SecurityGroupLister{},
			&EKSLister{},
			&ECSLister{},
			&AutoScalingGroupLister{},
			&VPCEndpointLister{},
			&RDSSnapshotLister{},
			&IAMRoleLister{},
			&NetworkInterfaceLister{},
			&ECRLister{},
			&Route53Lister{},
			&KMSLister{},
			&AuroraLister{},
			&RedshiftLister{},
			&RedshiftSnapshotLister{},
			&MemoryDBLister{},
			&DynamoDBLister{},
			&DynamoDBBackupLister{},
			&SQSLister{},
		},
	}
}

// ListAll lists all resources using registered listers
func (r *ResourceListerRegistry) ListAll(ctx context.Context, p *RealAWSProvider, filter types.ResourceFilter) ([]types.Resource, error) {
	var allResources []types.Resource
	var criticalErrors []error

	for _, lister := range r.listers {
		resources, err := lister.List(ctx, p, filter)
		if err != nil {
			if lister.IsCritical() {
				criticalErrors = append(criticalErrors, fmt.Errorf("%s: %w", lister.Name(), err))
			} else {
				fmt.Printf("Warning: failed to list %s: %v\n", lister.Name(), err)
			}
			continue
		}
		allResources = append(allResources, resources...)
	}

	if len(criticalErrors) > 0 {
		return nil, fmt.Errorf("critical resource listing failed: %v", criticalErrors)
	}

	return allResources, nil
}
