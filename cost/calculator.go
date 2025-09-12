package cost

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/yairfalse/ovi/types"
)

// Calculator interface for cost calculation
type Calculator interface {
	// CalculateCost calculates cost for a resource
	CalculateCost(ctx context.Context, resource *types.Resource) (*types.CostInfo, error)
	
	// CalculateBatch calculates cost for multiple resources efficiently
	CalculateBatch(ctx context.Context, resources []*types.Resource) error
	
	// EstimateWaste estimates waste for orphaned resources
	EstimateWaste(ctx context.Context, resource *types.Resource) (float64, error)
	
	// GetPricing gets pricing information for resource type
	GetPricing(ctx context.Context, provider, region, resourceType string) (*PricingInfo, error)
	
	// SupportsProvider returns true if this calculator supports the provider
	SupportsProvider(provider string) bool
}

// PricingInfo represents pricing data for a resource type
type PricingInfo struct {
	Provider     string    `json:"provider"`
	Region       string    `json:"region"`
	ResourceType string    `json:"resource_type"`
	Unit         string    `json:"unit"`
	PricePerUnit float64   `json:"price_per_unit"`
	Currency     string    `json:"currency"`
	LastUpdated  time.Time `json:"last_updated"`
}

// CalculatorFactory creates calculator instances
type CalculatorFactory func(config CalculatorConfig) (Calculator, error)

// CalculatorConfig holds calculator configuration
type CalculatorConfig struct {
	Provider string                 `json:"provider"`
	Config   map[string]interface{} `json:"config"`
}

// CalculatorRegistry manages cost calculators
type CalculatorRegistry struct {
	calculators map[string]CalculatorFactory
	mu          sync.RWMutex
}

// NewCalculatorRegistry creates a calculator registry
func NewCalculatorRegistry() *CalculatorRegistry {
	return &CalculatorRegistry{
		calculators: make(map[string]CalculatorFactory),
	}
}

// Register registers a calculator factory
func (r *CalculatorRegistry) Register(provider string, factory CalculatorFactory) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.calculators[provider] = factory
}

// Create creates a calculator instance
func (r *CalculatorRegistry) Create(config CalculatorConfig) (Calculator, error) {
	r.mu.RLock()
	factory, exists := r.calculators[config.Provider]
	r.mu.RUnlock()
	
	if !exists {
		return nil, fmt.Errorf("no cost calculator for provider: %s", config.Provider)
	}
	
	return factory(config)
}

// MultiProviderCalculator manages calculators for multiple providers
type MultiProviderCalculator struct {
	calculators map[string]Calculator
	registry    *CalculatorRegistry
	mu          sync.RWMutex
}

// NewMultiProviderCalculator creates a multi-provider calculator
func NewMultiProviderCalculator() *MultiProviderCalculator {
	return &MultiProviderCalculator{
		calculators: make(map[string]Calculator),
		registry:    NewCalculatorRegistry(),
	}
}

// RegisterCalculator registers a calculator for a provider
func (m *MultiProviderCalculator) RegisterCalculator(provider string, calculator Calculator) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.calculators[provider] = calculator
}

// CalculateCost calculates cost using appropriate provider calculator
func (m *MultiProviderCalculator) CalculateCost(ctx context.Context, resource *types.Resource) (*types.CostInfo, error) {
	m.mu.RLock()
	calculator, exists := m.calculators[resource.Provider]
	m.mu.RUnlock()
	
	if !exists {
		return nil, fmt.Errorf("no cost calculator for provider: %s", resource.Provider)
	}
	
	return calculator.CalculateCost(ctx, resource)
}

// CalculateBatch calculates costs for multiple resources
func (m *MultiProviderCalculator) CalculateBatch(ctx context.Context, resources []*types.Resource) error {
	// Group resources by provider
	providerGroups := make(map[string][]*types.Resource)
	for _, resource := range resources {
		providerGroups[resource.Provider] = append(providerGroups[resource.Provider], resource)
	}
	
	// Calculate costs for each provider group
	for provider, providerResources := range providerGroups {
		m.mu.RLock()
		calculator, exists := m.calculators[provider]
		m.mu.RUnlock()
		
		if !exists {
			// Skip resources for unsupported providers
			continue
		}
		
		if err := calculator.CalculateBatch(ctx, providerResources); err != nil {
			return fmt.Errorf("cost calculation failed for %s provider: %w", provider, err)
		}
	}
	
	return nil
}

// EstimateWaste estimates waste across all resources
func (m *MultiProviderCalculator) EstimateWaste(ctx context.Context, resources []*types.Resource) (float64, error) {
	var totalWaste float64
	
	for _, resource := range resources {
		if !resource.IsOrphaned {
			continue
		}
		
		m.mu.RLock()
		calculator, exists := m.calculators[resource.Provider]
		m.mu.RUnlock()
		
		if !exists {
			continue
		}
		
		waste, err := calculator.EstimateWaste(ctx, resource)
		if err != nil {
			continue // Skip errors, best effort
		}
		
		totalWaste += waste
	}
	
	return totalWaste, nil
}

// Global calculator instance
var globalCalculator = NewMultiProviderCalculator()

// RegisterCalculator registers a calculator in the global instance
func RegisterCalculator(provider string, calculator Calculator) {
	globalCalculator.RegisterCalculator(provider, calculator)
}

// CalculateResourceCost calculates cost using global calculator
func CalculateResourceCost(ctx context.Context, resource *types.Resource) (*types.CostInfo, error) {
	return globalCalculator.CalculateCost(ctx, resource)
}

// CalculateResourceCosts calculates costs for multiple resources
func CalculateResourceCosts(ctx context.Context, resources []*types.Resource) error {
	return globalCalculator.CalculateBatch(ctx, resources)
}

// EstimateResourceWaste estimates waste for resources
func EstimateResourceWaste(ctx context.Context, resources []*types.Resource) (float64, error) {
	return globalCalculator.EstimateWaste(ctx, resources)
}