package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoad_ValidConfig(t *testing.T) {
	content := `
[aws]
regions = ["us-east-1", "eu-west-1"]
profile = "production"

[otel]
endpoint = "localhost:4317"
insecure = true
service_name = "elava"

[otel.traces]
enabled = true
sample_rate = 1.0

[otel.metrics]
enabled = true

[scanner]
interval = "5m"
one_shot = false

[log]
level = "info"
`
	path := writeTempConfig(t, content)
	cfg, err := Load(path)

	require.NoError(t, err)
	assert.Equal(t, []string{"us-east-1", "eu-west-1"}, cfg.AWS.Regions)
	assert.Equal(t, "production", cfg.AWS.Profile)
	assert.Equal(t, "localhost:4317", cfg.OTEL.Endpoint)
	assert.True(t, cfg.OTEL.Insecure)
	assert.Equal(t, "elava", cfg.OTEL.ServiceName)
	assert.True(t, cfg.OTEL.Traces.Enabled)
	assert.Equal(t, 1.0, cfg.OTEL.Traces.SampleRate)
	assert.True(t, cfg.OTEL.Metrics.Enabled)
	assert.Equal(t, 5*time.Minute, cfg.Scanner.Interval)
	assert.False(t, cfg.Scanner.OneShot)
	assert.Equal(t, "info", cfg.Log.Level)
}

func TestLoad_Defaults(t *testing.T) {
	content := `
[aws]
regions = ["us-east-1"]
`
	path := writeTempConfig(t, content)
	cfg, err := Load(path)

	require.NoError(t, err)
	// Check defaults are applied
	assert.Equal(t, "elava", cfg.OTEL.ServiceName)
	assert.Equal(t, 5*time.Minute, cfg.Scanner.Interval)
	assert.Equal(t, "info", cfg.Log.Level)
}

func TestLoad_FileNotFound(t *testing.T) {
	_, err := Load("/nonexistent/config.toml")
	require.Error(t, err)
}

func TestLoad_InvalidTOML(t *testing.T) {
	content := `
[aws
regions = "not an array"
`
	path := writeTempConfig(t, content)
	_, err := Load(path)
	require.Error(t, err)
}

func TestLoad_InvalidDuration(t *testing.T) {
	content := `
[aws]
regions = ["us-east-1"]

[scanner]
interval = "not-a-duration"
`
	path := writeTempConfig(t, content)
	_, err := Load(path)
	require.Error(t, err)
}

func TestConfig_Validate_NoRegions(t *testing.T) {
	cfg := &Config{
		AWS: AWSConfig{Regions: []string{}},
	}
	err := cfg.Validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "at least one region")
}

func TestConfig_Validate_Valid(t *testing.T) {
	cfg := &Config{
		AWS: AWSConfig{Regions: []string{"us-east-1", "eu-west-1"}},
	}
	err := cfg.Validate()
	require.NoError(t, err)
}

func writeTempConfig(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")
	err := os.WriteFile(path, []byte(content), 0644)
	require.NoError(t, err)
	return path
}
