package providers

import (
	"context"
	"testing"

	"github.com/yairfalse/elava/types"
)

// MockProvider for testing
type MockProvider struct {
	name      string
	region    string
	resources []types.Resource
}

func (m *MockProvider) Name() string {
	return m.name
}

func (m *MockProvider) Region() string {
	return m.region
}

func (m *MockProvider) ListResources(ctx context.Context, filter types.ResourceFilter) ([]types.Resource, error) {
	var result []types.Resource
	for _, r := range m.resources {
		if r.Matches(filter) {
			result = append(result, r)
		}
	}
	return result, nil
}

func (m *MockProvider) CreateResource(ctx context.Context, spec types.ResourceSpec) (*types.Resource, error) {
	return &types.Resource{
		ID:       "new-resource",
		Type:     spec.Type,
		Provider: m.name,
		Region:   m.region,
		Tags:     spec.Tags,
	}, nil
}

func (m *MockProvider) DeleteResource(ctx context.Context, id string) error {
	return nil
}

func (m *MockProvider) TagResource(ctx context.Context, id string, tags map[string]string) error {
	return nil
}

func TestProviderInterface(t *testing.T) {
	// Ensure MockProvider implements CloudProvider
	var _ CloudProvider = (*MockProvider)(nil)

	provider := &MockProvider{
		name:   "mock",
		region: "us-test-1",
		resources: []types.Resource{
			{ID: "r1", Type: "ec2", Region: "us-test-1"},
			{ID: "r2", Type: "rds", Region: "us-test-1"},
		},
	}

	// Test Name and Region
	if provider.Name() != "mock" {
		t.Errorf("Name() = %v, want mock", provider.Name())
	}
	if provider.Region() != "us-test-1" {
		t.Errorf("Region() = %v, want us-test-1", provider.Region())
	}

	// Test ListResources
	ctx := context.Background()
	resources, err := provider.ListResources(ctx, types.ResourceFilter{Type: "ec2"})
	if err != nil {
		t.Fatalf("ListResources() error = %v", err)
	}
	if len(resources) != 1 {
		t.Errorf("ListResources() returned %d resources, want 1", len(resources))
	}
}

func TestProviderRegistry(t *testing.T) {
	// Register a mock provider
	RegisterProvider("test", func(ctx context.Context, config ProviderConfig) (CloudProvider, error) {
		return &MockProvider{
			name:   "test",
			region: config.Region,
		}, nil
	})

	// Get the provider
	ctx := context.Background()
	provider, err := GetProvider(ctx, "test", ProviderConfig{Region: "us-test-1"})
	if err != nil {
		t.Fatalf("GetProvider() error = %v", err)
	}
	if provider.Name() != "test" {
		t.Errorf("provider.Name() = %v, want test", provider.Name())
	}

	// Try to get non-existent provider
	_, err = GetProvider(ctx, "nonexistent", ProviderConfig{})
	if err == nil {
		t.Error("GetProvider() should error for non-existent provider")
	}
}
