// Package config handles TOML configuration for Elava.
package config

import (
	"fmt"
	"os"
	"time"

	"github.com/BurntSushi/toml"
)

// Config is the root configuration structure.
type Config struct {
	AWS     AWSConfig     `toml:"aws"`
	OTEL    OTELConfig    `toml:"otel"`
	Scanner ScannerConfig `toml:"scanner"`
	Log     LogConfig     `toml:"log"`
}

// AWSConfig holds AWS provider settings.
type AWSConfig struct {
	Regions []string `toml:"regions"`
	Profile string   `toml:"profile"`
}

// OTELConfig holds OpenTelemetry settings.
type OTELConfig struct {
	Endpoint    string        `toml:"endpoint"`
	Insecure    bool          `toml:"insecure"`
	ServiceName string        `toml:"service_name"`
	Traces      TracesConfig  `toml:"traces"`
	Metrics     MetricsConfig `toml:"metrics"`
}

// TracesConfig holds tracing settings.
type TracesConfig struct {
	Enabled    bool    `toml:"enabled"`
	SampleRate float64 `toml:"sample_rate"`
}

// MetricsConfig holds metrics settings.
type MetricsConfig struct {
	Enabled bool `toml:"enabled"`
}

// ScannerConfig holds scanner settings.
type ScannerConfig struct {
	IntervalStr string `toml:"interval"`
	Interval    time.Duration
	OneShot     bool `toml:"one_shot"`
}

// LogConfig holds logging settings.
type LogConfig struct {
	Level string `toml:"level"`
}

// Load reads and parses a TOML config file.
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config file: %w", err)
	}

	cfg := &Config{}
	if err := toml.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}

	applyDefaults(cfg)

	if err := parseInterval(cfg); err != nil {
		return nil, err
	}

	return cfg, nil
}

func applyDefaults(cfg *Config) {
	if cfg.OTEL.ServiceName == "" {
		cfg.OTEL.ServiceName = "elava"
	}
	if cfg.Scanner.IntervalStr == "" {
		cfg.Scanner.IntervalStr = "5m"
	}
	if cfg.Log.Level == "" {
		cfg.Log.Level = "info"
	}
}

func parseInterval(cfg *Config) error {
	d, err := time.ParseDuration(cfg.Scanner.IntervalStr)
	if err != nil {
		return fmt.Errorf("parse interval %q: %w", cfg.Scanner.IntervalStr, err)
	}
	cfg.Scanner.Interval = d
	return nil
}

// Validate checks the configuration is valid.
func (c *Config) Validate() error {
	if len(c.AWS.Regions) == 0 {
		return fmt.Errorf("aws: at least one region required")
	}
	if c.OTEL.Traces.SampleRate < 0.0 || c.OTEL.Traces.SampleRate > 1.0 {
		return fmt.Errorf("otel: traces.sample_rate must be between 0.0 and 1.0 (got %v)", c.OTEL.Traces.SampleRate)
	}
	return nil
}
