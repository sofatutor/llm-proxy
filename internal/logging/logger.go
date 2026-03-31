package logging

import (
	"context"
	"errors"
	"os"
	"strings"
	"syscall"

	"github.com/sofatutor/llm-proxy/internal/obfuscate"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

type safeWriteSyncer struct {
	zapcore.WriteSyncer
	ignoreSyncErrors bool
}

func (s safeWriteSyncer) Sync() error {
	err := s.WriteSyncer.Sync()
	if err == nil {
		return nil
	}
	if s.ignoreSyncErrors && isIgnorableSyncError(err) {
		return nil
	}
	return err
}

func isIgnorableSyncError(err error) bool {
	if errors.Is(err, syscall.EINVAL) {
		return true
	}

	return strings.Contains(strings.ToLower(err.Error()), "inappropriate ioctl for device")
}

func shouldIgnoreSyncErrors(file *os.File, filePath string) bool {
	if file == nil {
		return false
	}

	if filePath == "" || filePath == "/dev/stdout" || filePath == "/dev/stderr" {
		return true
	}

	info, err := file.Stat()
	if err != nil {
		return false
	}

	return info.Mode()&os.ModeType != 0
}

func newWriteSyncer(filePath string) (zapcore.WriteSyncer, error) {
	if filePath == "" {
		return safeWriteSyncer{
			WriteSyncer:      zapcore.AddSync(os.Stdout),
			ignoreSyncErrors: true,
		}, nil
	}

	f, err := os.OpenFile(filePath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return nil, err
	}

	return safeWriteSyncer{
		WriteSyncer:      f,
		ignoreSyncErrors: shouldIgnoreSyncErrors(f, filePath),
	}, nil
}

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

	ws, err := newWriteSyncer(filePath)
	if err != nil {
		return nil, err
	}

	core := zapcore.NewCore(encoder, ws, lvl)
	return zap.New(core), nil
}

// Context keys for request and correlation IDs
type contextKey string

const (
	requestIDKey     contextKey = "request_id"
	correlationIDKey contextKey = "correlation_id"
)

// Canonical field helpers for structured logging

// RequestFields returns fields for HTTP request logging
func RequestFields(requestID, method, path string, statusCode, durationMs int) []zap.Field {
	return []zap.Field{
		zap.String("request_id", requestID),
		zap.String("method", method),
		zap.String("path", path),
		zap.Int("status_code", statusCode),
		zap.Int("duration_ms", durationMs),
	}
}

// CorrelationID returns a field for correlation ID
func CorrelationID(id string) zap.Field {
	return zap.String("correlation_id", id)
}

// ProjectID returns a field for project ID
func ProjectID(id string) zap.Field {
	return zap.String("project_id", id)
}

// TokenID returns a field for token ID (obfuscated for security)
func TokenID(token string) zap.Field {
	return zap.String("token_id", obfuscate.ObfuscateTokenGeneric(token))
}

// ClientIP returns a field for client IP address
func ClientIP(ip string) zap.Field {
	return zap.String("client_ip", ip)
}

// Context management for request/correlation IDs

// WithRequestID adds a request ID to the context
func WithRequestID(ctx context.Context, requestID string) context.Context {
	return context.WithValue(ctx, requestIDKey, requestID)
}

// WithCorrelationID adds a correlation ID to the context
func WithCorrelationID(ctx context.Context, correlationID string) context.Context {
	return context.WithValue(ctx, correlationIDKey, correlationID)
}

// GetRequestID retrieves the request ID from context
func GetRequestID(ctx context.Context) (string, bool) {
	id, ok := ctx.Value(requestIDKey).(string)
	return id, ok
}

// GetCorrelationID retrieves the correlation ID from context
func GetCorrelationID(ctx context.Context) (string, bool) {
	id, ok := ctx.Value(correlationIDKey).(string)
	return id, ok
}

// Logger enhancement helpers

// WithRequestContext adds request ID from context to logger if present
func WithRequestContext(ctx context.Context, logger *zap.Logger) *zap.Logger {
	if requestID, ok := GetRequestID(ctx); ok {
		return logger.With(zap.String("request_id", requestID))
	}
	return logger
}

// WithCorrelationContext adds correlation ID from context to logger if present
func WithCorrelationContext(ctx context.Context, logger *zap.Logger) *zap.Logger {
	if correlationID, ok := GetCorrelationID(ctx); ok {
		return logger.With(zap.String("correlation_id", correlationID))
	}
	return logger
}

// NewChildLogger creates a child logger with a component field
func NewChildLogger(parent *zap.Logger, component string) *zap.Logger {
	return parent.With(zap.String("component", component))
}
