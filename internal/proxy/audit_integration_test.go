package proxy

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/sofatutor/llm-proxy/internal/audit"
	"github.com/sofatutor/llm-proxy/internal/logging"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// SimpleAuditCollector collects audit events for testing
type SimpleAuditCollector struct {
	events []*audit.Event
}

func (c *SimpleAuditCollector) Log(event *audit.Event) error {
	c.events = append(c.events, event)
	return nil
}

func (c *SimpleAuditCollector) Close() error {
	return nil
}

func (c *SimpleAuditCollector) GetEvents() []*audit.Event {
	return c.events
}

// TestProxyAuditIntegration tests end-to-end audit event emission for proxy requests
func TestProxyAuditIntegration(t *testing.T) {
	// Test cases for audit event integration
	tests := []struct {
		name            string
		projectID       string
		projectActive   bool
		getProjectError error
		expectedStatus  int
		expectedAction  string
		expectedResult  audit.ResultType
		expectedReason  string
	}{
		{
			name:           "project_inactive_403_audit",
			projectID:      "inactive-project",
			projectActive:  false,
			expectedStatus: http.StatusForbidden,
			expectedAction: audit.ActionProxyRequest,
			expectedResult: audit.ResultDenied,
			expectedReason: "project_inactive",
		},
		{
			name:            "db_error_503_audit",
			projectID:       "error-project",
			getProjectError: errors.New("database connection failed"),
			expectedStatus:  http.StatusServiceUnavailable,
			expectedAction:  audit.ActionProxyRequest,
			expectedResult:  audit.ResultError,
			expectedReason:  "service_unavailable",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup audit collector
			auditCollector := &SimpleAuditCollector{}

			// Setup mock project checker
			mockChecker := &MockProjectActiveChecker{}
			if tt.getProjectError != nil {
				mockChecker.On("GetProjectActive", mock.Anything, tt.projectID).Return(false, tt.getProjectError)
			} else {
				mockChecker.On("GetProjectActive", mock.Anything, tt.projectID).Return(tt.projectActive, nil)
			}

			// Create test request
			req := httptest.NewRequest("POST", "/v1/chat/completions", nil)
			req.Header.Set("User-Agent", "test-integration/1.0")
			req.RemoteAddr = "203.0.113.100:54321"

			// Add request ID and project ID to context
			ctx := context.WithValue(req.Context(), ctxKeyRequestID, "integration-test-123")
			ctx = logging.WithRequestID(ctx, "integration-test-123") // Use proper logging context
			ctx = context.WithValue(ctx, ctxKeyProjectID, tt.projectID)
			req = req.WithContext(ctx)

			// Create middleware with audit collector
			middleware := ProjectActiveGuardMiddleware(true, mockChecker, auditCollector)

			// Setup handler that should not be called if request is denied
			handlerCalled := false
			handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				handlerCalled = true
				w.WriteHeader(http.StatusOK)
			})

			// Execute request
			rr := httptest.NewRecorder()
			middleware(handler).ServeHTTP(rr, req)

			// Verify HTTP response
			assert.Equal(t, tt.expectedStatus, rr.Code)
			if tt.expectedStatus != http.StatusOK {
				assert.False(t, handlerCalled, "Handler should not be called for denied requests")
			}

			// Verify audit event was collected
			events := auditCollector.GetEvents()
			require.Len(t, events, 1, "Expected exactly one audit event")

			event := events[0]
			assert.Equal(t, tt.expectedAction, event.Action)
			assert.Equal(t, tt.expectedResult, event.Result)
			assert.Equal(t, audit.ActorSystem, event.Actor)
			assert.Equal(t, tt.projectID, event.ProjectID)
			assert.Equal(t, "integration-test-123", event.RequestID)
			assert.Equal(t, "203.0.113.100", event.ClientIP)

			// Verify details
			method, ok := event.Details["http_method"].(string)
			assert.True(t, ok)
			assert.Equal(t, "POST", method)

			endpoint, ok := event.Details["endpoint"].(string)
			assert.True(t, ok)
			assert.Equal(t, "/v1/chat/completions", endpoint)

			userAgent, ok := event.Details["user_agent"].(string)
			assert.True(t, ok)
			assert.Equal(t, "test-integration/1.0", userAgent)

			// Verify reason
			if tt.expectedReason != "" {
				reason, ok := event.Details["reason"].(string)
				assert.True(t, ok)
				assert.Equal(t, tt.expectedReason, reason)
			}

			// Cleanup
			mockChecker.AssertExpectations(t)
		})
	}
}

// TestProxyActiveProject_NoAuditEvent tests that active projects don't generate audit events
func TestProxyActiveProject_NoAuditEvent(t *testing.T) {
	// Setup audit collector
	auditCollector := &SimpleAuditCollector{}

	// Setup mock project checker for active project
	mockChecker := &MockProjectActiveChecker{}
	mockChecker.On("GetProjectActive", mock.Anything, "active-project").Return(true, nil)

	// Create test request
	req := httptest.NewRequest("GET", "/v1/models", nil)
	ctx := context.WithValue(req.Context(), ctxKeyRequestID, "no-audit-test")
	ctx = context.WithValue(ctx, ctxKeyProjectID, "active-project")
	req = req.WithContext(ctx)

	// Create middleware
	middleware := ProjectActiveGuardMiddleware(true, mockChecker, auditCollector)

	// Setup handler
	handlerCalled := false
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handlerCalled = true
		w.WriteHeader(http.StatusOK)
	})

	// Execute request
	rr := httptest.NewRecorder()
	middleware(handler).ServeHTTP(rr, req)

	// Verify success
	assert.Equal(t, http.StatusOK, rr.Code)
	assert.True(t, handlerCalled)

	// Verify no audit events were created
	events := auditCollector.GetEvents()
	assert.Len(t, events, 0, "Expected no audit events for successful requests")

	mockChecker.AssertExpectations(t)
}
