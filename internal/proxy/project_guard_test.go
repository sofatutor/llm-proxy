package proxy

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// MockProjectActiveChecker is a mock for the ProjectActiveChecker interface
type MockProjectActiveChecker struct {
	mock.Mock
}

func (m *MockProjectActiveChecker) GetProjectActive(ctx context.Context, projectID string) (bool, error) {
	args := m.Called(ctx, projectID)
	return args.Bool(0), args.Error(1)
}

func TestProjectActiveGuardMiddleware(t *testing.T) {
	tests := []struct {
		name           string
		enforceActive  bool
		projectID      string
		isActive       bool
		getActiveError error
		expectedStatus int
		expectedError  string
	}{
		{
			name:           "enforce disabled - allow inactive project",
			enforceActive:  false,
			projectID:      "inactive-project",
			isActive:       false,
			getActiveError: nil,
			expectedStatus: http.StatusOK,
		},
		{
			name:           "enforce enabled - allow active project",
			enforceActive:  true,
			projectID:      "active-project",
			isActive:       true,
			getActiveError: nil,
			expectedStatus: http.StatusOK,
		},
		{
			name:           "enforce enabled - deny inactive project",
			enforceActive:  true,
			projectID:      "inactive-project",
			isActive:       false,
			getActiveError: nil,
			expectedStatus: http.StatusForbidden,
			expectedError:  "project_inactive",
		},
		{
			name:           "database error - return 503",
			enforceActive:  true,
			projectID:      "any-project",
			isActive:       false,
			getActiveError: errors.New("database error"),
			expectedStatus: http.StatusServiceUnavailable,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup mock
			mockChecker := &MockProjectActiveChecker{}
			// Only set expectation if enforce is enabled and we expect a call
			if tt.enforceActive && tt.projectID != "" {
				mockChecker.On("GetProjectActive", mock.Anything, tt.projectID).Return(tt.isActive, tt.getActiveError)
			}

			// Setup middleware
			middleware := ProjectActiveGuardMiddleware(tt.enforceActive, mockChecker)

			// Setup handler that should be called if request is allowed
			handlerCalled := false
			handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				handlerCalled = true
				w.WriteHeader(http.StatusOK)
			})

			// Setup request with project ID in context
			req := httptest.NewRequest("POST", "/v1/chat/completions", nil)
			if tt.projectID != "" {
				ctx := context.WithValue(req.Context(), ctxKeyProjectID, tt.projectID)
				req = req.WithContext(ctx)
			}

			// Setup response recorder
			rr := httptest.NewRecorder()

			// Execute middleware
			middleware(handler).ServeHTTP(rr, req)

			// Verify response
			assert.Equal(t, tt.expectedStatus, rr.Code)

			if tt.expectedStatus == http.StatusOK {
				assert.True(t, handlerCalled, "Handler should have been called")
			} else {
				assert.False(t, handlerCalled, "Handler should not have been called")
			}

			if tt.expectedError != "" {
				assert.Contains(t, rr.Body.String(), tt.expectedError)
			}

			// Verify mock expectations
			mockChecker.AssertExpectations(t)
		})
	}
}

func TestProjectActiveGuardMiddleware_MissingProjectID(t *testing.T) {
	mockChecker := &MockProjectActiveChecker{}
	middleware := ProjectActiveGuardMiddleware(true, mockChecker)

	handlerCalled := false
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handlerCalled = true
		w.WriteHeader(http.StatusOK)
	})

	// Request without project ID in context
	req := httptest.NewRequest("POST", "/v1/chat/completions", nil)
	rr := httptest.NewRecorder()

	middleware(handler).ServeHTTP(rr, req)

	// Should return 500 Internal Server Error when project ID is missing
	assert.Equal(t, http.StatusInternalServerError, rr.Code)
	assert.False(t, handlerCalled)
	assert.Contains(t, rr.Body.String(), "missing project ID")
}
