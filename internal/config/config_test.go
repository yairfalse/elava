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
		AWS:     AWSConfig{Regions: []string{"us-east-1", "eu-west-1"}},
		Scanner: ScannerConfig{MaxConcurrency: 5},
	}
	err := cfg.Validate()
	require.NoError(t, err)
}

func TestLoad_MaxConcurrency(t *testing.T) {
	content := `
[aws]
regions = ["us-east-1"]

[scanner]
max_concurrency = 10
`
	path := writeTempConfig(t, content)
	cfg, err := Load(path)

	require.NoError(t, err)
	assert.Equal(t, 10, cfg.Scanner.MaxConcurrency)
}

func TestLoad_MaxConcurrency_Default(t *testing.T) {
	content := `
[aws]
regions = ["us-east-1"]
`
	path := writeTempConfig(t, content)
	cfg, err := Load(path)

	require.NoError(t, err)
	assert.Equal(t, 5, cfg.Scanner.MaxConcurrency)
}

func TestLoad_FilterConfig(t *testing.T) {
	content := `
[aws]
regions = ["us-east-1"]

[scanner]
exclude_types = ["cloudwatch_logs", "iam_role"]

[scanner.include_tags]
env = "prod"
team = "platform"

[scanner.exclude_tags]
"do-not-scan" = "true"
`
	path := writeTempConfig(t, content)
	cfg, err := Load(path)

	require.NoError(t, err)
	assert.Equal(t, []string{"cloudwatch_logs", "iam_role"}, cfg.Scanner.ExcludeTypes)
	assert.Equal(t, map[string]string{"env": "prod", "team": "platform"}, cfg.Scanner.IncludeTags)
	assert.Equal(t, map[string]string{"do-not-scan": "true"}, cfg.Scanner.ExcludeTags)
}

func TestLoad_FilterConfig_Empty(t *testing.T) {
	content := `
[aws]
regions = ["us-east-1"]
`
	path := writeTempConfig(t, content)
	cfg, err := Load(path)

	require.NoError(t, err)
	assert.Nil(t, cfg.Scanner.ExcludeTypes)
	assert.Nil(t, cfg.Scanner.IncludeTags)
	assert.Nil(t, cfg.Scanner.ExcludeTags)
}

func TestConfig_Validate_InvalidMaxConcurrency(t *testing.T) {
	// Test Validate() directly (bypassing Load which applies defaults)
	// to ensure validation catches invalid values
	cfg := &Config{
		AWS:     AWSConfig{Regions: []string{"us-east-1"}},
		Scanner: ScannerConfig{MaxConcurrency: 0},
	}
	err := cfg.Validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "max_concurrency")
}

func writeTempConfig(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")
	err := os.WriteFile(path, []byte(content), 0644)
	require.NoError(t, err)
	return path
}
