package audit

import (
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewEvent(t *testing.T) {
	tests := []struct {
		name   string
		action string
		actor  string
		result ResultType
	}{
		{
			name:   "create token success event",
			action: ActionTokenCreate,
			actor:  ActorManagement,
			result: ResultSuccess,
		},
		{
			name:   "project delete failure event",
			action: ActionProjectDelete,
			actor:  ActorAdmin,
			result: ResultFailure,
		},
		{
			name:   "admin login with custom actor",
			action: ActionAdminLogin,
			actor:  "user123",
			result: ResultSuccess,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			before := time.Now().UTC()
			event := NewEvent(tt.action, tt.actor, tt.result)
			after := time.Now().UTC()

			// Verify basic fields
			assert.Equal(t, tt.action, event.Action)
			assert.Equal(t, tt.actor, event.Actor)
			assert.Equal(t, tt.result, event.Result)
			assert.NotNil(t, event.Details)

			// Verify timestamp is recent and in UTC
			assert.True(t, event.Timestamp.After(before) || event.Timestamp.Equal(before))
			assert.True(t, event.Timestamp.Before(after) || event.Timestamp.Equal(after))
			assert.Equal(t, time.UTC, event.Timestamp.Location())

			// Verify optional fields are empty
			assert.Empty(t, event.ProjectID)
			assert.Empty(t, event.RequestID)
			assert.Empty(t, event.CorrelationID)
		})
	}
}

func TestEvent_WithProjectID(t *testing.T) {
	event := NewEvent(ActionTokenCreate, ActorManagement, ResultSuccess)
	projectID := "project-123"

	result := event.WithProjectID(projectID)

	assert.Equal(t, projectID, result.ProjectID)
	assert.Same(t, event, result) // Should return same instance for chaining
}

func TestEvent_WithRequestID(t *testing.T) {
	event := NewEvent(ActionTokenCreate, ActorManagement, ResultSuccess)
	requestID := "req-456"

	result := event.WithRequestID(requestID)

	assert.Equal(t, requestID, result.RequestID)
	assert.Same(t, event, result)
}

func TestEvent_WithCorrelationID(t *testing.T) {
	event := NewEvent(ActionTokenCreate, ActorManagement, ResultSuccess)
	correlationID := "corr-789"

	result := event.WithCorrelationID(correlationID)

	assert.Equal(t, correlationID, result.CorrelationID)
	assert.Same(t, event, result)
}

func TestEvent_WithDetail(t *testing.T) {
	tests := []struct {
		name  string
		key   string
		value interface{}
	}{
		{
			name:  "string value",
			key:   "method",
			value: "POST",
		},
		{
			name:  "integer value",
			key:   "status_code",
			value: 201,
		},
		{
			name:  "boolean value",
			key:   "success",
			value: true,
		},
		{
			name:  "nil value",
			key:   "error",
			value: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			event := NewEvent(ActionTokenCreate, ActorManagement, ResultSuccess)

			result := event.WithDetail(tt.key, tt.value)

			assert.Equal(t, tt.value, result.Details[tt.key])
			assert.Same(t, event, result)
		})
	}
}

func TestEvent_WithDetail_InitializesDetailsMap(t *testing.T) {
	event := &Event{
		Action: ActionTokenCreate,
		Actor:  ActorManagement,
		Result: ResultSuccess,
		// Details is nil
	}

	result := event.WithDetail("key", "value")

	require.NotNil(t, result.Details)
	assert.Equal(t, "value", result.Details["key"])
}

func TestEvent_WithTokenID(t *testing.T) {
	event := NewEvent(ActionTokenValidate, ActorSystem, ResultFailure)
	token := "tk_1234567890abcdef1234567890abcdef"

	result := event.WithTokenID(token)

	// Should obfuscate the token
	obfuscated, ok := result.Details["token_id"].(string)
	require.True(t, ok)
	assert.Contains(t, obfuscated, "...")
	assert.NotEqual(t, token, obfuscated)
}

func TestEvent_WithError(t *testing.T) {
	t.Run("with error", func(t *testing.T) {
		event := NewEvent(ActionTokenCreate, ActorManagement, ResultFailure)
		err := errors.New("database connection failed")

		result := event.WithError(err)

		assert.Equal(t, "database connection failed", result.Details["error"])
	})

	t.Run("with nil error", func(t *testing.T) {
		event := NewEvent(ActionTokenCreate, ActorManagement, ResultSuccess)

		result := event.WithError(nil)

		_, exists := result.Details["error"]
		assert.False(t, exists)
	})
}

func TestEvent_ChainedMethodCalls(t *testing.T) {
	projectID := "proj-123"
	requestID := "req-456"
	correlationID := "corr-789"
	token := "tk_1234567890abcdef1234567890abcdef"

	event := NewEvent(ActionTokenCreate, ActorManagement, ResultSuccess).
		WithProjectID(projectID).
		WithRequestID(requestID).
		WithCorrelationID(correlationID).
		WithTokenID(token).
		WithDetail("method", "POST").
		WithError(errors.New("test error"))

	assert.Equal(t, ActionTokenCreate, event.Action)
	assert.Equal(t, ActorManagement, event.Actor)
	assert.Equal(t, ResultSuccess, event.Result)
	assert.Equal(t, projectID, event.ProjectID)
	assert.Equal(t, requestID, event.RequestID)
	assert.Equal(t, correlationID, event.CorrelationID)
	assert.Equal(t, "POST", event.Details["method"])
	assert.Equal(t, "test error", event.Details["error"])
	assert.Contains(t, event.Details["token_id"].(string), "...")
}

func TestEvent_WithIPAddress(t *testing.T) {
	event := NewEvent(ActionAdminLogin, ActorAdmin, ResultSuccess)
	ip := "192.168.1.100"

	result := event.WithIPAddress(ip)

	assert.Equal(t, ip, result.Details["ip_address"])
}

func TestEvent_WithUserAgent(t *testing.T) {
	event := NewEvent(ActionAdminLogin, ActorAdmin, ResultSuccess)
	userAgent := "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36"

	result := event.WithUserAgent(userAgent)

	assert.Equal(t, userAgent, result.Details["user_agent"])
}

func TestEvent_WithHTTPMethod(t *testing.T) {
	event := NewEvent(ActionTokenCreate, ActorManagement, ResultSuccess)
	method := "POST"

	result := event.WithHTTPMethod(method)

	assert.Equal(t, method, result.Details["http_method"])
}

func TestEvent_WithEndpoint(t *testing.T) {
	event := NewEvent(ActionTokenCreate, ActorManagement, ResultSuccess)
	endpoint := "/manage/tokens"

	result := event.WithEndpoint(endpoint)

	assert.Equal(t, endpoint, result.Details["endpoint"])
}

func TestEvent_WithDuration(t *testing.T) {
	event := NewEvent(ActionTokenCreate, ActorManagement, ResultSuccess)
	duration := 150 * time.Millisecond

	result := event.WithDuration(duration)

	assert.Equal(t, int64(150), result.Details["duration_ms"])
}

func TestResultTypeConstants(t *testing.T) {
	assert.Equal(t, ResultType("success"), ResultSuccess)
	assert.Equal(t, ResultType("failure"), ResultFailure)
}

func TestActionConstants(t *testing.T) {
	// Test a few key action constants
	assert.Equal(t, "token.create", ActionTokenCreate)
	assert.Equal(t, "token.validate", ActionTokenValidate)
	assert.Equal(t, "project.create", ActionProjectCreate)
	assert.Equal(t, "project.delete", ActionProjectDelete)
	assert.Equal(t, "admin.login", ActionAdminLogin)
}

func TestActorConstants(t *testing.T) {
	assert.Equal(t, "system", ActorSystem)
	assert.Equal(t, "anonymous", ActorAnonymous)
	assert.Equal(t, "admin", ActorAdmin)
	assert.Equal(t, "management_api", ActorManagement)
}
