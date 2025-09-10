package config

import (
	"os"
	"testing"
)

func TestLoadConfig(t *testing.T) {
	// Create a temp config file
	content := `
version: v1
region: us-east-1
provider: aws

resources:
  web-servers:
    type: ec2
    count: 3
    tags:
      team: platform

rules:
  protect_blessed: true
  notify_on_orphans: true
  grace_period: 5m
`
	tmpfile, err := os.CreateTemp("", "ovi-config-*.yaml")
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		_ = os.Remove(tmpfile.Name())
	}()

	if _, err := tmpfile.Write([]byte(content)); err != nil {
		t.Fatal(err)
	}
	if err := tmpfile.Close(); err != nil {
		t.Fatal(err)
	}

	// Load the config
	cfg, err := LoadConfig(tmpfile.Name())
	if err != nil {
		t.Fatalf("LoadConfig() error = %v", err)
	}

	// Verify config
	if cfg.Version != "v1" {
		t.Errorf("Version = %v, want v1", cfg.Version)
	}
	if cfg.Region != "us-east-1" {
		t.Errorf("Region = %v, want us-east-1", cfg.Region)
	}
	if cfg.Provider != "aws" {
		t.Errorf("Provider = %v, want aws", cfg.Provider)
	}
	if len(cfg.Resources) != 1 {
		t.Errorf("Resources count = %v, want 1", len(cfg.Resources))
	}
	if !cfg.Rules.ProtectBlessed {
		t.Error("ProtectBlessed should be true")
	}
}

func TestConfig_Validate(t *testing.T) {
	tests := []struct {
		name    string
		config  Config
		wantErr bool
	}{
		{
			name: "valid config",
			config: Config{
				Version:  "v1",
				Provider: "aws",
				Region:   "us-east-1",
			},
			wantErr: false,
		},
		{
			name: "missing version",
			config: Config{
				Provider: "aws",
				Region:   "us-east-1",
			},
			wantErr: true,
		},
		{
			name: "missing provider",
			config: Config{
				Version: "v1",
				Region:  "us-east-1",
			},
			wantErr: true,
		},
		{
			name: "missing region",
			config: Config{
				Version:  "v1",
				Provider: "aws",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
