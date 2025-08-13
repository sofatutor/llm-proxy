package logging

import (
	"context"
	"time"

	"go.uber.org/zap"
)

// AuditEventType represents the type of audit event
type AuditEventType string

const (
	// Project events
	AuditEventProjectCreate AuditEventType = "project_create"
	AuditEventProjectUpdate AuditEventType = "project_update"
	AuditEventProjectDelete AuditEventType = "project_delete"
	AuditEventProjectAccess AuditEventType = "project_access"

	// Token events
	AuditEventTokenCreate AuditEventType = "token_create"
	AuditEventTokenRevoke AuditEventType = "token_revoke"
	AuditEventTokenAccess AuditEventType = "token_access"
	AuditEventTokenExpire AuditEventType = "token_expire"

	// Authentication events
	AuditEventAdminLogin  AuditEventType = "admin_login"
	AuditEventAdminLogout AuditEventType = "admin_logout"
	AuditEventAuthFailure AuditEventType = "auth_failure"

	// Configuration events
	AuditEventConfigChange AuditEventType = "config_change"

	// API events
	AuditEventAPIAccess AuditEventType = "api_access"
	AuditEventAPIError  AuditEventType = "api_error"
)

// AuditOutcome represents the outcome of an audit event
type AuditOutcome string

const (
	AuditOutcomeSuccess AuditOutcome = "success"
	AuditOutcomeFailure AuditOutcome = "failure"
	AuditOutcomeError   AuditOutcome = "error"
)

// AuditEvent represents a security-sensitive event
type AuditEvent struct {
	EventType     AuditEventType `json:"event_type"`
	Actor         string         `json:"actor,omitempty"`          // User/token/system that performed the action
	Target        string         `json:"target,omitempty"`         // Resource being acted upon
	Outcome       AuditOutcome   `json:"outcome"`                  // Success/failure/error
	Reason        string         `json:"reason,omitempty"`         // Additional context for the outcome
	RequestID     string         `json:"request_id,omitempty"`     // Associated request ID
	CorrelationID string         `json:"correlation_id,omitempty"` // Associated correlation ID
	ProjectID     string         `json:"project_id,omitempty"`     // Associated project ID
	TokenID       string         `json:"token_id,omitempty"`       // Associated token ID (obfuscated)
	ClientIP      string         `json:"client_ip,omitempty"`      // Client IP address
	UserAgent     string         `json:"user_agent,omitempty"`     // Client user agent
	Timestamp     time.Time      `json:"timestamp"`                // When the event occurred
	Details       map[string]any `json:"details,omitempty"`        // Additional event-specific details
}

// AuditLogger provides structured audit logging functionality
type AuditLogger struct {
	logger *zap.Logger
}

// NewAuditLogger creates a new audit logger using the provided base logger
func NewAuditLogger(baseLogger *zap.Logger) *AuditLogger {
	return &AuditLogger{
		logger: baseLogger.With(zap.String("log_type", "audit")),
	}
}

// LogEvent logs an audit event with structured fields
func (a *AuditLogger) LogEvent(ctx context.Context, event AuditEvent) {
	// Set timestamp if not provided
	if event.Timestamp.IsZero() {
		event.Timestamp = time.Now().UTC()
	}

	// Extract context fields if not already set
	if event.RequestID == "" {
		event.RequestID = GetRequestID(ctx)
	}
	if event.CorrelationID == "" {
		event.CorrelationID = GetCorrelationID(ctx)
	}
	if event.ProjectID == "" {
		event.ProjectID = GetProjectID(ctx)
	}

	// Build zap fields
	fields := []zap.Field{
		zap.String(FieldEventType, string(event.EventType)),
		zap.String(FieldOutcome, string(event.Outcome)),
		zap.Time("timestamp", event.Timestamp),
	}

	// Add optional fields
	if event.Actor != "" {
		fields = append(fields, zap.String(FieldActor, event.Actor))
	}
	if event.Target != "" {
		fields = append(fields, zap.String(FieldTarget, event.Target))
	}
	if event.Reason != "" {
		fields = append(fields, zap.String(FieldReason, event.Reason))
	}
	if event.RequestID != "" {
		fields = append(fields, zap.String(FieldRequestID, event.RequestID))
	}
	if event.CorrelationID != "" {
		fields = append(fields, zap.String(FieldCorrelationID, event.CorrelationID))
	}
	if event.ProjectID != "" {
		fields = append(fields, zap.String(FieldProjectID, event.ProjectID))
	}
	if event.TokenID != "" {
		fields = append(fields, zap.String(FieldTokenID, event.TokenID))
	}
	if event.ClientIP != "" {
		fields = append(fields, zap.String(FieldClientIP, event.ClientIP))
	}
	if event.UserAgent != "" {
		fields = append(fields, zap.String(FieldUserAgent, event.UserAgent))
	}
	if len(event.Details) > 0 {
		fields = append(fields, zap.Any("details", event.Details))
	}

	// Log at appropriate level based on outcome
	switch event.Outcome {
	case AuditOutcomeFailure, AuditOutcomeError:
		a.logger.Warn("Audit event", fields...)
	default:
		a.logger.Info("Audit event", fields...)
	}
}

// LogProjectCreate logs a project creation event
func (a *AuditLogger) LogProjectCreate(ctx context.Context, projectID, projectName, actor string, outcome AuditOutcome, reason string) {
	a.LogEvent(ctx, AuditEvent{
		EventType: AuditEventProjectCreate,
		Actor:     actor,
		Target:    projectID,
		Outcome:   outcome,
		Reason:    reason,
		Details: map[string]any{
			"project_name": projectName,
		},
	})
}

// LogProjectUpdate logs a project update event
func (a *AuditLogger) LogProjectUpdate(ctx context.Context, projectID, projectName, actor string, outcome AuditOutcome, reason string, changes map[string]any) {
	details := map[string]any{
		"project_name": projectName,
	}
	if len(changes) > 0 {
		details["changes"] = changes
	}

	a.LogEvent(ctx, AuditEvent{
		EventType: AuditEventProjectUpdate,
		Actor:     actor,
		Target:    projectID,
		Outcome:   outcome,
		Reason:    reason,
		Details:   details,
	})
}

// LogProjectDelete logs a project deletion event
func (a *AuditLogger) LogProjectDelete(ctx context.Context, projectID, projectName, actor string, outcome AuditOutcome, reason string) {
	a.LogEvent(ctx, AuditEvent{
		EventType: AuditEventProjectDelete,
		Actor:     actor,
		Target:    projectID,
		Outcome:   outcome,
		Reason:    reason,
		Details: map[string]any{
			"project_name": projectName,
		},
	})
}

// LogTokenCreate logs a token creation event
func (a *AuditLogger) LogTokenCreate(ctx context.Context, obfuscatedToken, projectID, actor string, outcome AuditOutcome, reason string, expiresAt *time.Time) {
	details := map[string]any{
		"project_id": projectID,
	}
	if expiresAt != nil {
		details["expires_at"] = expiresAt.Format(time.RFC3339)
	}

	a.LogEvent(ctx, AuditEvent{
		EventType: AuditEventTokenCreate,
		Actor:     actor,
		Target:    obfuscatedToken,
		TokenID:   obfuscatedToken,
		ProjectID: projectID,
		Outcome:   outcome,
		Reason:    reason,
		Details:   details,
	})
}

// LogTokenRevoke logs a token revocation event
func (a *AuditLogger) LogTokenRevoke(ctx context.Context, obfuscatedToken, projectID, actor string, outcome AuditOutcome, reason string) {
	a.LogEvent(ctx, AuditEvent{
		EventType: AuditEventTokenRevoke,
		Actor:     actor,
		Target:    obfuscatedToken,
		TokenID:   obfuscatedToken,
		ProjectID: projectID,
		Outcome:   outcome,
		Reason:    reason,
	})
}

// LogAuthFailure logs an authentication failure event
func (a *AuditLogger) LogAuthFailure(ctx context.Context, actor, reason string, clientIP, userAgent string) {
	a.LogEvent(ctx, AuditEvent{
		EventType: AuditEventAuthFailure,
		Actor:     actor,
		Outcome:   AuditOutcomeFailure,
		Reason:    reason,
		ClientIP:  clientIP,
		UserAgent: userAgent,
	})
}

// LogConfigChange logs a configuration change event
func (a *AuditLogger) LogConfigChange(ctx context.Context, component, actor string, outcome AuditOutcome, reason string, changes map[string]any) {
	a.LogEvent(ctx, AuditEvent{
		EventType: AuditEventConfigChange,
		Actor:     actor,
		Target:    component,
		Outcome:   outcome,
		Reason:    reason,
		Details:   changes,
	})
}

// LogAPIAccess logs successful API access
func (a *AuditLogger) LogAPIAccess(ctx context.Context, method, path, obfuscatedToken, projectID string, statusCode int, durationMs float64) {
	a.LogEvent(ctx, AuditEvent{
		EventType: AuditEventAPIAccess,
		Actor:     obfuscatedToken,
		Target:    method + " " + path,
		TokenID:   obfuscatedToken,
		ProjectID: projectID,
		Outcome:   AuditOutcomeSuccess,
		Details: map[string]any{
			"method":      method,
			"path":        path,
			"status_code": statusCode,
			"duration_ms": durationMs,
		},
	})
}

// LogAPIError logs API access errors
func (a *AuditLogger) LogAPIError(ctx context.Context, method, path, obfuscatedToken, projectID, reason string, statusCode int) {
	a.LogEvent(ctx, AuditEvent{
		EventType: AuditEventAPIError,
		Actor:     obfuscatedToken,
		Target:    method + " " + path,
		TokenID:   obfuscatedToken,
		ProjectID: projectID,
		Outcome:   AuditOutcomeError,
		Reason:    reason,
		Details: map[string]any{
			"method":      method,
			"path":        path,
			"status_code": statusCode,
		},
	})
}