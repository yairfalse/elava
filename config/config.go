package config

import (
	"fmt"
	"os"
	"path/filepath"
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
	Scanning  ScanningConfig                `yaml:"scanning,omitempty"`
}

// ScanningConfig defines the tiered scanning strategy
type ScanningConfig struct {
	Enabled         bool                  `yaml:"enabled"`
	AdaptiveHours   bool                  `yaml:"adaptive_hours"`
	Tiers           map[string]TierConfig `yaml:"tiers"`
	ChangeDetection ChangeDetectionConfig `yaml:"change_detection"`
	Performance     PerformanceConfig     `yaml:"performance"`
}

// TierConfig defines a scanning tier
type TierConfig struct {
	Description  string        `yaml:"description"`
	ScanInterval time.Duration `yaml:"scan_interval"`
	Patterns     []TierPattern `yaml:"patterns"`
}

// TierPattern defines resource matching patterns
type TierPattern struct {
	Type                string            `yaml:"type,omitempty"`
	Types               []string          `yaml:"types,omitempty"`
	Status              string            `yaml:"status,omitempty"`
	Tags                map[string]string `yaml:"tags,omitempty"`
	InstanceTypePattern string            `yaml:"instance_type_pattern,omitempty"`
	SizePattern         string            `yaml:"size_pattern,omitempty"`
}

// ChangeDetectionConfig defines what changes to track
type ChangeDetectionConfig struct {
	Enabled       bool          `yaml:"enabled"`
	CheckInterval time.Duration `yaml:"check_interval"`
	AlertOn       AlertOnConfig `yaml:"alert_on"`
}

// AlertOnConfig defines which changes trigger alerts
type AlertOnConfig struct {
	NewUntaggedResources bool     `yaml:"new_untagged_resources"`
	StatusChanges        bool     `yaml:"status_changes"`
	DisappearedResources bool     `yaml:"disappeared_resources"`
	CostIncreases        bool     `yaml:"cost_increases"`
	TagChanges           []string `yaml:"tag_changes"`
}

// PerformanceConfig defines performance tuning
type PerformanceConfig struct {
	BatchSize       int `yaml:"batch_size"`
	ParallelWorkers int `yaml:"parallel_workers"`
	RateLimit       int `yaml:"rate_limit"`
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

// LoadFromPath loads configuration from a specific path or searches standard locations
func LoadFromPath(path string) (*Config, error) {
	if path == "" {
		path = findConfigFile()
		if path == "" {
			return LoadDefault(), nil
		}
	}

	// Expand home directory
	if len(path) >= 2 && path[:2] == "~/" {
		home, _ := os.UserHomeDir()
		path = filepath.Join(home, path[2:])
	}

	return LoadConfig(path)
}

// LoadDefault loads default configuration with sensible tiered scanning
func LoadDefault() *Config {
	config := &Config{
		Version:  "1.0",
		Provider: "aws",
		Region:   "us-east-1",
		Scanning: ScanningConfig{
			Enabled:       true,
			AdaptiveHours: true,
			Tiers:         defaultTiers(),
			ChangeDetection: ChangeDetectionConfig{
				Enabled:       true,
				CheckInterval: 5 * time.Minute,
				AlertOn: AlertOnConfig{
					NewUntaggedResources: true,
					StatusChanges:        true,
					DisappearedResources: true,
					TagChanges:           []string{"Owner", "Environment"},
				},
			},
			Performance: PerformanceConfig{
				BatchSize:       1000,
				ParallelWorkers: 4,
				RateLimit:       100,
			},
		},
	}
	return config
}

// findConfigFile searches for config in standard locations
func findConfigFile() string {
	locations := []string{
		"elava.yaml",
		".elava.yaml",
		"~/.elava/config.yaml",
		"/etc/elava/config.yaml",
	}

	for _, loc := range locations {
		if len(loc) >= 2 && loc[:2] == "~/" {
			home, _ := os.UserHomeDir()
			loc = filepath.Join(home, loc[2:])
		}
		if _, err := os.Stat(loc); err == nil {
			return loc
		}
	}
	return ""
}

// defaultTiers returns sensible default tier configuration
func defaultTiers() map[string]TierConfig {
	return map[string]TierConfig{
		"critical": {
			Description:  "Critical production resources",
			ScanInterval: 15 * time.Minute,
			Patterns: []TierPattern{
				{Type: "rds", Tags: map[string]string{"environment": "production"}},
				{Type: "nat_gateway"},
				{Type: "ec2", InstanceTypePattern: "*xlarge"},
			},
		},
		"production": {
			Description:  "Production resources",
			ScanInterval: 1 * time.Hour,
			Patterns: []TierPattern{
				{Tags: map[string]string{"environment": "production"}},
				{Status: "running"},
			},
		},
		"standard": {
			Description:  "Development and staging",
			ScanInterval: 4 * time.Hour,
			Patterns: []TierPattern{
				{Tags: map[string]string{"environment": "development"}},
				{Tags: map[string]string{"environment": "staging"}},
			},
		},
		"archive": {
			Description:  "Rarely changing resources",
			ScanInterval: 24 * time.Hour,
			Patterns: []TierPattern{
				{Types: []string{"snapshot", "ami"}},
				{Status: "stopped"},
			},
		},
	}
}
