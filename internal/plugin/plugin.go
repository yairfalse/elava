// Package plugin defines the scanner plugin interface for Elava.
package plugin

import (
	"context"
	"sync"

	"github.com/yairfalse/elava/pkg/resource"
)

// Plugin is the interface all cloud provider plugins must implement.
// Keep it simple: Name + Scan. That's it.
type Plugin interface {
	// Name returns the plugin identifier (e.g., "aws", "gcp", "azure")
	Name() string

	// Scan returns all resources from this provider.
	// Called on every scan interval - must return current state.
	Scan(ctx context.Context) ([]resource.Resource, error)
}

// Registry holds registered plugins.
var (
	registry = make(map[string]Plugin)
	mu       sync.RWMutex
)

// Register adds a plugin to the registry.
func Register(p Plugin) {
	mu.Lock()
	defer mu.Unlock()
	registry[p.Name()] = p
}

// Get returns a plugin by name.
func Get(name string) (Plugin, bool) {
	mu.RLock()
	defer mu.RUnlock()
	p, ok := registry[name]
	return p, ok
}

// All returns all registered plugins.
func All() []Plugin {
	mu.RLock()
	defer mu.RUnlock()
	plugins := make([]Plugin, 0, len(registry))
	for _, p := range registry {
		plugins = append(plugins, p)
	}
	return plugins
}

// Names returns all registered plugin names.
func Names() []string {
	mu.RLock()
	defer mu.RUnlock()
	names := make([]string, 0, len(registry))
	for name := range registry {
		names = append(names, name)
	}
	return names
}

// Clear removes all plugins from the registry. Used for testing.
func Clear() {
	mu.Lock()
	defer mu.Unlock()
	registry = make(map[string]Plugin)
}
