package logging

import (
	"context"
	"os"
	"strings"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// Context keys for logging fields
type ctxKey string

const (
	ctxKeyRequestID     ctxKey = "request_id"
	ctxKeyCorrelationID ctxKey = "correlation_id"
	ctxKeyProjectID     ctxKey = "project_id"
	ctxKeyTokenID       ctxKey = "token_id"
	ctxKeyClientIP      ctxKey = "client_ip"
	ctxKeyUserAgent     ctxKey = "user_agent"
	ctxKeyComponent     ctxKey = "component"
)

// Component names for structured logging
const (
	ComponentServer     = "server"
	ComponentProxy      = "proxy"
	ComponentDatabase   = "database"
	ComponentToken      = "token"
	ComponentMiddleware = "middleware"
	ComponentAdmin      = "admin"
	ComponentDispatcher = "dispatcher"
	ComponentEventBus   = "eventbus"
)

// Canonical logging field names for consistency across the application
const (
	FieldRequestID     = "request_id"
	FieldCorrelationID = "correlation_id"
	FieldMethod        = "method"
	FieldPath          = "path"
	FieldStatusCode    = "status_code"
	FieldDurationMs    = "duration_ms"
	FieldProjectID     = "project_id"
	FieldTokenID       = "token_id"
	FieldClientIP      = "client_ip"
	FieldUserAgent     = "user_agent"
	FieldComponent     = "component"
	FieldRemoteAddr    = "remote_addr"
	FieldOperation     = "operation"
	FieldTarget        = "target"
	FieldActor         = "actor"
	FieldOutcome       = "outcome"
	FieldReason        = "reason"
	FieldEventType     = "event_type"
)

// NewLogger creates a zap.Logger with the specified level, format, and optional file output.
// level can be debug, info, warn, or error. format can be json or console.
// If filePath is empty, logs are written to stdout.
func NewLogger(level, format, filePath string) (*zap.Logger, error) {
	var lvl zapcore.Level
	switch strings.ToLower(level) {
	case "debug":
		lvl = zapcore.DebugLevel
	case "info", "":
		lvl = zapcore.InfoLevel
	case "warn":
		lvl = zapcore.WarnLevel
	case "error":
		lvl = zapcore.ErrorLevel
	default:
		lvl = zapcore.InfoLevel
	}

	encCfg := zapcore.EncoderConfig{
		TimeKey:        "ts",
		LevelKey:       "level",
		NameKey:        "logger",
		MessageKey:     "msg",
		CallerKey:      "caller",
		StacktraceKey:  "stacktrace",
		EncodeTime:     zapcore.ISO8601TimeEncoder,
		EncodeDuration: zapcore.StringDurationEncoder,
		EncodeLevel:    zapcore.LowercaseLevelEncoder,
	}

	var encoder zapcore.Encoder
	if strings.ToLower(format) == "console" {
		encoder = zapcore.NewConsoleEncoder(encCfg)
	} else {
		encoder = zapcore.NewJSONEncoder(encCfg)
	}

	var ws = zapcore.AddSync(os.Stdout)
	if filePath != "" {
		f, err := os.OpenFile(filePath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
		if err != nil {
			return nil, err
		}
		ws = f
	}

	core := zapcore.NewCore(encoder, ws, lvl)
	return zap.New(core), nil
}

// NewComponentLogger creates a logger with a component field pre-populated
func NewComponentLogger(level, format, filePath, component string) (*zap.Logger, error) {
	logger, err := NewLogger(level, format, filePath)
	if err != nil {
		return nil, err
	}
	return logger.With(zap.String(FieldComponent, component)), nil
}

// WithContext adds context fields to the logger
func WithContext(logger *zap.Logger, ctx context.Context) *zap.Logger {
	fields := ExtractContextFields(ctx)
	if len(fields) == 0 {
		return logger
	}
	return logger.With(fields...)
}

// ExtractContextFields extracts logging fields from context
func ExtractContextFields(ctx context.Context) []zap.Field {
	var fields []zap.Field

	if v := ctx.Value(ctxKeyRequestID); v != nil {
		if id, ok := v.(string); ok && id != "" {
			fields = append(fields, zap.String(FieldRequestID, id))
		}
	}

	if v := ctx.Value(ctxKeyCorrelationID); v != nil {
		if id, ok := v.(string); ok && id != "" {
			fields = append(fields, zap.String(FieldCorrelationID, id))
		}
	}

	if v := ctx.Value(ctxKeyProjectID); v != nil {
		if id, ok := v.(string); ok && id != "" {
			fields = append(fields, zap.String(FieldProjectID, id))
		}
	}

	if v := ctx.Value(ctxKeyTokenID); v != nil {
		if id, ok := v.(string); ok && id != "" {
			fields = append(fields, zap.String(FieldTokenID, id))
		}
	}

	if v := ctx.Value(ctxKeyClientIP); v != nil {
		if ip, ok := v.(string); ok && ip != "" {
			fields = append(fields, zap.String(FieldClientIP, ip))
		}
	}

	if v := ctx.Value(ctxKeyUserAgent); v != nil {
		if ua, ok := v.(string); ok && ua != "" {
			fields = append(fields, zap.String(FieldUserAgent, ua))
		}
	}

	if v := ctx.Value(ctxKeyComponent); v != nil {
		if comp, ok := v.(string); ok && comp != "" {
			fields = append(fields, zap.String(FieldComponent, comp))
		}
	}

	return fields
}

// WithRequestID adds request ID to context
func WithRequestID(ctx context.Context, requestID string) context.Context {
	return context.WithValue(ctx, ctxKeyRequestID, requestID)
}

// WithCorrelationID adds correlation ID to context
func WithCorrelationID(ctx context.Context, correlationID string) context.Context {
	return context.WithValue(ctx, ctxKeyCorrelationID, correlationID)
}

// WithProjectID adds project ID to context
func WithProjectID(ctx context.Context, projectID string) context.Context {
	return context.WithValue(ctx, ctxKeyProjectID, projectID)
}

// WithTokenID adds token ID to context
func WithTokenID(ctx context.Context, tokenID string) context.Context {
	return context.WithValue(ctx, ctxKeyTokenID, tokenID)
}

// WithClientIP adds client IP to context
func WithClientIP(ctx context.Context, clientIP string) context.Context {
	return context.WithValue(ctx, ctxKeyClientIP, clientIP)
}

// WithUserAgent adds user agent to context
func WithUserAgent(ctx context.Context, userAgent string) context.Context {
	return context.WithValue(ctx, ctxKeyUserAgent, userAgent)
}

// WithComponent adds component to context
func WithComponent(ctx context.Context, component string) context.Context {
	return context.WithValue(ctx, ctxKeyComponent, component)
}

// GetRequestID extracts request ID from context
func GetRequestID(ctx context.Context) string {
	if v := ctx.Value(ctxKeyRequestID); v != nil {
		if id, ok := v.(string); ok {
			return id
		}
	}
	return ""
}

// GetCorrelationID extracts correlation ID from context
func GetCorrelationID(ctx context.Context) string {
	if v := ctx.Value(ctxKeyCorrelationID); v != nil {
		if id, ok := v.(string); ok {
			return id
		}
	}
	return ""
}

// GetProjectID extracts project ID from context
func GetProjectID(ctx context.Context) string {
	if v := ctx.Value(ctxKeyProjectID); v != nil {
		if id, ok := v.(string); ok {
			return id
		}
	}
	return ""
}
