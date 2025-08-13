package logging

import (
	"context"
	"testing"
	"time"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"
)

func TestNewAuditLogger(t *testing.T) {
	// Create a logger using the observer
	core, _ := observer.New(zapcore.InfoLevel)
	logger := zap.New(core)
	auditLogger := NewAuditLogger(logger)
	
	if auditLogger == nil {
		t.Fatal("NewAuditLogger() returned nil")
	}
	if auditLogger.logger == nil {
		t.Fatal("AuditLogger has nil logger")
	}
}

func TestAuditLogger_LogEvent(t *testing.T) {
	// Create a logger with observer to capture logs
	core, recorded := observer.New(zapcore.InfoLevel)
	logger := zap.New(core)
	auditLogger := NewAuditLogger(logger)

	ctx := context.Background()
	ctx = WithRequestID(ctx, "test-request-id")
	ctx = WithProjectID(ctx, "test-project-id")

	event := AuditEvent{
		EventType: AuditEventProjectCreate,
		Actor:     "admin@example.com",
		Target:    "project-123",
		Outcome:   AuditOutcomeSuccess,
		Reason:    "Created via API",
		Details: map[string]any{
			"project_name": "Test Project",
		},
	}

	auditLogger.LogEvent(ctx, event)

	// Verify log was recorded
	entries := recorded.All()
	if len(entries) != 1 {
		t.Fatalf("Expected 1 log entry, got %d", len(entries))
	}

	entry := entries[0]
	if entry.Message != "Audit event" {
		t.Errorf("Expected message 'Audit event', got %v", entry.Message)
	}
	if entry.Level != zapcore.InfoLevel {
		t.Errorf("Expected InfoLevel, got %v", entry.Level)
	}

	// Check required fields
	fields := entry.Context
	hasEventType := false
	hasOutcome := false
	hasActor := false
	hasTarget := false
	hasRequestID := false

	for _, field := range fields {
		switch field.Key {
		case FieldEventType:
			hasEventType = true
			if field.String != string(AuditEventProjectCreate) {
				t.Errorf("Expected event_type %v, got %v", AuditEventProjectCreate, field.String)
			}
		case FieldOutcome:
			hasOutcome = true
			if field.String != string(AuditOutcomeSuccess) {
				t.Errorf("Expected outcome %v, got %v", AuditOutcomeSuccess, field.String)
			}
		case FieldActor:
			hasActor = true
			if field.String != "admin@example.com" {
				t.Errorf("Expected actor admin@example.com, got %v", field.String)
			}
		case FieldTarget:
			hasTarget = true
			if field.String != "project-123" {
				t.Errorf("Expected target project-123, got %v", field.String)
			}
		case FieldRequestID:
			hasRequestID = true
			if field.String != "test-request-id" {
				t.Errorf("Expected request_id test-request-id, got %v", field.String)
			}
		}
	}

	if !hasEventType {
		t.Error("Missing event_type field")
	}
	if !hasOutcome {
		t.Error("Missing outcome field")
	}
	if !hasActor {
		t.Error("Missing actor field")
	}
	if !hasTarget {
		t.Error("Missing target field")
	}
	if !hasRequestID {
		t.Error("Missing request_id field")
	}
}

func TestAuditLogger_LogEvent_FailureLevel(t *testing.T) {
	// Create a logger with observer to capture logs
	core, recorded := observer.New(zapcore.WarnLevel)
	logger := zap.New(core)
	auditLogger := NewAuditLogger(logger)

	event := AuditEvent{
		EventType: AuditEventAuthFailure,
		Actor:     "malicious-user",
		Outcome:   AuditOutcomeFailure,
		Reason:    "Invalid credentials",
	}

	auditLogger.LogEvent(context.Background(), event)

	// Verify log was recorded at WARN level for failure
	entries := recorded.All()
	if len(entries) != 1 {
		t.Fatalf("Expected 1 log entry, got %d", len(entries))
	}

	entry := entries[0]
	if entry.Level != zapcore.WarnLevel {
		t.Errorf("Expected WarnLevel for failure outcome, got %v", entry.Level)
	}
}

func TestAuditLogger_LogProjectCreate(t *testing.T) {
	core, recorded := observer.New(zapcore.InfoLevel)
	logger := zap.New(core)
	auditLogger := NewAuditLogger(logger)

	ctx := WithRequestID(context.Background(), "req-123")

	auditLogger.LogProjectCreate(ctx, "proj-456", "Test Project", "admin", AuditOutcomeSuccess, "")

	entries := recorded.All()
	if len(entries) != 1 {
		t.Fatalf("Expected 1 log entry, got %d", len(entries))
	}

	entry := entries[0]
	if entry.Message != "Audit event" {
		t.Errorf("Expected message 'Audit event', got %v", entry.Message)
	}

	// Verify specific fields for project creation
	hasProjectName := false
	for _, field := range entry.Context {
		if field.Key == "details" && field.Interface != nil {
			if details, ok := field.Interface.(map[string]any); ok {
				if name, exists := details["project_name"]; exists && name == "Test Project" {
					hasProjectName = true
				}
			}
		}
	}

	if !hasProjectName {
		t.Error("Missing project_name in details")
	}
}

func TestAuditLogger_LogTokenCreate(t *testing.T) {
	core, recorded := observer.New(zapcore.InfoLevel)
	logger := zap.New(core)
	auditLogger := NewAuditLogger(logger)

	ctx := WithRequestID(context.Background(), "req-123")
	expiresAt := time.Now().Add(24 * time.Hour)

	auditLogger.LogTokenCreate(ctx, "sk-****1234", "proj-456", "admin", AuditOutcomeSuccess, "", &expiresAt)

	entries := recorded.All()
	if len(entries) != 1 {
		t.Fatalf("Expected 1 log entry, got %d", len(entries))
	}

	entry := entries[0]
	
	// Verify token-specific fields
	hasTokenID := false
	hasProjectID := false
	hasExpiresAt := false

	for _, field := range entry.Context {
		switch field.Key {
		case FieldTokenID:
			hasTokenID = true
			if field.String != "sk-****1234" {
				t.Errorf("Expected token_id sk-****1234, got %v", field.String)
			}
		case FieldProjectID:
			hasProjectID = true
			if field.String != "proj-456" {
				t.Errorf("Expected project_id proj-456, got %v", field.String)
			}
		case "details":
			if details, ok := field.Interface.(map[string]any); ok {
				if _, exists := details["expires_at"]; exists {
					hasExpiresAt = true
				}
			}
		}
	}

	if !hasTokenID {
		t.Error("Missing token_id field")
	}
	if !hasProjectID {
		t.Error("Missing project_id field")
	}
	if !hasExpiresAt {
		t.Error("Missing expires_at in details")
	}
}

func TestAuditLogger_LogAuthFailure(t *testing.T) {
	core, recorded := observer.New(zapcore.WarnLevel)
	logger := zap.New(core)
	auditLogger := NewAuditLogger(logger)

	auditLogger.LogAuthFailure(context.Background(), "malicious-user", "Invalid token", "192.168.1.1", "malicious-agent")

	entries := recorded.All()
	if len(entries) != 1 {
		t.Fatalf("Expected 1 log entry, got %d", len(entries))
	}

	entry := entries[0]
	if entry.Level != zapcore.WarnLevel {
		t.Errorf("Expected WarnLevel for auth failure, got %v", entry.Level)
	}

	// Verify auth failure specific fields
	hasClientIP := false
	hasUserAgent := false

	for _, field := range entry.Context {
		switch field.Key {
		case FieldClientIP:
			hasClientIP = true
			if field.String != "192.168.1.1" {
				t.Errorf("Expected client_ip 192.168.1.1, got %v", field.String)
			}
		case FieldUserAgent:
			hasUserAgent = true
			if field.String != "malicious-agent" {
				t.Errorf("Expected user_agent malicious-agent, got %v", field.String)
			}
		}
	}

	if !hasClientIP {
		t.Error("Missing client_ip field")
	}
	if !hasUserAgent {
		t.Error("Missing user_agent field")
	}
}

func TestAuditLogger_LogAPIAccess(t *testing.T) {
	core, recorded := observer.New(zapcore.InfoLevel)
	logger := zap.New(core)
	auditLogger := NewAuditLogger(logger)

	ctx := WithRequestID(context.Background(), "req-123")

	auditLogger.LogAPIAccess(ctx, "POST", "/v1/chat/completions", "sk-****1234", "proj-456", 200, 1250.5)

	entries := recorded.All()
	if len(entries) != 1 {
		t.Fatalf("Expected 1 log entry, got %d", len(entries))
	}

	entry := entries[0]
	
	// Verify API access specific fields
	hasTarget := false
	hasDetails := false

	for _, field := range entry.Context {
		switch field.Key {
		case FieldTarget:
			hasTarget = true
			expected := "POST /v1/chat/completions"
			if field.String != expected {
				t.Errorf("Expected target %v, got %v", expected, field.String)
			}
		case "details":
			hasDetails = true
			if details, ok := field.Interface.(map[string]any); ok {
				if method, exists := details["method"]; !exists || method != "POST" {
					t.Error("Missing or incorrect method in details")
				}
				if statusCode, exists := details["status_code"]; !exists || statusCode != 200 {
					t.Error("Missing or incorrect status_code in details")
				}
				if durationMs, exists := details["duration_ms"]; !exists || durationMs != 1250.5 {
					t.Error("Missing or incorrect duration_ms in details")
				}
			}
		}
	}

	if !hasTarget {
		t.Error("Missing target field")
	}
	if !hasDetails {
		t.Error("Missing details field")
	}
}

func TestAuditEvent_TimestampAutoSet(t *testing.T) {
	core, recorded := observer.New(zapcore.InfoLevel)
	logger := zap.New(core)
	auditLogger := NewAuditLogger(logger)

	event := AuditEvent{
		EventType: AuditEventProjectCreate,
		Actor:     "admin",
		Target:    "project-123",
		Outcome:   AuditOutcomeSuccess,
		// Timestamp not set - should be auto-generated
	}

	auditLogger.LogEvent(context.Background(), event)

	entries := recorded.All()
	if len(entries) != 1 {
		t.Fatalf("Expected 1 log entry, got %d", len(entries))
	}

	entry := entries[0]
	
	// Find timestamp field - the timestamp is automatically set during LogEvent
	// Just verify that the log entry was created with a timestamp field
	var timestampFound bool
	for _, field := range entry.Context {
		if field.Key == "timestamp" {
			timestampFound = true
			// The field.Interface contains the actual time.Time value
			// We just verify it exists - the exact timing test is brittle
			break
		}
	}

	if !timestampFound {
		t.Error("Missing timestamp field")
	}
}

func TestAuditEventTypes(t *testing.T) {
	// Test that all audit event type constants are defined and not empty
	eventTypes := []AuditEventType{
		AuditEventProjectCreate,
		AuditEventProjectUpdate,
		AuditEventProjectDelete,
		AuditEventProjectAccess,
		AuditEventTokenCreate,
		AuditEventTokenRevoke,
		AuditEventTokenAccess,
		AuditEventTokenExpire,
		AuditEventAdminLogin,
		AuditEventAdminLogout,
		AuditEventAuthFailure,
		AuditEventConfigChange,
		AuditEventAPIAccess,
		AuditEventAPIError,
	}

	for _, eventType := range eventTypes {
		if string(eventType) == "" {
			t.Errorf("Event type is empty: %v", eventType)
		}
	}
}

func TestAuditOutcomes(t *testing.T) {
	// Test that all audit outcome constants are defined and not empty
	outcomes := []AuditOutcome{
		AuditOutcomeSuccess,
		AuditOutcomeFailure,
		AuditOutcomeError,
	}

	for _, outcome := range outcomes {
		if string(outcome) == "" {
			t.Errorf("Outcome is empty: %v", outcome)
		}
	}
}