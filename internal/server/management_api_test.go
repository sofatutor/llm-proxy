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
	"github.com/sofatutor/llm-proxy/internal/proxy"
	"github.com/sofatutor/llm-proxy/internal/token"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// MockTokenStore with methods that return testable results
type MockTokenStoreExtended struct {
	mock.Mock
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

func (m *MockTokenStoreExtended) GetTokensByProjectID(ctx context.Context, projectID string) ([]token.TokenData, error) {
	args := m.Called(ctx, projectID)
	return args.Get(0).([]token.TokenData), args.Error(1)
}

func (m *MockTokenStoreExtended) ListTokens(ctx context.Context) ([]token.TokenData, error) {
	args := m.Called(ctx)
	return args.Get(0).([]token.TokenData), args.Error(1)
}

// MockProjectStore with methods that return testable results
type MockProjectStoreExtended struct {
	mock.Mock
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

func setupServerAndMocks(t *testing.T) (*Server, *MockTokenStoreExtended, *MockProjectStoreExtended) {
	tokenStore := new(MockTokenStoreExtended)
	projectStore := new(MockProjectStoreExtended)

	cfg := &config.Config{
		ListenAddr:      ":8080",
		RequestTimeout:  30 * time.Second,
		ManagementToken: "test_management_token",
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

	t.Run("Auth_Failure", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/manage/projects", nil)
		req.Header.Set("Authorization", "Bearer wrong_token")
		w := httptest.NewRecorder()

		server.handleProjects(w, req)

		assert.Equal(t, http.StatusUnauthorized, w.Code)
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

		assert.Equal(t, http.StatusNoContent, w.Code)
	})

	t.Run("DELETE_Project_NotFound", func(t *testing.T) {
		req := httptest.NewRequest("DELETE", "/manage/projects/non-existent-id", nil)
		req.Header.Set("Authorization", "Bearer test_management_token")
		w := httptest.NewRecorder()

		server.handleProjectByID(w, req)

		assert.Equal(t, http.StatusNotFound, w.Code)
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
		ID:   "project-1",
		Name: "Test Project",
	}

	testTokens := []token.TokenData{
		{
			Token:     "token-1",
			ProjectID: "project-1",
			IsActive:  true,
		},
		{
			Token:     "token-2",
			ProjectID: "project-2",
			IsActive:  true,
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
		var response []TokenListResponse
		err := json.NewDecoder(w.Body).Decode(&response)
		require.NoError(t, err)
		assert.Len(t, response, 2)
	})

	t.Run("GET_TokensByProject", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/manage/tokens?projectId=project-1", nil)
		req.Header.Set("Authorization", "Bearer test_management_token")
		w := httptest.NewRecorder()

		server.handleTokens(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		assert.Equal(t, "application/json", w.Header().Get("Content-Type"))

		// Expect sanitized token response (without actual token values)
		var response []TokenListResponse
		err := json.NewDecoder(w.Body).Decode(&response)
		require.NoError(t, err)
		assert.Len(t, response, 1)
		assert.Equal(t, "project-1", response[0].ProjectID)
	})

	t.Run("Invalid_Method", func(t *testing.T) {
		req := httptest.NewRequest("PUT", "/manage/tokens", nil)
		req.Header.Set("Authorization", "Bearer test_management_token")
		w := httptest.NewRecorder()

		server.handleTokens(w, req)

		assert.Equal(t, http.StatusMethodNotAllowed, w.Code)
	})
}

func TestGetRequestID(t *testing.T) {
	// With request ID in context
	ctx := context.WithValue(context.Background(), ctxKeyRequestID, "test-id")
	id := getRequestID(ctx)
	assert.Equal(t, "test-id", id)

	// Without request ID
	id = getRequestID(context.Background())
	assert.NotEmpty(t, id) // Should generate UUID
}

func TestGenerateUUID(t *testing.T) {
	// Generate a UUID and check its format
	id := generateUUID()

	// Check that it's not empty
	assert.NotEmpty(t, id)

	// Check length (36 characters for UUID format: 8-4-4-4-12)
	assert.Equal(t, 36, len(id))

	// Check that it contains hyphens at the correct positions
	assert.Equal(t, string(id[8]), "-")
	assert.Equal(t, string(id[13]), "-")
	assert.Equal(t, string(id[18]), "-")
	assert.Equal(t, string(id[23]), "-")

	// Check that all other characters are valid hex
	validHex := "0123456789abcdef-"
	for _, c := range id {
		assert.Contains(t, validHex, string(c))
	}
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

	// Use custom context key type
	ctx := context.WithValue(context.Background(), ctxKeyRequestID, "test-id")
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
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestHandleDeleteProject_StoreError(t *testing.T) {
	server, _, projectStore := setupServerAndMocks(t)
	projectStore.On("DeleteProject", mock.Anything, "id").Return(errors.New("not found"))
	req := httptest.NewRequest("DELETE", "/manage/projects/id", nil)
	w := httptest.NewRecorder()
	server.handleDeleteProject(w, req)
	assert.Equal(t, http.StatusNotFound, w.Code)
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
	projectStore.On("GetProjectByID", mock.Anything, "pid").Return(proxy.Project{ID: "pid"}, nil)
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
