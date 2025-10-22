package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/spf13/cobra"
	"github.com/yairfalse/elava/internal/daemon"
)

var (
	daemonInterval    time.Duration
	daemonMetricsPort int
	daemonRegion      string
	daemonProvider    string
	daemonStoragePath string
)

// daemonCmd represents the daemon command
var daemonCmd = &cobra.Command{
	Use:   "daemon",
	Short: "Run continuous reconciliation daemon",
	Long: `Run Elava in daemon mode for continuous infrastructure reconciliation.

The daemon continuously scans your cloud infrastructure at configured intervals,
detecting changes, tracking resource history, and exporting metrics.

Features:
- Continuous reconciliation loop (Kubernetes-style)
- Prometheus metrics on /metrics endpoint
- Health checks on /health, /-/healthy, /-/ready
- MVCC storage for full resource history
- Graceful shutdown on SIGTERM/SIGINT`,
	Example: `  elava daemon                                    # Run with defaults
  elava daemon --interval 5m                      # Scan every 5 minutes
  elava daemon --metrics-port 9090                # Custom metrics port
  elava daemon --region us-west-2                 # Specific region
  elava daemon --provider aws --region us-east-1  # AWS in us-east-1`,
	RunE: runDaemon,
}

func init() {
	rootCmd.AddCommand(daemonCmd)

	daemonCmd.Flags().DurationVar(&daemonInterval, "interval", 5*time.Minute, "Reconciliation interval")
	daemonCmd.Flags().IntVar(&daemonMetricsPort, "metrics-port", 2112, "Metrics HTTP server port")
	daemonCmd.Flags().StringVar(&daemonRegion, "region", "us-east-1", "Cloud region")
	daemonCmd.Flags().StringVar(&daemonProvider, "provider", "aws", "Cloud provider (aws, gcp)")
	daemonCmd.Flags().StringVar(&daemonStoragePath, "storage", "./elava.db", "Storage database path")
}

func runDaemon(cmd *cobra.Command, args []string) error {
	fmt.Printf("🚀 Starting Elava daemon...\n")
	fmt.Printf("   Provider: %s\n", daemonProvider)
	fmt.Printf("   Region: %s\n", daemonRegion)
	fmt.Printf("   Interval: %s\n", daemonInterval)
	fmt.Printf("   Metrics port: %d\n", daemonMetricsPort)
	fmt.Printf("   Storage: %s\n\n", daemonStoragePath)

	// Create daemon config
	config := daemon.Config{
		Interval:    daemonInterval,
		MetricsPort: daemonMetricsPort,
		Region:      daemonRegion,
		Provider:    daemonProvider,
		StoragePath: daemonStoragePath,
	}

	// Initialize daemon
	d, err := daemon.NewDaemon(config)
	if err != nil {
		return fmt.Errorf("failed to create daemon: %w", err)
	}
	defer func() { _ = d.Close() }()

	// Setup signal handling
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGTERM, syscall.SIGINT)

	go func() {
		sig := <-sigCh
		fmt.Printf("\n📋 Received %s, shutting down gracefully...\n", sig)
		cancel()
	}()

	// Print metrics URL after short delay to let server start
	go func() {
		time.Sleep(200 * time.Millisecond)
		port := d.MetricsPort()
		if port > 0 {
			fmt.Printf("📊 Metrics: http://localhost:%d/metrics\n", port)
			fmt.Printf("💚 Health: http://localhost:%d/health\n", port)
			fmt.Printf("✅ Ready: http://localhost:%d/-/ready\n\n", port)
		}
	}()

	// Start daemon
	fmt.Println("✨ Daemon running (Ctrl+C to stop)...")
	if err := d.Start(ctx); err != nil {
		return fmt.Errorf("daemon error: %w", err)
	}

	fmt.Println("\n👋 Daemon stopped")
	return nil
}
