package daemon

import (
	"context"
	"sync/atomic"
	"time"
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
	metricsPort    int
	region         string
	storagePath    string
	startTime      time.Time
	reconcileCount atomic.Int64
}

// NewDaemon creates a new daemon instance
func NewDaemon(config Config) (*Daemon, error) {
	return &Daemon{
		interval:    config.Interval,
		metricsPort: config.MetricsPort,
		region:      config.Region,
		storagePath: config.StoragePath,
		startTime:   time.Now(),
	}, nil
}

// Start begins the daemon's reconciliation loop
func (d *Daemon) Start(ctx context.Context) error {
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
