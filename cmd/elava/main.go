// Elava - Stateless Cloud Resource Scanner
// Scan. Emit. Done.
package main

import (
	"context"
	"flag"
	"io"
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

type config struct {
	region      string
	interval    time.Duration
	metricsAddr string
	oneShot     bool
	debug       bool
}

func parseFlags() config {
	cfg := config{}
	flag.StringVar(&cfg.region, "region", "us-east-1", "AWS region to scan")
	flag.DurationVar(&cfg.interval, "interval", 5*time.Minute, "Scan interval")
	flag.StringVar(&cfg.metricsAddr, "metrics", ":9090", "Metrics server address")
	flag.BoolVar(&cfg.oneShot, "once", false, "Run once and exit")
	flag.BoolVar(&cfg.debug, "debug", false, "Enable debug logging")
	flag.Parse()
	return cfg
}

func setupLogging(debug bool) {
	zerolog.TimeFieldFormat = zerolog.TimeFormatUnix
	if debug {
		zerolog.SetGlobalLevel(zerolog.DebugLevel)
	} else {
		zerolog.SetGlobalLevel(zerolog.InfoLevel)
	}
	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr})
}

func setupMetrics() error {
	promExporter, err := prometheus.New()
	if err != nil {
		return err
	}
	provider := sdkmetric.NewMeterProvider(sdkmetric.WithReader(promExporter))
	otel.SetMeterProvider(provider)
	return nil
}

func startMetricsServer(addr string) {
	srv := &http.Server{
		Addr:              addr,
		Handler:           promhttp.Handler(),
		ReadHeaderTimeout: 10 * time.Second,
	}
	http.Handle("/metrics", promhttp.Handler())
	log.Info().Str("addr", addr).Msg("starting metrics server")
	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Error().Err(err).Msg("metrics server error")
	}
}

func closeEmitter(emit io.Closer) {
	if err := emit.Close(); err != nil {
		log.Error().Err(err).Msg("emitter close error")
	}
}

func main() {
	cfg := parseFlags()
	setupLogging(cfg.debug)

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	if err := setupMetrics(); err != nil {
		log.Fatal().Err(err).Msg("failed to setup metrics")
	}

	go startMetricsServer(cfg.metricsAddr)

	awsPlugin, err := aws.New(ctx, aws.Config{Region: cfg.region})
	if err != nil {
		log.Fatal().Err(err).Msg("failed to create aws plugin")
	}
	plugin.Register(awsPlugin)

	emit, err := emitter.NewPrometheusEmitter()
	if err != nil {
		log.Fatal().Err(err).Msg("failed to create emitter")
	}
	defer closeEmitter(emit)

	log.Info().
		Str("region", cfg.region).
		Dur("interval", cfg.interval).
		Bool("one_shot", cfg.oneShot).
		Msg("elava starting")

	scan(ctx, plugin.All(), emit)

	if cfg.oneShot {
		log.Info().Msg("one-shot mode, exiting")
		return
	}

	runDaemon(ctx, cfg.interval, emit)
}

func runDaemon(ctx context.Context, interval time.Duration, emit emitter.Emitter) {
	ticker := time.NewTicker(interval)
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

func scan(ctx context.Context, plugins []plugin.Plugin, emit emitter.Emitter) {
	log.Info().Int("plugins", len(plugins)).Msg("starting scan")

	for _, p := range plugins {
		start := time.Now()
		resources, err := p.Scan(ctx)
		duration := time.Since(start)

		result := resource.ScanResult{
			Provider:  p.Name(),
			Region:    "",
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
