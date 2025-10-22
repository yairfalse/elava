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
	"github.com/rs/zerolog"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"

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
	changeMetrics  *observer.ChangeEventMetrics
	daemonMetrics  *DaemonMetrics
	cloudProvider  providers.CloudProvider
	logger         zerolog.Logger
}

// NewDaemon creates a new daemon instance
func NewDaemon(config Config) (*Daemon, error) {
	logger := newLogger(config.Region)
	logger.Info().
		Dur("interval", config.Interval).
		Int("metrics_port", config.MetricsPort).
		Str("provider", config.Provider).
		Msg("initializing daemon")

	store, err := storage.NewMVCCStorage(config.StoragePath)
	if err != nil {
		logger.Error().Err(err).Msg("failed to open storage")
		return nil, fmt.Errorf("failed to open storage: %w", err)
	}

	detector := analyzer.NewChangeDetector(store)

	changeMetrics, err := observer.NewChangeEventMetrics()
	if err != nil {
		logger.Error().Err(err).Msg("failed to create change metrics")
		_ = store.Close()
		return nil, fmt.Errorf("failed to create change metrics: %w", err)
	}

	daemonMetrics, err := NewDaemonMetrics()
	if err != nil {
		logger.Error().Err(err).Msg("failed to create daemon metrics")
		_ = store.Close()
		return nil, fmt.Errorf("failed to create daemon metrics: %w", err)
	}

	cloudProvider, err := initCloudProvider(config, logger)
	if err != nil {
		_ = store.Close()
		return nil, err
	}

	logger.Info().Msg("daemon initialized successfully")

	return &Daemon{
		interval:       config.Interval,
		configPort:     config.MetricsPort,
		region:         config.Region,
		provider:       config.Provider,
		storagePath:    config.StoragePath,
		startTime:      time.Now(),
		storage:        store,
		changeDetector: detector,
		changeMetrics:  changeMetrics,
		daemonMetrics:  daemonMetrics,
		cloudProvider:  cloudProvider,
		logger:         logger,
	}, nil
}

func newLogger(region string) zerolog.Logger {
	return zerolog.New(zerolog.NewConsoleWriter()).
		With().
		Timestamp().
		Str("service", "elava-daemon").
		Str("region", region).
		Logger()
}

func initCloudProvider(config Config, logger zerolog.Logger) (providers.CloudProvider, error) {
	if config.CloudProvider != nil {
		logger.Info().Msg("using injected cloud provider")
		return config.CloudProvider, nil
	}

	if config.Provider == "" {
		logger.Warn().Msg("no cloud provider configured - reconciliation will be skipped")
		return nil, nil
	}

	logger.Info().Str("provider", config.Provider).Msg("initializing cloud provider")
	providerConfig := providers.ProviderConfig{
		Type:   config.Provider,
		Region: config.Region,
	}

	cloudProvider, err := providers.GetProvider(context.Background(), config.Provider, providerConfig)
	if err != nil {
		logger.Error().Err(err).Msg("failed to create provider")
		return nil, fmt.Errorf("failed to create provider: %w", err)
	}

	logger.Info().Msg("cloud provider initialized successfully")
	return cloudProvider, nil
}

// Start begins the daemon's reconciliation loop
func (d *Daemon) Start(ctx context.Context) error {
	d.logger.Info().Msg("starting daemon")

	var g run.Group

	// Metrics HTTP server
	g.Add(func() error {
		return d.runMetricsServer(ctx)
	}, func(error) {
		d.logger.Info().Msg("metrics server shutting down")
	})

	// Reconciliation loop
	g.Add(func() error {
		return d.runReconcileLoop(ctx)
	}, func(error) {
		d.logger.Info().Msg("reconciliation loop shutting down")
	})

	d.logger.Info().Msg("all actors started")
	err := g.Run()

	if err != nil {
		d.logger.Error().Err(err).Msg("daemon stopped with error")
	} else {
		d.logger.Info().Msg("daemon stopped gracefully")
	}

	return err
}

func (d *Daemon) runReconcileLoop(ctx context.Context) error {
	d.logger.Info().Dur("interval", d.interval).Msg("reconciliation loop started")

	ticker := time.NewTicker(d.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			d.logger.Info().Msg("reconciliation loop stopped by context")
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
		d.logger.Error().Err(err).Int("port", d.configPort).Msg("failed to listen")
		return fmt.Errorf("metrics server listen: %w", err)
	}

	port := listener.Addr().(*net.TCPAddr).Port
	if port < 0 || port > 65535 {
		return fmt.Errorf("invalid port: %d", port)
	}
	d.actualPort.Store(int32(port))

	d.logger.Info().Int("port", port).Msg("metrics server listening")

	srv := &http.Server{
		Handler:           mux,
		ReadHeaderTimeout: 10 * time.Second,
	}

	go func() {
		<-ctx.Done()
		d.logger.Info().Msg("shutting down metrics server")
		_ = srv.Close()
	}()

	if err := srv.Serve(listener); err != http.ErrServerClosed {
		d.logger.Error().Err(err).Msg("metrics server error")
		return fmt.Errorf("metrics server: %w", err)
	}
	return nil
}

func (d *Daemon) runReconciliation(ctx context.Context) {
	runNum := d.reconcileCount.Add(1)
	logger := d.logger.With().Int64("run", runNum).Logger()
	logger.Debug().Msg("reconciliation started")
	startTime := time.Now()

	// Record run attempt
	d.daemonMetrics.reconcileRunsTotal.Add(ctx, 1)

	if d.cloudProvider == nil {
		logger.Debug().Msg("no cloud provider - skipping reconciliation")
		return
	}

	resources, err := d.listResources(ctx)
	if err != nil {
		logger.Error().Err(err).Msg("failed to list resources from cloud")
		d.daemonMetrics.reconcileFailuresTotal.Add(ctx, 1)
		return
	}
	logger.Info().Int("count", len(resources)).Msg("resources discovered")
	d.daemonMetrics.resourcesDiscovered.Record(ctx, int64(len(resources)))

	if err := d.storeObservations(resources, logger); err != nil {
		d.daemonMetrics.reconcileFailuresTotal.Add(ctx, 1)
		return
	}

	events, err := d.detectAndLogChanges(ctx, resources, logger)
	if err != nil {
		d.daemonMetrics.reconcileFailuresTotal.Add(ctx, 1)
		return
	}

	d.storeAndRecordMetrics(ctx, events, logger)

	duration := time.Since(startTime)
	d.daemonMetrics.reconcileDuration.Record(ctx, duration.Seconds())
	logger.Info().Dur("duration", duration).Msg("reconciliation completed")
}

func (d *Daemon) storeObservations(resources []types.Resource, logger zerolog.Logger) error {
	revision, err := d.storage.RecordObservationBatch(resources)
	if err != nil {
		logger.Error().Err(err).Msg("failed to store observations")
		d.daemonMetrics.storageWritesFailed.Add(context.Background(), 1)
		return err
	}
	logger.Debug().Int64("revision", revision).Msg("observations stored")
	return nil
}

func (d *Daemon) detectAndLogChanges(ctx context.Context, resources []types.Resource, logger zerolog.Logger) ([]storage.ChangeEvent, error) {
	events, err := d.changeDetector.DetectChanges(ctx, resources)
	if err != nil {
		logger.Error().Err(err).Msg("failed to detect changes")
		return nil, err
	}

	created, modified, disappeared := d.countChangeEvents(events)
	logger.Info().
		Int("created", created).
		Int("modified", modified).
		Int("disappeared", disappeared).
		Msg("changes detected")

	return events, nil
}

func (d *Daemon) storeAndRecordMetrics(ctx context.Context, events []storage.ChangeEvent, logger zerolog.Logger) {
	if len(events) > 0 {
		if err := d.storage.StoreChangeEventBatch(ctx, events); err != nil {
			logger.Error().Err(err).Msg("failed to store change events")
			d.daemonMetrics.storageWritesFailed.Add(ctx, 1)
		}
	}

	// Record change events in both metrics systems
	d.changeMetrics.RecordChangeEvents(ctx, events)

	// Record counts by type in daemon metrics
	for _, event := range events {
		d.daemonMetrics.changeEventsTotal.Add(ctx, 1,
			metric.WithAttributes(
				attribute.String("type", event.ChangeType),
			),
		)
	}
}

func (d *Daemon) countChangeEvents(events []storage.ChangeEvent) (created, modified, disappeared int) {
	for _, e := range events {
		switch e.ChangeType {
		case "created":
			created++
		case "modified":
			modified++
		case "disappeared":
			disappeared++
		}
	}
	return
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
