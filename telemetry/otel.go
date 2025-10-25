package telemetry

import (
	"context"
	"fmt"
	"os"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/exporters/prometheus"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/propagation"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.21.0"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	promclient "github.com/prometheus/client_golang/prometheus"
)

// Global telemetry handles - CLAUDE.md: Direct OTEL, no wrappers
var (
	// Tracer for distributed tracing
	Tracer = otel.Tracer("github.com/yairfalse/elava")

	// Meter for metrics
	Meter = otel.Meter("github.com/yairfalse/elava")

	// PrometheusRegistry for Prometheus scraping (dual export pattern)
	// The OTEL exporter automatically registers itself with this registry
	PrometheusRegistry *promclient.Registry

	// Metrics - following OTEL naming conventions
	ResourcesScanned   metric.Int64Counter
	UntrackedFound     metric.Int64Counter
	ScanDuration       metric.Float64Histogram
	StorageWrites      metric.Int64Counter
	StorageRevision    metric.Int64Gauge
	ResourcesInStorage metric.Int64Gauge
)

// Config for OTEL initialization
type Config struct {
	ServiceName    string
	ServiceVersion string
	Environment    string
	OTELEndpoint   string // e.g., "localhost:4317" for Urpo
	Insecure       bool   // true for local dev
}

// InitOTEL initializes OpenTelemetry with traces and metrics
func InitOTEL(ctx context.Context, cfg Config) (shutdown func(context.Context) error, err error) {
	cfg = applyConfigDefaults(cfg)

	res, err := createOTELResource(cfg)
	if err != nil {
		return nil, err
	}

	return setupProviders(ctx, cfg, res)
}

// applyConfigDefaults applies default values to config
func applyConfigDefaults(cfg Config) Config {
	if cfg.OTELEndpoint == "" {
		cfg.OTELEndpoint = os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT")
		if cfg.OTELEndpoint == "" {
			cfg.OTELEndpoint = "localhost:4317" // Default to local Urpo
		}
	}

	if cfg.ServiceName == "" {
		cfg.ServiceName = "ovi"
	}

	return cfg
}

// createOTELResource creates the OTEL resource with service information
func createOTELResource(cfg Config) (*resource.Resource, error) {
	res, err := resource.Merge(
		resource.Default(),
		resource.NewWithAttributes(
			semconv.SchemaURL,
			semconv.ServiceName(cfg.ServiceName),
			semconv.ServiceVersion(cfg.ServiceVersion),
			attribute.String("environment", cfg.Environment),
		),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create resource: %w", err)
	}
	return res, nil
}

// setupProviders sets up trace and metric providers
func setupProviders(ctx context.Context, cfg Config, res *resource.Resource) (func(context.Context) error, error) {
	traceShutdown, err := setupTraceProvider(ctx, cfg, res)
	if err != nil {
		return nil, fmt.Errorf("failed to setup traces: %w", err)
	}

	metricShutdown, err := setupMetricProvider(ctx, cfg, res)
	if err != nil {
		_ = traceShutdown(ctx)
		return nil, fmt.Errorf("failed to setup metrics: %w", err)
	}

	if err := initMetrics(); err != nil {
		_ = traceShutdown(ctx)
		_ = metricShutdown(ctx)
		return nil, fmt.Errorf("failed to initialize metrics: %w", err)
	}

	return createCombinedShutdown(traceShutdown, metricShutdown), nil
}

// createCombinedShutdown creates a combined shutdown function
func createCombinedShutdown(traceShutdown, metricShutdown func(context.Context) error) func(context.Context) error {
	return func(ctx context.Context) error {
		var err error
		if e := traceShutdown(ctx); e != nil {
			err = fmt.Errorf("trace shutdown failed: %w", e)
		}
		if e := metricShutdown(ctx); e != nil && err == nil {
			err = fmt.Errorf("metric shutdown failed: %w", e)
		}
		return err
	}
}

// setupTraceProvider configures trace provider with OTLP exporter
func setupTraceProvider(ctx context.Context, cfg Config, res *resource.Resource) (func(context.Context) error, error) {
	// Configure connection
	opts := []otlptracegrpc.Option{
		otlptracegrpc.WithEndpoint(cfg.OTELEndpoint),
	}

	if cfg.Insecure {
		opts = append(opts, otlptracegrpc.WithDialOption(
			grpc.WithTransportCredentials(insecure.NewCredentials()),
		))
	}

	// Create OTLP trace exporter
	exporter, err := otlptracegrpc.New(ctx, opts...)
	if err != nil {
		return nil, fmt.Errorf("failed to create trace exporter: %w", err)
	}

	// Create trace provider
	provider := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter,
			sdktrace.WithBatchTimeout(5*time.Second),
		),
		sdktrace.WithResource(res),
		sdktrace.WithSampler(sdktrace.AlwaysSample()), // Sample everything for now
	)

	// Set global provider
	otel.SetTracerProvider(provider)
	otel.SetTextMapPropagator(propagation.TraceContext{})

	// Update global tracer
	Tracer = provider.Tracer("github.com/yairfalse/elava")

	return provider.Shutdown, nil
}

// setupMetricProvider configures metric provider with dual export (Prometheus + OTLP)
// Following Beyla pattern: Prometheus for pull-based scraping + OTLP for push-based export
func setupMetricProvider(ctx context.Context, cfg Config, res *resource.Resource) (func(context.Context) error, error) {
	var readers []sdkmetric.Reader

	// 1. Prometheus exporter (pull-based)
	// Create a custom registry for the OTEL exporter
	registry := promclient.NewRegistry()
	PrometheusRegistry = registry

	prometheusExporter, err := prometheus.New(
		prometheus.WithRegisterer(registry),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create prometheus exporter: %w", err)
	}
	readers = append(readers, prometheusExporter)

	// 2. OTLP exporter (push-based) - optional, controlled by env var
	if cfg.OTELEndpoint != "" {
		otlpReader, err := createOTLPReader(ctx, cfg)
		if err != nil {
			return nil, fmt.Errorf("failed to create OTLP metric reader: %w", err)
		}
		readers = append(readers, otlpReader)
	}

	// Create metric provider with both readers
	providerOpts := []sdkmetric.Option{
		sdkmetric.WithResource(res),
	}
	for _, reader := range readers {
		providerOpts = append(providerOpts, sdkmetric.WithReader(reader))
	}

	provider := sdkmetric.NewMeterProvider(providerOpts...)

	// Set global provider
	otel.SetMeterProvider(provider)

	// Update global meter
	Meter = provider.Meter("github.com/yairfalse/elava")

	return provider.Shutdown, nil
}

// createOTLPReader creates an OTLP periodic reader for push-based export
func createOTLPReader(ctx context.Context, cfg Config) (sdkmetric.Reader, error) {
	opts := []otlpmetricgrpc.Option{
		otlpmetricgrpc.WithEndpoint(cfg.OTELEndpoint),
	}

	if cfg.Insecure {
		opts = append(opts, otlpmetricgrpc.WithDialOption(
			grpc.WithTransportCredentials(insecure.NewCredentials()),
		))
	}

	exporter, err := otlpmetricgrpc.New(ctx, opts...)
	if err != nil {
		return nil, fmt.Errorf("failed to create OTLP exporter: %w", err)
	}

	return sdkmetric.NewPeriodicReader(exporter,
		sdkmetric.WithInterval(10*time.Second), // Export every 10s
	), nil
}

// initMetrics initializes all metric instruments
func initMetrics() error {
	if err := initCounters(); err != nil {
		return err
	}

	if err := initHistograms(); err != nil {
		return err
	}

	return initGauges()
}

// initCounters initializes counter metrics
func initCounters() error {
	var err error

	ResourcesScanned, err = Meter.Int64Counter("ovi.resources.scanned.total",
		metric.WithDescription("Total number of resources scanned"),
		metric.WithUnit("1"),
	)
	if err != nil {
		return fmt.Errorf("failed to create resources_scanned counter: %w", err)
	}

	UntrackedFound, err = Meter.Int64Counter("ovi.untracked.found.total",
		metric.WithDescription("Total number of untracked resources found"),
		metric.WithUnit("1"),
	)
	if err != nil {
		return fmt.Errorf("failed to create untracked_found counter: %w", err)
	}

	StorageWrites, err = Meter.Int64Counter("ovi.storage.writes.total",
		metric.WithDescription("Total number of storage write operations"),
		metric.WithUnit("1"),
	)
	if err != nil {
		return fmt.Errorf("failed to create storage_writes counter: %w", err)
	}

	return nil
}

// initHistograms initializes histogram metrics
func initHistograms() error {
	var err error

	ScanDuration, err = Meter.Float64Histogram("ovi.scan.duration.seconds",
		metric.WithDescription("Duration of scan operations"),
		metric.WithUnit("s"),
	)
	if err != nil {
		return fmt.Errorf("failed to create scan_duration histogram: %w", err)
	}

	return nil
}

// initGauges initializes gauge metrics
func initGauges() error {
	var err error

	StorageRevision, err = Meter.Int64Gauge("ovi.storage.revision.current",
		metric.WithDescription("Current storage revision number"),
		metric.WithUnit("1"),
	)
	if err != nil {
		return fmt.Errorf("failed to create storage_revision gauge: %w", err)
	}

	ResourcesInStorage, err = Meter.Int64Gauge("ovi.storage.resources.current",
		metric.WithDescription("Current number of resources in storage"),
		metric.WithUnit("1"),
	)
	if err != nil {
		return fmt.Errorf("failed to create resources_in_storage gauge: %w", err)
	}

	return nil
}
