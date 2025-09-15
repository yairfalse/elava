package config

import (
	"fmt"
	"os"
	"time"

	"github.com/yairfalse/elava/types"
	"gopkg.in/yaml.v3"
)

// Config represents the main configuration
type Config struct {
	Version   string                        `yaml:"version"`
	Provider  string                        `yaml:"provider"`
	Region    string                        `yaml:"region"`
	Resources map[string]types.ResourceSpec `yaml:"resources,omitempty"`
	Rules     Rules                         `yaml:"rules,omitempty"`
}

// Rules defines behavior rules
type Rules struct {
	ProtectBlessed  bool          `yaml:"protect_blessed"`
	NotifyOnOrphans bool          `yaml:"notify_on_orphans"`
	GracePeriod     time.Duration `yaml:"grace_period"`
	AutoDelete      bool          `yaml:"auto_delete"`
	RequireApproval bool          `yaml:"require_approval"`
}

// LoadConfig loads configuration from file
func LoadConfig(path string) (*Config, error) {
	data, err := os.ReadFile(path) // #nosec G304 -- path is intentional user input
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config: %w", err)
	}

	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("invalid config: %w", err)
	}

	return &cfg, nil
}

// Validate ensures config has required fields
func (c *Config) Validate() error {
	if c.Version == "" {
		return fmt.Errorf("version is required")
	}
	if c.Provider == "" {
		return fmt.Errorf("provider is required")
	}
	if c.Region == "" {
		return fmt.Errorf("region is required")
	}
	return nil
}
