package daemon

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Test NewDaemon constructor
func TestNewDaemon(t *testing.T) {
	config := Config{
		Interval:    5 * time.Minute,
		MetricsPort: 2112,
		Region:      "us-east-1",
		StoragePath: t.TempDir(),
	}

	daemon, err := NewDaemon(config)

	require.NoError(t, err)
	assert.NotNil(t, daemon)
	assert.Equal(t, config.Interval, daemon.interval)
	assert.Equal(t, config.MetricsPort, daemon.metricsPort)
}

// Test daemon starts successfully
func TestDaemon_Start(t *testing.T) {
	config := Config{
		Interval:    1 * time.Second,
		MetricsPort: 0, // Random port
		Region:      "us-east-1",
		StoragePath: t.TempDir(),
	}

	daemon, err := NewDaemon(config)
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	errCh := make(chan error, 1)
	go func() {
		errCh <- daemon.Start(ctx)
	}()

	// Give it time to start
	time.Sleep(100 * time.Millisecond)

	// Should be running (no error yet)
	select {
	case err := <-errCh:
		t.Fatalf("Daemon exited early: %v", err)
	default:
		// Good - still running
	}

	// Stop it
	cancel()

	// Should exit cleanly
	err = <-errCh
	assert.NoError(t, err)
}

// Test daemon stops gracefully
func TestDaemon_GracefulShutdown(t *testing.T) {
	config := Config{
		Interval:    1 * time.Second,
		MetricsPort: 0,
		Region:      "us-east-1",
		StoragePath: t.TempDir(),
	}

	daemon, err := NewDaemon(config)
	require.NoError(t, err)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	errCh := make(chan error, 1)
	go func() {
		errCh <- daemon.Start(ctx)
	}()

	// Let it run briefly
	time.Sleep(200 * time.Millisecond)

	// Cancel context (simulate SIGTERM)
	cancel()

	// Should shutdown within timeout
	select {
	case err := <-errCh:
		assert.NoError(t, err)
	case <-time.After(2 * time.Second):
		t.Fatal("Daemon did not shutdown within timeout")
	}
}

// Test health check returns status
func TestDaemon_Health(t *testing.T) {
	config := Config{
		Interval:    5 * time.Minute,
		MetricsPort: 0,
		Region:      "us-east-1",
		StoragePath: t.TempDir(),
	}

	daemon, err := NewDaemon(config)
	require.NoError(t, err)

	health := daemon.Health()

	assert.NotEmpty(t, health.Status)
	assert.GreaterOrEqual(t, health.Uptime, int64(0))
}
