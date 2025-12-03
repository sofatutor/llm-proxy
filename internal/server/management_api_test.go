package server

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/sofatutor/llm-proxy/internal/config"
	"github.com/sofatutor/llm-proxy/internal/logging"
	"github.com/sofatutor/llm-proxy/internal/proxy"
	"github.com/sofatutor/llm-proxy/internal/token"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// MockTokenStore with methods that return testable results
type MockTokenStoreExtended struct {
	*mock.Mock
	tokens []token.TokenData
}

func (m *MockTokenStoreExtended) GetTokenByID(ctx context.Context, tokenID string) (token.TokenData, error) {
	args := m.Called(ctx, tokenID)
	return args.Get(0).(token.TokenData), args.Error(1)
}

func (m *MockTokenStoreExtended) IncrementTokenUsage(ctx context.Context, tokenID string) error {
	args := m.Called(ctx, tokenID)
	return args.Error(0)
}

func (m *MockTokenStoreExtended) CreateToken(ctx context.Context, td token.TokenData) error {
	args := m.Called(ctx, td)
	if args.Error(0) == nil {
		m.tokens = append(m.tokens, td)
	}
	return args.Error(0)
}

func (m *MockTokenStoreExtended) UpdateToken(ctx context.Context, td token.TokenData) error {
	args := m.Called(ctx, td)
	if args.Error(0) == nil {
		// Update token in mock storage
		for i, token := range m.tokens {
			if token.Token == td.Token {
				m.tokens[i] = td
				break
			}
		}
	}
	return args.Error(0)
}

func (m *MockTokenStoreExtended) GetTokensByProjectID(ctx context.Context, projectID string) ([]token.TokenData, error) {
	args := m.Called(ctx, projectID)
	return args.Get(0).([]token.TokenData), args.Error(1)
}

func (m *MockTokenStoreExtended) ListTokens(ctx context.Context) ([]token.TokenData, error) {
	args := m.Called(ctx)
	return args.Get(0).([]token.TokenData), args.Error(1)
}

func (m *MockTokenStoreExtended) RevokeToken(ctx context.Context, tokenID string) error {
	args := m.Called(ctx, tokenID)
	return args.Error(0)
}

func (m *MockTokenStoreExtended) RevokeProjectTokens(ctx context.Context, projectID string) error {
	args := m.Called(ctx, projectID)
	return args.Error(0)
}

// MockProjectStore with methods that return testable results
type MockProjectStoreExtended struct {
	*mock.Mock
	projects []proxy.Project
}

func (m *MockProjectStoreExtended) GetAPIKeyForProject(ctx context.Context, projectID string) (string, error) {
	args := m.Called(ctx, projectID)
	return args.String(0), args.Error(1)
}

func (m *MockProjectStoreExtended) ListProjects(ctx context.Context) ([]proxy.Project, error) {
	args := m.Called(ctx)
	return args.Get(0).([]proxy.Project), args.Error(1)
}

func (m *MockProjectStoreExtended) CreateProject(ctx context.Context, p proxy.Project) error {
	args := m.Called(ctx, p)
	if args.Error(0) == nil {
		m.projects = append(m.projects, p)
	}
	return args.Error(0)
}

func (m *MockProjectStoreExtended) GetProjectByID(ctx context.Context, id string) (proxy.Project, error) {
	args := m.Called(ctx, id)
	return args.Get(0).(proxy.Project), args.Error(1)
}

func (m *MockProjectStoreExtended) UpdateProject(ctx context.Context, p proxy.Project) error {
	args := m.Called(ctx, p)
	return args.Error(0)
}

func (m *MockProjectStoreExtended) DeleteProject(ctx context.Context, id string) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func (m *MockProjectStoreExtended) GetProjectActive(ctx context.Context, projectID string) (bool, error) {
	args := m.Called(ctx, projectID)
	return args.Bool(0), args.Error(1)
}

func setupServerAndMocks(t *testing.T) (*Server, *MockTokenStoreExtended, *MockProjectStoreExtended) {
	tokenStore := &MockTokenStoreExtended{Mock: &mock.Mock{}}
	projectStore := &MockProjectStoreExtended{Mock: &mock.Mock{}}

	cfg := &config.Config{
		ListenAddr:      ":8080",
		RequestTimeout:  30 * time.Second,
		ManagementToken: "test_management_token",
		EventBusBackend: "in-memory",
	}

	server, err := New(cfg, tokenStore, projectStore)
	require.NoError(t, err)

	return server, tokenStore, projectStore
}

func TestManagementAuthMiddleware(t *testing.T) {
	server, _, _ := setupServerAndMocks(t)

	testCases := []struct {
		name           string
		authHeader     string
		expectedStatus int
	}{
		{
			name:           "Missing auth header",
			authHeader:     "",
			expectedStatus: http.StatusUnauthorized,
		},
		{
			name:           "Invalid auth format",
			authHeader:     "InvalidFormat test_token",
			expectedStatus: http.StatusUnauthorized,
		},
		{
			name:           "Bearer without token",
			authHeader:     "Bearer ",
			expectedStatus: http.StatusUnauthorized,
		},
		{
			name:           "Invalid token",
			authHeader:     "Bearer wrong_token",
			expectedStatus: http.StatusUnauthorized,
		},
		{
			name:           "Valid token",
			authHeader:     "Bearer test_management_token",
			expectedStatus: http.StatusOK,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Create a simple handler to test the middleware
			handlerCalled := false
			testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				handlerCalled = true
				w.WriteHeader(http.StatusOK)
			})

			// Apply the middleware
			middlewareHandler := server.managementAuthMiddleware(testHandler)

			// Create a request with the test auth header
			req := httptest.NewRequest("GET", "/test", nil)
			if tc.authHeader != "" {
				req.Header.Set("Authorization", tc.authHeader)
			}

			// Record the response
			w := httptest.NewRecorder()
			middlewareHandler.ServeHTTP(w, req)

			// Check results
			assert.Equal(t, tc.expectedStatus, w.Code)
			assert.Equal(t, tc.expectedStatus == http.StatusOK, handlerCalled,
				"Handler should only be called when auth is successful")
		})
	}
}

func TestCheckManagementAuth(t *testing.T) {
	server, _, _ := setupServerAndMocks(t)

	testCases := []struct {
		name       string
		authHeader string
		expected   bool
	}{
		{
			name:       "Missing auth header",
			authHeader: "",
			expected:   false,
		},
		{
			name:       "Invalid auth format",
			authHeader: "InvalidFormat test_token",
			expected:   false,
		},
		{
			name:       "Bearer without token",
			authHeader: "Bearer ",
			expected:   false,
		},
		{
			name:       "Invalid token",
			authHeader: "Bearer wrong_token",
			expected:   false,
		},
		{
			name:       "Valid token",
			authHeader: "Bearer test_management_token",
			expected:   true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/test", nil)
			if tc.authHeader != "" {
				req.Header.Set("Authorization", tc.authHeader)
			}

			w := httptest.NewRecorder()
			result := server.checkManagementAuth(w, req)

			assert.Equal(t, tc.expected, result)
			if !tc.expected {
				assert.Equal(t, http.StatusUnauthorized, w.Code)
			}
		})
	}
}

func TestHandleProjects(t *testing.T) {
	server, _, projectStore := setupServerAndMocks(t)

	testProjects := []proxy.Project{
		{
			ID:   "proj1",
			Name: "Test Project 1",
		},
		{
			ID:   "proj2",
			Name: "Test Project 2",
		},
	}

	// Setup mock for listing projects
	projectStore.On("ListProjects", mock.Anything).Return(testProjects, nil)

	// Setup mock for creating a project
	projectStore.On("CreateProject", mock.Anything, mock.AnythingOfType("proxy.Project")).Return(nil)

	t.Run("GET_Projects_Success", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/manage/projects", nil)
		req.Header.Set("Authorization", "Bearer test_management_token")
		w := httptest.NewRecorder()

		server.handleProjects(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		assert.Equal(t, "application/json", w.Header().Get("Content-Type"))

		var response []proxy.Project
		err := json.NewDecoder(w.Body).Decode(&response)
		require.NoError(t, err)
		assert.Len(t, response, 2)
		assert.Equal(t, testProjects[0].ID, response[0].ID)
		assert.Equal(t, testProjects[1].Name, response[1].Name)
	})

	t.Run("POST_Project_Success", func(t *testing.T) {
		reqBody := map[string]string{
			"name":           "New Project",
			"openai_api_key": "key123",
		}
		body, _ := json.Marshal(reqBody)
		req := httptest.NewRequest("POST", "/manage/projects", bytes.NewReader(body))
		req.Header.Set("Authorization", "Bearer test_management_token")
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		server.handleProjects(w, req)

		assert.Equal(t, http.StatusCreated, w.Code)
		assert.Equal(t, "application/json", w.Header().Get("Content-Type"))

		var response proxy.Project
		err := json.NewDecoder(w.Body).Decode(&response)
		require.NoError(t, err)
		assert.Equal(t, "New Project", response.Name)
		assert.Equal(t, "key123", response.OpenAIAPIKey)
		assert.NotEmpty(t, response.ID)
	})

	t.Run("Invalid_Method", func(t *testing.T) {
		req := httptest.NewRequest("PUT", "/manage/projects", nil)
		req.Header.Set("Authorization", "Bearer test_management_token")
		w := httptest.NewRecorder()

		server.handleProjects(w, req)

		assert.Equal(t, http.StatusMethodNotAllowed, w.Code)
	})

}

func TestHandleProjectByID(t *testing.T) {
	server, _, projectStore := setupServerAndMocks(t)

	testProject := proxy.Project{
		ID:           "test-project-id",
		Name:         "Test Project",
		OpenAIAPIKey: "secret-key",
		CreatedAt:    time.Now().UTC(),
		UpdatedAt:    time.Now().UTC(),
	}

	// Setup mock for get project
	projectStore.On("GetProjectByID", mock.Anything, "test-project-id").Return(testProject, nil)
	projectStore.On("GetProjectByID", mock.Anything, "non-existent-id").Return(proxy.Project{}, errors.New("not found"))

	// Setup mock for update project
	projectStore.On("UpdateProject", mock.Anything, mock.AnythingOfType("proxy.Project")).Return(nil)

	// Setup mock for delete project
	projectStore.On("DeleteProject", mock.Anything, "test-project-id").Return(nil)
	projectStore.On("DeleteProject", mock.Anything, "non-existent-id").Return(errors.New("not found"))

	t.Run("GET_Project_Success", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/manage/projects/test-project-id", nil)
		req.Header.Set("Authorization", "Bearer test_management_token")
		w := httptest.NewRecorder()

		server.handleProjectByID(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		assert.Equal(t, "application/json", w.Header().Get("Content-Type"))

		var response proxy.Project
		err := json.NewDecoder(w.Body).Decode(&response)
		require.NoError(t, err)
		assert.Equal(t, testProject.ID, response.ID)
		assert.Equal(t, testProject.Name, response.Name)
	})

	t.Run("GET_Project_NotFound", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/manage/projects/non-existent-id", nil)
		req.Header.Set("Authorization", "Bearer test_management_token")
		w := httptest.NewRecorder()

		server.handleProjectByID(w, req)

		assert.Equal(t, http.StatusNotFound, w.Code)
	})

	t.Run("PATCH_Project_Success", func(t *testing.T) {
		reqBody := map[string]string{
			"name": "Updated Project",
		}
		body, _ := json.Marshal(reqBody)
		req := httptest.NewRequest("PATCH", "/manage/projects/test-project-id", bytes.NewReader(body))
		req.Header.Set("Authorization", "Bearer test_management_token")
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		server.handleProjectByID(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		assert.Equal(t, "application/json", w.Header().Get("Content-Type"))

		var response proxy.Project
		err := json.NewDecoder(w.Body).Decode(&response)
		require.NoError(t, err)
		assert.Equal(t, "Updated Project", response.Name)
	})

	t.Run("DELETE_Project_Success", func(t *testing.T) {
		req := httptest.NewRequest("DELETE", "/manage/projects/test-project-id", nil)
		req.Header.Set("Authorization", "Bearer test_management_token")
		w := httptest.NewRecorder()

		server.handleProjectByID(w, req)

		assert.Equal(t, http.StatusMethodNotAllowed, w.Code)
		assert.Contains(t, w.Header().Get("Allow"), "GET, PATCH")
	})

	t.Run("DELETE_Project_NotFound", func(t *testing.T) {
		req := httptest.NewRequest("DELETE", "/manage/projects/non-existent-id", nil)
		req.Header.Set("Authorization", "Bearer test_management_token")
		w := httptest.NewRecorder()

		server.handleProjectByID(w, req)

		assert.Equal(t, http.StatusMethodNotAllowed, w.Code)
		assert.Contains(t, w.Header().Get("Allow"), "GET, PATCH")
	})

	t.Run("Invalid_Method", func(t *testing.T) {
		req := httptest.NewRequest("PUT", "/manage/projects/test-project-id", nil)
		req.Header.Set("Authorization", "Bearer test_management_token")
		w := httptest.NewRecorder()

		server.handleProjectByID(w, req)

		assert.Equal(t, http.StatusMethodNotAllowed, w.Code)
	})
}

func TestHandleTokens(t *testing.T) {
	server, tokenStore, projectStore := setupServerAndMocks(t)

	testProject := proxy.Project{
		ID:       "project-1",
		Name:     "Test Project",
		IsActive: true,
	}

	testTokens := []token.TokenData{
		{
			Token:         "token-1",
			ProjectID:     "project-1",
			IsActive:      true,
			CacheHitCount: 42,
		},
		{
			Token:         "token-2",
			ProjectID:     "project-2",
			IsActive:      true,
			CacheHitCount: 100,
		},
	}

	// Setup mocks
	projectStore.On("GetProjectByID", mock.Anything, "project-1").Return(testProject, nil)
	projectStore.On("GetProjectByID", mock.Anything, "non-existent-project").Return(proxy.Project{}, errors.New("not found"))
	tokenStore.On("CreateToken", mock.Anything, mock.AnythingOfType("token.TokenData")).Return(nil)
	tokenStore.On("ListTokens", mock.Anything).Return(testTokens, nil)
	tokenStore.On("GetTokensByProjectID", mock.Anything, "project-1").Return([]token.TokenData{testTokens[0]}, nil)

	t.Run("POST_Token_Success", func(t *testing.T) {
		reqBody := map[string]interface{}{
			"project_id":       "project-1",
			"duration_minutes": 60 * 24,
		}
		body, _ := json.Marshal(reqBody)
		req := httptest.NewRequest("POST", "/manage/tokens", bytes.NewReader(body))
		req.Header.Set("Authorization", "Bearer test_management_token")
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		server.handleTokens(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		assert.Equal(t, "application/json", w.Header().Get("Content-Type"))

		var response map[string]interface{}
		err := json.NewDecoder(w.Body).Decode(&response)
		require.NoError(t, err)
		assert.Contains(t, response, "token")
		assert.Contains(t, response, "expires_at")
	})

	t.Run("POST_Token_InvalidRequest", func(t *testing.T) {
		reqBody := map[string]interface{}{
			"project_id": "project-1",
			// Missing duration_minutes
		}
		body, _ := json.Marshal(reqBody)
		req := httptest.NewRequest("POST", "/manage/tokens", bytes.NewReader(body))
		req.Header.Set("Authorization", "Bearer test_management_token")
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		server.handleTokens(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("POST_Token_ProjectNotFound", func(t *testing.T) {
		reqBody := map[string]interface{}{
			"project_id":       "non-existent-project",
			"duration_minutes": 60 * 24,
		}
		body, _ := json.Marshal(reqBody)
		req := httptest.NewRequest("POST", "/manage/tokens", bytes.NewReader(body))
		req.Header.Set("Authorization", "Bearer test_management_token")
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		server.handleTokens(w, req)

		assert.Equal(t, http.StatusNotFound, w.Code)
	})

	t.Run("GET_AllTokens", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/manage/tokens", nil)
		req.Header.Set("Authorization", "Bearer test_management_token")
		w := httptest.NewRecorder()

		server.handleTokens(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		assert.Equal(t, "application/json", w.Header().Get("Content-Type"))

		// Expect sanitized token response (without actual token values)
		raw := w.Body.Bytes()

		var response []TokenListResponse
		err := json.NewDecoder(bytes.NewReader(raw)).Decode(&response)
		require.NoError(t, err)
		assert.Len(t, response, 2)

		// Verify cache_hit_count is returned in the response
		assert.Equal(t, 42, response[0].CacheHitCount, "cache_hit_count should be returned for token-1")
		assert.Equal(t, 100, response[1].CacheHitCount, "cache_hit_count should be returned for token-2")

		// Explicitly assert that no raw token field is present
		var generic []map[string]interface{}
		err = json.NewDecoder(bytes.NewReader(raw)).Decode(&generic)
		require.NoError(t, err)
		for _, item := range generic {
			if _, exists := item["token"]; exists {
				t.Fatalf("response item unexpectedly contains raw token field: %v", item)
			}
			// Also verify cache_hit_count is present in raw JSON
			if _, exists := item["cache_hit_count"]; !exists {
				t.Fatalf("response item missing cache_hit_count field: %v", item)
			}
		}
	})

	t.Run("GET_TokensByProject", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/manage/tokens?projectId=project-1", nil)
		req.Header.Set("Authorization", "Bearer test_management_token")
		w := httptest.NewRecorder()

		server.handleTokens(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		assert.Equal(t, "application/json", w.Header().Get("Content-Type"))

		// Expect sanitized token response (without actual token values)
		raw := w.Body.Bytes()

		var response []TokenListResponse
		err := json.NewDecoder(bytes.NewReader(raw)).Decode(&response)
		require.NoError(t, err)
		assert.Len(t, response, 1)
		assert.Equal(t, "project-1", response[0].ProjectID)

		// Explicitly assert that no raw token field is present
		var generic []map[string]interface{}
		err = json.NewDecoder(bytes.NewReader(raw)).Decode(&generic)
		require.NoError(t, err)
		for _, item := range generic {
			if _, exists := item["token"]; exists {
				t.Fatalf("response item unexpectedly contains raw token field: %v", item)
			}
		}
	})

	t.Run("Invalid_Method", func(t *testing.T) {
		req := httptest.NewRequest("PUT", "/manage/tokens", nil)
		req.Header.Set("Authorization", "Bearer test_management_token")
		w := httptest.NewRecorder()

		server.handleTokens(w, req)

		assert.Equal(t, http.StatusMethodNotAllowed, w.Code)
	})
}

func TestHandleTokens_ProjectInactiveForbidden(t *testing.T) {
	server, _, projectStore := setupServerAndMocks(t)

	// Project exists but is inactive
	inactive := proxy.Project{ID: "pid-inactive", Name: "Inactive", IsActive: false}
	projectStore.On("GetProjectByID", mock.Anything, "pid-inactive").Return(inactive, nil)

	body, _ := json.Marshal(map[string]interface{}{"project_id": "pid-inactive", "duration_minutes": 5})
	req := httptest.NewRequest("POST", "/manage/tokens", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer test_management_token")
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	server.handleTokens(w, req)

	assert.Equal(t, http.StatusForbidden, w.Code)
	assert.Contains(t, w.Body.String(), "project_inactive")
}

func TestGetRequestID(t *testing.T) {
	// With request ID in context using new logging helpers
	ctx := logging.WithRequestID(context.Background(), "test-id")
	id := getRequestID(ctx)
	assert.Equal(t, "test-id", id)

	// Without request ID
	id = getRequestID(context.Background())
	assert.NotEmpty(t, id) // Should generate UUID
}

func TestGenerateUUID(t *testing.T) {
	// Ensure request IDs are generated when missing
	id := getRequestID(context.Background())
	assert.NotEmpty(t, id)
}

func TestInitializeComponentsAndStart(t *testing.T) {
	server, _, _ := setupServerAndMocks(t)

	// Create a test HTTP server that will be automatically closed
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	// Create a temporary API config file
	tempFile, err := os.CreateTemp("", "api_config_*.yaml")
	require.NoError(t, err)
	defer func() { require.NoError(t, os.Remove(tempFile.Name())) }()

	// Write a simple valid API config
	configData := `
defaultAPI: test
apis:
  test:
    baseURL: https://api.test.com
    allowedEndpoints:
      - /v1/test
    allowedMethods:
      - GET
      - POST
    timeouts:
      request: 30s
      responseHeader: 30s
      idleConnection: 90s
      flushInterval: 100ms
    connection:
      maxIdleConns: 100
      maxIdleConnsPerHost: 20
`
	_, err = tempFile.WriteString(configData)
	require.NoError(t, err)
	require.NoError(t, tempFile.Close())

	// Update the server config to use the temp file
	server.config.APIConfigPath = tempFile.Name()

	// Test initializing components
	err = server.initializeComponents()
	assert.NoError(t, err)

	// Create a minimal server for Shutdown testing
	minServer := &http.Server{
		Addr:    ":0",
		Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}),
	}
	server.server = minServer

	// Create context with timeout for shutdown
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	// Because our server isn't really running, shutdown will error
	// but we're just testing that the method is called correctly
	_ = server.Shutdown(ctx)
}

func TestHandleListProjects_Error(t *testing.T) {
	server, _, projectStore := setupServerAndMocks(t)
	projectStore.On("ListProjects", mock.Anything).Return([]proxy.Project{}, errors.New("db error"))

	req := httptest.NewRequest("GET", "/manage/projects", nil)
	req.Header.Set("Authorization", "Bearer test_management_token")
	w := httptest.NewRecorder()

	// Use new logging context helpers
	ctx := logging.WithRequestID(context.Background(), "test-id")
	req = req.WithContext(ctx)

	server.handleListProjects(w, req)
	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestHandleCreateProject_InvalidBody(t *testing.T) {
	server, _, _ := setupServerAndMocks(t)
	badBody := strings.NewReader("not-json")
	req := httptest.NewRequest("POST", "/manage/projects", badBody)
	req.Header.Set("Authorization", "Bearer test_management_token")
	w := httptest.NewRecorder()

	server.handleCreateProject(w, req)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestHandleCreateProject_MissingFields(t *testing.T) {
	server, _, _ := setupServerAndMocks(t)
	body, _ := json.Marshal(map[string]string{"name": ""})
	req := httptest.NewRequest("POST", "/manage/projects", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer test_management_token")
	w := httptest.NewRecorder()

	server.handleCreateProject(w, req)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestHandleCreateProject_StoreError(t *testing.T) {
	server, _, projectStore := setupServerAndMocks(t)
	projectStore.On("CreateProject", mock.Anything, mock.AnythingOfType("proxy.Project")).Return(errors.New("db error"))
	body, _ := json.Marshal(map[string]string{"name": "foo", "openai_api_key": "bar"})
	req := httptest.NewRequest("POST", "/manage/projects", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer test_management_token")
	w := httptest.NewRecorder()

	server.handleCreateProject(w, req)
	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestHandleGetProject_InvalidID(t *testing.T) {
	server, _, _ := setupServerAndMocks(t)
	req := httptest.NewRequest("GET", "/manage/projects/", nil)
	w := httptest.NewRecorder()
	server.handleGetProject(w, req)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestHandleGetProject_NotFound(t *testing.T) {
	server, _, projectStore := setupServerAndMocks(t)
	projectStore.On("GetProjectByID", mock.Anything, "notfound").Return(proxy.Project{}, errors.New("not found"))
	req := httptest.NewRequest("GET", "/manage/projects/notfound", nil)
	w := httptest.NewRecorder()
	server.handleGetProject(w, req)
	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestHandleUpdateProject_InvalidID(t *testing.T) {
	server, _, _ := setupServerAndMocks(t)
	req := httptest.NewRequest("PATCH", "/manage/projects/", nil)
	w := httptest.NewRecorder()
	server.handleUpdateProject(w, req)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestHandleUpdateProject_InvalidBody(t *testing.T) {
	server, _, _ := setupServerAndMocks(t)
	req := httptest.NewRequest("PATCH", "/manage/projects/id", strings.NewReader("not-json"))
	w := httptest.NewRecorder()
	server.handleUpdateProject(w, req)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestHandleUpdateProject_NotFound(t *testing.T) {
	server, _, projectStore := setupServerAndMocks(t)
	projectStore.On("GetProjectByID", mock.Anything, "notfound").Return(proxy.Project{}, errors.New("not found"))
	req := httptest.NewRequest("PATCH", "/manage/projects/notfound", bytes.NewReader([]byte(`{"name":"foo"}`)))
	w := httptest.NewRecorder()
	server.handleUpdateProject(w, req)
	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestHandleUpdateProject_StoreError(t *testing.T) {
	server, _, projectStore := setupServerAndMocks(t)
	projectStore.On("GetProjectByID", mock.Anything, "id").Return(proxy.Project{ID: "id"}, nil)
	projectStore.On("UpdateProject", mock.Anything, mock.AnythingOfType("proxy.Project")).Return(errors.New("db error"))
	req := httptest.NewRequest("PATCH", "/manage/projects/id", bytes.NewReader([]byte(`{"name":"foo"}`)))
	w := httptest.NewRecorder()
	server.handleUpdateProject(w, req)
	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestHandleDeleteProject_InvalidID(t *testing.T) {
	server, _, _ := setupServerAndMocks(t)
	req := httptest.NewRequest("DELETE", "/manage/projects/", nil)
	w := httptest.NewRecorder()
	server.handleDeleteProject(w, req)
	assert.Equal(t, http.StatusMethodNotAllowed, w.Code)
}

func TestHandleDeleteProject_StoreError(t *testing.T) {
	server, _, _ := setupServerAndMocks(t)
	req := httptest.NewRequest("DELETE", "/manage/projects/id", nil)
	w := httptest.NewRecorder()
	server.handleDeleteProject(w, req)
	assert.Equal(t, http.StatusMethodNotAllowed, w.Code)
}

func TestHandleTokens_InvalidMethod(t *testing.T) {
	server, _, _ := setupServerAndMocks(t)
	req := httptest.NewRequest("PUT", "/manage/tokens", nil)
	w := httptest.NewRecorder()
	server.handleTokens(w, req)
	assert.Equal(t, http.StatusMethodNotAllowed, w.Code)
}

func TestHandleTokens_InvalidBody(t *testing.T) {
	server, _, _ := setupServerAndMocks(t)
	badBody := strings.NewReader("not-json")
	req := httptest.NewRequest("POST", "/manage/tokens", badBody)
	req.Header.Set("Authorization", "Bearer test_management_token")
	w := httptest.NewRecorder()

	server.handleTokens(w, req)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestHandleTokens_MissingFields(t *testing.T) {
	server, _, _ := setupServerAndMocks(t)
	body, _ := json.Marshal(map[string]string{"project_id": ""})
	req := httptest.NewRequest("POST", "/manage/tokens", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer test_management_token")
	w := httptest.NewRecorder()

	server.handleTokens(w, req)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestHandleTokens_ProjectNotFound(t *testing.T) {
	server, _, projectStore := setupServerAndMocks(t)
	projectStore.On("GetProjectByID", mock.Anything, "notfound").Return(proxy.Project{}, errors.New("not found"))
	body, _ := json.Marshal(map[string]interface{}{"project_id": "notfound", "duration_minutes": 1})
	req := httptest.NewRequest("POST", "/manage/tokens", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer test_management_token")
	w := httptest.NewRecorder()

	server.handleTokens(w, req)
	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestHandleTokens_TokenStoreError(t *testing.T) {
	server, tokenStore, projectStore := setupServerAndMocks(t)
	projectStore.On("GetProjectByID", mock.Anything, "pid").Return(proxy.Project{ID: "pid", IsActive: true}, nil)
	tokenStore.On("CreateToken", mock.Anything, mock.AnythingOfType("token.TokenData")).Return(errors.New("db error"))
	body, _ := json.Marshal(map[string]interface{}{"project_id": "pid", "duration_minutes": 1})
	req := httptest.NewRequest("POST", "/manage/tokens", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer test_management_token")
	w := httptest.NewRecorder()

	server.handleTokens(w, req)
	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestHandleTokens_ListTokensError(t *testing.T) {
	server, tokenStore, _ := setupServerAndMocks(t)
	tokenStore.On("ListTokens", mock.Anything).Return([]token.TokenData{}, errors.New("db error"))

	req := httptest.NewRequest("GET", "/manage/tokens", nil)
	req.Header.Set("Authorization", "Bearer test_management_token")
	w := httptest.NewRecorder()

	server.handleTokens(w, req)
	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestHandleTokens_GetTokensByProjectIDError(t *testing.T) {
	server, tokenStore, _ := setupServerAndMocks(t)
	tokenStore.On("GetTokensByProjectID", mock.Anything, "pid").Return(nil, errors.New("db error"))
	tokenStore.On("ListTokens", mock.Anything).Return([]token.TokenData{}, errors.New("db error"))

	req := httptest.NewRequest("GET", "/manage/tokens?project_id=pid", nil)
	req.Header.Set("Authorization", "Bearer test_management_token")
	w := httptest.NewRecorder()

	server.handleTokens(w, req)
	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestHandleTokenByID(t *testing.T) {
	server, tokenStore, _ := setupServerAndMocks(t)

	testToken := token.TokenData{
		Token:        "sk-test123456789",
		ProjectID:    "project-1",
		IsActive:     true,
		MaxRequests:  intPtr(1000),
		RequestCount: 100,
		CreatedAt:    time.Now().UTC(),
		ExpiresAt:    timePtr(time.Now().Add(24 * time.Hour).UTC()),
	}

	// Setup mocks
	tokenStore.On("GetTokenByID", mock.Anything, "sk-test123456789").Return(testToken, nil)
	tokenStore.On("GetTokenByID", mock.Anything, "sk-nonexistent").Return(token.TokenData{}, errors.New("not found"))
	tokenStore.On("UpdateToken", mock.Anything, mock.AnythingOfType("token.TokenData")).Return(nil)

	t.Run("GET_Token_Success", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/manage/tokens/sk-test123456789", nil)
		req.Header.Set("Authorization", "Bearer test_management_token")
		w := httptest.NewRecorder()

		server.handleTokenByID(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		assert.Equal(t, "application/json", w.Header().Get("Content-Type"))

		// Test both structured response and raw JSON to ensure no token field
		raw := w.Body.Bytes()

		var response TokenListResponse
		err := json.NewDecoder(bytes.NewReader(raw)).Decode(&response)
		require.NoError(t, err)
		assert.Equal(t, testToken.ProjectID, response.ProjectID)
		assert.Equal(t, testToken.IsActive, response.IsActive)
		assert.Equal(t, testToken.MaxRequests, response.MaxRequests)
		assert.Equal(t, testToken.RequestCount, response.RequestCount)

		// Security check: ensure no raw token field is present
		var generic map[string]interface{}
		err = json.NewDecoder(bytes.NewReader(raw)).Decode(&generic)
		require.NoError(t, err)
		if _, exists := generic["token"]; exists {
			t.Fatalf("GET /manage/tokens/{id} response unexpectedly contains raw token field: %v", generic)
		}
	})

	t.Run("GET_Token_NotFound", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/manage/tokens/sk-nonexistent", nil)
		req.Header.Set("Authorization", "Bearer test_management_token")
		w := httptest.NewRecorder()

		server.handleTokenByID(w, req)

		assert.Equal(t, http.StatusNotFound, w.Code)
		assert.Contains(t, w.Body.String(), "token not found")
	})

	t.Run("PATCH_Token_Success", func(t *testing.T) {
		reqBody := map[string]interface{}{
			"is_active":    false,
			"max_requests": 2000,
		}
		body, _ := json.Marshal(reqBody)
		req := httptest.NewRequest("PATCH", "/manage/tokens/sk-test123456789", bytes.NewReader(body))
		req.Header.Set("Authorization", "Bearer test_management_token")
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		server.handleTokenByID(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		assert.Equal(t, "application/json", w.Header().Get("Content-Type"))

		var response TokenListResponse
		err := json.NewDecoder(w.Body).Decode(&response)
		require.NoError(t, err)
		assert.Equal(t, false, response.IsActive)
		assert.Equal(t, intPtr(2000), response.MaxRequests)
	})

	t.Run("DELETE_Token_Success", func(t *testing.T) {
		// Mock revocation success
		tokenStore.On("RevokeToken", mock.Anything, "sk-test123456789").Return(nil)

		req := httptest.NewRequest("DELETE", "/manage/tokens/sk-test123456789", nil)
		req.Header.Set("Authorization", "Bearer test_management_token")
		w := httptest.NewRecorder()

		server.handleTokenByID(w, req)

		assert.Equal(t, http.StatusNoContent, w.Code)
	})

	t.Run("Invalid_Method", func(t *testing.T) {
		req := httptest.NewRequest("PUT", "/manage/tokens/sk-test123456789", nil)
		req.Header.Set("Authorization", "Bearer test_management_token")
		w := httptest.NewRecorder()

		server.handleTokenByID(w, req)

		assert.Equal(t, http.StatusMethodNotAllowed, w.Code)
	})

	t.Run("Invalid_TokenID", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/manage/tokens/", nil)
		req.Header.Set("Authorization", "Bearer test_management_token")
		w := httptest.NewRecorder()

		server.handleTokenByID(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
		assert.Contains(t, w.Body.String(), "token ID is required")
	})

}

func TestHandleGetToken(t *testing.T) {
	server, tokenStore, _ := setupServerAndMocks(t)

	testToken := token.TokenData{
		Token:        "sk-test123456789",
		ProjectID:    "project-1",
		IsActive:     true,
		MaxRequests:  intPtr(1000),
		RequestCount: 100,
		CreatedAt:    time.Now().UTC(),
		ExpiresAt:    timePtr(time.Now().Add(24 * time.Hour).UTC()),
	}

	t.Run("Success", func(t *testing.T) {
		tokenStore.On("GetTokenByID", mock.Anything, "sk-test123456789").Return(testToken, nil).Once()

		req := httptest.NewRequest("GET", "/manage/tokens/sk-test123456789", nil)
		req.Header.Set("Authorization", "Bearer test_management_token")
		w := httptest.NewRecorder()

		server.handleGetToken(w, req, "sk-test123456789")

		assert.Equal(t, http.StatusOK, w.Code)
		assert.Equal(t, "application/json", w.Header().Get("Content-Type"))

		var response TokenListResponse
		err := json.NewDecoder(w.Body).Decode(&response)
		require.NoError(t, err)
		assert.Equal(t, testToken.ProjectID, response.ProjectID)
		assert.Equal(t, testToken.IsActive, response.IsActive)
	})

	t.Run("TokenNotFound", func(t *testing.T) {
		tokenStore.On("GetTokenByID", mock.Anything, "sk-nonexistent").Return(token.TokenData{}, errors.New("not found")).Once()

		req := httptest.NewRequest("GET", "/manage/tokens/sk-nonexistent", nil)
		req.Header.Set("Authorization", "Bearer test_management_token")
		w := httptest.NewRecorder()

		server.handleGetToken(w, req, "sk-nonexistent")

		assert.Equal(t, http.StatusNotFound, w.Code)
		assert.Contains(t, w.Body.String(), "token not found")
	})
}

func TestHandleUpdateToken(t *testing.T) {
	server, tokenStore, _ := setupServerAndMocks(t)

	testToken := token.TokenData{
		Token:        "sk-test123456789",
		ProjectID:    "project-1",
		IsActive:     true,
		MaxRequests:  intPtr(1000),
		RequestCount: 100,
		CreatedAt:    time.Now().UTC(),
		ExpiresAt:    timePtr(time.Now().Add(24 * time.Hour).UTC()),
	}

	t.Run("Success_IsActive", func(t *testing.T) {
		tokenStore.On("GetTokenByID", mock.Anything, "sk-test123456789").Return(testToken, nil).Once()
		tokenStore.On("UpdateToken", mock.Anything, mock.AnythingOfType("token.TokenData")).Return(nil).Once()

		reqBody := map[string]interface{}{
			"is_active": false,
		}
		body, _ := json.Marshal(reqBody)
		req := httptest.NewRequest("PATCH", "/manage/tokens/sk-test123456789", bytes.NewReader(body))
		req.Header.Set("Authorization", "Bearer test_management_token")
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		server.handleUpdateToken(w, req, "sk-test123456789")

		assert.Equal(t, http.StatusOK, w.Code)
		assert.Equal(t, "application/json", w.Header().Get("Content-Type"))

		var response TokenListResponse
		err := json.NewDecoder(w.Body).Decode(&response)
		require.NoError(t, err)
		assert.Equal(t, false, response.IsActive)
	})

	t.Run("Success_MaxRequests", func(t *testing.T) {
		tokenStore.On("GetTokenByID", mock.Anything, "sk-test123456789").Return(testToken, nil).Once()
		tokenStore.On("UpdateToken", mock.Anything, mock.AnythingOfType("token.TokenData")).Return(nil).Once()

		reqBody := map[string]interface{}{
			"max_requests": 2000,
		}
		body, _ := json.Marshal(reqBody)
		req := httptest.NewRequest("PATCH", "/manage/tokens/sk-test123456789", bytes.NewReader(body))
		req.Header.Set("Authorization", "Bearer test_management_token")
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		server.handleUpdateToken(w, req, "sk-test123456789")

		assert.Equal(t, http.StatusOK, w.Code)

		var response TokenListResponse
		err := json.NewDecoder(w.Body).Decode(&response)
		require.NoError(t, err)
		assert.Equal(t, intPtr(2000), response.MaxRequests)
	})

	t.Run("InvalidRequestBody", func(t *testing.T) {
		req := httptest.NewRequest("PATCH", "/manage/tokens/sk-test123456789", strings.NewReader("invalid-json"))
		req.Header.Set("Authorization", "Bearer test_management_token")
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		server.handleUpdateToken(w, req, "sk-test123456789")

		assert.Equal(t, http.StatusBadRequest, w.Code)
		assert.Contains(t, w.Body.String(), "invalid request body")
	})

	t.Run("TokenNotFound", func(t *testing.T) {
		tokenStore.On("GetTokenByID", mock.Anything, "sk-nonexistent").Return(token.TokenData{}, errors.New("not found")).Once()

		reqBody := map[string]interface{}{
			"is_active": false,
		}
		body, _ := json.Marshal(reqBody)
		req := httptest.NewRequest("PATCH", "/manage/tokens/sk-nonexistent", bytes.NewReader(body))
		req.Header.Set("Authorization", "Bearer test_management_token")
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		server.handleUpdateToken(w, req, "sk-nonexistent")

		assert.Equal(t, http.StatusNotFound, w.Code)
		assert.Contains(t, w.Body.String(), "token not found")
	})

	t.Run("NoFieldsToUpdate", func(t *testing.T) {
		tokenStore.On("GetTokenByID", mock.Anything, "sk-no-fields").Return(testToken, nil).Once()

		reqBody := map[string]interface{}{}
		body, _ := json.Marshal(reqBody)
		req := httptest.NewRequest("PATCH", "/manage/tokens/sk-no-fields", bytes.NewReader(body))
		req.Header.Set("Authorization", "Bearer test_management_token")
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		server.handleUpdateToken(w, req, "sk-no-fields")

		assert.Equal(t, http.StatusBadRequest, w.Code)
		assert.Contains(t, w.Body.String(), "no fields to update")
	})

	t.Run("UpdateStorageError", func(t *testing.T) {
		tokenStore.On("GetTokenByID", mock.Anything, "sk-storage-error").Return(testToken, nil).Once()
		tokenStore.On("UpdateToken", mock.Anything, mock.AnythingOfType("token.TokenData")).Return(errors.New("storage error")).Once()

		reqBody := map[string]interface{}{
			"is_active": false,
		}
		body, _ := json.Marshal(reqBody)
		req := httptest.NewRequest("PATCH", "/manage/tokens/sk-storage-error", bytes.NewReader(body))
		req.Header.Set("Authorization", "Bearer test_management_token")
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		server.handleUpdateToken(w, req, "sk-storage-error")

		assert.Equal(t, http.StatusInternalServerError, w.Code)
		assert.Contains(t, w.Body.String(), "failed to update token")
	})
}

func TestHandleRevokeToken(t *testing.T) {
	server, tokenStore, _ := setupServerAndMocks(t)

	testToken := token.TokenData{
		Token:        "sk-test123456789",
		ProjectID:    "project-1",
		IsActive:     true,
		MaxRequests:  intPtr(1000),
		RequestCount: 100,
		CreatedAt:    time.Now().UTC(),
		ExpiresAt:    timePtr(time.Now().Add(24 * time.Hour).UTC()),
	}

	inactiveToken := token.TokenData{
		Token:        "sk-inactive123",
		ProjectID:    "project-1",
		IsActive:     false,
		MaxRequests:  intPtr(1000),
		RequestCount: 100,
		CreatedAt:    time.Now().UTC(),
		ExpiresAt:    timePtr(time.Now().Add(24 * time.Hour).UTC()),
	}

	t.Run("Success", func(t *testing.T) {
		tokenStore.On("GetTokenByID", mock.Anything, "sk-revoke-success").Return(testToken, nil).Once()
		tokenStore.On("UpdateToken", mock.Anything, mock.AnythingOfType("token.TokenData")).Return(nil).Once()

		req := httptest.NewRequest("DELETE", "/manage/tokens/sk-revoke-success", nil)
		req.Header.Set("Authorization", "Bearer test_management_token")
		w := httptest.NewRecorder()

		server.handleRevokeToken(w, req, "sk-revoke-success")

		assert.Equal(t, http.StatusNoContent, w.Code)
	})

	t.Run("TokenNotFound", func(t *testing.T) {
		tokenStore.On("GetTokenByID", mock.Anything, "sk-nonexistent").Return(token.TokenData{}, errors.New("not found")).Once()

		req := httptest.NewRequest("DELETE", "/manage/tokens/sk-nonexistent", nil)
		req.Header.Set("Authorization", "Bearer test_management_token")
		w := httptest.NewRecorder()

		server.handleRevokeToken(w, req, "sk-nonexistent")

		assert.Equal(t, http.StatusNotFound, w.Code)
		assert.Contains(t, w.Body.String(), "token not found")
	})

	t.Run("AlreadyRevoked", func(t *testing.T) {
		tokenStore.On("GetTokenByID", mock.Anything, "sk-inactive123").Return(inactiveToken, nil).Once()

		req := httptest.NewRequest("DELETE", "/manage/tokens/sk-inactive123", nil)
		req.Header.Set("Authorization", "Bearer test_management_token")
		w := httptest.NewRecorder()

		server.handleRevokeToken(w, req, "sk-inactive123")

		assert.Equal(t, http.StatusNoContent, w.Code)
	})

	t.Run("StorageError", func(t *testing.T) {
		tokenStore.On("GetTokenByID", mock.Anything, "sk-storage-error").Return(testToken, nil).Once()
		tokenStore.On("UpdateToken", mock.Anything, mock.AnythingOfType("token.TokenData")).Return(errors.New("storage error")).Once()

		req := httptest.NewRequest("DELETE", "/manage/tokens/sk-storage-error", nil)
		req.Header.Set("Authorization", "Bearer test_management_token")
		w := httptest.NewRecorder()

		server.handleRevokeToken(w, req, "sk-storage-error")

		assert.Equal(t, http.StatusInternalServerError, w.Code)
		assert.Contains(t, w.Body.String(), "failed to revoke token")
	})
}

func TestHandleBulkRevokeProjectTokens(t *testing.T) {
	server, tokenStore, projectStore := setupServerAndMocks(t)

	testProject := proxy.Project{
		ID:   "project-1",
		Name: "Test Project",
	}

	t.Run("Success", func(t *testing.T) {
		testTokens := []token.TokenData{
			{Token: "sk-token1", ProjectID: "project-1", IsActive: true},
			{Token: "sk-token2", ProjectID: "project-1", IsActive: false}, // Already revoked
		}

		projectStore.On("GetProjectByID", mock.Anything, "project-1").Return(testProject, nil).Once()
		tokenStore.On("GetTokensByProjectID", mock.Anything, "project-1").Return(testTokens, nil).Once()
		tokenStore.On("UpdateToken", mock.Anything, mock.AnythingOfType("token.TokenData")).Return(nil).Once()

		req := httptest.NewRequest("POST", "/manage/projects/project-1/tokens/revoke", nil)
		req.Header.Set("Authorization", "Bearer test_management_token")
		w := httptest.NewRecorder()

		server.handleBulkRevokeProjectTokens(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		assert.Equal(t, "application/json", w.Header().Get("Content-Type"))

		var response map[string]interface{}
		err := json.NewDecoder(w.Body).Decode(&response)
		require.NoError(t, err)
		assert.Contains(t, response, "revoked_count")
		assert.Contains(t, response, "already_revoked_count")
		assert.Contains(t, response, "total_tokens")
		assert.Equal(t, float64(1), response["revoked_count"])
		assert.Equal(t, float64(1), response["already_revoked_count"])
		assert.Equal(t, float64(2), response["total_tokens"])
	})

	t.Run("ProjectNotFound", func(t *testing.T) {
		projectStore.On("GetProjectByID", mock.Anything, "nonexistent").Return(proxy.Project{}, errors.New("not found")).Once()

		req := httptest.NewRequest("POST", "/manage/projects/nonexistent/tokens/revoke", nil)
		req.Header.Set("Authorization", "Bearer test_management_token")
		w := httptest.NewRecorder()

		server.handleBulkRevokeProjectTokens(w, req)

		assert.Equal(t, http.StatusNotFound, w.Code)
		assert.Contains(t, w.Body.String(), "project not found")
	})

	t.Run("GetTokensError", func(t *testing.T) {
		projectStore.On("GetProjectByID", mock.Anything, "project-1").Return(testProject, nil).Once()
		tokenStore.On("GetTokensByProjectID", mock.Anything, "project-1").Return([]token.TokenData{}, errors.New("storage error")).Once()

		req := httptest.NewRequest("POST", "/manage/projects/project-1/tokens/revoke", nil)
		req.Header.Set("Authorization", "Bearer test_management_token")
		w := httptest.NewRecorder()

		server.handleBulkRevokeProjectTokens(w, req)

		assert.Equal(t, http.StatusInternalServerError, w.Code)
		assert.Contains(t, w.Body.String(), "failed to get project tokens")
	})
}

// Helper function to create int pointers
func intPtr(i int) *int {
	return &i
}

// Helper function to create time pointers
func timePtr(t time.Time) *time.Time {
	return &t
}

func TestHandleTokenByID_SecurityValidation(t *testing.T) {
	server, tokenStore, _ := setupServerAndMocks(t)

	testToken := token.TokenData{
		Token:        "sk-test123456789",
		ProjectID:    "project-1",
		IsActive:     true,
		MaxRequests:  intPtr(1000),
		RequestCount: 100,
		CreatedAt:    time.Now().UTC(),
		ExpiresAt:    timePtr(time.Now().Add(24 * time.Hour).UTC()),
	}

	tokenStore.On("GetTokenByID", mock.Anything, "sk-test123456789").Return(testToken, nil)

	t.Run("GET_Token_Never_Returns_Raw_Token", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/manage/tokens/sk-test123456789", nil)
		req.Header.Set("Authorization", "Bearer test_management_token")
		w := httptest.NewRecorder()

		server.handleTokenByID(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		// Parse response as generic map to check all fields
		raw := w.Body.Bytes()
		var responseMap map[string]interface{}
		err := json.NewDecoder(bytes.NewReader(raw)).Decode(&responseMap)
		require.NoError(t, err)

		// Ensure no raw token field exists
		_, hasToken := responseMap["token"]
		assert.False(t, hasToken, "Response must not contain raw token field")

		// Ensure expected fields are present
		assert.Contains(t, responseMap, "project_id")
		assert.Contains(t, responseMap, "is_active")
		assert.Contains(t, responseMap, "request_count")
		assert.Equal(t, "project-1", responseMap["project_id"])
		assert.Equal(t, true, responseMap["is_active"])
		assert.Equal(t, float64(100), responseMap["request_count"])
	})
}

func TestHandleBulkRevokeProjectTokens_EdgeCases(t *testing.T) {
	server, tokenStore, projectStore := setupServerAndMocks(t)

	testProject := proxy.Project{
		ID:   "project-1",
		Name: "Test Project",
	}

	t.Run("Invalid_Path_Format", func(t *testing.T) {
		req := httptest.NewRequest("POST", "/manage/projects/project-1/invalid/path", nil)
		req.Header.Set("Authorization", "Bearer test_management_token")
		w := httptest.NewRecorder()

		server.handleBulkRevokeProjectTokens(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
		assert.Contains(t, w.Body.String(), "invalid path")
	})

	t.Run("Empty_Project_ID", func(t *testing.T) {
		req := httptest.NewRequest("POST", "/manage/projects//tokens/revoke", nil)
		req.Header.Set("Authorization", "Bearer test_management_token")
		w := httptest.NewRecorder()

		server.handleBulkRevokeProjectTokens(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
		assert.Contains(t, w.Body.String(), "invalid path")
	})

	t.Run("Partial_Revocation_Failures", func(t *testing.T) {
		// Test case where some tokens fail to revoke
		testTokens := []token.TokenData{
			{Token: "sk-token1", ProjectID: "project-1", IsActive: true},
			{Token: "sk-token2", ProjectID: "project-1", IsActive: true},
			{Token: "sk-token3", ProjectID: "project-1", IsActive: false}, // Already revoked
		}

		projectStore.On("GetProjectByID", mock.Anything, "project-1").Return(testProject, nil).Once()
		tokenStore.On("GetTokensByProjectID", mock.Anything, "project-1").Return(testTokens, nil).Once()

		// First token update succeeds, second fails
		tokenStore.On("UpdateToken", mock.Anything, mock.MatchedBy(func(td token.TokenData) bool {
			return td.Token == "sk-token1"
		})).Return(nil).Once()

		tokenStore.On("UpdateToken", mock.Anything, mock.MatchedBy(func(td token.TokenData) bool {
			return td.Token == "sk-token2"
		})).Return(errors.New("storage error")).Once()

		req := httptest.NewRequest("POST", "/manage/projects/project-1/tokens/revoke", nil)
		req.Header.Set("Authorization", "Bearer test_management_token")
		w := httptest.NewRecorder()

		server.handleBulkRevokeProjectTokens(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var response map[string]interface{}
		err := json.NewDecoder(w.Body).Decode(&response)
		require.NoError(t, err)
		assert.Equal(t, float64(1), response["revoked_count"])
		assert.Equal(t, float64(1), response["already_revoked_count"])
		assert.Equal(t, float64(3), response["total_tokens"])
		assert.Contains(t, response, "failed_count")
		assert.Equal(t, float64(1), response["failed_count"])
	})
}

func TestHandleUpdateProject_WithTokenRevocation(t *testing.T) {
	server, tokenStore, projectStore := setupServerAndMocks(t)

	testProject := proxy.Project{
		ID:        "project-1",
		Name:      "Test Project",
		IsActive:  true,
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
	}

	testTokens := []token.TokenData{
		{Token: "sk-active1", ProjectID: "project-1", IsActive: true},
		{Token: "sk-active2", ProjectID: "project-1", IsActive: true},
		{Token: "sk-inactive", ProjectID: "project-1", IsActive: false},
	}

	projectStore.On("GetProjectByID", mock.Anything, "project-1").Return(testProject, nil)
	projectStore.On("UpdateProject", mock.Anything, mock.AnythingOfType("proxy.Project")).Return(nil)
	tokenStore.On("GetTokensByProjectID", mock.Anything, "project-1").Return(testTokens, nil)
	tokenStore.On("UpdateToken", mock.Anything, mock.AnythingOfType("token.TokenData")).Return(nil)

	t.Run("Deactivate_Project_With_Token_Revocation", func(t *testing.T) {
		reqBody := map[string]interface{}{
			"is_active":     false,
			"revoke_tokens": true,
		}
		body, _ := json.Marshal(reqBody)
		req := httptest.NewRequest("PATCH", "/manage/projects/project-1", bytes.NewReader(body))
		req.Header.Set("Authorization", "Bearer test_management_token")
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		server.handleUpdateProject(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var response proxy.Project
		err := json.NewDecoder(w.Body).Decode(&response)
		require.NoError(t, err)
		assert.False(t, response.IsActive)
		assert.NotNil(t, response.DeactivatedAt)
	})

	t.Run("Reactivate_Project_Clears_Deactivated_Timestamp", func(t *testing.T) {
		// Set up project as deactivated
		deactivatedProject := testProject
		now := time.Now().UTC()
		deactivatedProject.IsActive = false
		deactivatedProject.DeactivatedAt = &now

		projectStore.On("GetProjectByID", mock.Anything, "project-reactivate").Return(deactivatedProject, nil)
		projectStore.On("UpdateProject", mock.Anything, mock.AnythingOfType("proxy.Project")).Return(nil)

		reqBody := map[string]interface{}{
			"is_active": true,
		}
		body, _ := json.Marshal(reqBody)
		req := httptest.NewRequest("PATCH", "/manage/projects/project-reactivate", bytes.NewReader(body))
		req.Header.Set("Authorization", "Bearer test_management_token")
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		server.handleUpdateProject(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var response proxy.Project
		err := json.NewDecoder(w.Body).Decode(&response)
		require.NoError(t, err)
		assert.True(t, response.IsActive)
		assert.Nil(t, response.DeactivatedAt)
	})
}
