package logging

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"
)

func TestNewLogger_FileOutput(t *testing.T) {
	tmpDir := t.TempDir()
	logFile := filepath.Join(tmpDir, "test.log")

	logger, err := NewLogger("debug", "json", logFile)
	require.NoError(t, err)
	logger.Info("hello", zap.String("foo", "bar"))
	require.NoError(t, logger.Sync())

	data, err := os.ReadFile(logFile)
	require.NoError(t, err)
	assert.Contains(t, string(data), "\"foo\":\"bar\"")
}

func TestNewLogger_StdoutOutput(t *testing.T) {
	logger, err := NewLogger("info", "json", "")
	require.NoError(t, err)
	assert.NotNil(t, logger)
}

func TestNewLogger_AllLevels(t *testing.T) {
	tests := []struct {
		level string
	}{
		{"debug"},
		{"info"},
		{"warn"},
		{"error"},
		{""},        // defaults to info
		{"invalid"}, // defaults to info
		{"DEBUG"},   // case insensitive
		{"INFO"},
		{"WARN"},
		{"ERROR"},
	}

	for _, tt := range tests {
		t.Run(tt.level, func(t *testing.T) {
			logger, err := NewLogger(tt.level, "json", "")
			require.NoError(t, err)
			assert.NotNil(t, logger)
		})
	}
}

func TestNewLogger_AllFormats(t *testing.T) {
	tests := []struct {
		format string
	}{
		{"json"},
		{"console"},
		{"JSON"},    // case insensitive
		{"CONSOLE"}, // case insensitive
		{"invalid"}, // defaults to json
		{""},        // defaults to json
	}

	for _, tt := range tests {
		t.Run(tt.format, func(t *testing.T) {
			logger, err := NewLogger("info", tt.format, "")
			require.NoError(t, err)
			assert.NotNil(t, logger)
		})
	}
}

func TestNewLogger_ConsoleFormat(t *testing.T) {
	tmpDir := t.TempDir()
	logFile := filepath.Join(tmpDir, "console.log")

	logger, err := NewLogger("debug", "console", logFile)
	require.NoError(t, err)
	logger.Info("test message", zap.String("key", "value"))
	require.NoError(t, logger.Sync())

	data, err := os.ReadFile(logFile)
	require.NoError(t, err)
	// Console format should contain the message and structured field
	assert.Contains(t, string(data), "test message")
	assert.Contains(t, string(data), "key")
}

func TestNewLogger_FileError(t *testing.T) {
	// Try to create a file in a directory that doesn't exist
	invalidPath := "/non/existent/directory/test.log"

	logger, err := NewLogger("info", "json", invalidPath)
	assert.Error(t, err)
	assert.Nil(t, logger)
}

// Test canonical field helpers
func TestCanonicalFields(t *testing.T) {
	tests := []struct {
		name     string
		fields   func() []zap.Field
		expected map[string]interface{}
	}{
		{
			name: "request fields",
			fields: func() []zap.Field {
				return RequestFields("req-123", "GET", "/v1/chat", 200, 150)
			},
			expected: map[string]interface{}{
				"request_id":  "req-123",
				"method":      "GET",
				"path":        "/v1/chat",
				"status_code": int64(200), // zap uses int64 internally
				"duration_ms": int64(150), // zap uses int64 internally
			},
		},
		{
			name: "correlation field",
			fields: func() []zap.Field {
				return []zap.Field{CorrelationID("corr-456")}
			},
			expected: map[string]interface{}{
				"correlation_id": "corr-456",
			},
		},
		{
			name: "project field",
			fields: func() []zap.Field {
				return []zap.Field{ProjectID("proj-789")}
			},
			expected: map[string]interface{}{
				"project_id": "proj-789",
			},
		},
		{
			name: "token field - obfuscated",
			fields: func() []zap.Field {
				return []zap.Field{TokenID("token-abcdef123456")}
			},
			expected: map[string]interface{}{
				"token_id": "token-ab...3456", // Should be obfuscated using admin.ObfuscateToken
			},
		},
		{
			name: "client IP field",
			fields: func() []zap.Field {
				return []zap.Field{ClientIP("192.168.1.1")}
			},
			expected: map[string]interface{}{
				"client_ip": "192.168.1.1",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create test logger with observer
			core, recorded := observer.New(zapcore.InfoLevel)
			logger := zap.New(core)

			// Log with fields
			logger.Info("test message", tt.fields()...)

			// Check recorded log entry
			require.Len(t, recorded.All(), 1)
			entry := recorded.All()[0]

			// Verify each expected field
			for key, expectedValue := range tt.expected {
				field, exists := entry.ContextMap()[key]
				require.True(t, exists, "field %s should exist", key)
				assert.Equal(t, expectedValue, field, "field %s should match expected value", key)
			}
		})
	}
}

// Test context-based logger helpers
func TestWithRequestContext(t *testing.T) {
	// Create test logger with observer
	core, recorded := observer.New(zapcore.InfoLevel)
	logger := zap.New(core)

	// Create context with request ID
	ctx := WithRequestID(context.Background(), "req-test-123")

	// Get logger with request context
	ctxLogger := WithRequestContext(ctx, logger)
	ctxLogger.Info("test message")

	// Verify request_id field was added
	require.Len(t, recorded.All(), 1)
	entry := recorded.All()[0]

	requestID, exists := entry.ContextMap()["request_id"]
	require.True(t, exists)
	assert.Equal(t, "req-test-123", requestID)
}

func TestWithCorrelationContext(t *testing.T) {
	// Create test logger with observer
	core, recorded := observer.New(zapcore.InfoLevel)
	logger := zap.New(core)

	// Create context with correlation ID
	ctx := WithCorrelationID(context.Background(), "corr-test-456")

	// Get logger with correlation context
	ctxLogger := WithCorrelationContext(ctx, logger)
	ctxLogger.Info("test message")

	// Verify correlation_id field was added
	require.Len(t, recorded.All(), 1)
	entry := recorded.All()[0]

	correlationID, exists := entry.ContextMap()["correlation_id"]
	require.True(t, exists)
	assert.Equal(t, "corr-test-456", correlationID)
}

// Test child logger creation
func TestNewChildLogger(t *testing.T) {
	// Create parent logger with observer
	core, recorded := observer.New(zapcore.InfoLevel)
	parent := zap.New(core)

	// Create child logger with component name
	child := NewChildLogger(parent, "proxy")
	child.Info("test message")

	// Verify component field was added
	require.Len(t, recorded.All(), 1)
	entry := recorded.All()[0]

	component, exists := entry.ContextMap()["component"]
	require.True(t, exists)
	assert.Equal(t, "proxy", component)
}

// Test context key retrieval
func TestGetRequestID(t *testing.T) {
	tests := []struct {
		name     string
		ctx      context.Context
		expected string
		found    bool
	}{
		{
			name:     "with request ID",
			ctx:      WithRequestID(context.Background(), "req-123"),
			expected: "req-123",
			found:    true,
		},
		{
			name:     "without request ID",
			ctx:      context.Background(),
			expected: "",
			found:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, found := GetRequestID(tt.ctx)
			assert.Equal(t, tt.expected, result)
			assert.Equal(t, tt.found, found)
		})
	}
}

func TestGetCorrelationID(t *testing.T) {
	tests := []struct {
		name     string
		ctx      context.Context
		expected string
		found    bool
	}{
		{
			name:     "with correlation ID",
			ctx:      WithCorrelationID(context.Background(), "corr-456"),
			expected: "corr-456",
			found:    true,
		},
		{
			name:     "without correlation ID",
			ctx:      context.Background(),
			expected: "",
			found:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, found := GetCorrelationID(tt.ctx)
			assert.Equal(t, tt.expected, result)
			assert.Equal(t, tt.found, found)
		})
	}
}
