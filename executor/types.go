package executor

import (
	"context"
	"time"

	"github.com/yairfalse/elava/providers"
	"github.com/yairfalse/elava/types"
)

// Executor executes decisions against cloud providers with safety checks
type Executor interface {
	Execute(ctx context.Context, decisions []types.Decision) (*ExecutionResult, error)
	ExecuteSingle(ctx context.Context, decision types.Decision) (*SingleExecutionResult, error)
	DryRun(ctx context.Context, decisions []types.Decision) (*DryRunResult, error)
}

// ExecutionResult contains the outcome of executing multiple decisions
type ExecutionResult struct {
	StartTime        time.Time               `json:"start_time"`
	EndTime          time.Time               `json:"end_time"`
	Duration         time.Duration           `json:"duration"`
	TotalDecisions   int                     `json:"total_decisions"`
	SuccessfulCount  int                     `json:"successful_count"`
	FailedCount      int                     `json:"failed_count"`
	SkippedCount     int                     `json:"skipped_count"`
	Results          []SingleExecutionResult `json:"results"`
	PartialFailure   bool                    `json:"partial_failure"`
	RollbackRequired bool                    `json:"rollback_required"`
}

// SingleExecutionResult contains the outcome of executing a single decision
type SingleExecutionResult struct {
	Decision     types.Decision  `json:"decision"`
	Status       ExecutionStatus `json:"status"`
	StartTime    time.Time       `json:"start_time"`
	EndTime      time.Time       `json:"end_time"`
	Duration     time.Duration   `json:"duration"`
	Error        string          `json:"error,omitempty"`
	ResourceID   string          `json:"resource_id,omitempty"`
	SkipReason   string          `json:"skip_reason,omitempty"`
	ProviderInfo ProviderInfo    `json:"provider_info"`
}

// ExecutionStatus tracks the status of decision execution
type ExecutionStatus string

const (
	StatusPending    ExecutionStatus = "pending"
	StatusExecuting  ExecutionStatus = "executing"
	StatusSuccess    ExecutionStatus = "success"
	StatusFailed     ExecutionStatus = "failed"
	StatusSkipped    ExecutionStatus = "skipped"
	StatusRolledBack ExecutionStatus = "rolled_back"
)

// ProviderInfo contains provider-specific execution details
type ProviderInfo struct {
	Provider   string `json:"provider"`
	Region     string `json:"region"`
	APIVersion string `json:"api_version,omitempty"`
	RequestID  string `json:"request_id,omitempty"`
}

// DryRunResult contains the outcome of a dry run execution
type DryRunResult struct {
	TotalDecisions       int               `json:"total_decisions"`
	SafeDecisions        int               `json:"safe_decisions"`
	DestructiveDecisions int               `json:"destructive_decisions"`
	BlessedDecisions     int               `json:"blessed_decisions"`
	BlockedDecisions     []BlockedDecision `json:"blocked_decisions"`
	EstimatedDuration    time.Duration     `json:"estimated_duration"`
}

// BlockedDecision represents a decision that would be blocked
type BlockedDecision struct {
	Decision types.Decision `json:"decision"`
	Reason   string         `json:"reason"`
	Severity BlockSeverity  `json:"severity"`
}

// BlockSeverity indicates how critical a block is
type BlockSeverity string

const (
	SeverityWarning  BlockSeverity = "warning"
	SeverityError    BlockSeverity = "error"
	SeverityCritical BlockSeverity = "critical"
)

// ExecutorOptions configure executor behavior
type ExecutorOptions struct {
	DryRun                bool          `json:"dry_run"`
	MaxConcurrency        int           `json:"max_concurrency"`
	Timeout               time.Duration `json:"timeout"`
	SkipConfirmation      bool          `json:"skip_confirmation"`
	AllowDestructive      bool          `json:"allow_destructive"`
	AllowBlessedChanges   bool          `json:"allow_blessed_changes"`
	ContinueOnFailure     bool          `json:"continue_on_failure"`
	EnableRollback        bool          `json:"enable_rollback"`
	RollbackOnPartialFail bool          `json:"rollback_on_partial_fail"`
}

// SafetyCheck represents a pre-execution safety validation
type SafetyCheck struct {
	Name        string        `json:"name"`
	Description string        `json:"description"`
	Severity    BlockSeverity `json:"severity"`
	Passed      bool          `json:"passed"`
	Message     string        `json:"message,omitempty"`
}

// RollbackEntry tracks operations that can be rolled back
type RollbackEntry struct {
	Decision       types.Decision  `json:"decision"`
	OriginalState  *types.Resource `json:"original_state,omitempty"`
	ReverseAction  string          `json:"reverse_action"`
	ExecutedAt     time.Time       `json:"executed_at"`
	CanRollback    bool            `json:"can_rollback"`
	RollbackReason string          `json:"rollback_reason,omitempty"`
}

// ConfirmationRequest represents a request for user confirmation
type ConfirmationRequest struct {
	Decision  types.Decision `json:"decision"`
	Message   string         `json:"message"`
	Severity  BlockSeverity  `json:"severity"`
	DefaultNo bool           `json:"default_no"`
	Timeout   time.Duration  `json:"timeout"`
}

// ConfirmationResponse represents user's response to confirmation
type ConfirmationResponse struct {
	Approved       bool   `json:"approved"`
	Message        string `json:"message,omitempty"`
	RememberChoice bool   `json:"remember_choice"`
}

// Confirmer handles user confirmation for dangerous operations
type Confirmer interface {
	RequestConfirmation(ctx context.Context, req ConfirmationRequest) (*ConfirmationResponse, error)
}

// SafetyChecker validates decisions before execution
type SafetyChecker interface {
	CheckSafety(ctx context.Context, decision types.Decision, provider providers.CloudProvider) ([]SafetyCheck, error)
}

// RollbackManager handles operation rollbacks
type RollbackManager interface {
	RecordExecution(ctx context.Context, entry RollbackEntry) error
	Rollback(ctx context.Context, entries []RollbackEntry) error
	CanRollback(ctx context.Context, decision types.Decision) (bool, string)
}
