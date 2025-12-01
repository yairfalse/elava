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

	"github.com/yairfalse/elava/internal/config"
	"github.com/yairfalse/elava/internal/emitter"
	"github.com/yairfalse/elava/internal/plugin"
	"github.com/yairfalse/elava/internal/plugin/aws"
	"github.com/yairfalse/elava/internal/telemetry"
	"github.com/yairfalse/elava/pkg/resource"
)

func main() {
	configPath := flag.String("config", "", "Path to TOML config file")
	metricsAddr := flag.String("metrics", ":9090", "Metrics server address")
	debug := flag.Bool("debug", false, "Enable debug logging")
	flag.Parse()

	setupLogging(*debug)

	cfg, err := loadConfig(*configPath)
	if err != nil {
		log.Fatal().Err(err).Msg("failed to load config")
	}

	if *debug {
		cfg.Log.Level = "debug"
	}

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	tp, err := setupTelemetry(ctx, cfg)
	if err != nil {
		log.Fatal().Err(err).Msg("failed to setup telemetry")
	}
	defer shutdownTelemetry(ctx, tp)

	if err := setupPrometheusMetrics(); err != nil {
		log.Fatal().Err(err).Msg("failed to setup prometheus")
	}
	go startMetricsServer(*metricsAddr)

	if err := registerPlugins(ctx, cfg); err != nil {
		log.Fatal().Err(err).Msg("failed to register plugins")
	}

	emit, err := emitter.NewPrometheusEmitter()
	if err != nil {
		log.Fatal().Err(err).Msg("failed to create emitter")
	}
	defer closeEmitter(emit)

	log.Info().
		Strs("regions", cfg.AWS.Regions).
		Dur("interval", cfg.Scanner.Interval).
		Bool("one_shot", cfg.Scanner.OneShot).
		Msg("elava starting")

	scan(ctx, plugin.All(), emit, tp)

	if cfg.Scanner.OneShot {
		log.Info().Msg("one-shot mode, exiting")
		return
	}

	runDaemon(ctx, cfg.Scanner.Interval, emit, tp)
}

func loadConfig(path string) (*config.Config, error) {
	if path != "" {
		cfg, err := config.Load(path)
		if err != nil {
			return nil, err
		}
		if err := cfg.Validate(); err != nil {
			return nil, err
		}
		return cfg, nil
	}
	// Default config when no file specified
	return &config.Config{
		AWS:     config.AWSConfig{Regions: []string{"us-east-1"}},
		OTEL:    config.OTELConfig{ServiceName: "elava"},
		Scanner: config.ScannerConfig{Interval: 5 * time.Minute},
		Log:     config.LogConfig{Level: "info"},
	}, nil
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

func setupTelemetry(ctx context.Context, cfg *config.Config) (*telemetry.Provider, error) {
	return telemetry.NewProvider(ctx, cfg.OTEL)
}

func shutdownTelemetry(ctx context.Context, tp *telemetry.Provider) {
	if tp == nil {
		return
	}
	shutdownCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if err := tp.Shutdown(shutdownCtx); err != nil {
		log.Error().Err(err).Msg("telemetry shutdown error")
	}
}

func setupPrometheusMetrics() error {
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
	log.Info().Str("addr", addr).Msg("starting metrics server")
	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Error().Err(err).Msg("metrics server error")
	}
}

func registerPlugins(ctx context.Context, cfg *config.Config) error {
	for _, region := range cfg.AWS.Regions {
		awsPlugin, err := aws.New(ctx, aws.Config{Region: region})
		if err != nil {
			return err
		}
		plugin.Register(&awsPluginWithRegionName{Plugin: awsPlugin, Region: region})
	}
	return nil
}

// awsPluginWithRegionName wraps an AWS plugin and overrides Name() to include the region.
type awsPluginWithRegionName struct {
	plugin.Plugin
	Region string
}

func (p *awsPluginWithRegionName) Name() string {
	return "aws-" + p.Region
}
func closeEmitter(emit io.Closer) {
	if err := emit.Close(); err != nil {
		log.Error().Err(err).Msg("emitter close error")
	}
}

func runDaemon(ctx context.Context, interval time.Duration, emit emitter.Emitter, tp *telemetry.Provider) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			scan(ctx, plugin.All(), emit, tp)
		case <-ctx.Done():
			log.Info().Msg("shutting down")
			return
		}
	}
}

func scan(ctx context.Context, plugins []plugin.Plugin, emit emitter.Emitter, tp *telemetry.Provider) {
	ctx, span := tp.StartSpan(ctx, "scan")
	defer span.End()

	log.Info().Int("plugins", len(plugins)).Msg("starting scan")

	for _, p := range plugins {
		scanPlugin(ctx, p, emit, tp)
	}

	log.Info().Msg("scan complete")
}

func scanPlugin(ctx context.Context, p plugin.Plugin, emit emitter.Emitter, tp *telemetry.Provider) {
	ctx, span := tp.StartSpan(ctx, "scan."+p.Name())
	defer span.End()

	start := time.Now()
	resources, err := p.Scan(ctx)
	duration := time.Since(start)

	tp.RecordScanDuration(ctx, p.Name(), "", "all", duration)

	if err != nil {
		tp.RecordError(ctx, p.Name(), "", "all")
		log.Error().Err(err).Str("plugin", p.Name()).Msg("scan failed")
		return
	}

	tp.RecordResourceCount(ctx, p.Name(), "", "all", len(resources))

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
