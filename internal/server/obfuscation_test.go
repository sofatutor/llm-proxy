package server

import (
	"bytes"
	"encoding/json"
	"net/http/httptest"
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

// TestObfuscation_TokenResponses verifies that tokens are obfuscated in API responses
func TestObfuscation_TokenResponses(t *testing.T) {
	cfg := &config.Config{
		ManagementToken: "test_management_token",
		LogLevel:        "error",
		EventBusBackend: "in-memory",
	}

	tokenStore := &MockTokenStoreExtended{Mock: &mock.Mock{}}
	projectStore := &MockProjectStoreExtended{Mock: &mock.Mock{}}

	server, err := New(cfg, tokenStore, projectStore)
	require.NoError(t, err)

	fullToken := "sk-1234567890abcdefghijklmnop"
	expiresAt := time.Now().Add(24 * time.Hour)

	t.Run("GET_TokenList_ReturnsObfuscatedTokens", func(t *testing.T) {
		tokenStore.On("ListTokens", mock.Anything).Return([]token.TokenData{
			{
				Token:        fullToken,
				ProjectID:    "project-1",
				ExpiresAt:    &expiresAt,
				IsActive:     true,
				RequestCount: 10,
				CreatedAt:    time.Now(),
			},
		}, nil)

		req := httptest.NewRequest("GET", "/manage/tokens", nil)
		req.Header.Set("Authorization", "Bearer test_management_token")
		w := httptest.NewRecorder()

		server.handleTokens(w, req)

		assert.Equal(t, 200, w.Code)

		var response []TokenListResponse
		err := json.NewDecoder(w.Body).Decode(&response)
		require.NoError(t, err)
		require.Len(t, response, 1)

		// Token should be obfuscated (sk-1234****mnop format)
		assert.NotEqual(t, fullToken, response[0].TokenID, "Token should be obfuscated")
		assert.Contains(t, response[0].TokenID, "sk-", "Obfuscated token should contain prefix")
		assert.Contains(t, response[0].TokenID, "****", "Obfuscated token should contain asterisks")
		assert.True(t, strings.HasPrefix(response[0].TokenID, "sk-1234"), "Should show first 4 chars after prefix")
		assert.True(t, strings.HasSuffix(response[0].TokenID, "mnop"), "Should show last 4 chars")
	})

	t.Run("GET_TokenByID_ReturnsObfuscatedToken", func(t *testing.T) {
		tokenStore.On("GetTokenByID", mock.Anything, fullToken).Return(token.TokenData{
			Token:        fullToken,
			ProjectID:    "project-1",
			ExpiresAt:    &expiresAt,
			IsActive:     true,
			RequestCount: 5,
			CreatedAt:    time.Now(),
		}, nil)

		req := httptest.NewRequest("GET", "/manage/tokens/"+fullToken, nil)
		req.Header.Set("Authorization", "Bearer test_management_token")
		w := httptest.NewRecorder()

		server.handleTokenByID(w, req)

		assert.Equal(t, 200, w.Code)

		var response TokenListResponse
		err := json.NewDecoder(w.Body).Decode(&response)
		require.NoError(t, err)

		// Token should be obfuscated
		assert.NotEqual(t, fullToken, response.TokenID, "Token should be obfuscated")
		assert.Contains(t, response.TokenID, "sk-", "Obfuscated token should contain prefix")
		assert.Contains(t, response.TokenID, "****", "Obfuscated token should contain asterisks")
	})

	t.Run("POST_TokenCreate_ReturnsFullToken", func(t *testing.T) {
		projectStore.On("GetProjectByID", mock.Anything, "project-1").Return(proxy.Project{
			ID:           "project-1",
			Name:         "Test Project",
			OpenAIAPIKey: "sk-test-key",
			IsActive:     true,
		}, nil)
		tokenStore.On("CreateToken", mock.Anything, mock.AnythingOfType("token.TokenData")).Return(nil)

		reqBody := map[string]interface{}{
			"project_id":       "project-1",
			"duration_minutes": 1440,
		}
		body, _ := json.Marshal(reqBody)
		req := httptest.NewRequest("POST", "/manage/tokens", bytes.NewReader(body))
		req.Header.Set("Authorization", "Bearer test_management_token")
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		server.handleTokens(w, req)

		assert.Equal(t, 200, w.Code)

		var response map[string]interface{}
		err := json.NewDecoder(w.Body).Decode(&response)
		require.NoError(t, err)

		// Token should be present and NOT obfuscated (full token on creation)
		tokenValue, ok := response["token"].(string)
		require.True(t, ok, "Response should contain token field")
		assert.True(t, strings.HasPrefix(tokenValue, "sk-"), "Token should have sk- prefix")
		assert.NotContains(t, tokenValue, "****", "Created token should NOT be obfuscated")
		assert.Greater(t, len(tokenValue), 20, "Token should be full length")
	})
}

// TestObfuscation_ProjectResponses verifies that API keys are obfuscated in project API responses
func TestObfuscation_ProjectResponses(t *testing.T) {
	cfg := &config.Config{
		ManagementToken: "test_management_token",
		LogLevel:        "error",
		EventBusBackend: "in-memory",
	}

	tokenStore := &MockTokenStoreExtended{Mock: &mock.Mock{}}
	projectStore := &MockProjectStoreExtended{Mock: &mock.Mock{}}

	server, err := New(cfg, tokenStore, projectStore)
	require.NoError(t, err)

	fullAPIKey := "sk-proj-1234567890abcdefghijklmnopqrstuvwxyz"

	t.Run("GET_ProjectList_ReturnsObfuscatedAPIKeys", func(t *testing.T) {
		projectStore.On("ListProjects", mock.Anything).Return([]proxy.Project{
			{
				ID:           "project-1",
				Name:         "Test Project",
				OpenAIAPIKey: fullAPIKey,
				IsActive:     true,
				CreatedAt:    time.Now(),
				UpdatedAt:    time.Now(),
			},
		}, nil)

		req := httptest.NewRequest("GET", "/manage/projects", nil)
		req.Header.Set("Authorization", "Bearer test_management_token")
		w := httptest.NewRecorder()

		server.handleListProjects(w, req)

		assert.Equal(t, 200, w.Code)

		var response []ProjectResponse
		err := json.NewDecoder(w.Body).Decode(&response)
		require.NoError(t, err)
		require.Len(t, response, 1)

		// API key should be obfuscated
		assert.NotEqual(t, fullAPIKey, response[0].OpenAIAPIKey, "API key should be obfuscated")
		assert.Contains(t, response[0].OpenAIAPIKey, "...", "Obfuscated API key should contain ellipsis")
	})

	t.Run("GET_ProjectByID_ReturnsObfuscatedAPIKey", func(t *testing.T) {
		projectStore.On("GetProjectByID", mock.Anything, "project-1").Return(proxy.Project{
			ID:           "project-1",
			Name:         "Test Project",
			OpenAIAPIKey: fullAPIKey,
			IsActive:     true,
			CreatedAt:    time.Now(),
			UpdatedAt:    time.Now(),
		}, nil)

		req := httptest.NewRequest("GET", "/manage/projects/project-1", nil)
		req.Header.Set("Authorization", "Bearer test_management_token")
		w := httptest.NewRecorder()

		server.handleGetProject(w, req)

		assert.Equal(t, 200, w.Code)

		var response ProjectResponse
		err := json.NewDecoder(w.Body).Decode(&response)
		require.NoError(t, err)

		// API key should be obfuscated
		assert.NotEqual(t, fullAPIKey, response.OpenAIAPIKey, "API key should be obfuscated")
		assert.Contains(t, response.OpenAIAPIKey, "...", "Obfuscated API key should contain ellipsis")
	})

	t.Run("PATCH_Project_EmptyAPIKey_DoesNotUpdate", func(t *testing.T) {
		existingProject := proxy.Project{
			ID:           "project-1",
			Name:         "Test Project",
			OpenAIAPIKey: fullAPIKey,
			IsActive:     true,
			CreatedAt:    time.Now(),
			UpdatedAt:    time.Now(),
		}

		projectStore.On("GetProjectByID", mock.Anything, "project-1").Return(existingProject, nil)
		projectStore.On("UpdateProject", mock.Anything, mock.MatchedBy(func(p proxy.Project) bool {
			// Verify that API key was NOT changed when empty string is sent
			return p.OpenAIAPIKey == fullAPIKey
		})).Return(nil)

		reqBody := map[string]interface{}{
			"name":           "Updated Name",
			"openai_api_key": "", // Empty string should not update
		}
		body, _ := json.Marshal(reqBody)
		req := httptest.NewRequest("PATCH", "/manage/projects/project-1", bytes.NewReader(body))
		req.Header.Set("Authorization", "Bearer test_management_token")
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		server.handleUpdateProject(w, req)

		assert.Equal(t, 200, w.Code)

		// Verify UpdateProject was called
		projectStore.AssertCalled(t, "UpdateProject", mock.Anything, mock.MatchedBy(func(p proxy.Project) bool {
			return p.OpenAIAPIKey == fullAPIKey // Should still have original key
		}))
	})

	t.Run("PATCH_Project_WithAPIKey_DoesUpdate", func(t *testing.T) {
		existingProject := proxy.Project{
			ID:           "project-1",
			Name:         "Test Project",
			OpenAIAPIKey: fullAPIKey,
			IsActive:     true,
			CreatedAt:    time.Now(),
			UpdatedAt:    time.Now(),
		}

		newAPIKey := "sk-proj-newkey9876543210"
		projectStore.On("GetProjectByID", mock.Anything, "project-2").Return(existingProject, nil)
		projectStore.On("UpdateProject", mock.Anything, mock.MatchedBy(func(p proxy.Project) bool {
			// Verify that API key WAS changed when non-empty string is sent
			return p.OpenAIAPIKey == newAPIKey
		})).Return(nil)

		reqBody := map[string]interface{}{
			"openai_api_key": newAPIKey,
		}
		body, _ := json.Marshal(reqBody)
		req := httptest.NewRequest("PATCH", "/manage/projects/project-2", bytes.NewReader(body))
		req.Header.Set("Authorization", "Bearer test_management_token")
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		server.handleUpdateProject(w, req)

		assert.Equal(t, 200, w.Code)

		// Verify UpdateProject was called with new key
		projectStore.AssertCalled(t, "UpdateProject", mock.Anything, mock.MatchedBy(func(p proxy.Project) bool {
			return p.OpenAIAPIKey == newAPIKey
		}))
	})
}
