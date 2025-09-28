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
	Engine        string `json:"engine,omitempty"`
	EngineVersion string `json:"engine_version,omitempty"`
	InstanceClass string `json:"instance_class,omitempty"`
	DBName        string `json:"db_name,omitempty"`
	Endpoint      string `json:"endpoint,omitempty"`
	Port          int32  `json:"port,omitempty"`
	BackupWindow  string `json:"backup_window,omitempty"`
	ClusterID     string `json:"cluster_id,omitempty"`
	NodeCount     int    `json:"node_count,omitempty"`

	// Storage specific
	BucketName         string `json:"bucket_name,omitempty"`
	StorageClass       string `json:"storage_class,omitempty"`
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

	// Identity/Security specific
	RoleName         string   `json:"role_name,omitempty"`
	AssumeRolePolicy string   `json:"assume_role_policy,omitempty"`
	AttachedPolicies []string `json:"attached_policies,omitempty"`
	KeyID            string   `json:"key_id,omitempty"`
	KeyState         string   `json:"key_state,omitempty"`
	KeyUsage         string   `json:"key_usage,omitempty"`
	RepositoryURI    string   `json:"repository_uri,omitempty"`
	ImageCount       int      `json:"image_count,omitempty"`
	ZoneID           string   `json:"zone_id,omitempty"`
	ZoneName         string   `json:"zone_name,omitempty"`
	RecordCount      int      `json:"record_count,omitempty"`

	// Operational metadata
	IsIdle            bool      `json:"is_idle,omitempty"`
	IsPaused          bool      `json:"is_paused,omitempty"`
	IsOld             bool      `json:"is_old,omitempty"`
	IsTemp            bool      `json:"is_temp,omitempty"`
	AgeDays           int       `json:"age_days,omitempty"`
	DaysSinceModified int       `json:"days_since_modified,omitempty"`
	LastAccessTime    time.Time `json:"last_access_time,omitempty"`

	// Cost/Usage hints (not calculated, just metadata)
	InstanceFamily  string  `json:"instance_family,omitempty"`
	NormalizedUnits float64 `json:"normalized_units,omitempty"`
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
