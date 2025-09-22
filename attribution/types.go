package attribution

import (
	"time"
)

// Attribution explains who made a change
type Attribution struct {
	ResourceID string    `json:"resource_id"`
	Actor      string    `json:"actor"`      // Username or service
	ActorType  string    `json:"actor_type"` // human, service, automation
	Action     string    `json:"action"`     // API call that caused change
	Timestamp  time.Time `json:"timestamp"`  // When it happened
	SourceIP   string    `json:"source_ip"`
	UserAgent  string    `json:"user_agent"`
	RequestID  string    `json:"request_id"` // AWS request ID for tracing
	Confidence float64   `json:"confidence"` // How sure we are (0-1)
	Method     string    `json:"method"`     // How we determined this
}

// ActorType categorizes the type of actor
type ActorType string

const (
	ActorTypeHuman      ActorType = "human"
	ActorTypeService    ActorType = "service"
	ActorTypeAutomation ActorType = "automation"
	ActorTypeUnknown    ActorType = "unknown"
)

// AttributionMethod describes how attribution was determined
type AttributionMethod string

const (
	MethodCloudTrail AttributionMethod = "cloudtrail"
	MethodHeuristic  AttributionMethod = "heuristic"
	MethodManual     AttributionMethod = "manual"
	MethodUnknown    AttributionMethod = "unknown"
)

// ConfidenceLevel categorizes confidence scores
type ConfidenceLevel string

const (
	ConfidenceHigh   ConfidenceLevel = "high"   // > 0.8
	ConfidenceMedium ConfidenceLevel = "medium" // 0.5 - 0.8
	ConfidenceLow    ConfidenceLevel = "low"    // < 0.5
)

// GetConfidenceLevel returns the confidence level category
func (a *Attribution) GetConfidenceLevel() ConfidenceLevel {
	if a.Confidence > 0.8 {
		return ConfidenceHigh
	}
	if a.Confidence > 0.5 {
		return ConfidenceMedium
	}
	return ConfidenceLow
}

// IsHighConfidence checks if attribution has high confidence
func (a *Attribution) IsHighConfidence() bool {
	return a.Confidence > 0.8
}
