package main

import (
	"context"
	"log"
	"os"

	"github.com/yairfalse/ovi/telemetry"
)

// initTelemetry initializes OTEL for Ovi
// Can be disabled with OVI_TELEMETRY_DISABLED=true
func initTelemetry(ctx context.Context) func() {
	// Check if telemetry is disabled
	if os.Getenv("OVI_TELEMETRY_DISABLED") == "true" {
		log.Println("📡 Telemetry disabled")
		return func() {}
	}

	// Configure OTEL
	cfg := telemetry.Config{
		ServiceName:    "ovi",
		ServiceVersion: "0.1.0", // TODO: Get from build
		Environment:    os.Getenv("OVI_ENVIRONMENT"),
		OTELEndpoint:   os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT"),
		Insecure:       true, // For local development
	}

	// Default endpoint for local Urpo
	if cfg.OTELEndpoint == "" {
		cfg.OTELEndpoint = "localhost:4317"
	}

	// Initialize OTEL
	shutdown, err := telemetry.InitOTEL(ctx, cfg)
	if err != nil {
		// Don't fail if OTEL init fails - just warn
		log.Printf("⚠️  Telemetry initialization failed: %v", err)
		log.Println("📡 Running without telemetry")
		return func() {}
	}

	log.Printf("📡 Telemetry enabled → %s", cfg.OTELEndpoint)

	// Return cleanup function
	return func() {
		if err := shutdown(context.Background()); err != nil {
			log.Printf("Error shutting down telemetry: %v", err)
		}
	}
}

// Environment variables for configuration:
// - OTEL_EXPORTER_OTLP_ENDPOINT: Where to send telemetry (default: localhost:4317)
// - OVI_TELEMETRY_DISABLED: Set to "true" to disable telemetry
// - OVI_ENVIRONMENT: Environment name (dev, staging, prod)
