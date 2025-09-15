package telemetry

import (
	"context"
	"os"

	"github.com/rs/zerolog"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

// OTELHook adds trace and span IDs to every log entry
type OTELHook struct{}

func (h OTELHook) Run(e *zerolog.Event, level zerolog.Level, msg string) {
	// Skip if no context
	ctx := e.GetCtx()
	if ctx == nil {
		return
	}

	// Extract span from context
	span := trace.SpanFromContext(ctx)
	if !span.SpanContext().IsValid() {
		return
	}

	// Add trace context to log
	e.Str("trace_id", span.SpanContext().TraceID().String())
	e.Str("span_id", span.SpanContext().SpanID().String())

	// Add span attributes as log fields for correlation
	if level == zerolog.ErrorLevel {
		span.SetStatus(codes.Error, msg)
	}
}

// Logger wraps zerolog with OTEL integration
type Logger struct {
	zerolog.Logger
}

// NewLogger creates a new logger with OTEL hooks
func NewLogger(service string) *Logger {
	// Configure zerolog
	zerolog.TimeFieldFormat = zerolog.TimeFormatUnixMs

	// Create base logger with service context
	logger := zerolog.New(os.Stdout).
		With().
		Timestamp().
		Str("service", service).
		Logger().
		Hook(OTELHook{})

	return &Logger{Logger: logger}
}

// WithContext returns a logger with context (for trace propagation)
func (l *Logger) WithContext(ctx context.Context) *zerolog.Logger {
	logger := l.Logger.With().Ctx(ctx).Logger()
	return &logger
}

// LogSpanStart logs the start of a span with attributes
func (l *Logger) LogSpanStart(ctx context.Context, spanName string, attrs ...attribute.KeyValue) {
	logger := l.WithContext(ctx)

	event := logger.Info().Str("span_name", spanName)
	for _, attr := range attrs {
		event = addAttributeToEvent(event, attr)
	}
	event.Msg("span started")
}

// LogSpanEnd logs the end of a span with results
func (l *Logger) LogSpanEnd(ctx context.Context, spanName string, err error) {
	logger := l.WithContext(ctx)

	if err != nil {
		logger.Error().
			Err(err).
			Str("span_name", spanName).
			Msg("span failed")
	} else {
		logger.Debug().
			Str("span_name", spanName).
			Msg("span completed")
	}
}

// Helper to convert OTEL attributes to zerolog fields
func addAttributeToEvent(event *zerolog.Event, attr attribute.KeyValue) *zerolog.Event {
	key := string(attr.Key)

	switch attr.Value.Type() {
	case attribute.STRING:
		return event.Str(key, attr.Value.AsString())
	case attribute.INT64:
		return event.Int64(key, attr.Value.AsInt64())
	case attribute.FLOAT64:
		return event.Float64(key, attr.Value.AsFloat64())
	case attribute.BOOL:
		return event.Bool(key, attr.Value.AsBool())
	default:
		return event.Str(key, attr.Value.AsString())
	}
}

// Convenience methods for storage operations

func (l *Logger) LogCompaction(ctx context.Context, keepRevisions int64, currentRev int64) {
	l.WithContext(ctx).Info().
		Int64("keep_revisions", keepRevisions).
		Int64("current_revision", currentRev).
		Str("operation", "compaction").
		Msg("starting compaction")
}

func (l *Logger) LogCompactionComplete(ctx context.Context, deletedCount int, duration float64) {
	l.WithContext(ctx).Info().
		Int("deleted_keys", deletedCount).
		Float64("duration_ms", duration).
		Str("operation", "compaction").
		Msg("compaction completed")
}

func (l *Logger) LogRebuildIndex(ctx context.Context, indexSize int) {
	l.WithContext(ctx).Info().
		Int("index_size", indexSize).
		Str("operation", "rebuild_index").
		Msg("starting index rebuild")
}

func (l *Logger) LogRebuildComplete(ctx context.Context, resourceCount int, duration float64) {
	l.WithContext(ctx).Info().
		Int("resources_indexed", resourceCount).
		Float64("duration_ms", duration).
		Str("operation", "rebuild_index").
		Msg("index rebuild completed")
}

func (l *Logger) LogBatchOperation(ctx context.Context, operation string, batchSize int) {
	l.WithContext(ctx).Info().
		Str("operation", operation).
		Int("batch_size", batchSize).
		Msg("processing batch")
}

func (l *Logger) LogStorageError(ctx context.Context, operation string, err error) {
	l.WithContext(ctx).Error().
		Err(err).
		Str("operation", operation).
		Msg("storage operation failed")
}
