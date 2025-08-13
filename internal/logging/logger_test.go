package logging

import (
	"context"
	"os"
	"strings"
	"testing"

	"go.uber.org/zap/zaptest"
)

func TestNewLogger(t *testing.T) {
	tests := []struct {
		name     string
		level    string
		format   string
		filePath string
		wantErr  bool
	}{
		{
			name:    "debug level json format",
			level:   "debug",
			format:  "json",
			wantErr: false,
		},
		{
			name:    "info level console format",
			level:   "info",
			format:  "console",
			wantErr: false,
		},
		{
			name:    "warn level",
			level:   "warn",
			format:  "json",
			wantErr: false,
		},
		{
			name:    "error level",
			level:   "error",
			format:  "json",
			wantErr: false,
		},
		{
			name:    "default level (empty)",
			level:   "",
			format:  "json",
			wantErr: false,
		},
		{
			name:    "invalid level defaults to info",
			level:   "invalid",
			format:  "json",
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logger, err := NewLogger(tt.level, tt.format, tt.filePath)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewLogger() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && logger == nil {
				t.Errorf("NewLogger() returned nil logger")
			}
		})
	}
}

func TestNewComponentLogger(t *testing.T) {
	logger, err := NewComponentLogger("info", "json", "", ComponentServer)
	if err != nil {
		t.Fatalf("NewComponentLogger() error = %v", err)
	}
	if logger == nil {
		t.Fatalf("NewComponentLogger() returned nil logger")
	}
}

func TestContextFields(t *testing.T) {
	ctx := context.Background()
	
	// Test adding various context values
	ctx = WithRequestID(ctx, "req-123")
	ctx = WithCorrelationID(ctx, "corr-456")
	ctx = WithProjectID(ctx, "proj-789")
	ctx = WithTokenID(ctx, "token-abc")
	ctx = WithClientIP(ctx, "192.168.1.1")
	ctx = WithUserAgent(ctx, "test-agent")
	ctx = WithComponent(ctx, ComponentProxy)

	// Test extracting fields
	fields := ExtractContextFields(ctx)
	if len(fields) != 7 {
		t.Errorf("ExtractContextFields() returned %d fields, expected 7", len(fields))
	}

	// Test individual getters
	if GetRequestID(ctx) != "req-123" {
		t.Errorf("GetRequestID() = %v, want req-123", GetRequestID(ctx))
	}
	if GetCorrelationID(ctx) != "corr-456" {
		t.Errorf("GetCorrelationID() = %v, want corr-456", GetCorrelationID(ctx))
	}
	if GetProjectID(ctx) != "proj-789" {
		t.Errorf("GetProjectID() = %v, want proj-789", GetProjectID(ctx))
	}
}

func TestWithContext(t *testing.T) {
	// Create a test logger
	logger := zaptest.NewLogger(t)
	
	// Create context with fields
	ctx := context.Background()
	ctx = WithRequestID(ctx, "test-request-id")
	ctx = WithProjectID(ctx, "test-project-id")

	// Test adding context to logger
	contextLogger := WithContext(logger, ctx)
	if contextLogger == nil {
		t.Fatalf("WithContext() returned nil logger")
	}

	// Test with empty context
	emptyLogger := WithContext(logger, context.Background())
	if emptyLogger != logger {
		t.Errorf("WithContext() with empty context should return original logger")
	}
}

func TestExtractContextFields_EmptyContext(t *testing.T) {
	ctx := context.Background()
	fields := ExtractContextFields(ctx)
	if len(fields) != 0 {
		t.Errorf("ExtractContextFields() with empty context returned %d fields, expected 0", len(fields))
	}
}

func TestExtractContextFields_PartialContext(t *testing.T) {
	ctx := context.Background()
	ctx = WithRequestID(ctx, "req-123")
	ctx = WithProjectID(ctx, "proj-789")
	// Don't set correlation ID or others

	fields := ExtractContextFields(ctx)
	if len(fields) != 2 {
		t.Errorf("ExtractContextFields() with partial context returned %d fields, expected 2", len(fields))
	}
}

func TestContextGettersWithEmptyContext(t *testing.T) {
	ctx := context.Background()

	if GetRequestID(ctx) != "" {
		t.Errorf("GetRequestID() with empty context = %v, want empty string", GetRequestID(ctx))
	}
	if GetCorrelationID(ctx) != "" {
		t.Errorf("GetCorrelationID() with empty context = %v, want empty string", GetCorrelationID(ctx))
	}
	if GetProjectID(ctx) != "" {
		t.Errorf("GetProjectID() with empty context = %v, want empty string", GetProjectID(ctx))
	}
}

func TestLoggingWithFile(t *testing.T) {
	// Create a temporary file
	tmpFile, err := os.CreateTemp("", "test-log-*.log")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())
	tmpFile.Close()

	// Create logger with file output
	logger, err := NewLogger("info", "json", tmpFile.Name())
	if err != nil {
		t.Fatalf("NewLogger() with file error = %v", err)
	}

	// Log a message
	logger.Info("test message")

	// Check that file contains the message
	content, err := os.ReadFile(tmpFile.Name())
	if err != nil {
		t.Fatalf("Failed to read log file: %v", err)
	}

	if !strings.Contains(string(content), "test message") {
		t.Errorf("Log file does not contain expected message")
	}
}

func TestConstants(t *testing.T) {
	// Test that constants are defined and not empty
	constants := []string{
		ComponentServer,
		ComponentProxy,
		ComponentDatabase,
		ComponentToken,
		ComponentMiddleware,
		ComponentAdmin,
		ComponentDispatcher,
		ComponentEventBus,
		FieldRequestID,
		FieldCorrelationID,
		FieldMethod,
		FieldPath,
		FieldStatusCode,
		FieldDurationMs,
		FieldProjectID,
		FieldTokenID,
		FieldClientIP,
		FieldUserAgent,
		FieldComponent,
		FieldRemoteAddr,
		FieldOperation,
		FieldTarget,
		FieldActor,
		FieldOutcome,
		FieldReason,
		FieldEventType,
	}

	for _, constant := range constants {
		if constant == "" {
			t.Errorf("Constant is empty")
		}
	}
}
