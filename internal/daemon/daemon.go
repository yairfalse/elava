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
	"github.com/yairfalse/elava/analyzer"
	"github.com/yairfalse/elava/observer"
	"github.com/yairfalse/elava/providers"
	_ "github.com/yairfalse/elava/providers/aws" // Register AWS provider
	"github.com/yairfalse/elava/storage"
	"github.com/yairfalse/elava/types"
)

// Config holds daemon configuration
type Config struct {
	Interval      time.Duration
	MetricsPort   int
	Region        string
	StoragePath   string
	Provider      string                  // Cloud provider type (e.g., "aws")
	CloudProvider providers.CloudProvider // Optional: inject for testing
}

// Daemon manages continuous reconciliation
type Daemon struct {
	interval       time.Duration
	configPort     int
	actualPort     atomic.Int32
	region         string
	provider       string
	storagePath    string
	startTime      time.Time
	reconcileCount atomic.Int64

	// Infrastructure components
	storage        *storage.MVCCStorage
	changeDetector *analyzer.ChangeDetectorImpl
	metrics        *observer.ChangeEventMetrics
	cloudProvider  providers.CloudProvider
}

// NewDaemon creates a new daemon instance
func NewDaemon(config Config) (*Daemon, error) {
	// Open MVCC storage
	store, err := storage.NewMVCCStorage(config.StoragePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open storage: %w", err)
	}

	// Create change detector
	detector := analyzer.NewChangeDetector(store)

	// Create metrics observer
	metrics, err := observer.NewChangeEventMetrics()
	if err != nil {
		_ = store.Close()
		return nil, fmt.Errorf("failed to create metrics: %w", err)
	}

	// Initialize cloud provider (use injected or create new)
	var cloudProvider providers.CloudProvider
	if config.CloudProvider != nil {
		cloudProvider = config.CloudProvider
	} else if config.Provider != "" {
		providerConfig := providers.ProviderConfig{
			Type:   config.Provider,
			Region: config.Region,
		}
		var err error
		cloudProvider, err = providers.GetProvider(context.Background(), config.Provider, providerConfig)
		if err != nil {
			_ = store.Close()
			return nil, fmt.Errorf("failed to create provider: %w", err)
		}
	}

	return &Daemon{
		interval:       config.Interval,
		configPort:     config.MetricsPort,
		region:         config.Region,
		provider:       config.Provider,
		storagePath:    config.StoragePath,
		startTime:      time.Now(),
		storage:        store,
		changeDetector: detector,
		metrics:        metrics,
		cloudProvider:  cloudProvider,
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
	if port < 0 || port > 65535 {
		return fmt.Errorf("invalid port: %d", port)
	}
	d.actualPort.Store(int32(port))

	srv := &http.Server{
		Handler:           mux,
		ReadHeaderTimeout: 10 * time.Second,
	}

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

	// Skip if no provider configured (testing mode)
	if d.cloudProvider == nil {
		return
	}

	// List all resources from cloud
	resources, err := d.listResources(ctx)
	if err != nil {
		return // Errors logged internally
	}

	// Store observations
	if _, err := d.storage.RecordObservationBatch(resources); err != nil {
		return // Errors logged internally
	}

	// Detect changes
	events, err := d.changeDetector.DetectChanges(ctx, resources)
	if err != nil {
		return // Errors logged internally
	}

	// Store change events
	if len(events) > 0 {
		_ = d.storage.StoreChangeEventBatch(ctx, events)
	}

	// Record metrics
	d.metrics.RecordChangeEvents(ctx, events)
}

func (d *Daemon) listResources(ctx context.Context) ([]types.Resource, error) {
	filter := types.ResourceFilter{} // Empty filter = all resources
	return d.cloudProvider.ListResources(ctx, filter)
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

// Close shuts down daemon and releases resources
func (d *Daemon) Close() error {
	if d.storage != nil {
		return d.storage.Close()
	}
	return nil
}
