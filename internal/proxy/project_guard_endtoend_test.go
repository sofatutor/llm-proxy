package proxy

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"go.uber.org/zap"
)

// TestProjectActiveGuard_EndToEnd tests the full project active guard flow
func TestProjectActiveGuard_EndToEnd(t *testing.T) {
	// Create mock API server
	mockAPI := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"message": "success"}`))
	}))
	defer mockAPI.Close()

	tests := []struct {
		name           string
		enforceActive  bool
		projectActive  bool
		expectedStatus int
		expectedBody   string
	}{
		{
			name:           "enforce enabled + active project = 200",
			enforceActive:  true,
			projectActive:  true,
			expectedStatus: http.StatusOK,
			expectedBody:   "success",
		},
		{
			name:           "enforce enabled + inactive project = 403",
			enforceActive:  true,
			projectActive:  false,
			expectedStatus: http.StatusForbidden,
			expectedBody:   "project_inactive",
		},
		{
			name:           "enforce disabled + inactive project = 200",
			enforceActive:  false,
			projectActive:  false,
			expectedStatus: http.StatusOK,
			expectedBody:   "success",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create mocks
			mockValidator := new(MockTokenValidator)
			mockStore := new(MockProjectStore)

			// Set up expectations
			mockValidator.On("ValidateTokenWithTracking", mock.Anything, "test-token").Return("test-project", nil).Maybe()
			mockStore.On("GetAPIKeyForProject", mock.Anything, "test-project").Return("api-key", nil).Maybe()

			// Only expect GetProjectActive call if enforcement is enabled
			if tt.enforceActive {
				mockStore.On("GetProjectActive", mock.Anything, "test-project").Return(tt.projectActive, nil).Maybe()
			}

			// Create proxy configuration
			config := ProxyConfig{
				TargetBaseURL:        mockAPI.URL,
				AllowedEndpoints:     []string{"/v1/chat/completions"},
				AllowedMethods:       []string{"POST"},
				EnforceProjectActive: tt.enforceActive,
			}

			// Create proxy
			proxy, err := NewTransparentProxyWithLogger(config, mockValidator, mockStore, zap.NewNop())
			assert.NoError(t, err)

			// Create request
			reqBody := strings.NewReader(`{"model": "gpt-4", "messages": [{"role": "user", "content": "Hello"}]}`)
			req := httptest.NewRequest("POST", "/v1/chat/completions", reqBody)
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("Authorization", "Bearer test-token")

			// Record response
			w := httptest.NewRecorder()

			// Process request
			proxy.Handler().ServeHTTP(w, req)

			// Verify response
			assert.Equal(t, tt.expectedStatus, w.Code)
			assert.Contains(t, w.Body.String(), tt.expectedBody)

			// Verify mock expectations
			mockValidator.AssertExpectations(t)
			mockStore.AssertExpectations(t)
		})
	}
}
