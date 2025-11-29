// Elava - Stateless Cloud Resource Scanner
// Scan. Emit. Done.
package main

import (
	"context"
	"flag"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/prometheus"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"

	"github.com/yairfalse/elava/internal/emitter"
	"github.com/yairfalse/elava/internal/plugin"
	"github.com/yairfalse/elava/internal/plugin/aws"
	"github.com/yairfalse/elava/pkg/resource"
)

func main() {
	// Flags
	var (
		region      = flag.String("region", "us-east-1", "AWS region to scan")
		interval    = flag.Duration("interval", 5*time.Minute, "Scan interval")
		metricsAddr = flag.String("metrics", ":9090", "Metrics server address")
		oneShot     = flag.Bool("once", false, "Run once and exit")
		debug       = flag.Bool("debug", false, "Enable debug logging")
	)
	flag.Parse()

	// Setup logging
	zerolog.TimeFieldFormat = zerolog.TimeFormatUnix
	if *debug {
		zerolog.SetGlobalLevel(zerolog.DebugLevel)
	} else {
		zerolog.SetGlobalLevel(zerolog.InfoLevel)
	}
	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr})

	// Setup context with signal handling
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	// Initialize OTEL metrics with Prometheus exporter
	promExporter, err := prometheus.New()
	if err != nil {
		log.Fatal().Err(err).Msg("failed to create prometheus exporter")
	}

	provider := sdkmetric.NewMeterProvider(sdkmetric.WithReader(promExporter))
	otel.SetMeterProvider(provider)

	// Start metrics server
	go func() {
		http.Handle("/metrics", promhttp.Handler())
		log.Info().Str("addr", *metricsAddr).Msg("starting metrics server")
		if err := http.ListenAndServe(*metricsAddr, nil); err != nil && err != http.ErrServerClosed {
			log.Error().Err(err).Msg("metrics server error")
		}
	}()

	// Initialize plugins
	awsPlugin, err := aws.New(ctx, aws.Config{Region: *region})
	if err != nil {
		log.Fatal().Err(err).Msg("failed to create aws plugin")
	}
	plugin.Register(awsPlugin)

	// Initialize emitter
	emit, err := emitter.NewPrometheusEmitter()
	if err != nil {
		log.Fatal().Err(err).Msg("failed to create emitter")
	}
	defer emit.Close()

	log.Info().
		Str("region", *region).
		Dur("interval", *interval).
		Bool("one_shot", *oneShot).
		Msg("elava starting")

	// Run initial scan
	scan(ctx, plugin.All(), emit)

	// One-shot mode
	if *oneShot {
		log.Info().Msg("one-shot mode, exiting")
		return
	}

	// Daemon mode - scan on interval
	ticker := time.NewTicker(*interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			scan(ctx, plugin.All(), emit)
		case <-ctx.Done():
			log.Info().Msg("shutting down")
			return
		}
	}
}

// scan runs all plugins and emits results
func scan(ctx context.Context, plugins []plugin.Plugin, emit emitter.Emitter) {
	log.Info().Int("plugins", len(plugins)).Msg("starting scan")

	for _, p := range plugins {
		start := time.Now()

		resources, err := p.Scan(ctx)
		duration := time.Since(start)

		result := resource.ScanResult{
			Provider:  p.Name(),
			Region:    "", // Plugin knows its region
			Resources: resources,
			Duration:  duration,
			Error:     err,
		}

		if err := emit.Emit(ctx, result); err != nil {
			log.Error().Err(err).Str("plugin", p.Name()).Msg("emit failed")
		}
	}

	log.Info().Msg("scan complete")
}
