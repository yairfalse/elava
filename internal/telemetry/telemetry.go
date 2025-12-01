// Package telemetry provides OpenTelemetry instrumentation for Elava.
package telemetry

import (
	"context"
	"fmt"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/metric"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
	"go.opentelemetry.io/otel/trace"

	"github.com/yairfalse/elava/internal/config"
)

// Provider wraps OTEL tracer and meter providers.
type Provider struct {
	tracerProvider *sdktrace.TracerProvider
	meterProvider  *sdkmetric.MeterProvider
	tracer         trace.Tracer
	meter          metric.Meter

	// Metrics
	scanDuration  metric.Float64Histogram
	resourceCount metric.Int64Counter
	scanErrors    metric.Int64Counter
}

// NewProvider creates a new telemetry provider.
func NewProvider(ctx context.Context, cfg config.OTELConfig) (*Provider, error) {
	res, err := resource.New(ctx,
		resource.WithAttributes(
			semconv.ServiceName(cfg.ServiceName),
		),
	)
	if err != nil {
		return nil, fmt.Errorf("create resource: %w", err)
	}

	p := &Provider{}

	if err := p.setupTracing(ctx, cfg, res); err != nil {
		return nil, err
	}

	if err := p.setupMetrics(ctx, cfg, res); err != nil {
		if p.tracerProvider != nil {
			_ = p.tracerProvider.Shutdown(ctx)
		}
		return nil, err
	}

	if err := p.initMetrics(); err != nil {
		return nil, err
	}

	return p, nil
}

func (p *Provider) setupTracing(ctx context.Context, cfg config.OTELConfig, res *resource.Resource) error {
	opts := []sdktrace.TracerProviderOption{
		sdktrace.WithResource(res),
	}

	if cfg.Traces.Enabled && cfg.Endpoint != "" {
		exp, err := createTraceExporter(ctx, cfg)
		if err != nil {
			return fmt.Errorf("create trace exporter: %w", err)
		}
		sampler := sdktrace.TraceIDRatioBased(cfg.Traces.SampleRate)
		opts = append(opts, sdktrace.WithBatcher(exp), sdktrace.WithSampler(sampler))
	}

	p.tracerProvider = sdktrace.NewTracerProvider(opts...)
	otel.SetTracerProvider(p.tracerProvider)
	p.tracer = p.tracerProvider.Tracer("elava")

	return nil
}

func (p *Provider) setupMetrics(ctx context.Context, cfg config.OTELConfig, res *resource.Resource) error {
	opts := []sdkmetric.Option{
		sdkmetric.WithResource(res),
	}

	if cfg.Metrics.Enabled && cfg.Endpoint != "" {
		exp, err := createMetricExporter(ctx, cfg)
		if err != nil {
			return fmt.Errorf("create metric exporter: %w", err)
		}
		opts = append(opts, sdkmetric.WithReader(sdkmetric.NewPeriodicReader(exp)))
	}

	p.meterProvider = sdkmetric.NewMeterProvider(opts...)
	otel.SetMeterProvider(p.meterProvider)
	p.meter = p.meterProvider.Meter("elava")

	return nil
}

func createTraceExporter(ctx context.Context, cfg config.OTELConfig) (sdktrace.SpanExporter, error) {
	opts := []otlptracegrpc.Option{
		otlptracegrpc.WithEndpoint(cfg.Endpoint),
	}
	if cfg.Insecure {
		opts = append(opts, otlptracegrpc.WithInsecure())
	}
	return otlptracegrpc.New(ctx, opts...)
}

func createMetricExporter(ctx context.Context, cfg config.OTELConfig) (sdkmetric.Exporter, error) {
	opts := []otlpmetricgrpc.Option{
		otlpmetricgrpc.WithEndpoint(cfg.Endpoint),
	}
	if cfg.Insecure {
		opts = append(opts, otlpmetricgrpc.WithInsecure())
	}
	return otlpmetricgrpc.New(ctx, opts...)
}

func (p *Provider) initMetrics() error {
	var err error

	p.scanDuration, err = p.meter.Float64Histogram(
		"elava_scan_duration_seconds",
		metric.WithDescription("Duration of resource scans"),
		metric.WithUnit("s"),
	)
	if err != nil {
		return fmt.Errorf("create scan_duration: %w", err)
	}

	p.resourceCount, err = p.meter.Int64Counter(
		"elava_resources_scanned_total",
		metric.WithDescription("Total resources scanned"),
	)
	if err != nil {
		return fmt.Errorf("create resource_count: %w", err)
	}

	p.scanErrors, err = p.meter.Int64Counter(
		"elava_scan_errors_total",
		metric.WithDescription("Total scan errors"),
	)
	if err != nil {
		return fmt.Errorf("create scan_errors: %w", err)
	}

	return nil
}

// Tracer returns the tracer.
func (p *Provider) Tracer() trace.Tracer {
	return p.tracer
}

// Meter returns the meter.
func (p *Provider) Meter() metric.Meter {
	return p.meter
}

// StartSpan starts a new span.
func (p *Provider) StartSpan(ctx context.Context, name string) (context.Context, trace.Span) {
	return p.tracer.Start(ctx, name)
}

// RecordScanDuration records scan duration.
func (p *Provider) RecordScanDuration(ctx context.Context, provider, region, scanner string, d time.Duration) {
	p.scanDuration.Record(ctx, d.Seconds(), metric.WithAttributes(
		attribute.String("provider", provider),
		attribute.String("region", region),
		attribute.String("scanner", scanner),
	))
}

// RecordResourceCount records the number of resources scanned.
func (p *Provider) RecordResourceCount(ctx context.Context, provider, region, scanner string, count int) {
	p.resourceCount.Add(ctx, int64(count), metric.WithAttributes(
		attribute.String("provider", provider),
		attribute.String("region", region),
		attribute.String("scanner", scanner),
	))
}

// RecordError records a scan error.
func (p *Provider) RecordError(ctx context.Context, provider, region, scanner string) {
	p.scanErrors.Add(ctx, 1, metric.WithAttributes(
		attribute.String("provider", provider),
		attribute.String("region", region),
		attribute.String("scanner", scanner),
	))
}

// Shutdown flushes and shuts down the providers.
func (p *Provider) Shutdown(ctx context.Context) error {
	if p.tracerProvider != nil {
		if err := p.tracerProvider.Shutdown(ctx); err != nil {
			return fmt.Errorf("shutdown tracer: %w", err)
		}
	}
	if p.meterProvider != nil {
		if err := p.meterProvider.Shutdown(ctx); err != nil {
			return fmt.Errorf("shutdown meter: %w", err)
		}
	}
	return nil
}
