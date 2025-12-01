package telemetry

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/yairfalse/elava/internal/config"
)

func TestNewProvider_Disabled(t *testing.T) {
	cfg := config.OTELConfig{
		ServiceName: "test-elava",
		Traces:      config.TracesConfig{Enabled: false},
		Metrics:     config.MetricsConfig{Enabled: false},
	}

	p, err := NewProvider(context.Background(), cfg)
	require.NoError(t, err)
	require.NotNil(t, p)

	assert.NotNil(t, p.Tracer())
	assert.NotNil(t, p.Meter())

	err = p.Shutdown(context.Background())
	require.NoError(t, err)
}

func TestNewProvider_WithEndpoint(t *testing.T) {
	cfg := config.OTELConfig{
		Endpoint:    "localhost:4317",
		Insecure:    true,
		ServiceName: "test-elava",
		Traces:      config.TracesConfig{Enabled: true, SampleRate: 1.0},
		Metrics:     config.MetricsConfig{Enabled: true},
	}

	// Provider setup should succeed even without a real collector
	p, err := NewProvider(context.Background(), cfg)
	require.NoError(t, err)
	require.NotNil(t, p)

	// Use short timeout for shutdown - collector isn't running
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	// Shutdown may fail due to no collector, that's OK for this test
	_ = p.Shutdown(ctx)
}

func TestProvider_StartSpan(t *testing.T) {
	cfg := config.OTELConfig{
		ServiceName: "test-elava",
		Traces:      config.TracesConfig{Enabled: false},
		Metrics:     config.MetricsConfig{Enabled: false},
	}

	p, err := NewProvider(context.Background(), cfg)
	require.NoError(t, err)

	ctx, span := p.StartSpan(context.Background(), "test-operation")
	require.NotNil(t, ctx)
	require.NotNil(t, span)

	span.End()
	_ = p.Shutdown(context.Background())
}

func TestProvider_RecordScanDuration(t *testing.T) {
	cfg := config.OTELConfig{
		ServiceName: "test-elava",
		Traces:      config.TracesConfig{Enabled: false},
		Metrics:     config.MetricsConfig{Enabled: false},
	}

	p, err := NewProvider(context.Background(), cfg)
	require.NoError(t, err)

	// Should not panic
	p.RecordScanDuration(context.Background(), "aws", "us-east-1", "ec2", 100*time.Millisecond)

	_ = p.Shutdown(context.Background())
}

func TestProvider_RecordResourceCount(t *testing.T) {
	cfg := config.OTELConfig{
		ServiceName: "test-elava",
		Traces:      config.TracesConfig{Enabled: false},
		Metrics:     config.MetricsConfig{Enabled: false},
	}

	p, err := NewProvider(context.Background(), cfg)
	require.NoError(t, err)

	// Should not panic
	p.RecordResourceCount(context.Background(), "aws", "us-east-1", "ec2", 42)

	_ = p.Shutdown(context.Background())
}

func TestProvider_RecordError(t *testing.T) {
	cfg := config.OTELConfig{
		ServiceName: "test-elava",
		Traces:      config.TracesConfig{Enabled: false},
		Metrics:     config.MetricsConfig{Enabled: false},
	}

	p, err := NewProvider(context.Background(), cfg)
	require.NoError(t, err)

	// Should not panic
	p.RecordError(context.Background(), "aws", "us-east-1", "ec2")

	_ = p.Shutdown(context.Background())
}
