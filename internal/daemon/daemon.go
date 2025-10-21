package daemon

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"sync/atomic"
	"time"

	"github.com/oklog/run"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// Config holds daemon configuration
type Config struct {
	Interval    time.Duration
	MetricsPort int
	Region      string
	StoragePath string
}

// Daemon manages continuous reconciliation
type Daemon struct {
	interval       time.Duration
	configPort     int
	actualPort     atomic.Int32
	region         string
	storagePath    string
	startTime      time.Time
	reconcileCount atomic.Int64
}

// NewDaemon creates a new daemon instance
func NewDaemon(config Config) (*Daemon, error) {
	return &Daemon{
		interval:    config.Interval,
		configPort:  config.MetricsPort,
		region:      config.Region,
		storagePath: config.StoragePath,
		startTime:   time.Now(),
	}, nil
}

// Start begins the daemon's reconciliation loop
func (d *Daemon) Start(ctx context.Context) error {
	var g run.Group

	// Metrics HTTP server
	g.Add(func() error {
		return d.runMetricsServer(ctx)
	}, func(error) {})

	// Reconciliation loop
	g.Add(func() error {
		return d.runReconcileLoop(ctx)
	}, func(error) {})

	return g.Run()
}

func (d *Daemon) runReconcileLoop(ctx context.Context) error {
	ticker := time.NewTicker(d.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			d.runReconciliation(ctx)
		}
	}
}

func (d *Daemon) runMetricsServer(ctx context.Context) error {
	mux := http.NewServeMux()
	mux.Handle("/metrics", promhttp.Handler())

	listener, err := net.Listen("tcp", fmt.Sprintf(":%d", d.configPort))
	if err != nil {
		return fmt.Errorf("metrics server listen: %w", err)
	}

	port := listener.Addr().(*net.TCPAddr).Port
	d.actualPort.Store(int32(port))

	srv := &http.Server{Handler: mux}

	go func() {
		<-ctx.Done()
		_ = srv.Close()
	}()

	if err := srv.Serve(listener); err != http.ErrServerClosed {
		return fmt.Errorf("metrics server: %w", err)
	}
	return nil
}

func (d *Daemon) runReconciliation(ctx context.Context) {
	d.reconcileCount.Add(1)
}

// Health returns daemon health status
func (d *Daemon) Health() HealthStatus {
	return HealthStatus{
		Status: "healthy",
		Uptime: int64(time.Since(d.startTime).Seconds()),
	}
}

// HealthStatus represents daemon health
type HealthStatus struct {
	Status string
	Uptime int64
}

// ReconciliationCount returns total reconciliations run
func (d *Daemon) ReconciliationCount() int64 {
	return d.reconcileCount.Load()
}

// MetricsPort returns the actual port metrics server is listening on
func (d *Daemon) MetricsPort() int {
	return int(d.actualPort.Load())
}
