# Type System Design

## Philosophy: No Maps Allowed

Ovi enforces stricter typing than even CLAUDE.md requires. The core principle: **no `map[string]string` or `map[string]interface{}` anywhere in the codebase**.

## Structured Tags

### The Problem with Maps
Traditional infrastructure tools use maps for tags:
```go
// ❌ Rejected approach
type Resource struct {
    ID   string
    Tags map[string]string  // Untyped, error-prone
}
```

Problems with this approach:
- **No compile-time validation**: Typos in tag names go undetected
- **No IDE support**: No autocomplete or refactoring assistance
- **Runtime errors**: Missing tags cause nil pointer exceptions
- **Poor documentation**: No clear schema for expected tags

### Ovi's Structured Approach
```go
// ✅ Ovi's approach
type Tags struct {
    // Ovi management tags
    OviOwner      string `json:"ovi_owner,omitempty"`
    OviManaged    bool   `json:"ovi_managed,omitempty"`
    OviBlessed    bool   `json:"ovi_blessed,omitempty"`
    OviGeneration string `json:"ovi_generation,omitempty"`
    OviClaimedAt  string `json:"ovi_claimed_at,omitempty"`
    
    // Standard infrastructure tags
    Name        string `json:"name,omitempty"`
    Environment string `json:"environment,omitempty"`
    Team        string `json:"team,omitempty"`
    Project     string `json:"project,omitempty"`
    CostCenter  string `json:"cost_center,omitempty"`
    
    // AWS common tags
    Application string `json:"application,omitempty"`
    Owner       string `json:"owner,omitempty"`
    Contact     string `json:"contact,omitempty"`
    CreatedBy   string `json:"created_by,omitempty"`
    CreatedDate string `json:"created_date,omitempty"`
}
```

### Benefits of Structured Tags
1. **Compile-time safety**: Typos caught at build time
2. **IDE support**: Full autocomplete and refactoring
3. **Self-documenting**: Clear schema in code
4. **Type safety**: No runtime type assertions needed
5. **Validation**: Easy to validate required fields

## Resource Type Hierarchy

### Core Resource Type
```go
type Resource struct {
    ID        string    `json:"id"`
    Type      string    `json:"type"`
    Provider  string    `json:"provider"`
    Region    string    `json:"region"`
    Name      string    `json:"name"`
    Status    string    `json:"status"`
    Tags      Tags      `json:"tags"`          // Structured, not map
    CreatedAt time.Time `json:"created_at"`
    UpdatedAt time.Time `json:"updated_at"`
}
```

### Resource Specifications
```go
type ResourceSpec struct {
    Type   string `yaml:"type" json:"type"`
    Count  int    `yaml:"count,omitempty" json:"count,omitempty"`
    Size   string `yaml:"size,omitempty" json:"size,omitempty"`
    Region string `yaml:"region,omitempty" json:"region,omitempty"`
    Tags   Tags   `yaml:"tags,omitempty" json:"tags,omitempty"`
}
```

### Resource Filters
```go
type ResourceFilter struct {
    Type     string   `json:"type,omitempty"`
    Region   string   `json:"region,omitempty"`
    Provider string   `json:"provider,omitempty"`
    Owner    string   `json:"owner,omitempty"`
    Managed  bool     `json:"managed,omitempty"`
    IDs      []string `json:"ids,omitempty"`
}
```

## Decision Types

### Structured Decisions
```go
type Decision struct {
    Action       string    `json:"action"`
    ResourceID   string    `json:"resource_id"`
    ResourceType string    `json:"resource_type,omitempty"`
    Reason       string    `json:"reason"`
    IsBlessed    bool      `json:"is_blessed,omitempty"`
    CreatedAt    time.Time `json:"created_at"`
    ExecutedAt   time.Time `json:"executed_at,omitempty"`
}
```

### Action Constants
```go
const (
    ActionCreate    = "create"
    ActionUpdate    = "update"
    ActionDelete    = "delete"
    ActionTerminate = "terminate"
    ActionNotify    = "notify"
    ActionTag       = "tag"
    ActionNoop      = "noop"
)
```

## Reconciler Types

### Diff Representation
```go
type Diff struct {
    Type       DiffType `json:"type"`
    ResourceID string   `json:"resource_id"`
    Current    *Resource `json:"current,omitempty"`
    Desired    *Resource `json:"desired,omitempty"`
    Reason     string   `json:"reason"`
}

type DiffType string

const (
    DiffMissing    DiffType = "missing"
    DiffUnwanted   DiffType = "unwanted"
    DiffDrifted    DiffType = "drifted"
    DiffUnmanaged  DiffType = "unmanaged"
)
```

### Configuration Types
```go
type Config struct {
    Version   string        `yaml:"version"`
    Provider  string        `yaml:"provider"`
    Region    string        `yaml:"region"`
    Resources []ResourceSpec `yaml:"resources"`
}

type ReconcilerOptions struct {
    DryRun          bool          `json:"dry_run"`
    MaxConcurrency  int           `json:"max_concurrency"`
    ClaimTTL        time.Duration `json:"claim_ttl"`
    SkipDestructive bool          `json:"skip_destructive"`
}
```

## Storage Types

### MVCC Types
```go
type ResourceState struct {
    ResourceID     string `json:"resource_id"`
    Owner          string `json:"owner"`
    Type           string `json:"type"`
    FirstSeenRev   int64  `json:"first_seen_rev"`
    LastSeenRev    int64  `json:"last_seen_rev"`
    DisappearedRev int64  `json:"disappeared_rev"`
    Exists         bool   `json:"exists"`
}
```

### WAL Types
```go
type Entry struct {
    Timestamp  time.Time       `json:"timestamp"`
    Sequence   int64           `json:"sequence"`
    Type       EntryType       `json:"type"`
    ResourceID string          `json:"resource_id,omitempty"`
    Data       json.RawMessage `json:"data"`
    Error      string          `json:"error,omitempty"`
}

type EntryType string

const (
    EntryObserved  EntryType = "observed"
    EntryDecided   EntryType = "decided"
    EntryExecuting EntryType = "executing"
    EntryExecuted  EntryType = "executed"
    EntryFailed    EntryType = "failed"
    EntrySkipped   EntryType = "skipped"
)
```

## Provider Interface Types

### Cloud Provider Abstraction
```go
type CloudProvider interface {
    Name() string
    Region() string
    ListResources(ctx context.Context, filter ResourceFilter) ([]Resource, error)
    CreateResource(ctx context.Context, spec ResourceSpec) (*Resource, error)
    UpdateResource(ctx context.Context, resource Resource) error
    DeleteResource(ctx context.Context, resourceID string) error
    TagResource(ctx context.Context, resourceID string, tags Tags) error
}
```

### AWS-Specific Types
```go
// AWS types maintain map[string]string for API compatibility
type EC2Instance struct {
    InstanceID   string
    InstanceType string
    State        string
    LaunchTime   time.Time
    Tags         map[string]string // Required by AWS API
}

type InstanceSpec struct {
    InstanceType string
    Tags         map[string]string // Required by AWS API
}
```

## Type Conversion

### API Compatibility
Since cloud provider APIs use maps, we provide conversion functions:

```go
// Convert structured tags to map for AWS API
func (t Tags) ToMap() map[string]string {
    tags := make(map[string]string)
    
    if t.OviOwner != "" {
        tags["elava:owner"] = t.OviOwner
    }
    if t.OviManaged {
        tags["elava:managed"] = "true"
    }
    // ... other fields
    
    return tags
}

// Convert map from AWS API to structured tags
func TagsFromMap(tagMap map[string]string) Tags {
    tags := Tags{}
    
    if val, ok := tagMap["elava:owner"]; ok {
        tags.OviOwner = val
    }
    if val, ok := tagMap["elava:managed"]; ok && val == "true" {
        tags.OviManaged = true
    }
    // ... other fields
    
    return tags
}
```

### Usage Pattern
```go
// In AWS provider
func (p *AWSProvider) CreateResource(ctx context.Context, spec ResourceSpec) (*Resource, error) {
    instanceSpec := InstanceSpec{
        InstanceType: p.getInstanceType(spec.Size),
        Tags:         spec.Tags.ToMap(), // Convert to map for AWS
    }
    
    instance, err := p.client.RunInstances(ctx, instanceSpec)
    if err != nil {
        return nil, err
    }
    
    resource := Resource{
        ID:   instance.InstanceID,
        Type: "ec2",
        Tags: TagsFromMap(instance.Tags), // Convert back to struct
    }
    
    return &resource, nil
}
```

## Validation

### Type Methods
```go
// Resource methods
func (r *Resource) IsManaged() bool {
    return r.Tags.IsManaged()
}

func (r *Resource) IsBlessed() bool {
    return r.Tags.IsBlessed()
}

func (r *Resource) Matches(filter ResourceFilter) bool {
    return r.matchesBasicFields(filter) && 
           r.matchesIDs(filter) && 
           r.matchesTags(filter)
}

// Tags methods
func (t Tags) IsManaged() bool {
    return t.OviOwner != "" || t.OviManaged
}

func (t Tags) IsBlessed() bool {
    return t.OviBlessed
}

func (t Tags) GetOwner() string {
    if t.OviOwner != "" {
        return t.OviOwner
    }
    return t.Team // Fallback
}

// Decision methods
func (d *Decision) Validate() error {
    if d.Action == "" {
        return fmt.Errorf("action cannot be empty")
    }
    if d.ResourceID == "" {
        return fmt.Errorf("resource ID cannot be empty")
    }
    if d.Reason == "" {
        return fmt.Errorf("reason cannot be empty")
    }
    return nil
}

func (d *Decision) IsDestructive() bool {
    return d.Action == ActionDelete || d.Action == ActionTerminate
}

func (d *Decision) RequiresConfirmation() bool {
    return d.IsDestructive() || d.IsBlessed
}
```

## Error Types

### Structured Errors
```go
type ValidationError struct {
    Field   string `json:"field"`
    Value   string `json:"value"`
    Message string `json:"message"`
}

func (e ValidationError) Error() string {
    return fmt.Sprintf("validation error on field %s: %s", e.Field, e.Message)
}

type ReconciliationError struct {
    Phase       string `json:"phase"`
    ResourceID  string `json:"resource_id,omitempty"`
    Underlying  error  `json:"underlying"`
}

func (e ReconciliationError) Error() string {
    return fmt.Sprintf("reconciliation failed in %s phase: %v", e.Phase, e.Underlying)
}
```

## Benefits of This Approach

### 1. Compile-Time Safety
```go
// ✅ This compiles
resource.Tags.Environment = "prod"

// ❌ This doesn't compile
resource.Tags["environment"] = "prod" // No Tags field access
```

### 2. IDE Support
- Full autocomplete on tag fields
- Refactoring support (rename fields across codebase)
- Go to definition works for tag fields
- Documentation appears in IDE tooltips

### 3. Self-Documenting Code
```go
// Clear what tags are available and their types
func createResource() Resource {
    return Resource{
        Tags: Tags{
            Environment: "prod",    // String, clear intent
            OviManaged:  true,      // Boolean, no "true" strings
            Team:        "platform", // Explicit field names
        },
    }
}
```

### 4. Easy Validation
```go
func validateTags(tags Tags) error {
    if tags.OviOwner == "" {
        return fmt.Errorf("ovi_owner is required")
    }
    if tags.Environment == "" {
        return fmt.Errorf("environment is required")
    }
    if !isValidEnvironment(tags.Environment) {
        return fmt.Errorf("invalid environment: %s", tags.Environment)
    }
    return nil
}
```

### 5. Testing Benefits
```go
func TestResource_IsManaged(t *testing.T) {
    tests := []struct {
        name     string
        tags     Tags
        expected bool
    }{
        {
            name:     "managed with owner",
            tags:     Tags{OviOwner: "team-web"},
            expected: true,
        },
        {
            name:     "managed with flag",
            tags:     Tags{OviManaged: true},
            expected: true,
        },
        {
            name:     "not managed",
            tags:     Tags{}, // Clear empty state
            expected: false,
        },
    }
    
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            resource := Resource{Tags: tt.tags}
            if got := resource.IsManaged(); got != tt.expected {
                t.Errorf("IsManaged() = %v, want %v", got, tt.expected)
            }
        })
    }
}
```

## Guidelines for New Types

### 1. Prefer Structs Over Maps
```go
// ✅ Good
type Config struct {
    Provider string `yaml:"provider"`
    Region   string `yaml:"region"`
    Options  ConfigOptions `yaml:"options"`
}

type ConfigOptions struct {
    DryRun     bool `yaml:"dry_run"`
    Verbose    bool `yaml:"verbose"`
    Timeout    time.Duration `yaml:"timeout"`
}

// ❌ Avoid
type Config struct {
    Provider string                 `yaml:"provider"`
    Region   string                 `yaml:"region"`
    Options  map[string]interface{} `yaml:"options"` // Too flexible
}
```

### 2. Use Constants for Enums
```go
// ✅ Good
type ResourceType string

const (
    ResourceTypeEC2 ResourceType = "ec2"
    ResourceTypeRDS ResourceType = "rds"
    ResourceTypeS3  ResourceType = "s3"
)

// ❌ Avoid
func createResource(resourceType string) { // Untyped
    // No compile-time validation of resourceType
}
```

### 3. Provide Validation Methods
```go
type Config struct {
    Provider string `yaml:"provider"`
    Region   string `yaml:"region"`
}

func (c Config) Validate() error {
    if c.Provider == "" {
        return ValidationError{
            Field:   "provider",
            Value:   c.Provider,
            Message: "provider is required",
        }
    }
    return nil
}
```

---

This type system makes Ovi's codebase more maintainable, safer, and easier to work with than traditional infrastructure tools that rely heavily on untyped maps and interfaces.