package proxy

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/sofatutor/llm-proxy/internal/audit"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// MockAuditLogger is a mock for the audit.Logger
type MockAuditLogger struct {
	mock.Mock
}

func (m *MockAuditLogger) Log(event *audit.Event) error {
	args := m.Called(event)
	return args.Error(0)
}

func (m *MockAuditLogger) Close() error {
	args := m.Called()
	return args.Error(0)
}

func TestShouldAllowProject_AuditEmission(t *testing.T) {
	tests := []struct {
		name              string
		enforceActive     bool
		projectActive     bool
		checkError        error
		expectedAllowed   bool
		expectedStatus    int
		expectedCode      string
		expectAuditCall   bool
		expectedAction    string
		expectedResult    audit.ResultType
		expectedReason    string
	}{
		{
			name:            "enforcement_disabled_no_audit",
			enforceActive:   false,
			projectActive:   false,
			expectedAllowed: true,
			expectAuditCall: false,
		},
		{
			name:            "project_active_no_audit",
			enforceActive:   true,
			projectActive:   true,
			expectedAllowed: true,
			expectAuditCall: false,
		},
		{
			name:              "project_inactive_audit_denied",
			enforceActive:     true,
			projectActive:     false,
			expectedAllowed:   false,
			expectedStatus:    http.StatusForbidden,
			expectedCode:      "project_inactive",
			expectAuditCall:   true,
			expectedAction:    audit.ActionProxyRequest,
			expectedResult:    audit.ResultDenied,
			expectedReason:    "project_inactive",
		},
		{
			name:              "db_error_audit_error",
			enforceActive:     true,
			checkError:        errors.New("database connection failed"),
			expectedAllowed:   false,
			expectedStatus:    http.StatusServiceUnavailable,
			expectedCode:      "service_unavailable",
			expectAuditCall:   true,
			expectedAction:    audit.ActionProxyRequest,
			expectedResult:    audit.ResultError,
			expectedReason:    "service_unavailable",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup mock checker
			mockChecker := &MockProjectActiveChecker{}
			if tt.enforceActive {
				mockChecker.On("GetProjectActive", mock.Anything, "test-project").Return(tt.projectActive, tt.checkError)
			}

			// Setup mock audit logger
			mockAuditLogger := &MockAuditLogger{}
			if tt.expectAuditCall {
				mockAuditLogger.On("Log", mock.MatchedBy(func(event *audit.Event) bool {
					// Verify the audit event has expected fields
					assert.Equal(t, tt.expectedAction, event.Action)
					assert.Equal(t, tt.expectedResult, event.Result)
					assert.Equal(t, audit.ActorSystem, event.Actor)
					assert.Equal(t, "test-project", event.ProjectID)
					
					// Check details
					if tt.expectedReason != "" {
						reason, ok := event.Details["reason"].(string)
						assert.True(t, ok, "reason should be present in details")
						assert.Equal(t, tt.expectedReason, reason)
					}
					
					// Check for HTTP method and endpoint
					method, ok := event.Details["http_method"].(string)
					assert.True(t, ok, "http_method should be present")
					assert.Equal(t, "POST", method)
					
					endpoint, ok := event.Details["endpoint"].(string)
					assert.True(t, ok, "endpoint should be present")
					assert.Equal(t, "/v1/chat/completions", endpoint)
					
					return true
				})).Return(nil)
			}

			// Create test request
			req := httptest.NewRequest("POST", "/v1/chat/completions", nil)
			req.Header.Set("User-Agent", "test-client/1.0")
			req.RemoteAddr = "192.168.1.100:12345"
			ctx := context.WithValue(req.Context(), ctxKeyRequestID, "test-request-id")
			req = req.WithContext(ctx)

			// Call shouldAllowProject
			allowed, status, errorResp := shouldAllowProject(ctx, tt.enforceActive, mockChecker, "test-project", mockAuditLogger, req)

			// Verify results
			assert.Equal(t, tt.expectedAllowed, allowed)
			if !allowed {
				assert.Equal(t, tt.expectedStatus, status)
				assert.Equal(t, tt.expectedCode, errorResp.Code)
			}

			// Verify mock expectations
			mockChecker.AssertExpectations(t)
			mockAuditLogger.AssertExpectations(t)
		})
	}
}

func TestShouldAllowProject_NilAuditLogger(t *testing.T) {
	// Test that the function works properly when audit logger is nil
	mockChecker := &MockProjectActiveChecker{}
	mockChecker.On("GetProjectActive", mock.Anything, "test-project").Return(false, nil)

	req := httptest.NewRequest("POST", "/v1/chat/completions", nil)
	ctx := context.WithValue(req.Context(), ctxKeyRequestID, "test-request-id")
	req = req.WithContext(ctx)

	// Should not panic with nil audit logger
	allowed, status, errorResp := shouldAllowProject(ctx, true, mockChecker, "test-project", nil, req)

	assert.False(t, allowed)
	assert.Equal(t, http.StatusForbidden, status)
	assert.Equal(t, "project_inactive", errorResp.Code)

	mockChecker.AssertExpectations(t)
}

func TestGetClientIP(t *testing.T) {
	tests := []struct {
		name            string
		headers         map[string]string
		remoteAddr      string
		expectedIP      string
	}{
		{
			name:       "x_forwarded_for_single",
			headers:    map[string]string{"X-Forwarded-For": "192.168.1.100"},
			remoteAddr: "10.0.0.1:12345",
			expectedIP: "192.168.1.100",
		},
		{
			name:       "x_forwarded_for_multiple",
			headers:    map[string]string{"X-Forwarded-For": "192.168.1.100, 10.0.0.1, 203.0.113.1"},
			remoteAddr: "10.0.0.1:12345",
			expectedIP: "192.168.1.100",
		},
		{
			name:       "x_real_ip",
			headers:    map[string]string{"X-Real-IP": "192.168.1.200"},
			remoteAddr: "10.0.0.1:12345",
			expectedIP: "192.168.1.200",
		},
		{
			name:       "remote_addr_with_port",
			headers:    map[string]string{},
			remoteAddr: "192.168.1.300:45678",
			expectedIP: "192.168.1.300",
		},
		{
			name:       "remote_addr_without_port",
			headers:    map[string]string{},
			remoteAddr: "192.168.1.400",
			expectedIP: "192.168.1.400",
		},
		{
			name:       "x_forwarded_for_precedence",
			headers:    map[string]string{"X-Forwarded-For": "192.168.1.100", "X-Real-IP": "192.168.1.200"},
			remoteAddr: "10.0.0.1:12345",
			expectedIP: "192.168.1.100",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/test", nil)
			req.RemoteAddr = tt.remoteAddr
			for k, v := range tt.headers {
				req.Header.Set(k, v)
			}

			ip := getClientIP(req)
			assert.Equal(t, tt.expectedIP, ip)
		})
	}
}