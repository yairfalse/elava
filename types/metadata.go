package types

import "time"

// ResourceMetadata contains structured metadata for resources
// Replaces map[string]interface{} per CLAUDE.md requirements
type ResourceMetadata struct {
	// Common fields across resource types
	InstanceType       string    `json:"instance_type,omitempty"`
	AvailabilityZone   string    `json:"availability_zone,omitempty"`
	VpcID              string    `json:"vpc_id,omitempty"`
	SubnetID           string    `json:"subnet_id,omitempty"`
	PrivateIP          string    `json:"private_ip,omitempty"`
	PublicIP           string    `json:"public_ip,omitempty"`
	State              string    `json:"state,omitempty"`
	CreatedTime        time.Time `json:"created_time,omitempty"`
	ModifiedTime       time.Time `json:"modified_time,omitempty"`
	Size               int64     `json:"size,omitempty"`
	AllocatedStorage   int32     `json:"allocated_storage,omitempty"`
	Encrypted          bool      `json:"encrypted,omitempty"`
	MultiAZ            bool      `json:"multi_az,omitempty"`
	PubliclyAccessible bool      `json:"publicly_accessible,omitempty"`

	// Database specific
	Engine                      string `json:"engine,omitempty"`
	EngineVersion               string `json:"engine_version,omitempty"`
	InstanceClass               string `json:"instance_class,omitempty"`
	DBName                      string `json:"db_name,omitempty"`
	Endpoint                    string `json:"endpoint,omitempty"`
	Port                        int32  `json:"port,omitempty"`
	BackupWindow                string `json:"backup_window,omitempty"`
	ClusterID                   string `json:"cluster_id,omitempty"`
	NodeCount                   int    `json:"node_count,omitempty"`
	DBSubnetGroupName           string `json:"db_subnet_group_name,omitempty"`
	SecondaryAvailabilityZone   string `json:"secondary_availability_zone,omitempty"` // For Multi-AZ
	ReadReplicaIdentifiers      string `json:"read_replica_identifiers,omitempty"`    // Comma-separated
	ReadReplicaSourceIdentifier string `json:"read_replica_source_identifier,omitempty"`
	AvailabilityZones           string `json:"availability_zones,omitempty"` // Comma-separated AZs (for subnet groups)

	// Storage specific
	BucketName         string `json:"bucket_name,omitempty"`
	StorageClass       string `json:"storage_class,omitempty"`
	StorageGB          int    `json:"storage_gb,omitempty"`
	Versioning         bool   `json:"versioning,omitempty"`
	IsAttached         bool   `json:"is_attached,omitempty"`
	AttachedTo         string `json:"attached_to,omitempty"`
	VolumeType         string `json:"volume_type,omitempty"`
	IOPS               int32  `json:"iops,omitempty"`
	SnapshotID         string `json:"snapshot_id,omitempty"`
	SourceVolumeID     string `json:"source_volume_id,omitempty"`
	SourceImageID      string `json:"source_image_id,omitempty"`
	Architecture       string `json:"architecture,omitempty"`
	RootDeviceType     string `json:"root_device_type,omitempty"`
	VirtualizationType string `json:"virtualization_type,omitempty"`
	OwnerID            string `json:"owner_id,omitempty"`
	ImageLocation      string `json:"image_location,omitempty"`

	// Network specific
	DNSName            string   `json:"dns_name,omitempty"`
	Scheme             string   `json:"scheme,omitempty"`
	LoadBalancerType   string   `json:"type,omitempty"`
	SecurityGroups     []string `json:"security_groups,omitempty"`
	IsAssociated       bool     `json:"is_associated,omitempty"`
	AllocationID       string   `json:"allocation_id,omitempty"`
	AssociationID      string   `json:"association_id,omitempty"`
	NetworkInterfaceID string   `json:"network_interface_id,omitempty"`
	NatGatewayID       string   `json:"nat_gateway_id,omitempty"`
	GroupName          string   `json:"group_name,omitempty"`

	// Target Group specific
	TargetType                 string `json:"target_type,omitempty"`                   // instance, ip, lambda
	Protocol                   string `json:"protocol,omitempty"`                      // HTTP, HTTPS, TCP, etc.
	LoadBalancerARNs           string `json:"load_balancer_arns,omitempty"`            // Comma-separated ARNs
	HealthCheckProtocol        string `json:"health_check_protocol,omitempty"`         // Health check protocol
	HealthCheckPort            string `json:"health_check_port,omitempty"`             // Health check port
	HealthCheckPath            string `json:"health_check_path,omitempty"`             // Health check path (HTTP/HTTPS)
	HealthCheckIntervalSeconds int32  `json:"health_check_interval_seconds,omitempty"` // Interval between checks
	HealthCheckTimeoutSeconds  int32  `json:"health_check_timeout_seconds,omitempty"`  // Timeout for each check
	HealthyThresholdCount      int32  `json:"healthy_threshold_count,omitempty"`       // Consecutive successes for healthy
	UnhealthyThresholdCount    int32  `json:"unhealthy_threshold_count,omitempty"`     // Consecutive failures for unhealthy
	// VPC Networking specific
	CIDRBlock             string `json:"cidr_block,omitempty"`
	MapPublicIPOnLaunch   bool   `json:"map_public_ip_on_launch"`
	IsMainRouteTable      bool   `json:"is_main_route_table"`
	AssociatedSubnetIDs   []string `json:"associated_subnet_ids,omitempty"`
	Routes                string `json:"routes,omitempty"`                // Formatted route list
	AttachmentState       string `json:"attachment_state,omitempty"`      // IGW attachment state
	ElasticIPAllocationID string `json:"elastic_ip_allocation_id,omitempty"`
	RequesterVpcID        string `json:"requester_vpc_id,omitempty"`
	AccepterVpcID         string `json:"accepter_vpc_id,omitempty"`
	RequesterCIDRBlock    string `json:"requester_cidr_block,omitempty"`
	AccepterCIDRBlock     string `json:"accepter_cidr_block,omitempty"`
	PeerRegion            string `json:"peer_region,omitempty"`

	// Compute specific
	FunctionName    string `json:"function_name,omitempty"`
	Runtime         string `json:"runtime,omitempty"`
	Handler         string `json:"handler,omitempty"`
	CodeSize        int64  `json:"code_size,omitempty"`
	Timeout         int32  `json:"timeout,omitempty"`
	MemorySize      int32  `json:"memory_size,omitempty"`
	LastModified    string `json:"last_modified,omitempty"`
	ClusterVersion  string `json:"cluster_version,omitempty"`
	NodeGroupCount  int    `json:"node_group_count,omitempty"`
	TaskDefinitions int    `json:"task_definitions,omitempty"`
	Services        int    `json:"services,omitempty"`
	DesiredCapacity int32  `json:"desired_capacity,omitempty"`
	MinSize         int32  `json:"min_size,omitempty"`
	MaxSize         int32  `json:"max_size,omitempty"`
	TargetCapacity  int32  `json:"target_capacity,omitempty"`

	// EKS specific
	ClusterName          string            `json:"cluster_name,omitempty"`           // For node groups
	RoleArn              string            `json:"role_arn,omitempty"`               // IAM role ARN
	SubnetIDs            string            `json:"subnet_ids,omitempty"`             // Comma-separated subnet IDs
	SecurityGroupIDs     string            `json:"security_group_ids,omitempty"`     // Comma-separated SG IDs
	AutoScalingGroupName string            `json:"autoscaling_group_name,omitempty"` // Critical: EKS → ASG link!
	InstanceTypes        string            `json:"instance_types,omitempty"`         // Comma-separated instance types
	NodeLabels           map[string]string `json:"node_labels,omitempty"`            // K8s node labels
	NodeTaints           string            `json:"node_taints,omitempty"`            // Formatted taints (key=value:effect)
	// Auto Scaling Group specific
	CurrentSize        int32  `json:"current_size,omitempty"`
	InstanceIDs        string `json:"instance_ids,omitempty"`         // Comma-separated instance IDs
	LaunchTemplate     string `json:"launch_template,omitempty"`      // Launch template name or ID
	TargetGroupARNs    string `json:"target_group_arns,omitempty"`    // Comma-separated ARNs
	VPCZoneIdentifiers string `json:"vpc_zone_identifiers,omitempty"` // Comma-separated subnet IDs

	// Identity/Security specific
	RoleName          string   `json:"role_name,omitempty"`
	AssumeRolePolicy  string   `json:"assume_role_policy,omitempty"`
	AttachedPolicies  []string `json:"attached_policies,omitempty"`
	KeyID             string   `json:"key_id,omitempty"`
	KeyState          string   `json:"key_state,omitempty"`
	KeyUsage          string   `json:"key_usage,omitempty"`
	KeySpec           string   `json:"key_spec,omitempty"`
	IsPendingDeletion bool     `json:"is_pending_deletion,omitempty"`
	IsDisabled        bool     `json:"is_disabled,omitempty"`
	RepositoryURI     string   `json:"repository_uri,omitempty"`
	ImageCount        int      `json:"image_count,omitempty"`
	ZoneID            string   `json:"zone_id,omitempty"`
	ZoneName          string   `json:"zone_name,omitempty"`
	RecordCount       int      `json:"record_count,omitempty"`
	IsPrivateZone     bool     `json:"is_private_zone,omitempty"`
	Comment           string   `json:"comment,omitempty"`

	// Operational metadata
	IsIdle                bool       `json:"is_idle,omitempty"`
	IsPaused              bool       `json:"is_paused,omitempty"`
	IsOld                 bool       `json:"is_old,omitempty"`
	IsTemp                bool       `json:"is_temp,omitempty"`
	IsEmpty               bool       `json:"is_empty,omitempty"`
	AgeDays               int        `json:"age_days,omitempty"`
	DaysSinceModified     int        `json:"days_since_modified,omitempty"`
	LastAccessTime        *time.Time `json:"last_access_time,omitempty"`
	DeletionProtection    bool       `json:"deletion_protection,omitempty"`
	BackupRetentionPeriod int        `json:"backup_retention_period,omitempty"`

	// Cost/Usage hints (not calculated, just metadata)
	InstanceFamily      string  `json:"instance_family,omitempty"`
	NormalizedUnits     float64 `json:"normalized_units,omitempty"`
	MonthlyCostEstimate float64 `json:"monthly_cost_estimate,omitempty"`

	// DynamoDB backup specific
	TableName       string    `json:"table_name,omitempty"`
	BackupSizeBytes int64     `json:"backup_size_bytes,omitempty"`
	BackupType      string    `json:"backup_type,omitempty"`
	ExpiresAt       time.Time `json:"expires_at,omitempty"`
	ItemCount       int64     `json:"item_count,omitempty"`
}

// HasCompute returns true if metadata contains compute-related fields
func (m ResourceMetadata) HasCompute() bool {
	return m.InstanceType != "" || m.FunctionName != "" ||
		m.ClusterID != "" || m.TaskDefinitions > 0
}

// HasStorage returns true if metadata contains storage-related fields
func (m ResourceMetadata) HasStorage() bool {
	return m.BucketName != "" || m.VolumeType != "" ||
		m.SnapshotID != "" || m.Size > 0
}

// HasNetwork returns true if metadata contains network-related fields
func (m ResourceMetadata) HasNetwork() bool {
	return m.VpcID != "" || m.SubnetID != "" ||
		m.DNSName != "" || m.NatGatewayID != ""
}

// IsActive returns true if resource appears to be actively used
func (m ResourceMetadata) IsActive() bool {
	return !m.IsIdle && !m.IsPaused && m.State != "stopped"
}
