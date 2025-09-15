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
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/propagation"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.21.0"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// Global telemetry handles - CLAUDE.md: Direct OTEL, no wrappers
var (
	// Tracer for distributed tracing
	Tracer = otel.Tracer("github.com/yairfalse/elava")

	// Meter for metrics
	Meter = otel.Meter("github.com/yairfalse/elava")

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
	// Default to env vars if not provided
	if cfg.OTELEndpoint == "" {
		cfg.OTELEndpoint = os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT")
		if cfg.OTELEndpoint == "" {
			cfg.OTELEndpoint = "localhost:4317" // Default to local Urpo
		}
	}

	if cfg.ServiceName == "" {
		cfg.ServiceName = "ovi"
	}

	// Create resource with service information
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

	// Setup trace provider
	traceShutdown, err := setupTraceProvider(ctx, cfg, res)
	if err != nil {
		return nil, fmt.Errorf("failed to setup traces: %w", err)
	}

	// Setup metric provider
	metricShutdown, err := setupMetricProvider(ctx, cfg, res)
	if err != nil {
		_ = traceShutdown(ctx)
		return nil, fmt.Errorf("failed to setup metrics: %w", err)
	}

	// Initialize metrics
	if err := initMetrics(); err != nil {
		_ = traceShutdown(ctx)
		_ = metricShutdown(ctx)
		return nil, fmt.Errorf("failed to initialize metrics: %w", err)
	}

	// Return combined shutdown function
	return func(ctx context.Context) error {
		var err error
		if e := traceShutdown(ctx); e != nil {
			err = fmt.Errorf("trace shutdown failed: %w", e)
		}
		if e := metricShutdown(ctx); e != nil && err == nil {
			err = fmt.Errorf("metric shutdown failed: %w", e)
		}
		return err
	}, nil
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

// setupMetricProvider configures metric provider with OTLP exporter
func setupMetricProvider(ctx context.Context, cfg Config, res *resource.Resource) (func(context.Context) error, error) {
	// Configure connection
	opts := []otlpmetricgrpc.Option{
		otlpmetricgrpc.WithEndpoint(cfg.OTELEndpoint),
	}

	if cfg.Insecure {
		opts = append(opts, otlpmetricgrpc.WithDialOption(
			grpc.WithTransportCredentials(insecure.NewCredentials()),
		))
	}

	// Create OTLP metric exporter
	exporter, err := otlpmetricgrpc.New(ctx, opts...)
	if err != nil {
		return nil, fmt.Errorf("failed to create metric exporter: %w", err)
	}

	// Create metric provider
	provider := sdkmetric.NewMeterProvider(
		sdkmetric.WithReader(
			sdkmetric.NewPeriodicReader(exporter,
				sdkmetric.WithInterval(10*time.Second), // Export every 10s
			),
		),
		sdkmetric.WithResource(res),
	)

	// Set global provider
	otel.SetMeterProvider(provider)

	// Update global meter
	Meter = provider.Meter("github.com/yairfalse/elava")

	return provider.Shutdown, nil
}

// initMetrics initializes all metric instruments
func initMetrics() error {
	var err error

	// Counters - use _total suffix per OTEL conventions
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

	// Histogram - include unit in name
	ScanDuration, err = Meter.Float64Histogram("ovi.scan.duration.seconds",
		metric.WithDescription("Duration of scan operations"),
		metric.WithUnit("s"),
	)
	if err != nil {
		return fmt.Errorf("failed to create scan_duration histogram: %w", err)
	}

	// Gauges - current values
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
