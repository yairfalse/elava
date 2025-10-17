# Advanced AWS Scanners Design

## Design Session Checklist

- [x] What problem are we solving?
- [x] How will this interact with storage (read/write/query)?
- [x] What historical data do we need to track?
- [x] What's the simplest solution?
- [x] Can we break it into smaller functions?
- [x] What interfaces do we need?
- [x] What can go wrong?
- [x] Draw the flow (ASCII or diagram)

## Problem Statement

**What are we solving?**
- Need to scan AWS resources with rich relationship data for Ahti integration
- Current scanners are basic - missing ASG relationships, EKS topology, RDS replication, VPC routing
- INTEGRATION_DESIGN.md defines comprehensive data structures, but we don't scan them yet

**Why does this matter?**
- **ASG → EC2**: Track which instances are managed by ASGs, detect scaling events
- **EKS → ASG → EC2**: Critical bridge between K8s (Tapio) and AWS infrastructure
- **RDS**: Multi-AZ placement, read replica chains, subnet groups for network troubleshooting
- **VPC Networking**: Routing topology, NAT/IGW connectivity, public vs private subnets

## Storage-First Thinking

### What observations to store?

All stored in existing `types.Resource` with `ResourceMetadata`:

```go
// Example: ASG stored as Resource
resource := types.Resource{
    ID: "web-asg-prod",
    Type: "autoscaling_group",
    Provider: "aws",
    Region: "us-east-1",
    AccountID: "123456789",
    Tags: map[string]string{
        "Name": "web-asg-prod",
    },
    Metadata: ResourceMetadata{
        // ASG-specific data
        "min_size": "2",
        "max_size": "10",
        "desired_capacity": "5",
        "current_size": "5",
        "instance_ids": "i-abc123,i-def456,i-ghi789",  // Comma-separated
        "launch_template": "lt-xyz789",
        "target_group_arns": "arn:aws:...,arn:aws:...",
    },
}
```

**No storage changes needed!** Use existing generic structure.

### What queries will we need?

1. "Which ASG owns instance i-abc123?" → Query resources where `type=autoscaling_group` and `instance_ids` contains i-abc123
2. "Show EKS cluster topology" → Query `eks_cluster` + related `eks_node_group` + ASG + EC2
3. "Which RDS instances have Multi-AZ?" → Query resources where `type=rds_instance` and `multi_az=true`
4. "Show routing for subnet-private-1a" → Query route tables where `subnet_ids` contains subnet-private-1a

### What patterns to detect?

- **ASG drift**: Current size != desired size (scaling issue)
- **EKS node group drift**: ASG current != node group desired
- **RDS replication lag**: Read replica missing or broken
- **VPC routing issues**: NAT gateway in `blackhole` state

### What history to maintain?

- **ASG scaling events**: Track when `current_size` changes (5 → 8 → 3)
- **EKS upgrades**: Track K8s version changes (1.27 → 1.28)
- **RDS replica additions**: Track when read replicas are added/removed
- **Route table changes**: Track when routes change state (active → blackhole)

## Architecture Flow

```
AWS API Call
    ↓
Parse Response (AWS SDK types)
    ↓
Build types.Resource with Metadata
    ↓
Storage.RecordObservation()
    ↓
BadgerDB (MVCC)
```

**No new components.** Just new scanner functions in `providers/aws/`.

## Implementation Plan

### Phase 1: Auto Scaling Groups ✨
**File**: `providers/aws/autoscaling.go`

```go
// ListAutoScalingGroups scans ASGs
func (p *RealAWSProvider) ListAutoScalingGroups(ctx context.Context) ([]types.Resource, error)

// buildASGResource converts AWS ASG to types.Resource
func buildASGResource(asg autoscalingtypes.AutoScalingGroup, region, accountID string) types.Resource
```

**What to capture:**
- Name, MinSize, MaxSize, DesiredCapacity, CurrentSize (len(Instances))
- InstanceIDs (slice of instance IDs)
- LaunchTemplate (name or ID)
- TargetGroupARNs (for ALB/NLB relationships)
- VPCZoneIdentifier (subnet IDs, comma-separated)
- Tags

**Test cases:**
- ASG with 5 instances
- ASG with launch template
- ASG with target groups
- ASG in multiple subnets

### Phase 2: EKS Clusters + Node Groups ✨
**File**: `providers/aws/eks.go`

```go
// ListEKSClusters scans EKS clusters
func (p *RealAWSProvider) ListEKSClusters(ctx context.Context) ([]types.Resource, error)

// ListEKSNodeGroups scans node groups for all clusters
func (p *RealAWSProvider) ListEKSNodeGroups(ctx context.Context) ([]types.Resource, error)

// buildEKSClusterResource converts AWS EKS cluster
func buildEKSClusterResource(cluster ekstypes.Cluster, region, accountID string) types.Resource

// buildEKSNodeGroupResource converts AWS EKS node group
func buildEKSNodeGroupResource(ng ekstypes.Nodegroup, clusterName, region, accountID string) types.Resource
```

**What to capture (EKS Cluster):**
- Name, Version, Status, Endpoint
- VpcID, SubnetIDs, SecurityGroupIDs
- RoleARN
- Tags

**What to capture (EKS Node Group):**
- ClusterName, NodeGroupName, Status
- AutoScalingGroupName (critical for ASG → EC2 link!)
- InstanceTypes, DesiredSize, MinSize, MaxSize, CurrentSize
- SubnetIDs
- Labels (K8s node labels - bridge to Tapio!)
- Taints
- Tags

**Test cases:**
- EKS cluster with 2 node groups
- Node group with K8s labels
- Node group linked to ASG

### Phase 3: RDS Instances + Subnet Groups ✨
**File**: `providers/aws/rds.go` (extend existing)

```go
// ListRDSInstances - ENHANCE existing function
func (p *RealAWSProvider) ListRDSInstances(ctx context.Context) ([]types.Resource, error)

// ListDBSubnetGroups scans DB subnet groups
func (p *RealAWSProvider) ListDBSubnetGroups(ctx context.Context) ([]types.Resource, error)

// buildRDSInstanceResource - ENHANCE to capture more data
func buildRDSInstanceResource(instance rdstypes.DBInstance, region, accountID string) types.Resource

// buildDBSubnetGroupResource converts DB subnet group
func buildDBSubnetGroupResource(sg rdstypes.DBSubnetGroup, region, accountID string) types.Resource
```

**What to capture (RDS Instance) - ADDITIONS:**
- DBSubnetGroupName
- AvailabilityZone, SecondaryAZ (if Multi-AZ)
- MultiAZ (boolean)
- VPCSecurityGroupIDs
- PubliclyAccessible
- Endpoint, Port
- ReadReplicaDBInstanceIdentifiers (slice)
- ReadReplicaSourceDBInstanceIdentifier
- AllocatedStorage, StorageEncrypted
- BackupRetentionPeriod

**What to capture (DB Subnet Group):**
- Name, Description
- VpcID
- SubnetIDs
- AvailabilityZones

**Test cases:**
- RDS with Multi-AZ enabled
- RDS primary with 2 read replicas
- DB subnet group spanning 3 AZs

### Phase 4: VPC Networking ✨
**File**: `providers/aws/network.go` (extend existing)

```go
// ListSubnets scans subnets
func (p *RealAWSProvider) ListSubnets(ctx context.Context) ([]types.Resource, error)

// ListRouteTables scans route tables
func (p *RealAWSProvider) ListRouteTables(ctx context.Context) ([]types.Resource, error)

// ListInternetGateways scans IGWs
func (p *RealAWSProvider) ListInternetGateways(ctx context.Context) ([]types.Resource, error)

// ListNATGateways scans NAT gateways
func (p *RealAWSProvider) ListNATGateways(ctx context.Context) ([]types.Resource, error)

// ListVPCPeeringConnections scans VPC peering
func (p *RealAWSProvider) ListVPCPeeringConnections(ctx context.Context) ([]types.Resource, error)
```

**What to capture (Subnet):**
- SubnetID, VpcID, CIDR, AvailabilityZone
- MapPublicIpOnLaunch (public vs private indicator)
- RouteTableID (requires lookup)
- Tags

**What to capture (Route Table):**
- RouteTableID, VpcID, IsMain
- SubnetIDs (associations)
- Routes: array of {DestinationCIDR, TargetType, TargetID, State}

**What to capture (Internet Gateway):**
- InternetGatewayID, VpcID, State

**What to capture (NAT Gateway):**
- NatGatewayID, VpcID, SubnetID
- ElasticIP, State

**What to capture (VPC Peering):**
- PeeringConnectionID, RequesterVpcID, AccepterVpcID, Status

### Phase 5: Target Groups ✨
**File**: `providers/aws/loadbalancer.go` (extend existing)

```go
// ListTargetGroups scans target groups
func (p *RealAWSProvider) ListTargetGroups(ctx context.Context) ([]types.Resource, error)

// buildTargetGroupResource converts target group
func buildTargetGroupResource(tg elbv2types.TargetGroup, region, accountID string) types.Resource
```

**What to capture:**
- TargetGroupARN, TargetGroupName, TargetType (instance, ip, lambda)
- Protocol, Port
- VpcID
- HealthCheckProtocol, HealthCheckPort, HealthCheckPath
- LoadBalancerARNs (which ALBs/NLBs use this TG)
- Tags

## Function Size Guidelines

Each scanner function should be <50 lines:

```go
// ✅ GOOD - Small, focused
func (p *RealAWSProvider) ListAutoScalingGroups(ctx context.Context) ([]types.Resource, error) {
    // 1. Call AWS API (1 function call)
    output, err := p.asgClient.DescribeAutoScalingGroups(ctx, &autoscaling.DescribeAutoScalingGroupsInput{})
    if err != nil {
        return nil, fmt.Errorf("failed to describe ASGs: %w", err)
    }

    // 2. Convert to resources (loop + helper function)
    resources := make([]types.Resource, 0, len(output.AutoScalingGroups))
    for _, asg := range output.AutoScalingGroups {
        resource := buildASGResource(asg, p.region, p.accountID)
        resources = append(resources, resource)
    }

    return resources, nil
}

// Helper function (also <50 lines)
func buildASGResource(asg autoscalingtypes.AutoScalingGroup, region, accountID string) types.Resource {
    // Extract instance IDs
    instanceIDs := make([]string, len(asg.Instances))
    for i, inst := range asg.Instances {
        instanceIDs[i] = aws.ToString(inst.InstanceId)
    }

    // Extract target group ARNs
    targetGroupARNs := make([]string, len(asg.TargetGroupARNs))
    for i, arn := range asg.TargetGroupARNs {
        targetGroupARNs[i] = arn
    }

    return types.Resource{
        ID:       aws.ToString(asg.AutoScalingGroupName),
        Type:     "autoscaling_group",
        Provider: "aws",
        Region:   region,
        AccountID: accountID,
        Name:     aws.ToString(asg.AutoScalingGroupName),
        Status:   "active",
        Tags:     convertASGTags(asg.Tags),
        Metadata: types.ResourceMetadata{
            "min_size":             fmt.Sprintf("%d", aws.ToInt32(asg.MinSize)),
            "max_size":             fmt.Sprintf("%d", aws.ToInt32(asg.MaxSize)),
            "desired_capacity":     fmt.Sprintf("%d", aws.ToInt32(asg.DesiredCapacity)),
            "current_size":         fmt.Sprintf("%d", len(asg.Instances)),
            "instance_ids":         strings.Join(instanceIDs, ","),
            "launch_template":      extractLaunchTemplateName(asg.LaunchTemplate),
            "target_group_arns":    strings.Join(targetGroupARNs, ","),
            "vpc_zone_identifiers": aws.ToString(asg.VPCZoneIdentifier),
        },
    }
}
```

## What Can Go Wrong?

### 1. AWS API Rate Limiting
**Problem:** Scanning large accounts hits API limits
**Solution:**
- Use pagination properly
- Add exponential backoff
- Scan in batches

### 2. Missing Permissions
**Problem:** User doesn't have `autoscaling:Describe*` or `eks:Describe*`
**Solution:**
- Handle permission errors gracefully
- Log which scanners failed
- Continue with other scanners

### 3. Empty/Null Fields
**Problem:** AWS returns nil for optional fields (LaunchTemplate, TargetGroups, etc.)
**Solution:**
- Use `aws.ToString()`, `aws.ToInt32()` (returns zero value if nil)
- Check for nil before accessing nested fields
- Store empty string in Metadata if field is nil

### 4. Large Metadata Fields
**Problem:** ASG with 100 instances → huge `instance_ids` string
**Solution:**
- Comma-separated strings work fine (BadgerDB handles large values)
- Alternative: Store first N instance IDs, add `instance_count` field

### 5. Circular References
**Problem:** EKS Node Group → ASG → EC2 instances (all in same scan)
**Solution:**
- Store as separate resources (no problem!)
- Relationship resolution happens in Ahti plugin, not storage
- Conservative approach: just store what we see

## Testing Strategy

### Unit Tests (per scanner)

```go
func TestBuildASGResource(t *testing.T) {
    // Given: AWS ASG with known values
    asg := autoscalingtypes.AutoScalingGroup{
        AutoScalingGroupName: aws.String("web-asg-prod"),
        MinSize:              aws.Int32(2),
        MaxSize:              aws.Int32(10),
        DesiredCapacity:      aws.Int32(5),
        Instances: []autoscalingtypes.Instance{
            {InstanceId: aws.String("i-abc123")},
            {InstanceId: aws.String("i-def456")},
        },
    }

    // When: Build resource
    resource := buildASGResource(asg, "us-east-1", "123456789")

    // Then: Verify metadata
    assert.Equal(t, "autoscaling_group", resource.Type)
    assert.Equal(t, "web-asg-prod", resource.ID)
    assert.Equal(t, "5", resource.Metadata["desired_capacity"])
    assert.Equal(t, "2", resource.Metadata["current_size"])
    assert.Contains(t, resource.Metadata["instance_ids"], "i-abc123")
}
```

### Integration Tests (with storage)

```go
func TestASGScanner_Integration(t *testing.T) {
    // Given: Mock AWS client + storage
    mockClient := &mockASGClient{asgs: testASGs}
    provider := &RealAWSProvider{asgClient: mockClient}
    storage := setupTestStorage(t)

    // When: Scan and store
    resources, err := provider.ListAutoScalingGroups(ctx)
    require.NoError(t, err)

    for _, resource := range resources {
        _, err := storage.RecordObservation(resource)
        require.NoError(t, err)
    }

    // Then: Query back
    state, err := storage.GetResourceState("web-asg-prod")
    require.NoError(t, err)
    assert.Equal(t, "autoscaling_group", state.Resource.Type)
}
```

## Interfaces Needed

**No new interfaces!** Use existing `CloudProvider`:

```go
type CloudProvider interface {
    ListResources(ctx context.Context, filter ResourceFilter) ([]Resource, error)
    // ... existing methods
}
```

Just add methods to `RealAWSProvider`:
- `ListAutoScalingGroups()`
- `ListEKSClusters()`
- `ListEKSNodeGroups()`
- `ListDBSubnetGroups()`
- `ListSubnets()`
- `ListRouteTables()`
- `ListInternetGateways()`
- `ListNATGateways()`
- `ListVPCPeeringConnections()`
- `ListTargetGroups()`

And update `ListResources()` to call them.

## Breaking Into Smaller Functions

Each scanner:
1. **AWS API call** (1 function)
2. **Convert to resources** (loop + helper)
3. **Helper: buildXResource()** (extract data)
4. **Helper: convertXTags()** (tag conversion)

Example:
```
ListAutoScalingGroups()          <-- 20 lines
  ├─ DescribeAutoScalingGroups() <-- AWS SDK call
  └─ buildASGResource()          <-- 30 lines
       ├─ convertASGTags()       <-- 10 lines
       └─ extractLaunchTemplate() <-- 5 lines
```

## Success Criteria

- [ ] ASG scanner captures instance_ids, launch_template, target_group_arns
- [ ] EKS scanner captures cluster + node groups with ASG references
- [ ] RDS scanner captures Multi-AZ, read replicas, subnet groups
- [ ] VPC scanners capture subnets, route tables, NAT/IGW, peering
- [ ] Target group scanner captures ALB/NLB relationships
- [ ] All functions <50 lines
- [ ] All scanners have unit tests
- [ ] Integration tests with storage pass
- [ ] No map[string]interface{} usage
- [ ] Error handling with context

## Next Steps

1. Start with ASG scanner (simplest, unlocks EKS)
2. Branch: `feat/asg-scanner`
3. Write test first: `TestBuildASGResource`
4. Implement `buildASGResource()`
5. Implement `ListAutoScalingGroups()`
6. Test → fmt → vet → lint → commit
7. Move to EKS scanner

---

**Decision: Start with ASG** - Foundation for EKS node group relationships and simplest of the five scanners.
