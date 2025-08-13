// Package audit provides audit logging functionality for security-sensitive events
// in the LLM proxy. It implements a separate audit sink with immutable semantics
// for compliance and security investigations.
package audit

import (
	"time"

	"github.com/sofatutor/llm-proxy/internal/obfuscate"
)

// Event represents a security audit event with canonical fields.
// All audit events must include these core fields for compliance and investigation purposes.
type Event struct {
	// Timestamp when the event occurred (ISO8601 format)
	Timestamp time.Time `json:"timestamp"`

	// Action describes what operation was performed (e.g., "token.create", "project.delete")
	Action string `json:"action"`

	// Actor identifies who performed the action (user ID, token ID, or system)
	Actor string `json:"actor"`

	// ProjectID identifies which project was affected (if applicable)
	ProjectID string `json:"project_id,omitempty"`

	// RequestID for correlation with request logs
	RequestID string `json:"request_id,omitempty"`

	// CorrelationID for tracing across services
	CorrelationID string `json:"correlation_id,omitempty"`

	// ClientIP is the IP address of the client making the request
	ClientIP string `json:"client_ip,omitempty"`

	// Result indicates success or failure of the operation
	Result ResultType `json:"result"`

	// Details contains additional context about the event (no secrets)
	Details map[string]interface{} `json:"details,omitempty"`
}

// ResultType represents the outcome of an audited operation
type ResultType string

const (
	// ResultSuccess indicates the operation completed successfully
	ResultSuccess ResultType = "success"

	// ResultFailure indicates the operation failed
	ResultFailure ResultType = "failure"
)

// Action constants for standardized audit event types
const (
	// Token lifecycle actions
	ActionTokenCreate   = "token.create"
	ActionTokenRevoke   = "token.revoke"
	ActionTokenDelete   = "token.delete"
	ActionTokenList     = "token.list"
	ActionTokenValidate = "token.validate"
	ActionTokenAccess   = "token.access"

	// Project lifecycle actions
	ActionProjectCreate = "project.create"
	ActionProjectRead   = "project.read"
	ActionProjectUpdate = "project.update"
	ActionProjectDelete = "project.delete"
	ActionProjectList   = "project.list"

	// Admin actions
	ActionAdminLogin  = "admin.login"
	ActionAdminLogout = "admin.logout"
	ActionAdminAccess = "admin.access"
)

// Actor types for common audit actors
const (
	ActorSystem     = "system"
	ActorAnonymous  = "anonymous"
	ActorAdmin      = "admin"
	ActorManagement = "management_api"
)

// NewEvent creates a new audit event with the specified action and result.
// The timestamp is automatically set to the current time.
func NewEvent(action string, actor string, result ResultType) *Event {
	return &Event{
		Timestamp: time.Now().UTC(),
		Action:    action,
		Actor:     actor,
		Result:    result,
		Details:   make(map[string]interface{}),
	}
}

// WithProjectID sets the project ID for the audit event
func (e *Event) WithProjectID(projectID string) *Event {
	e.ProjectID = projectID
	return e
}

// WithRequestID sets the request ID for correlation with request logs
func (e *Event) WithRequestID(requestID string) *Event {
	e.RequestID = requestID
	return e
}

// WithCorrelationID sets the correlation ID for tracing across services
func (e *Event) WithCorrelationID(correlationID string) *Event {
	e.CorrelationID = correlationID
	return e
}

// WithClientIP sets the client IP address for the audit event
func (e *Event) WithClientIP(clientIP string) *Event {
	e.ClientIP = clientIP
	return e
}

// WithDetail adds a detail key-value pair to the audit event.
// Secrets and sensitive information should be obfuscated before calling this method.
func (e *Event) WithDetail(key string, value interface{}) *Event {
	if e.Details == nil {
		e.Details = make(map[string]interface{})
	}
	e.Details[key] = value
	return e
}

// WithTokenID adds an obfuscated token ID to the audit event details
func (e *Event) WithTokenID(token string) *Event {
	return e.WithDetail("token_id", obfuscate.ObfuscateTokenGeneric(token))
}

// WithError adds error information to the audit event details
func (e *Event) WithError(err error) *Event {
	if err != nil {
		return e.WithDetail("error", err.Error())
	}
	return e
}

// WithIPAddress adds the client IP address to the audit event details
// Deprecated: Use WithClientIP instead for first-class IP field support
func (e *Event) WithIPAddress(ip string) *Event {
	return e.WithDetail("ip_address", ip)
}

// WithUserAgent adds the user agent to the audit event details
func (e *Event) WithUserAgent(userAgent string) *Event {
	return e.WithDetail("user_agent", userAgent)
}

// WithHTTPMethod adds the HTTP method to the audit event details
func (e *Event) WithHTTPMethod(method string) *Event {
	return e.WithDetail("http_method", method)
}

// WithEndpoint adds the API endpoint to the audit event details
func (e *Event) WithEndpoint(endpoint string) *Event {
	return e.WithDetail("endpoint", endpoint)
}

// WithDuration adds the operation duration to the audit event details
func (e *Event) WithDuration(duration time.Duration) *Event {
	return e.WithDetail("duration_ms", duration.Milliseconds())
}
