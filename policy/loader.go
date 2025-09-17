//go:build ignore

package policy

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/yairfalse/elava/telemetry"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

type PolicyLoader struct {
	bundlePath string
	engine     *PolicyEngine
	logger     *telemetry.Logger
	tracer     trace.Tracer
}

func NewPolicyLoader(bundlePath string, engine *PolicyEngine) *PolicyLoader {
	return &PolicyLoader{
		bundlePath: bundlePath,
		engine:     engine,
		logger:     telemetry.NewLogger("policy-loader"),
		tracer:     otel.Tracer("policy-loader"),
	}
}

func (pl *PolicyLoader) LoadPolicies(ctx context.Context) error {
	ctx, span := pl.tracer.Start(ctx, "policy_loader.load_policies",
		trace.WithAttributes(attribute.String("bundle_path", pl.bundlePath)))
	defer span.End()

	pl.logger.WithContext(ctx).Info().
		Str("bundle_path", pl.bundlePath).
		Msg("loading policy bundle")

	if _, err := os.Stat(pl.bundlePath); os.IsNotExist(err) {
		return fmt.Errorf("policy bundle path does not exist: %s", pl.bundlePath)
	}

	return filepath.Walk(pl.bundlePath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if !strings.HasSuffix(path, ".rego") {
			return nil
		}

		return pl.loadPolicyFile(ctx, path)
	})
}

func (pl *PolicyLoader) loadPolicyFile(ctx context.Context, filePath string) error {
	ctx, span := pl.tracer.Start(ctx, "policy_loader.load_file",
		trace.WithAttributes(attribute.String("file_path", filePath)))
	defer span.End()

	// Validate file path to prevent directory traversal
	if err := pl.validateFilePath(filePath); err != nil {
		return fmt.Errorf("invalid file path %s: %w", filePath, err)
	}

	pl.logger.WithContext(ctx).Debug().
		Str("file_path", filePath).
		Msg("loading policy file")

	content, err := os.ReadFile(filepath.Clean(filePath))
	if err != nil {
		pl.logger.WithContext(ctx).Error().
			Err(err).
			Str("file_path", filePath).
			Msg("failed to read policy file")
		return fmt.Errorf("failed to read policy file %s: %w", filePath, err)
	}

	policyName := strings.TrimSuffix(filepath.Base(filePath), ".rego")

	if err := pl.engine.LoadPolicy(ctx, policyName, string(content)); err != nil {
		pl.logger.WithContext(ctx).Error().
			Err(err).
			Str("policy_name", policyName).
			Str("file_path", filePath).
			Msg("failed to load policy")
		return fmt.Errorf("failed to load policy %s from %s: %w", policyName, filePath, err)
	}

	pl.logger.WithContext(ctx).Info().
		Str("policy_name", policyName).
		Str("file_path", filePath).
		Msg("policy loaded successfully")

	return nil
}

func (pl *PolicyLoader) LoadDefaultPolicies(ctx context.Context) error {
	ctx, span := pl.tracer.Start(ctx, "policy_loader.load_defaults")
	defer span.End()

	defaultPolicies := []struct {
		name    string
		content string
	}{
		{
			name: "default-allow",
			content: `package elava.default

import rego.v1

decision := "allow"
action := "ignore"
reason := "default allow policy"
confidence := 0.5
risk := "low"`,
		},
	}

	for _, policy := range defaultPolicies {
		if err := pl.engine.LoadPolicy(ctx, policy.name, policy.content); err != nil {
			return fmt.Errorf("failed to load default policy %s: %w", policy.name, err)
		}
	}

	pl.logger.WithContext(ctx).Info().
		Int("count", len(defaultPolicies)).
		Msg("loaded default policies")

	return nil
}

func (pl *PolicyLoader) validateFilePath(filePath string) error {
	cleanPath := filepath.Clean(filePath)

	// Ensure the path is within the bundle directory
	bundlePath := filepath.Clean(pl.bundlePath)
	relPath, err := filepath.Rel(bundlePath, cleanPath)
	if err != nil {
		return fmt.Errorf("failed to resolve relative path: %w", err)
	}

	// Check for directory traversal attempts
	if strings.HasPrefix(relPath, "..") || strings.Contains(relPath, ".."+string(filepath.Separator)) {
		return fmt.Errorf("path traversal detected")
	}

	return nil
}
