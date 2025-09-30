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
	// Provider-specific extended configuration
	AWSConfig   *AWSConfig   `json:"aws_config,omitempty"`
	GCPConfig   *GCPConfig   `json:"gcp_config,omitempty"`
	AzureConfig *AzureConfig `json:"azure_config,omitempty"`
}

// AWSConfig holds AWS-specific configuration
type AWSConfig struct {
	AssumeRoleARN        string `json:"assume_role_arn,omitempty"`
	ExternalID           string `json:"external_id,omitempty"`
	Profile              string `json:"profile,omitempty"`
	EndpointURL          string `json:"endpoint_url,omitempty"` // For testing
	MaxRetries           int    `json:"max_retries,omitempty"`
	STSRegionalEndpoints string `json:"sts_regional_endpoints,omitempty"`
}

// GCPConfig holds GCP-specific configuration
type GCPConfig struct {
	ServiceAccountJSON        string   `json:"service_account_json,omitempty"`
	ImpersonateServiceAccount string   `json:"impersonate_service_account,omitempty"`
	Quota                     int      `json:"quota,omitempty"`
	Scopes                    []string `json:"scopes,omitempty"`
}

// AzureConfig holds Azure-specific configuration
type AzureConfig struct {
	SubscriptionID string `json:"subscription_id,omitempty"`
	TenantID       string `json:"tenant_id,omitempty"`
	ClientID       string `json:"client_id,omitempty"`
	ClientSecret   string `json:"client_secret,omitempty"`
	Environment    string `json:"environment,omitempty"` // AzurePublicCloud, AzureUSGovernment, etc.
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
