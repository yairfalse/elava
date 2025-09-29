package providers

import (
	"context"
	"fmt"

	"github.com/yairfalse/elava/types"
)

// CloudProvider interface for all cloud providers
type CloudProvider interface {
	// Core operations
	ListResources(ctx context.Context, filter types.ResourceFilter) ([]types.Resource, error)
	CreateResource(ctx context.Context, spec types.ResourceSpec) (*types.Resource, error)
	DeleteResource(ctx context.Context, id string) error
	TagResource(ctx context.Context, id string, tags map[string]string) error

	// Provider info
	Name() string
	Region() string
}

// ProviderConfig holds provider configuration
type ProviderConfig struct {
	Type            string `json:"type"` // "aws", "gcp", "azure", etc.
	Region          string `json:"region"`
	AccessKeyID     string
	SecretAccessKey string
	SessionToken    string
	ProjectID       string // For GCP
	// Config stores provider-specific configuration that varies by cloud provider
	// This is intentionally a map because each provider (AWS, GCP, Azure) has
	// different config requirements that cannot be predetermined at compile time
	Config map[string]interface{} `json:"config"` // Provider-specific config
}

// ProviderFactory creates a provider instance
type ProviderFactory func(ctx context.Context, config ProviderConfig) (CloudProvider, error)

// Registry of available providers
var providers = make(map[string]ProviderFactory)

// RegisterProvider registers a new provider factory
func RegisterProvider(name string, factory ProviderFactory) {
	providers[name] = factory
}

// GetProvider creates a provider instance by name
func GetProvider(ctx context.Context, name string, config ProviderConfig) (CloudProvider, error) {
	factory, exists := providers[name]
	if !exists {
		return nil, fmt.Errorf("provider %s not found", name)
	}
	return factory(ctx, config)
}

// ListProviders returns available provider names
func ListProviders() []string {
	names := make([]string, 0, len(providers))
	for name := range providers {
		names = append(names, name)
	}
	return names
}
