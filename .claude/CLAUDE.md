YES! Let's update the `claude.md` with those critical points:

# Ovi - Living Infrastructure Engine

## 🚪 Project Overview
Ovi is a living infrastructure reconciliation engine that manages cloud resources without state files. Think "Kubernetes-style reconciliation for EC2/RDS/S3" - your cloud IS the state. Named after the Finnish word for "door" - it's the direct access to your infrastructure, no mazes needed.

## 🎯 Core Philosophy
- **No state files** - AWS/GCP is the source of truth
- **Living infrastructure** - Continuous reconciliation loop (like K8s for cloud resources)
- **Friendly, not aggressive** - Asks before destroying, notifies about orphans
- **Simple config** - Plain YAML/data, no programming languages
- **Direct API calls** - No providers, no abstractions, just AWS/GCP SDKs
- **Pluggable providers** - Easy to add new cloud providers

## 🏗️ Development Workflow

### 1. Design Session First
Before writing ANY code:
```markdown
## Design Session Checklist
- [ ] What problem are we solving?
- [ ] What's the simplest solution?
- [ ] Can we break it into smaller functions?
- [ ] What interfaces do we need?
- [ ] What can go wrong?
- [ ] Draw the flow (ASCII or diagram)
```

### 2. Write Tests Before Code
```go
// FIRST: Write the test
func TestReconciler_HandleOrphans(t *testing.T) {
    // Define expected behavior
    orphan := Resource{ID: "i-123", Tags: map[string]string{}}

    reconciler := NewReconciler()
    decision := reconciler.HandleOrphan(orphan)

    assert.Equal(t, "notify", decision.Action)
}

// THEN: Write minimal code to pass
```

### 3. Code in Small Chunks
```bash
# Work on dedicated branches
git checkout -b feat/orphan-detection

# Small iterations with verification
1. Write function (max 30 lines) → fmt → vet → lint → commit
2. Add validation → test → fmt → vet → lint → commit
3. Add error handling → test → fmt → vet → lint → commit

# MANDATORY before EVERY commit:
go fmt ./...
go vet ./...
golangci-lint run

# Push and PR when feature is complete
git push origin feat/orphan-detection
```

## 📦 Package Structure

### Core Types (shared by all)
```
types/          # Resource, Decision, State definitions
config/         # Configuration structures
```

### Service Packages (collaborate as needed)
```
providers/
  ├── aws/      # Implements CloudProvider interface
  ├── gcp/      # Implements CloudProvider interface
  └── provider.go # Interface definition
reconciler/     # Reconciliation engine
notifier/       # Slack/Discord/webhook notifications
registry/       # Resource tracking (not state!)
```

### Provider Interface (Pluggable!)
```go
// providers/provider.go
type CloudProvider interface {
    // Core operations
    ListResources(ctx context.Context, filter ResourceFilter) ([]Resource, error)
    CreateResource(ctx context.Context, spec ResourceSpec) (*Resource, error)
    DeleteResource(ctx context.Context, id string) error
    TagResource(ctx context.Context, id string, tags map[string]string) error

    // Provider info
    Name() string
    Region() string
}

// Easy to add new providers
func RegisterProvider(name string, factory ProviderFactory) {
    providers[name] = factory
}
```

## 🔧 Development Standards

### Function Design - Keep It Small!
```go
// ❌ BAD - Function doing too much
func ProcessResources(resources []Resource) error {
    // 200 lines of code...
    // validation, processing, notification, persistence...
}

// ✅ GOOD - Small, focused functions
func (r *Reconciler) ProcessResources(resources []Resource) error {
    if err := r.validateResources(resources); err != nil {
        return fmt.Errorf("validation failed: %w", err)
    }

    decisions := r.makeDecisions(resources)
    return r.executeDecisions(decisions)
}

func (r *Reconciler) validateResources(resources []Resource) error {
    // Just validation, <30 lines
}

func (r *Reconciler) makeDecisions(resources []Resource) []Decision {
    // Just decision logic, <30 lines
}

func (r *Reconciler) executeDecisions(decisions []Decision) error {
    // Just execution, <30 lines
}
```

### Smart Design Patterns

#### Strategy Pattern for Providers
```go
type AWSProvider struct{}
func (a *AWSProvider) ListResources(ctx context.Context, filter ResourceFilter) ([]Resource, error) {
    // AWS-specific implementation
}

type GCPProvider struct{}
func (g *GCPProvider) ListResources(ctx context.Context, filter ResourceFilter) ([]Resource, error) {
    // GCP-specific implementation
}

// Easy provider selection
provider := providers.Get(config.CloudProvider)
resources, err := provider.ListResources(ctx, filter)
```

#### Builder Pattern for Complex Objects
```go
// For complex resource creation
resource := NewResourceBuilder().
    WithType("ec2").
    WithRegion("us-east-1").
    WithTags(map[string]string{
        "ovi:owner": "team-web",
    }).
    Build()
```

#### Observer Pattern for Notifications
```go
type EventObserver interface {
    OnOrphanFound(resource Resource)
    OnResourceCreated(resource Resource)
    OnResourceDeleted(resource Resource)
}

// Multiple notifiers can observe
reconciler.AddObserver(slackNotifier)
reconciler.AddObserver(metricsCollector)
reconciler.AddObserver(auditLogger)
```

### Mandatory Verification (Pre-commit Hook)
```bash
#!/bin/bash
# .git/hooks/pre-commit

echo "🔍 Running Ovi pre-commit checks..."

# Format check
echo "→ Running go fmt..."
if ! go fmt ./...; then
    echo "❌ Format failed. Run 'go fmt ./...'"
    exit 1
fi

# Vet check
echo "→ Running go vet..."
if ! go vet ./...; then
    echo "❌ Vet failed. Fix issues and retry"
    exit 1
fi

# Lint check
echo "→ Running golangci-lint..."
if ! golangci-lint run; then
    echo "❌ Lint failed. Fix issues and retry"
    exit 1
fi

# Function length check
echo "→ Checking function lengths..."
for file in $(find . -name "*.go" -not -path "./vendor/*"); do
    awk '/^func/ {start=NR} /^}/ {if(start && NR-start>50) printf "%s:%d: Function too long (%d lines)\n", FILENAME, start, NR-start}' "$file"
done

echo "✅ All checks passed!"
```

### Linter Configuration (.golangci.yml)
```yaml
linters:
  enable:
    - gofmt
    - govet
    - errcheck
    - staticcheck
    - gosimple
    - ineffassign
    - unused
    - gocyclo
    - gocognit
    - bodyclose
    - gosec

linters-settings:
  gocyclo:
    min-complexity: 10
  gocognit:
    min-complexity: 20
  funlen:
    lines: 50
    statements: 30

issues:
  exclude-use-default: false
  max-issues-per-linter: 0
  max-same-issues: 0
```

## 🧪 Testing Requirements

### Test Organization
```go
// Small, focused test functions
func TestValidateResource_ValidResource(t *testing.T) {
    // One test, one assertion
}

func TestValidateResource_MissingID(t *testing.T) {
    // Test specific error case
}

func TestValidateResource_InvalidType(t *testing.T) {
    // Test another error case
}

// Table-driven tests for multiple cases
func TestResourceMatching(t *testing.T) {
    tests := []struct {
        name     string
        resource Resource
        filter   Filter
        want     bool
    }{
        {"matches type", Resource{Type: "ec2"}, Filter{Type: "ec2"}, true},
        {"no match", Resource{Type: "rds"}, Filter{Type: "ec2"}, false},
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            got := matches(tt.resource, tt.filter)
            assert.Equal(t, tt.want, got)
        })
    }
}
```

## ⛔ Code Quality Standards

### No map[string]interface{} - Ever
```go
// ❌ BANNED
func ProcessResource(data map[string]interface{}) error

// ✅ REQUIRED
type ResourceSpec struct {
    Type  string
    Count int
    Tags  map[string]string
}
```

### Functions Must Be Small and Focused
- Maximum 30-50 lines per function
- Single responsibility principle
- Extract complex logic into helper functions
- Use meaningful names

### Error Handling - Always Context
```go
// ❌ BAD
return err
return fmt.Errorf("failed")

// ✅ GOOD
return fmt.Errorf("failed to list EC2 instances in %s: %w", region, err)
```

## 📋 Verification Checklist

Before EVERY commit (automated by pre-commit hook):
```bash
# 1. Format - MANDATORY
go fmt ./...

# 2. Vet - MANDATORY
go vet ./...

# 3. Lint - MANDATORY
golangci-lint run

# 4. Test
go test ./... -race

# 5. Coverage
go test ./... -cover

# 6. Function length
# Check no function exceeds 50 lines
```

## ✅ Definition of Done

A feature is complete when:
- [ ] Design documented
- [ ] Functions are small (<50 lines)
- [ ] Provider interface used (if cloud-specific)
- [ ] Tests written and passing
- [ ] `go fmt` applied
- [ ] `go vet` passes
- [ ] `golangci-lint` passes
- [ ] 80%+ test coverage
- [ ] Error handling with context
- [ ] No map[string]interface{}

## 🎯 Smart Design Principles

1. **Small Functions** - If you can't understand it in 10 seconds, split it
2. **Interface-Driven** - Define interfaces first, implement later
3. **Provider Agnostic** - Core logic shouldn't know about AWS/GCP specifics
4. **Pluggable Everything** - Providers, notifiers, registries should be pluggable
5. **Composition over Inheritance** - Use interfaces and composition
6. **Fail Fast** - Validate early, return errors immediately
7. **No Magic** - Code should be obvious, not clever

## 🔌 Adding a New Provider

```go
// 1. Implement the interface
type AzureProvider struct {
    client *azure.Client
}

func (a *AzureProvider) ListResources(ctx context.Context, filter ResourceFilter) ([]Resource, error) {
    // Azure-specific implementation
}

// 2. Register it
func init() {
    providers.Register("azure", NewAzureProvider)
}

// 3. That's it! Ovi can now use Azure
```

## 💭 Design Patterns We Use

- **Strategy Pattern** - For providers (AWS/GCP/Azure)
- **Observer Pattern** - For notifications
- **Builder Pattern** - For complex object creation
- **Factory Pattern** - For provider creation
- **Repository Pattern** - For registry/storage

## 🔥 Remember

> "Small functions, clear interfaces, pluggable providers"

> "Format, vet, lint - every single commit"

> "Design first, test second, code third"

Keep Ovi modular, testable, and simple. Every function should do ONE thing well.

---

**False Systems**: Building infrastructure tools that actually make sense 🇫🇮
