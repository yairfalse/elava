# Elava → Ahti Integration Design

**Status:** Draft - Critical decisions pending
**Date:** 2025-10-15

## Product Vision

### Elava Standalone (Community)
**Positioning:** "Cloud Gatekeeper & Friend" - Infrastructure memory for teams with messy cloud accounts

**Core Value (No Ahti Required):**
- Inventory management (what do we have?)
- Change tracking (what changed?)
- Time-travel queries (what did we have?)
- Tag compliance (what's missing tags?)
- Natural language queries (friendly CLI)

**CLI Examples:**
```bash
# Inventory
elava show ec2 --account prod
elava inventory tags missing --required "Environment,Team,Owner"

# Temporal (unique!)
elava history changes --since "24h ago"
elava history resource sg-abc123 --at "2025-10-01"
elava history disappeared --type s3_bucket

# Friendly queries
elava explain i-abc123
elava find "resources with no Owner tag"
elava watch changes --alert-on "security_group_opened"
```

### Elava Enterprise + Ahti
**Additional Value:**
- Correlation with Tapio (K8s observability)
- Root cause analysis (infrastructure + runtime events)
- Cross-system visibility
- Continuous monitoring (not just scans)

**Example Correlation:**
```
Tapio: "Pod OOMKilled at 14:32"
  ↓
Ahti: "Which node?"
  ↓
Elava: "EC2 instance i-abc123 (t2.micro, 2 years old)"
  ↓
Ahti: "Root cause: Node undersized, recommend t3.medium"
```

---

## Integration Architecture

### High-Level Flow

```
┌──────────────────────────────────────┐
│         Elava (standalone)            │
│                                       │
│  AWS Scanner → BadgerDB → CLI/API    │
│                                       │
│  Enterprise: Send events to Ahti      │
│  via gRPC/webhooks                   │
└─────────────────┬────────────────────┘
                  │
                  │ InfrastructureChangeEvent
                  │ (Elava's native format)
                  ▼
┌──────────────────────────────────────┐
│          Ahti Plugin Registry         │
│                                       │
│  ┌────────────────────────────┐      │
│  │  Elava Plugin              │      │
│  │  - Receives Elava events   │      │
│  │  - Converts to UnifiedEvent│      │
│  │  - Extracts entities/rels  │      │
│  └──────────┬─────────────────┘      │
│             │                         │
│             ▼                         │
│  ┌────────────────────────────┐      │
│  │  Tapio Plugin              │      │
│  │  - Receives TapioEvent     │      │
│  │  - Already has entities    │      │
│  └──────────┬─────────────────┘      │
│             │                         │
│             ▼                         │
│       UnifiedEvent Pipeline           │
│       → Graph Database                │
└──────────────────────────────────────┘
```

---

## Design Decisions (Confirmed)

### 1. Plugin Location
**Decision:** Ahti repo (`ahti/plugins/elava/`)

**Rationale:**
- Ahti owns the integration logic
- Can update plugin independently of Elava
- Versioning controlled by Ahti
- Keeps Elava simple and decoupled

### 2. Event Transport
**Decision:** Elava pushes events (gRPC or webhooks)

**Rationale:**
- Real-time (no polling delay)
- Elava controls when to send
- Works for both SaaS and on-prem deployments

**Configuration Examples:**
```yaml
# SaaS deployment
ahti:
  endpoint: "grpc.ahti.yourcompany.com:443"
  token: "customer-api-token"

# On-Prem deployment
ahti:
  endpoint: "ahti-service.observability.svc.cluster.local:9090"
  token: "internal-token"
```

### 3. Event Format
**Decision:** Strongly typed infrastructure events

**Rationale:**
- Type safety (no map[string]interface{})
- Clear schema (protobuf or Go structs)
- Extensible (add new resource types easily)
- Testable (can validate structure)

### 4. Extraction Responsibility
**Decision:** Plugin extracts entities and relationships

**Rationale:**
- Keeps Elava simple (doesn't need to know about Ahti's graph model)
- Plugin can evolve extraction logic independently
- Elava just sends raw infrastructure data
- Separation of concerns

---

## Event Schema (Draft)

### InfrastructureChangeEvent (Elava's Native Format)

```go
type InfrastructureChangeEvent struct {
    // Core metadata
    ID           string    `json:"id"`
    Timestamp    time.Time `json:"timestamp"`
    ChangeType   string    `json:"change_type"` // created, modified, disappeared

    // AWS context
    Provider     string `json:"provider"`   // "aws" (future: "gcp", "azure")
    AccountID    string `json:"account_id"`
    Region       string `json:"region"`

    // Resource identification
    ResourceType string `json:"resource_type"` // "ec2_instance", "rds_instance", etc.
    ResourceID   string `json:"resource_id"`   // "i-abc123"

    // Resource data (strongly typed - one field per resource type)
    EC2Instance       *EC2InstanceData       `json:"ec2_instance,omitempty"`
    AutoScalingGroup  *AutoScalingGroupData  `json:"autoscaling_group,omitempty"`
    EKSCluster        *EKSClusterData        `json:"eks_cluster,omitempty"`
    EKSNodeGroup      *EKSNodeGroupData      `json:"eks_node_group,omitempty"`
    RDSInstance       *RDSInstanceData       `json:"rds_instance,omitempty"`
    DBSubnetGroup     *DBSubnetGroupData     `json:"db_subnet_group,omitempty"`
    S3Bucket          *S3BucketData          `json:"s3_bucket,omitempty"`
    VPC               *VPCData               `json:"vpc,omitempty"`
    Subnet            *SubnetData            `json:"subnet,omitempty"`
    RouteTable        *RouteTableData        `json:"route_table,omitempty"`
    InternetGateway   *InternetGatewayData   `json:"internet_gateway,omitempty"`
    NATGateway        *NATGatewayData        `json:"nat_gateway,omitempty"`
    VPCPeering        *VPCPeeringData        `json:"vpc_peering,omitempty"`
    SecurityGroup     *SecurityGroupData     `json:"security_group,omitempty"`
    LoadBalancer      *LoadBalancerData      `json:"load_balancer,omitempty"`
    TargetGroup       *TargetGroupData       `json:"target_group,omitempty"`
    // ... additional resource types as needed
}

type EC2InstanceData struct {
    InstanceType       string            `json:"instance_type"`
    State              string            `json:"state"` // running, stopped, terminated
    PrivateIP          string            `json:"private_ip,omitempty"`
    PublicIP           string            `json:"public_ip,omitempty"`
    VpcID              string            `json:"vpc_id,omitempty"`
    SubnetID           string            `json:"subnet_id,omitempty"`
    SecurityGroupIDs   []string          `json:"security_group_ids,omitempty"`
    IAMRole            string            `json:"iam_role,omitempty"`
    AutoScalingGroupName string          `json:"autoscaling_group_name,omitempty"` // Which ASG owns this
    Tags               map[string]string `json:"tags,omitempty"`
    LaunchTime         time.Time         `json:"launch_time,omitempty"`
}

type AutoScalingGroupData struct {
    Name                string            `json:"name"`
    MinSize             int32             `json:"min_size"`
    MaxSize             int32             `json:"max_size"`
    DesiredCapacity     int32             `json:"desired_capacity"`
    CurrentSize         int32             `json:"current_size"`         // Actual running instances
    InstanceIDs         []string          `json:"instance_ids"`         // References to EC2 instances
    LaunchTemplate      string            `json:"launch_template,omitempty"`
    LaunchConfigName    string            `json:"launch_config_name,omitempty"`
    TargetGroupARNs     []string          `json:"target_group_arns,omitempty"` // Load balancer targets
    LoadBalancerNames   []string          `json:"load_balancer_names,omitempty"`
    VPCZoneIdentifiers  []string          `json:"vpc_zone_identifiers,omitempty"` // Subnet IDs
    HealthCheckType     string            `json:"health_check_type,omitempty"`    // EC2, ELB
    HealthCheckGracePeriod int32          `json:"health_check_grace_period,omitempty"`
    DefaultCooldown     int32             `json:"default_cooldown,omitempty"`
    Tags                map[string]string `json:"tags,omitempty"`
    CreatedTime         time.Time         `json:"created_time,omitempty"`
}

type VPCData struct {
    CIDR      string            `json:"cidr"`
    State     string            `json:"state"`
    IsDefault bool              `json:"is_default"`
    Tags      map[string]string `json:"tags,omitempty"`
}

type SecurityGroupData struct {
    VpcID        string            `json:"vpc_id"`
    Description  string            `json:"description"`
    IngressRules []SecurityRule    `json:"ingress_rules,omitempty"`
    EgressRules  []SecurityRule    `json:"egress_rules,omitempty"`
    Tags         map[string]string `json:"tags,omitempty"`
}

type SecurityRule struct {
    Protocol   string `json:"protocol"` // tcp, udp, icmp, -1 (all)
    FromPort   int32  `json:"from_port"`
    ToPort     int32  `json:"to_port"`
    CIDRBlock  string `json:"cidr_block,omitempty"`
    SourceSG   string `json:"source_sg,omitempty"`
}

type LoadBalancerData struct {
    Type              string            `json:"type"`              // application, network, gateway
    Scheme            string            `json:"scheme"`            // internet-facing, internal
    DNSName           string            `json:"dns_name"`
    VpcID             string            `json:"vpc_id,omitempty"`
    SubnetIDs         []string          `json:"subnet_ids,omitempty"`
    SecurityGroupIDs  []string          `json:"security_group_ids,omitempty"`
    TargetGroupARNs   []string          `json:"target_group_arns,omitempty"` // References
    State             string            `json:"state,omitempty"`   // active, provisioning, failed
    IPAddressType     string            `json:"ip_address_type,omitempty"` // ipv4, dualstack
    Tags              map[string]string `json:"tags,omitempty"`
    CreatedTime       time.Time         `json:"created_time,omitempty"`
}

type TargetGroupData struct {
    Protocol          string            `json:"protocol"`          // HTTP, HTTPS, TCP, TLS
    Port              int32             `json:"port"`
    VpcID             string            `json:"vpc_id,omitempty"`
    TargetType        string            `json:"target_type"`       // instance, ip, lambda
    TargetIDs         []string          `json:"target_ids,omitempty"` // EC2 instance IDs or IPs
    HealthCheckPath   string            `json:"health_check_path,omitempty"`
    HealthCheckPort   int32             `json:"health_check_port,omitempty"`
    HealthCheckProtocol string          `json:"health_check_protocol,omitempty"`
    LoadBalancerARNs  []string          `json:"load_balancer_arns,omitempty"`
    Tags              map[string]string `json:"tags,omitempty"`
}

type EKSClusterData struct {
    Name              string            `json:"name"`
    Version           string            `json:"version"`             // K8s version (e.g., "1.28")
    Status            string            `json:"status"`              // ACTIVE, CREATING, DELETING
    Endpoint          string            `json:"endpoint"`            // API server endpoint
    RoleARN           string            `json:"role_arn"`            // Cluster IAM role
    VpcID             string            `json:"vpc_id"`
    SubnetIDs         []string          `json:"subnet_ids"`          // Control plane subnets
    SecurityGroupIDs  []string          `json:"security_group_ids"` // Cluster security group
    NodeGroupNames    []string          `json:"node_group_names,omitempty"` // Managed node groups
    FargateProfiles   []string          `json:"fargate_profiles,omitempty"`
    Tags              map[string]string `json:"tags,omitempty"`
    CreatedAt         time.Time         `json:"created_at,omitempty"`
}

type EKSNodeGroupData struct {
    ClusterName          string            `json:"cluster_name"`    // Which EKS cluster owns this
    NodeGroupName        string            `json:"node_group_name"`
    Status               string            `json:"status"`          // ACTIVE, CREATING, DEGRADED
    NodeRole             string            `json:"node_role"`       // IAM role for nodes
    InstanceTypes        []string          `json:"instance_types"`  // e.g., ["t3.medium", "t3.large"]
    AmiType              string            `json:"ami_type"`        // AL2_x86_64, AL2_ARM_64, etc.
    MinSize              int32             `json:"min_size"`
    MaxSize              int32             `json:"max_size"`
    DesiredSize          int32             `json:"desired_size"`
    CurrentSize          int32             `json:"current_size"`    // Actual running nodes
    AutoScalingGroupName string            `json:"autoscaling_group_name,omitempty"` // References ASG
    SubnetIDs            []string          `json:"subnet_ids"`      // Node placement subnets
    LaunchTemplate       string            `json:"launch_template,omitempty"`
    RemoteAccessSG       string            `json:"remote_access_sg,omitempty"` // SSH security group
    Labels               map[string]string `json:"labels,omitempty"`    // K8s node labels
    Taints               []NodeTaint       `json:"taints,omitempty"`    // K8s taints
    Tags                 map[string]string `json:"tags,omitempty"`
    CreatedAt            time.Time         `json:"created_at,omitempty"`
}

type NodeTaint struct {
    Key    string `json:"key"`
    Value  string `json:"value,omitempty"`
    Effect string `json:"effect"` // NoSchedule, NoExecute, PreferNoSchedule
}

type RDSInstanceData struct {
    // Instance identification
    DBInstanceIdentifier string `json:"db_instance_identifier"`
    DBInstanceClass      string `json:"db_instance_class"`  // db.t3.medium, db.r5.large
    Engine               string `json:"engine"`              // postgres, mysql, aurora
    EngineVersion        string `json:"engine_version"`

    // Network placement
    VpcID                string   `json:"vpc_id"`
    DBSubnetGroupName    string   `json:"db_subnet_group_name"`
    AvailabilityZone     string   `json:"availability_zone"`
    MultiAZ              bool     `json:"multi_az"`
    SecondaryAZ          string   `json:"secondary_az,omitempty"`
    VPCSecurityGroupIDs  []string `json:"vpc_security_group_ids"`
    PubliclyAccessible   bool     `json:"publicly_accessible"`
    Endpoint             string   `json:"endpoint"`  // DNS name
    Port                 int32    `json:"port"`

    // Storage
    AllocatedStorage     int32  `json:"allocated_storage"`  // GB
    StorageType          string `json:"storage_type"`       // gp2, gp3, io1
    StorageEncrypted     bool   `json:"storage_encrypted"`
    IOPS                 int32  `json:"iops,omitempty"`

    // Replication
    ReadReplicaDBIdentifiers []string `json:"read_replica_db_identifiers,omitempty"`
    ReadReplicaSourceDBIdentifier string `json:"read_replica_source_db_identifier,omitempty"`

    // Backup/HA
    BackupRetentionPeriod int32     `json:"backup_retention_period"`
    PreferredBackupWindow string    `json:"preferred_backup_window,omitempty"`
    LatestRestorableTime  time.Time `json:"latest_restorable_time,omitempty"`

    // Monitoring
    EnhancedMonitoring   bool   `json:"enhanced_monitoring"`
    PerformanceInsights  bool   `json:"performance_insights"`

    // State
    Status               string            `json:"status"` // available, backing-up, modifying
    Tags                 map[string]string `json:"tags,omitempty"`
    CreatedTime          time.Time         `json:"created_time,omitempty"`
}

type DBSubnetGroupData struct {
    Name        string   `json:"name"`
    VpcID       string   `json:"vpc_id"`
    SubnetIDs   []string `json:"subnet_ids"`  // Must be in different AZs for Multi-AZ
    Description string   `json:"description,omitempty"`
}

type SubnetData struct {
    SubnetID              string            `json:"subnet_id"`
    VpcID                 string            `json:"vpc_id"`
    CIDR                  string            `json:"cidr"`            // 10.0.1.0/24
    AvailabilityZone      string            `json:"availability_zone"`
    AvailabilityZoneID    string            `json:"availability_zone_id"` // use1-az1
    MapPublicIPOnLaunch   bool              `json:"map_public_ip_on_launch"` // Public subnet?
    RouteTableID          string            `json:"route_table_id"`
    Tags                  map[string]string `json:"tags,omitempty"`
}

type RouteTableData struct {
    RouteTableID string            `json:"route_table_id"`
    VpcID        string            `json:"vpc_id"`
    Routes       []Route           `json:"routes"`
    SubnetIDs    []string          `json:"subnet_ids"` // Associated subnets
    IsMain       bool              `json:"is_main"`    // Main route table?
    Tags         map[string]string `json:"tags,omitempty"`
}

type Route struct {
    DestinationCIDR      string `json:"destination_cidr"`       // 0.0.0.0/0
    TargetType           string `json:"target_type"`            // igw, nat, tgw, pcx
    TargetID             string `json:"target_id"`              // igw-abc123
    State                string `json:"state"`                  // active, blackhole
}

type InternetGatewayData struct {
    InternetGatewayID string   `json:"internet_gateway_id"`
    VpcID             string   `json:"vpc_id"`
    State             string   `json:"state"` // available, attached
    Tags              map[string]string `json:"tags,omitempty"`
}

type NATGatewayData struct {
    NatGatewayID     string   `json:"nat_gateway_id"`
    VpcID            string   `json:"vpc_id"`
    SubnetID         string   `json:"subnet_id"`      // Which subnet NAT is in (must be public)
    ElasticIP        string   `json:"elastic_ip"`
    State            string   `json:"state"`          // available, pending, deleting
    Tags             map[string]string `json:"tags,omitempty"`
}

type VPCPeeringData struct {
    VpcPeeringConnectionID string `json:"vpc_peering_connection_id"`
    RequesterVpcID         string `json:"requester_vpc_id"`
    AccepterVpcID          string `json:"accepter_vpc_id"`
    Status                 string `json:"status"` // active, pending-acceptance
    Tags                   map[string]string `json:"tags,omitempty"`
}

// ... more resource types
```

### Entity Type Naming Convention

**Decision:** Use namespaced format

```go
const (
    // Kubernetes entities (Tapio)
    EntityTypePod         EntityType = "k8s.pod"
    EntityTypeNode        EntityType = "k8s.node"
    EntityTypeDeployment  EntityType = "k8s.deployment"

    // AWS entities (Elava)
    EntityTypeEC2Instance      EntityType = "aws.ec2_instance"
    EntityTypeAutoScalingGroup EntityType = "aws.autoscaling_group"
    EntityTypeEKSCluster       EntityType = "aws.eks_cluster"
    EntityTypeEKSNodeGroup     EntityType = "aws.eks_node_group"
    EntityTypeRDSInstance      EntityType = "aws.rds_instance"
    EntityTypeDBSubnetGroup    EntityType = "aws.db_subnet_group"
    EntityTypeS3Bucket         EntityType = "aws.s3_bucket"
    EntityTypeVPC              EntityType = "aws.vpc"
    EntityTypeSubnet           EntityType = "aws.subnet"
    EntityTypeRouteTable       EntityType = "aws.route_table"
    EntityTypeInternetGateway  EntityType = "aws.internet_gateway"
    EntityTypeNATGateway       EntityType = "aws.nat_gateway"
    EntityTypeVPCPeering       EntityType = "aws.vpc_peering"
    EntityTypeSecurityGroup    EntityType = "aws.security_group"
    EntityTypeLoadBalancer     EntityType = "aws.load_balancer"
    EntityTypeTargetGroup      EntityType = "aws.target_group"
    EntityTypeLaunchTemplate   EntityType = "aws.launch_template"
    EntityTypeIAMRole          EntityType = "aws.iam_role"

    // Future: GCP, Azure
    EntityTypeGCEInstance    EntityType = "gcp.gce_instance"
    EntityTypeAzureVM        EntityType = "azure.vm"
)
```

**Rationale:**
- Clear provider separation
- Avoids naming conflicts
- Scales to multi-cloud
- Keeps domain package focused on Tapio's K8s entities

---

## Critical Design Decisions (RESOLVED)

### Core Philosophy

**"Show changes, show what we are sure is there."**

Elava's promise is to report exactly what exists in cloud infrastructure - nothing more, nothing less. No assumptions, no inferences, no ghost entities. Just facts: what exists, what changed, what disappeared.

**Why conservative matters for cloud infrastructure:**
- Cloud is low-level (raw VPC IDs, not managed relationships)
- Resources live for months/years (not seconds like K8s pods)
- Scans happen every 15-30 minutes (not milliseconds)
- Broken references are real problems to detect (drift, orphans)
- Stub entities could stay stubs forever (deleted resources)

---

### Decision 1: Entity Extraction Strategy ✅

**DECISION: Conservative (Option C) - Create only when resource is directly scanned**

```go
// EC2 instance scan
Entities: [
    {
        Type: "aws.ec2_instance",
        ID: "i-abc123",
        Name: "web-server-1",
        Attributes: {
            "instance_type": "t3.medium",
            "vpc_id": "vpc-xyz789",        // Stored as reference, NOT entity
            "subnet_id": "subnet-456",      // Stored as reference, NOT entity
            "security_group_ids": ["sg-111"] // Stored as reference, NOT entity
        }
    }
]

// Later: VPC scan creates VPC entity
Entities: [
    {Type: "aws.vpc", ID: "vpc-xyz789"}
]
// Ahti plugin creates relationship: i-abc123 → vpc-xyz789
```

**Rationale:**
- ✅ Only entities with confirmed, complete data
- ✅ No ghost entities from deleted resources
- ✅ References as attributes enable drift detection
- ✅ "VPC doesn't exist but EC2 references it" = orphaned reference
- ✅ Clean graph that reflects reality

**Trade-offs Accepted:**
- ⏳ Relationships delayed until both entities scanned (acceptable for infrastructure)
- ⏳ Can't query "show VPC topology" until VPC scan completes (scans finish in minutes)

---

### Decision 2: Relationship Inference Strategy ✅

**DECISION: Lazy (Scenario 2) - Create relationships only when both entities exist**

```go
// Implementation in Ahti plugin:

func (p *ElavaPlugin) Ingest(ctx context.Context, data []byte) ([]UnifiedEvent, error) {
    // 1. Create entity for scanned resource only
    entity := createEntityFromScan(event)

    // 2. Store pending relationships in attributes
    //    Example: EC2 entity has vpc_id="vpc-xyz789" in attributes

    // 3. Background reconciliation job:
    //    - Query graph: "Does vpc-xyz789 entity exist?"
    //    - If YES: Create relationship i-abc123 → vpc-xyz789
    //    - If NO: Keep as pending (will be created when VPC scanned)

    return []UnifiedEvent{unified}, nil
}
```

**Rationale:**
- ✅ No dangling relationships to non-existent entities
- ✅ Graph integrity maintained (all edges have valid nodes)
- ✅ Broken references become visible drift issues
- ✅ Relationships emerge organically as scans complete

**Implementation Notes:**
- Ahti plugin stores references in entity attributes
- Background job reconciles relationships after each scan
- Complexity contained in Ahti (Elava stays simple)

---

### Decision 3: Scan Ordering ✅

**DECISION: Order-independent - Scan any resource type in any order**

**Rationale:**
- ✅ Multi-account scans inherently unordered
- ✅ Each scan is independent (entities + references)
- ✅ Relationships resolved by background reconciliation
- ✅ Simpler implementation (no dependency tracking)

**Examples:**
```
Scenario A: EC2 → VPC order
1. Scan EC2 → Create EC2 entity (with vpc_id in attributes)
2. Scan VPC → Create VPC entity
3. Reconcile → Create relationship EC2 → VPC

Scenario B: VPC → EC2 order
1. Scan VPC → Create VPC entity
2. Scan EC2 → Create EC2 entity (with vpc_id in attributes)
3. Reconcile → Create relationship EC2 → VPC (immediately, VPC exists)

Result: Same final graph, regardless of order
```

---

### Summary: Conservative Graph Building

**Pattern: "Conservative entity creation with lazy relationship resolution"**

1. **Scan Phase:** Create entity only for directly scanned resource
2. **Reference Phase:** Store relationships as attributes (not edges)
3. **Reconciliation Phase:** Create edges when both entities exist

**Benefits:**
- Clean graph with only confirmed entities
- Drift detection (broken references visible)
- Order-independent scans
- No ghost entities
- Infrastructure reality reflected accurately

---

## Plugin Implementation (Draft)

### Plugin Interface (Ahti)

```go
// In ahti/plugins/elava/plugin.go

type ElavaPlugin struct {
    config ElavaConfig
}

func (p *ElavaPlugin) Name() string {
    return "elava"
}

func (p *ElavaPlugin) Version() string {
    return "1.0.0"
}

func (p *ElavaPlugin) Description() string {
    return "AWS infrastructure scanner - continuous cloud inventory with temporal memory"
}

// Ingest converts Elava infrastructure events → UnifiedEvent
func (p *ElavaPlugin) Ingest(ctx context.Context, data []byte) ([]UnifiedEvent, error) {
    var elavaEvent InfrastructureChangeEvent
    if err := json.Unmarshal(data, &elavaEvent); err != nil {
        return nil, fmt.Errorf("failed to parse Elava event: %w", err)
    }

    // Convert to UnifiedEvent
    unified := UnifiedEvent{
        ID:        elavaEvent.ID,
        Timestamp: elavaEvent.Timestamp,
        Type:      "infrastructure",  // NEW event type
        Subtype:   elavaEvent.ChangeType,  // created, modified, disappeared
        Severity:  mapSeverity(elavaEvent),
        Outcome:   OutcomeSuccess,

        // Extract entities (strategy TBD)
        Entities: extractEntities(elavaEvent),

        // Extract relationships (strategy TBD)
        Relationships: extractRelationships(elavaEvent),

        SourcePlugin: "elava",
        RawData: map[string]interface{}{
            "provider":      elavaEvent.Provider,
            "resource_type": elavaEvent.ResourceType,
            "resource_id":   elavaEvent.ResourceID,
            "account_id":    elavaEvent.AccountID,
            "region":        elavaEvent.Region,
        },
    }

    return []UnifiedEvent{unified}, nil
}

// extractEntities - Implementation depends on entity strategy decision
func extractEntities(event InfrastructureChangeEvent) []Entity {
    // TODO: Implement based on entity extraction strategy
    return nil
}

// extractRelationships - Implementation depends on relationship strategy decision
func extractRelationships(event InfrastructureChangeEvent) []Relationship {
    // TODO: Implement based on relationship inference strategy
    return nil
}
```

---

## Relationship Types (Draft)

```go
const (
    // Existing Tapio relationships
    RelationshipConnectsTo RelationshipType = "connects_to"
    RelationshipManages    RelationshipType = "manages"
    RelationshipDependsOn  RelationshipType = "depends_on"
    RelationshipContains   RelationshipType = "contains"

    // Infrastructure relationships (Elava)
    RelationshipRunsIn        RelationshipType = "runs_in"        // EC2 → VPC, EKSCluster → VPC, RDS → VPC
    RelationshipAttachedTo    RelationshipType = "attached_to"    // EC2 → Subnet
    RelationshipRoutesVia     RelationshipType = "routes_via"     // Subnet → RouteTable → IGW/NAT
    RelationshipProtectedBy   RelationshipType = "protected_by"   // EC2/RDS → SecurityGroup
    RelationshipAssumedBy     RelationshipType = "assumed_by"     // EC2 → IAMRole
    RelationshipTargets       RelationshipType = "targets"        // TargetGroup → EC2
    RelationshipRegistersTo   RelationshipType = "registers_to"   // ASG → TargetGroup
    RelationshipUsesTemplate  RelationshipType = "uses_template"  // ASG → LaunchTemplate
    RelationshipUsesSubnetGroup RelationshipType = "uses_subnet_group" // RDS → DBSubnetGroup
    RelationshipOwns          RelationshipType = "owns"           // ASG → EC2, EKSNodeGroup → EC2
    RelationshipRoutesThrough RelationshipType = "routes_through" // LoadBalancer → TargetGroup, RouteTable → IGW
    RelationshipDeploysIn     RelationshipType = "deploys_in"     // ASG → Subnet, EKSNodeGroup → Subnet
    RelationshipBelongsTo     RelationshipType = "belongs_to"     // EKSNodeGroup → EKSCluster, Subnet → VPC
    RelationshipManagedBy     RelationshipType = "managed_by"     // EC2 → EKSNodeGroup (via ASG)
    RelationshipReplicatesFrom RelationshipType = "replicates_from" // RDS ReadReplica → Primary
    RelationshipPeersVia      RelationshipType = "peers_via"      // VPC → VPCPeering → VPC
    RelationshipNATsThrough   RelationshipType = "nats_through"   // PrivateSubnet → NATGateway
    RelationshipPublicAccess  RelationshipType = "public_access"  // PublicSubnet → InternetGateway
)
```

---

## Example: Complete EC2 Event Flow

### 1. Elava Scans EC2 Instance

```json
{
  "id": "evt-123",
  "timestamp": "2025-10-15T14:32:00Z",
  "change_type": "created",
  "provider": "aws",
  "account_id": "123456789",
  "region": "us-east-1",
  "resource_type": "ec2_instance",
  "resource_id": "i-abc123",
  "ec2_instance": {
    "instance_type": "t3.medium",
    "state": "running",
    "private_ip": "10.0.1.50",
    "vpc_id": "vpc-xyz789",
    "subnet_id": "subnet-456",
    "security_group_ids": ["sg-111", "sg-222"],
    "tags": {
      "Name": "web-server-1",
      "Environment": "production",
      "Team": "platform"
    },
    "launch_time": "2025-10-15T14:30:00Z"
  }
}
```

### 2. Plugin Converts to UnifiedEvent

```go
UnifiedEvent{
    ID:        "evt-123",
    Timestamp: "2025-10-15T14:32:00Z",
    Type:      "infrastructure",
    Subtype:   "created",
    Severity:  "info",
    Outcome:   "success",

    Entities: [
        {
            Type: "aws.ec2_instance",
            ID:   "i-abc123",
            Name: "web-server-1",
            Attributes: {
                "account_id": "123456789",
                "region": "us-east-1",
                "instance_type": "t3.medium",
                "state": "running",
                "private_ip": "10.0.1.50",
            },
            Labels: {
                "Environment": "production",
                "Team": "platform",
            },
        },
        // Additional entities based on strategy decision
    ],

    Relationships: [
        // Relationships based on strategy decision
    ],

    SourcePlugin: "elava",
}
```

### 3. Ahti Stores in Graph

```
┌─────────────────┐
│  aws.vpc        │
│  vpc-xyz789     │
└────────┬────────┘
         │ runs_in
         ▼
┌─────────────────┐      protected_by     ┌──────────────────┐
│ aws.ec2_instance│◀────────────────────▶│ aws.security_group│
│ i-abc123        │                       │ sg-111           │
│ web-server-1    │                       └──────────────────┘
└────────┬────────┘
         │ attached_to
         ▼
┌─────────────────┐
│  aws.subnet     │
│  subnet-456     │
└─────────────────┘
```

### 4. Later: Correlation with Tapio

```
Tapio event: "Pod OOMKilled on node ip-10-0-1-50"
  ↓
Ahti query: "Which EC2 instance has private_ip = 10.0.1.50?"
  ↓
Graph result: aws.ec2_instance i-abc123 (t3.medium)
  ↓
Ahti analysis: "Node undersized - recommend t3.large"
```

---

## Example: Auto Scaling Group Graph

### ASG Scan Flow

**1. Scan Auto Scaling Group**
```json
{
  "resource_type": "autoscaling_group",
  "resource_id": "web-asg-prod",
  "autoscaling_group": {
    "name": "web-asg-prod",
    "min_size": 2,
    "max_size": 10,
    "desired_capacity": 5,
    "current_size": 5,
    "instance_ids": ["i-abc123", "i-def456", "i-ghi789", "i-jkl012", "i-mno345"],
    "launch_template": "lt-xyz789",
    "target_group_arns": ["arn:aws:elasticloadbalancing:...:targetgroup/web-tg/abc123"],
    "vpc_zone_identifiers": ["subnet-111", "subnet-222", "subnet-333"]
  }
}
```

**2. Resulting Graph (After All Scans Complete)**

```
┌──────────────────────┐
│ aws.launch_template  │
│ lt-xyz789            │
└──────────┬───────────┘
           │ uses_template
           ▼
┌──────────────────────┐      owns          ┌────────────────┐
│ aws.autoscaling_group├──────────────────►│ aws.ec2_instance│
│ web-asg-prod         │                    │ i-abc123       │
│ (5/5 instances)      ├──────┐             └────────┬───────┘
└──────────┬───────────┘      │                      │ runs_in
           │                  │ owns                 ▼
           │ registers_to     │             ┌────────────────┐
           │                  └────────────►│ aws.vpc        │
           ▼                                │ vpc-xyz789     │
┌──────────────────────┐                    └────────────────┘
│ aws.target_group     │
│ web-tg               │
│ (5 healthy targets)  │
└──────────┬───────────┘
           │ routes_through
           ▼
┌──────────────────────┐      runs_in      ┌────────────────┐
│ aws.load_balancer    ├──────────────────►│ aws.subnet     │
│ web-alb              │                    │ subnet-111     │
│ (internet-facing)    │                    └────────────────┘
└──────────────────────┘
```

**3. Query Examples**

```
Q: "Which ASG owns instance i-abc123?"
A: aws.autoscaling_group web-asg-prod

Q: "How many instances should this ASG have?"
A: Desired: 5, Current: 5, Min: 2, Max: 10

Q: "ASG scaled down from 5 to 3 - which instances disappeared?"
A: (Temporal query) i-jkl012, i-mno345 terminated at 14:32

Q: "Which load balancer receives traffic for this ASG?"
A: aws.load_balancer web-alb (via target group web-tg)

Q: "ASG health check failing - why?"
A: (Correlation) TargetGroup health check on port 8080, but SecurityGroup blocks 8080
```

**4. Incident Correlation: ASG + Tapio**

```
14:30 - ASG scales up (5 → 8 instances)
14:31 - Tapio: "New pods scheduled on nodes ip-10-0-1-60, ip-10-0-1-61, ip-10-0-1-62"
14:32 - Tapio: "Pods on new nodes failing health checks"
  ↓
Ahti correlation:
  - New EC2 instances: i-pqr678, i-stu901, i-vwx234
  - SecurityGroup sg-111 blocks port 8080
  - ASG launched instances without updating security group
  ↓
Root cause: "ASG scaled but security group not updated - new instances can't receive traffic"
```

---

## Example: EKS Cluster Graph (Tapio + Elava Bridge)

### The Critical Link: K8s Nodes → AWS Infrastructure

**1. Scan EKS Cluster**
```json
{
  "resource_type": "eks_cluster",
  "resource_id": "prod-cluster",
  "eks_cluster": {
    "name": "prod-cluster",
    "version": "1.28",
    "status": "ACTIVE",
    "endpoint": "https://ABCD1234.gr7.us-east-1.eks.amazonaws.com",
    "vpc_id": "vpc-xyz789",
    "subnet_ids": ["subnet-111", "subnet-222", "subnet-333"],
    "security_group_ids": ["sg-cluster-abc"],
    "node_group_names": ["prod-workers", "prod-spot"]
  }
}
```

**2. Scan EKS Node Group**
```json
{
  "resource_type": "eks_node_group",
  "resource_id": "prod-workers",
  "eks_node_group": {
    "cluster_name": "prod-cluster",
    "node_group_name": "prod-workers",
    "status": "ACTIVE",
    "instance_types": ["t3.large"],
    "min_size": 2,
    "max_size": 10,
    "desired_size": 5,
    "current_size": 5,
    "autoscaling_group_name": "eks-prod-workers-asg",
    "subnet_ids": ["subnet-111", "subnet-222"],
    "labels": {
      "node-role.kubernetes.io/worker": "true",
      "workload-type": "general"
    }
  }
}
```

**3. Resulting Graph**

```
┌────────────────────┐
│ k8s.node           │  ← From Tapio
│ ip-10-0-1-50       │
│ (K8s node)         │
└──────────┬─────────┘
           │
           │ Correlation: IP → EC2
           ▼
┌────────────────────┐      managed_by    ┌─────────────────────┐
│ aws.ec2_instance   ├──────────────────►│ aws.eks_node_group  │
│ i-abc123           │                    │ prod-workers        │
│ t3.large           │                    │ (5 nodes)           │
└──────────┬─────────┘                    └──────────┬──────────┘
           │                                         │
           │ runs_in                                 │ belongs_to
           │                                         ▼
           │                              ┌─────────────────────┐
           │                              │ aws.eks_cluster     │
           │                              │ prod-cluster        │
           │                              │ K8s v1.28           │
           ▼                              └──────────┬──────────┘
┌────────────────────┐                              │
│ aws.vpc            │◄─────────────────────────────┘ runs_in
│ vpc-xyz789         │
└────────────────────┘
```

**4. The Magic: Cross-System Correlation**

**Scenario: Pod Crashes**
```
14:30 - Tapio: "Pod web-app-xyz OOMKilled on node ip-10-0-1-50"
  ↓
Step 1: Tapio provides K8s node name
  - Node: ip-10-0-1-50
  - Labels: {node-role.kubernetes.io/worker: true}

Step 2: Match K8s node to EC2 instance (by private IP)
  - Elava: "Node ip-10-0-1-50 = EC2 instance i-abc123"

Step 3: Traverse infrastructure graph
  - EC2 i-abc123 → managed_by → EKS Node Group prod-workers
  - EKS Node Group → belongs_to → EKS Cluster prod-cluster
  - EC2 i-abc123 → Instance type: t3.large (8 GB RAM)

Step 4: Root cause analysis
  - Pod requested 6 GB memory
  - Node has 8 GB total, running 4 pods
  - Node is oversaturated (memory pressure)
  - Node Group: desired_size=5, current_size=5 (at max)

Step 5: Ahti recommendation
  "EKS Node Group prod-workers is saturated.
   Instance type t3.large (8GB) insufficient.
   Recommend: Scale to t3.xlarge (16GB) or increase desired_size to 7"
```

**5. Query Examples**

```
Q: "Which EKS cluster owns this EC2 instance i-abc123?"
A: aws.eks_cluster prod-cluster (via node group prod-workers)

Q: "Which K8s node labels does this EC2 instance have?"
A: {node-role.kubernetes.io/worker: true, workload-type: general}

Q: "EKS Node Group scaled down - which EC2 instances disappeared?"
A: (Temporal) i-def456, i-ghi789 terminated at 14:35

Q: "Which subnets are EKS nodes deployed in?"
A: subnet-111, subnet-222 (availability zones us-east-1a, us-east-1b)

Q: "Show me all infrastructure for prod-cluster"
A: EKS Cluster → Node Groups → EC2 Instances → VPC/Subnets/SecurityGroups
```

**6. Advanced Correlation: K8s Upgrade Impact**

```
Scenario: EKS cluster upgrade 1.27 → 1.28

15:00 - Elava: "EKS Cluster prod-cluster version changed: 1.27 → 1.28"
15:05 - Elava: "EKS Node Group prod-workers AMI changed (new K8s version)"
15:10 - Tapio: "Nodes being cordoned and drained"
15:15 - Tapio: "New nodes joining cluster with version 1.28"
15:20 - Tapio: "Pods rescheduled successfully"
  ↓
Ahti correlation timeline:
  "Cluster upgrade triggered node rotation.
   Old nodes (AMI ami-old-123) replaced with new nodes (AMI ami-new-456).
   5 EC2 instances terminated, 5 new instances created.
   All pods rescheduled within 10 minutes."
```

**7. The Value: Bridging K8s and AWS**

**Without Elava:**
- Tapio sees: "Pod failed on node ip-10-0-1-50"
- You manually check: "Which EC2 is that? What size? Part of which ASG?"

**With Elava + Ahti:**
- Automatic correlation: K8s node → EC2 → EKS Node Group → EKS Cluster
- Infrastructure context: Instance type, ASG settings, VPC placement
- Root cause: "Node undersized" or "Security group blocking traffic"
- Temporal analysis: "What infrastructure changed before the incident?"

---

## Example: RDS Database Infrastructure Graph

### The Database Layer: Multi-AZ, Replication, Network Topology

**1. Scan RDS Primary Instance**
```json
{
  "resource_type": "rds_instance",
  "resource_id": "prod-postgres-primary",
  "rds_instance": {
    "db_instance_identifier": "prod-postgres-primary",
    "db_instance_class": "db.r5.xlarge",
    "engine": "postgres",
    "engine_version": "15.3",
    "vpc_id": "vpc-xyz789",
    "db_subnet_group_name": "prod-db-subnets",
    "availability_zone": "us-east-1a",
    "multi_az": true,
    "secondary_az": "us-east-1b",
    "vpc_security_group_ids": ["sg-db-001"],
    "publicly_accessible": false,
    "endpoint": "prod-postgres-primary.abc123.us-east-1.rds.amazonaws.com",
    "port": 5432,
    "allocated_storage": 500,
    "storage_encrypted": true,
    "read_replica_db_identifiers": ["prod-postgres-read-1", "prod-postgres-read-2"],
    "backup_retention_period": 7
  }
}
```

**2. Scan DB Subnet Group**
```json
{
  "resource_type": "db_subnet_group",
  "resource_id": "prod-db-subnets",
  "db_subnet_group": {
    "name": "prod-db-subnets",
    "description": "Production database subnets",
    "vpc_id": "vpc-xyz789",
    "subnet_ids": ["subnet-db-1a", "subnet-db-1b", "subnet-db-1c"],
    "availability_zones": ["us-east-1a", "us-east-1b", "us-east-1c"]
  }
}
```

**3. Scan Read Replica**
```json
{
  "resource_type": "rds_instance",
  "resource_id": "prod-postgres-read-1",
  "rds_instance": {
    "db_instance_identifier": "prod-postgres-read-1",
    "db_instance_class": "db.r5.large",
    "engine": "postgres",
    "engine_version": "15.3",
    "availability_zone": "us-east-1a",
    "read_replica_source_db_identifier": "prod-postgres-primary",
    "endpoint": "prod-postgres-read-1.abc123.us-east-1.rds.amazonaws.com"
  }
}
```

**4. Resulting Database Graph**

```
┌─────────────────────────┐
│ aws.db_subnet_group     │
│ prod-db-subnets         │
│ (3 AZs)                 │
└──────────┬──────────────┘
           │ contains
           ├──────────────────┐
           │                  │
           ▼                  ▼
┌──────────────────┐  ┌──────────────────┐
│ aws.subnet       │  │ aws.subnet       │
│ subnet-db-1a     │  │ subnet-db-1b     │
│ us-east-1a       │  │ us-east-1b       │
└──────────┬───────┘  └──────────┬───────┘
           │                     │
           │                     │ Multi-AZ standby
           │                     │
           │          ┌──────────▼───────────────┐
           │          │ aws.rds_instance         │
           │          │ prod-postgres-primary    │
           │          │ db.r5.xlarge (Multi-AZ)  │
           │          │ Primary: us-east-1a      │
           │          │ Standby: us-east-1b      │
           │          └──────────┬───────────────┘
           │                     │ replicates_from
           │                     ▼
           │          ┌──────────────────────────┐
           └─────────►│ aws.rds_instance         │
                      │ prod-postgres-read-1     │
                      │ db.r5.large              │
                      │ Read Replica (us-east-1a)│
                      └──────────┬───────────────┘
                                 │ protected_by
                                 ▼
                      ┌──────────────────────────┐
                      │ aws.security_group       │
                      │ sg-db-001                │
                      │ (Allow port 5432 from    │
                      │  app subnets only)       │
                      └──────────────────────────┘
```

**5. Query Examples**

```
Q: "Which subnets is RDS deployed in?"
A: subnet-db-1a (us-east-1a), subnet-db-1b (us-east-1b), subnet-db-1c (us-east-1c)

Q: "Does this database have Multi-AZ enabled?"
A: Yes - Primary in us-east-1a, Standby in us-east-1b

Q: "Which read replicas exist for prod-postgres-primary?"
A: prod-postgres-read-1 (us-east-1a), prod-postgres-read-2 (us-east-1b)

Q: "Why is my app getting connection timeouts to RDS?"
A: (Correlation) SecurityGroup sg-db-001 only allows traffic from subnet-app-1a,
   but your EC2 instance is in subnet-app-1c (blocked)

Q: "Show me the replication chain"
A: prod-postgres-primary → prod-postgres-read-1 → prod-postgres-read-2

Q: "What changed in RDS configuration last week?"
A: (Temporal) 2025-10-10: Upgraded postgres 15.2 → 15.3
                2025-10-12: Added read replica prod-postgres-read-2
```

**6. Incident Correlation: Database Connectivity**

```
Scenario: Application can't connect to database

15:00 - Tapio: "Pod api-server-xyz connection refused to database"
15:01 - Developer: "Database is unreachable!"
  ↓
Ahti correlation:

Step 1: Identify RDS instance
  - App connects to: prod-postgres-primary.abc123.us-east-1.rds.amazonaws.com
  - Elava: RDS instance prod-postgres-primary

Step 2: Check RDS network placement
  - VPC: vpc-xyz789
  - Subnets: subnet-db-1a, subnet-db-1b, subnet-db-1c (private)
  - Security Group: sg-db-001

Step 3: Check app EC2 instance
  - Pod api-server-xyz → K8s node ip-10-0-5-20 → EC2 i-app123
  - EC2 subnet: subnet-app-1d (NEW subnet, just added)
  - Security Group: sg-app-001

Step 4: Analyze security group rules
  - sg-db-001 INGRESS: Allow 5432 from subnet-app-1a, subnet-app-1b, subnet-app-1c
  - sg-db-001 does NOT allow subnet-app-1d ← ROOT CAUSE

Step 5: Ahti recommendation
  "RDS security group sg-db-001 doesn't allow traffic from subnet-app-1d.
   Add ingress rule: Allow 5432 from 10.0.5.0/24 (subnet-app-1d CIDR)"
```

**7. Advanced Scenario: Cross-AZ Traffic Costs**

```
Scenario: Unexpected AWS data transfer charges

Context:
  - RDS Primary: us-east-1a
  - RDS Read Replica: us-east-1a (same AZ)
  - App EC2 instances: us-east-1b, us-east-1c

Ahti analysis:
  "80% of database queries are cross-AZ (us-east-1b/c → us-east-1a).
   Cross-AZ data transfer: $0.01/GB.
   Estimated monthly cost: $5,000.

   Recommendation:
   1. Move read replica to us-east-1b (serve local reads)
   2. Update app connection string to use nearest replica
   3. Estimated savings: $4,000/month"
```

---

## Example: VPC Network Topology Graph

### The Foundation: Routing, Gateways, Public vs Private

**1. Scan VPC and Subnets**

```json
// VPC
{
  "resource_type": "vpc",
  "resource_id": "vpc-xyz789",
  "vpc": {
    "vpc_id": "vpc-xyz789",
    "cidr_block": "10.0.0.0/16",
    "enable_dns_hostnames": true,
    "enable_dns_support": true
  }
}

// Public Subnet
{
  "resource_type": "subnet",
  "resource_id": "subnet-public-1a",
  "subnet": {
    "subnet_id": "subnet-public-1a",
    "vpc_id": "vpc-xyz789",
    "cidr": "10.0.1.0/24",
    "availability_zone": "us-east-1a",
    "map_public_ip_on_launch": true,
    "route_table_id": "rtb-public"
  }
}

// Private Subnet
{
  "resource_type": "subnet",
  "resource_id": "subnet-private-1a",
  "subnet": {
    "subnet_id": "subnet-private-1a",
    "vpc_id": "vpc-xyz789",
    "cidr": "10.0.10.0/24",
    "availability_zone": "us-east-1a",
    "map_public_ip_on_launch": false,
    "route_table_id": "rtb-private-1a"
  }
}

// DB Subnet (isolated)
{
  "resource_type": "subnet",
  "resource_id": "subnet-db-1a",
  "subnet": {
    "subnet_id": "subnet-db-1a",
    "vpc_id": "vpc-xyz789",
    "cidr": "10.0.20.0/24",
    "availability_zone": "us-east-1a",
    "map_public_ip_on_launch": false,
    "route_table_id": "rtb-db"
  }
}
```

**2. Scan Route Tables**

```json
// Public Route Table
{
  "resource_type": "route_table",
  "resource_id": "rtb-public",
  "route_table": {
    "route_table_id": "rtb-public",
    "vpc_id": "vpc-xyz789",
    "is_main": false,
    "subnet_ids": ["subnet-public-1a", "subnet-public-1b"],
    "routes": [
      {
        "destination_cidr": "10.0.0.0/16",
        "target_type": "local",
        "target_id": "local",
        "state": "active"
      },
      {
        "destination_cidr": "0.0.0.0/0",
        "target_type": "igw",
        "target_id": "igw-abc123",
        "state": "active"
      }
    ]
  }
}

// Private Route Table
{
  "resource_type": "route_table",
  "resource_id": "rtb-private-1a",
  "route_table": {
    "route_table_id": "rtb-private-1a",
    "vpc_id": "vpc-xyz789",
    "is_main": false,
    "subnet_ids": ["subnet-private-1a"],
    "routes": [
      {
        "destination_cidr": "10.0.0.0/16",
        "target_type": "local",
        "target_id": "local",
        "state": "active"
      },
      {
        "destination_cidr": "0.0.0.0/0",
        "target_type": "nat",
        "target_id": "nat-xyz789",
        "state": "active"
      }
    ]
  }
}

// DB Route Table (no internet)
{
  "resource_type": "route_table",
  "resource_id": "rtb-db",
  "route_table": {
    "route_table_id": "rtb-db",
    "vpc_id": "vpc-xyz789",
    "is_main": false,
    "subnet_ids": ["subnet-db-1a", "subnet-db-1b"],
    "routes": [
      {
        "destination_cidr": "10.0.0.0/16",
        "target_type": "local",
        "target_id": "local",
        "state": "active"
      }
    ]
  }
}
```

**3. Scan NAT Gateway and Internet Gateway**

```json
// Internet Gateway
{
  "resource_type": "internet_gateway",
  "resource_id": "igw-abc123",
  "internet_gateway": {
    "internet_gateway_id": "igw-abc123",
    "vpc_id": "vpc-xyz789",
    "state": "available"
  }
}

// NAT Gateway
{
  "resource_type": "nat_gateway",
  "resource_id": "nat-xyz789",
  "nat_gateway": {
    "nat_gateway_id": "nat-xyz789",
    "vpc_id": "vpc-xyz789",
    "subnet_id": "subnet-public-1a",
    "elastic_ip": "54.123.45.67",
    "state": "available"
  }
}
```

**4. Resulting VPC Network Graph**

```
                    ┌─────────────────────┐
                    │ INTERNET            │
                    └──────────┬──────────┘
                               │
                               ▼
                    ┌─────────────────────┐
                    │ aws.internet_gateway│
                    │ igw-abc123          │
                    └──────────┬──────────┘
                               │
                ┌──────────────┴──────────────┐
                │                             │
                │ (attached_to)               │
                ▼                             │
┌───────────────────────────────────────────┐ │
│ aws.vpc                                   │ │
│ vpc-xyz789 (10.0.0.0/16)                  │ │
└───────────────────────────────────────────┘ │
                │                             │
    ┌───────────┼───────────┬─────────────────┘
    │           │           │
    ▼           ▼           ▼
┌─────────┐ ┌─────────┐ ┌─────────┐
│ PUBLIC  │ │ PRIVATE │ │   DB    │
│ SUBNET  │ │ SUBNET  │ │ SUBNET  │
└────┬────┘ └────┬────┘ └────┬────┘
     │           │           │
     │           │           │
┌────▼──────────────────┐ ┌─▼────────────┐ ┌─▼────────────┐
│ aws.subnet            │ │ aws.subnet   │ │ aws.subnet   │
│ subnet-public-1a      │ │ subnet-priv  │ │ subnet-db-1a │
│ 10.0.1.0/24           │ │ 10.0.10.0/24 │ │ 10.0.20.0/24 │
│ map_public_ip: TRUE   │ │ (private)    │ │ (isolated)   │
└───────────┬───────────┘ └──────┬───────┘ └──────┬───────┘
            │                    │                 │
            │ routes_via         │ routes_via      │ routes_via
            ▼                    ▼                 ▼
┌───────────────────────┐ ┌──────────────────┐ ┌──────────────┐
│ aws.route_table       │ │ aws.route_table  │ │ aws.route_table│
│ rtb-public            │ │ rtb-private-1a   │ │ rtb-db       │
│                       │ │                  │ │              │
│ Routes:               │ │ Routes:          │ │ Routes:      │
│ • 10.0.0.0/16 → local │ │ • 10.0.0.0/16 →  │ │ • 10.0.0.0/16│
│ • 0.0.0.0/0 → IGW ✓   │ │   local          │ │   → local    │
└───────────┬───────────┘ │ • 0.0.0.0/0 →    │ │ • NO internet│
            │             │   NAT ✓          │ │   route      │
            │             └────────┬─────────┘ └──────────────┘
            │ public_access        │ nats_through
            │                      │
            ▼                      ▼
┌───────────────────────┐ ┌──────────────────────┐
│ aws.internet_gateway  │ │ aws.nat_gateway      │
│ igw-abc123            │ │ nat-xyz789           │
│ (direct internet)     │ │ (in subnet-public-1a)│
└───────────────────────┘ │ Elastic IP:          │
                          │ 54.123.45.67         │
                          └──────────────────────┘
```

**5. Routing Flow Visualization**

```
Public Subnet (10.0.1.0/24):
  Internet → IGW → Public Subnet → EC2 with Public IP
  EC2 → Public Subnet → IGW → Internet
  ✓ Bidirectional internet access

Private Subnet (10.0.10.0/24):
  Internet ✗ (no route in)
  EC2 → Private Subnet → NAT Gateway → IGW → Internet
  ✓ Egress only (can make outbound connections)

DB Subnet (10.0.20.0/24):
  Internet ✗ (no route in)
  RDS → DB Subnet → Local only (10.0.0.0/16)
  ✗ No internet access (fully isolated)
```

**6. Query Examples**

```
Q: "Why can't I SSH into this EC2 instance?"
A: (Correlation) EC2 i-private123 is in subnet-private-1a (no public IP).
   Route table rtb-private-1a has no IGW route.
   Solution: Use bastion host in public subnet or AWS SSM Session Manager.

Q: "Which subnets have internet access?"
A: Direct access: subnet-public-1a, subnet-public-1b (via IGW)
   Egress only: subnet-private-1a, subnet-private-1b (via NAT)
   No access: subnet-db-1a, subnet-db-1b (isolated)

Q: "Where is the NAT Gateway deployed?"
A: nat-xyz789 is in subnet-public-1a (requires public subnet for Elastic IP)

Q: "Show me the routing path for EC2 i-private123 to reach internet"
A: EC2 (10.0.10.50) → subnet-private-1a → rtb-private-1a →
   nat-xyz789 (54.123.45.67) → igw-abc123 → Internet

Q: "Why is my Lambda function slow?"
A: (Correlation) Lambda is in VPC (subnet-private-1a).
   All outbound traffic routes through NAT Gateway (cold start delay).
   Recommendation: Use VPC endpoints for AWS services or move to public subnet.

Q: "What changed in VPC routing last week?"
A: (Temporal) 2025-10-12: Route 0.0.0.0/0 → nat-xyz789 state changed to "blackhole"
   Cause: NAT Gateway was deleted at 14:30, recreated at 15:00
```

**7. Incident Correlation: Network Connectivity Failure**

```
Scenario: Private subnet instances lose internet access

16:00 - Monitoring: "API calls to external services timing out"
16:01 - Tapio: "Pods failing to pull Docker images"
16:02 - Elava: "NAT Gateway nat-xyz789 state: blackhole"
  ↓
Ahti correlation:

Step 1: Identify affected resources
  - Subnet: subnet-private-1a
  - Route Table: rtb-private-1a
  - Route: 0.0.0.0/0 → nat-xyz789 (state: blackhole)

Step 2: Analyze NAT Gateway
  - nat-xyz789 state: available → failed (at 16:00)
  - Elastic IP: 54.123.45.67 still allocated
  - Subnet: subnet-public-1a

Step 3: Impact analysis
  - 15 EC2 instances in subnet-private-1a (no internet)
  - 3 Lambda functions in VPC (can't reach AWS APIs)
  - EKS nodes in subnet-private-1b (also affected, shares same NAT)

Step 4: Root cause
  - NAT Gateway hardware failure (AWS-side)
  - Route automatically marked as "blackhole"

Step 5: Ahti recommendation
  "NAT Gateway nat-xyz789 failed at 16:00.
   15 EC2 instances and 3 Lambda functions affected.

   Immediate action: Create new NAT Gateway in subnet-public-1a
   Long-term: Deploy NAT Gateways in multiple AZs for HA

   Expected recovery time: 5 minutes"
```

**8. Advanced Scenario: VPC Peering Routing**

```
Scenario: Two VPCs need to communicate

Context:
  - VPC-Prod (vpc-xyz789): 10.0.0.0/16
  - VPC-Dev (vpc-dev456): 10.1.0.0/16

Step 1: Create VPC Peering
{
  "resource_type": "vpc_peering",
  "resource_id": "pcx-abc123",
  "vpc_peering": {
    "peering_connection_id": "pcx-abc123",
    "requester_vpc_id": "vpc-xyz789",
    "accepter_vpc_id": "vpc-dev456",
    "status": "active"
  }
}

Step 2: Update route tables
  - rtb-public (VPC-Prod): Add route 10.1.0.0/16 → pcx-abc123
  - rtb-dev (VPC-Dev): Add route 10.0.0.0/16 → pcx-abc123

Step 3: Ahti validation
  "VPC Peering pcx-abc123 established.
   Routing configured correctly.
   ✓ VPC-Prod can reach VPC-Dev
   ✗ WARNING: Security groups not updated - traffic will be blocked"

Graph:
┌────────────────┐           ┌────────────────┐
│ aws.vpc        │ peers_via │ aws.vpc        │
│ vpc-xyz789     ├──────────►│ vpc-dev456     │
│ 10.0.0.0/16    │◄──────────┤ 10.1.0.0/16    │
└────────────────┘           └────────────────┘
                pcx-abc123
```

---

## Next Steps

### Phase 1: Elava Standalone (Community Edition)
1. ✅ Define InfrastructureChangeEvent schema (complete)
2. ✅ Resolve entity/relationship strategy (conservative approach)
3. Implement Elava AWS scanner with BadgerDB storage
4. Build Elava CLI (show, history, watch commands)
5. Implement temporal queries (time-travel, disappeared resources)
6. Add multi-account support

### Phase 2: Ahti Integration (Enterprise Edition)
1. Set up gRPC service in Elava for streaming events
2. Implement Ahti Elava plugin with conservative entity extraction
3. Build background reconciliation job in Ahti plugin
4. Test end-to-end correlation (Tapio + Elava → Ahti)
5. Validate drift detection (broken references, orphaned resources)

### Future Considerations
- Multi-cloud support (GCP, Azure)
- Cost data correlation (CloudWatch billing → Ahti)
- Compliance events (AWS Config → Elava → Ahti)
- Natural language query layer for Elava CLI

---

## Open Questions

1. **Graph consistency:** Should Ahti's graph always be consistent (no dangling refs)?
2. **Partial data:** Is it acceptable to have entities with incomplete data temporarily?
3. **Scan frequency:** How often should Elava scan? Does it affect entity/relationship strategy?
4. **Multi-account:** Do entity IDs need to include account_id to be globally unique?
5. **Event replay:** If plugin logic changes, can we replay events to rebuild graph?
6. **Schema evolution:** How do we version InfrastructureChangeEvent schema?

---

## Appendix: AWS Resource Types (Planned)

**Compute:**
- ec2_instance
- autoscaling_group ✨
- launch_template ✨
- eks_cluster ✨
- eks_node_group ✨
- lambda_function
- ecs_task
- ecs_service

**Networking:**
- vpc
- subnet ✨ EXPANDED
- security_group
- network_interface
- load_balancer (alb, nlb, clb) ✨ EXPANDED
- target_group ✨ NEW
- nat_gateway ✨ EXPANDED
- internet_gateway ✨ EXPANDED
- route_table ✨ NEW
- vpc_peering ✨ NEW
- elastic_ip

**Database:**
- rds_instance ✨ EXPANDED
- rds_cluster
- db_subnet_group ✨ NEW

**Storage:**
- s3_bucket
- ebs_volume
- efs_filesystem

**Identity:**
- iam_role
- iam_policy
- iam_user

**Monitoring:**
- cloudwatch_alarm
- sns_topic
- sqs_queue

**Total: ~25 resource types for MVP**

**Key Relationships Added:**

**Compute Layer:**
- ASG → EC2 (owns)
- ASG → LaunchTemplate (uses_template)
- ASG → TargetGroup (registers_to)
- ASG → Subnet (deploys_in)
- LoadBalancer → TargetGroup (routes_through)
- TargetGroup → EC2 (targets)

**EKS Layer (K8s ↔ AWS Bridge):**
- EKSCluster → VPC (runs_in)
- EKSNodeGroup → EKSCluster (belongs_to)
- EKSNodeGroup → ASG (uses, via autoscaling_group_name)
- EKSNodeGroup → EC2 (owns/managed_by, via ASG)
- K8s Node (Tapio) ↔ EC2 (Elava) - **Critical bridge!**

**Database Layer:**
- RDS → DBSubnetGroup (uses_subnet_group)
- DBSubnetGroup → Subnet (contains)
- RDS Primary → RDS Read Replica (replicates_from)
- RDS → SecurityGroup (protected_by)

**Network Layer:**
- Subnet → RouteTable (routes_via)
- Subnet → VPC (belongs_to)
- RouteTable → InternetGateway (public_access)
- RouteTable → NATGateway (nats_through)
- NATGateway → Subnet (deployed_in, must be public subnet)
- VPC → VPCPeering → VPC (peers_via)
