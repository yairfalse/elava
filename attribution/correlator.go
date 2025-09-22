package attribution

import (
	"math"
	"strings"
	"time"

	"github.com/yairfalse/elava/analyzer"
	"github.com/yairfalse/elava/providers/aws"
	"github.com/yairfalse/elava/telemetry"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/trace"
)

// CorrelationEngine matches CloudTrail events to resource changes
type CorrelationEngine struct {
	logger *telemetry.Logger
	tracer trace.Tracer
}

// NewCorrelationEngine creates a new correlation engine
func NewCorrelationEngine() *CorrelationEngine {
	return &CorrelationEngine{
		logger: telemetry.NewLogger("correlation-engine"),
		tracer: otel.Tracer("correlation-engine"),
	}
}

// Correlate matches CloudTrail events with drift
func (c *CorrelationEngine) Correlate(
	drift analyzer.DriftEvent,
	events []aws.CloudTrailEvent,
) (*Attribution, error) {
	if len(events) == 0 {
		return nil, nil
	}

	// Find best matching event
	bestMatch := c.findBestMatch(drift, events)
	if bestMatch == nil {
		return nil, nil
	}

	// Convert to attribution
	return c.eventToAttribution(drift, bestMatch), nil
}

// findBestMatch finds the event most likely to have caused the drift
func (c *CorrelationEngine) findBestMatch(
	drift analyzer.DriftEvent,
	events []aws.CloudTrailEvent,
) *aws.CloudTrailEvent {
	var bestEvent *aws.CloudTrailEvent
	var bestScore float64

	for i := range events {
		event := &events[i]
		score := c.calculateScore(drift, event)

		if score > bestScore {
			bestScore = score
			bestEvent = event
		}
	}

	// Only return if confidence is high enough
	if bestScore < 0.5 {
		return nil
	}

	return bestEvent
}

// calculateScore calculates correlation confidence
func (c *CorrelationEngine) calculateScore(
	drift analyzer.DriftEvent,
	event *aws.CloudTrailEvent,
) float64 {
	score := 0.0

	// Time proximity (40% weight)
	timeScore := c.calculateTimeScore(drift.Timestamp, event.EventTime)
	score += timeScore * 0.4

	// Resource ID match (30% weight)
	if c.resourceMatches(drift.ResourceID, event) {
		score += 0.3
	}

	// API call relevance (30% weight)
	if c.isRelevantAPICall(event.EventName, analyzer.ChangeType(drift.Type)) {
		score += 0.3
	}

	return score
}

// calculateTimeScore scores based on time proximity
func (c *CorrelationEngine) calculateTimeScore(
	driftTime, eventTime time.Time,
) float64 {
	// Calculate time difference in seconds
	diff := math.Abs(driftTime.Sub(eventTime).Seconds())

	// Score decreases with time difference
	// Perfect match = 1.0, degrades to 0 at 5 minutes
	maxWindow := 300.0 // 5 minutes in seconds

	if diff > maxWindow {
		return 0
	}

	return 1.0 - (diff / maxWindow)
}

// resourceMatches checks if event matches resource
func (c *CorrelationEngine) resourceMatches(
	resourceID string,
	event *aws.CloudTrailEvent,
) bool {
	// Direct match
	if event.ResourceID == resourceID {
		return true
	}

	// Partial match (for events that don't have full resource ID)
	if event.ResourceName != "" && strings.Contains(resourceID, event.ResourceName) {
		return true
	}

	return false
}

// isRelevantAPICall checks if API call could cause this drift type
func (c *CorrelationEngine) isRelevantAPICall(
	eventName string,
	changeType analyzer.ChangeType,
) bool {
	switch changeType {
	case analyzer.ChangeCreated:
		return aws.IsCreationEvent(eventName)
	case analyzer.ChangeModified, analyzer.ChangeTagsChanged:
		return aws.IsModificationEvent(eventName)
	case analyzer.ChangeDisappeared:
		return c.isDeletionEvent(eventName)
	default:
		// For unknown changes, consider any event relevant
		return true
	}
}

// isDeletionEvent checks if event represents deletion
func (c *CorrelationEngine) isDeletionEvent(eventName string) bool {
	deletionEvents := map[string]bool{
		"TerminateInstances": true,
		"DeleteDBInstance":   true,
		"DeleteBucket":       true,
		"DeleteFunction":     true,
	}

	return deletionEvents[eventName]
}

// eventToAttribution converts CloudTrail event to Attribution
func (c *CorrelationEngine) eventToAttribution(
	drift analyzer.DriftEvent,
	event *aws.CloudTrailEvent,
) *Attribution {
	confidence := c.calculateScore(drift, event)

	return &Attribution{
		ResourceID: drift.ResourceID,
		Actor:      event.Username,
		ActorType:  c.determineActorType(event),
		Action:     event.EventName,
		Timestamp:  event.EventTime,
		SourceIP:   event.SourceIP,
		UserAgent:  event.UserAgent,
		RequestID:  event.RequestID,
		Confidence: confidence,
		Method:     string(MethodCloudTrail),
	}
}

// determineActorType determines the type of actor
func (c *CorrelationEngine) determineActorType(
	event *aws.CloudTrailEvent,
) string {
	// Check user agent for automation tools
	if strings.Contains(strings.ToLower(event.UserAgent), "terraform") ||
		strings.Contains(strings.ToLower(event.UserAgent), "cloudformation") {
		return string(ActorTypeAutomation)
	}

	// Check user type
	if event.UserType == "AssumedRole" {
		return string(ActorTypeService)
	}

	if event.UserType == "IAMUser" {
		return string(ActorTypeHuman)
	}

	return string(ActorTypeUnknown)
}
