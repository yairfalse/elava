package attribution

import (
	"context"
	"fmt"
	"time"

	"github.com/yairfalse/elava/analyzer"
	"github.com/yairfalse/elava/providers/aws"
	"github.com/yairfalse/elava/storage"
	"github.com/yairfalse/elava/telemetry"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/trace"
)

// Service provides attribution for drift events
type Service struct {
	cloudtrail *aws.CloudTrailClient
	correlator *CorrelationEngine
	storage    *storage.MVCCStorage
	logger     *telemetry.Logger
	tracer     trace.Tracer
}

// NewService creates a new attribution service
func NewService(
	cloudtrail *aws.CloudTrailClient,
	storage *storage.MVCCStorage,
) *Service {
	return &Service{
		cloudtrail: cloudtrail,
		correlator: NewCorrelationEngine(),
		storage:    storage,
		logger:     telemetry.NewLogger("attribution-service"),
		tracer:     otel.Tracer("attribution-service"),
	}
}

// GetAttribution gets attribution for a drift event
func (s *Service) GetAttribution(
	ctx context.Context,
	drift analyzer.DriftEvent,
) (*Attribution, error) {
	ctx, span := s.tracer.Start(ctx, "GetAttribution")
	defer span.End()

	// Check cache first
	cached, err := s.getFromCache(ctx, drift.ResourceID, drift.Timestamp)
	if err == nil && cached != nil {
		return cached, nil
	}

	// Query CloudTrail
	attribution, err := s.queryAndCorrelate(ctx, drift)
	if err != nil {
		return nil, fmt.Errorf("failed to get attribution: %w", err)
	}

	// Store in cache
	if attribution != nil {
		_ = s.storeInCache(ctx, attribution)
	}

	return attribution, nil
}

// queryAndCorrelate queries CloudTrail and correlates with drift
func (s *Service) queryAndCorrelate(
	ctx context.Context,
	drift analyzer.DriftEvent,
) (*Attribution, error) {
	// Query CloudTrail with 10 minute window
	window := 10 * time.Minute
	events, err := s.cloudtrail.QueryResourceEvents(
		ctx,
		drift.ResourceID,
		window,
	)
	if err != nil {
		return nil, fmt.Errorf("CloudTrail query failed: %w", err)
	}

	// Correlate events with drift
	attribution, err := s.correlator.Correlate(drift, events)
	if err != nil {
		return nil, fmt.Errorf("correlation failed: %w", err)
	}

	// If no attribution found, try heuristics
	if attribution == nil {
		attribution = s.applyHeuristics(drift)
	}

	return attribution, nil
}

// applyHeuristics applies heuristic attribution when CloudTrail fails
func (s *Service) applyHeuristics(drift analyzer.DriftEvent) *Attribution {
	attr := &Attribution{
		ResourceID: drift.ResourceID,
		Timestamp:  drift.Timestamp,
		Method:     string(MethodHeuristic),
		Confidence: 0.3,
	}

	// Heuristic: Business hours = likely human
	hour := drift.Timestamp.Hour()
	if hour >= 9 && hour <= 17 {
		attr.Actor = "unknown-user"
		attr.ActorType = string(ActorTypeHuman)
		attr.Confidence = 0.4
	} else {
		// After hours = likely automation
		attr.Actor = "unknown-automation"
		attr.ActorType = string(ActorTypeAutomation)
		attr.Confidence = 0.35
	}

	return attr
}

// getFromCache retrieves attribution from storage cache
func (s *Service) getFromCache(
	ctx context.Context,
	resourceID string,
	timestamp time.Time,
) (*Attribution, error) {
	// Implementation depends on storage interface
	// For now, return nil to always query CloudTrail
	return nil, nil
}

// storeInCache stores attribution in storage
func (s *Service) storeInCache(
	ctx context.Context,
	attr *Attribution,
) error {
	// Implementation depends on storage interface
	// For now, just log
	// Would cache attribution here
	return nil
}

// EnrichDriftEvent adds attribution to a drift event
func (s *Service) EnrichDriftEvent(
	ctx context.Context,
	drift analyzer.DriftEvent,
) (*analyzer.DriftEvent, error) {
	attribution, err := s.GetAttribution(ctx, drift)
	if err != nil {
		return nil, err
	}

	// Add attribution to drift metadata
	if drift.Metadata == nil {
		drift.Metadata = make(map[string]interface{})
	}

	if attribution != nil {
		drift.Metadata["attribution"] = map[string]interface{}{
			"actor":      attribution.Actor,
			"action":     attribution.Action,
			"timestamp":  attribution.Timestamp,
			"confidence": attribution.Confidence,
		}
	}

	return &drift, nil
}
