package admin

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestObfuscateAPIKey(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		output string
	}{
		{"short key", "abcd", "****"},
		{"medium key", "abcdefgh", "ab******"},
		{"long key", "sk-12345678ABCDEFGH", "sk-12345...EFGH"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ObfuscateAPIKey(tt.input)
			if got != tt.output {
				t.Errorf("ObfuscateAPIKey(%q) = %q, want %q", tt.input, got, tt.output)
			}
		})
	}
}

func TestObfuscateToken(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		output string
	}{
		{"short token", "abcd", "****"},
		{"medium token", "abcdefgh", "ab******"},
		{"long token", "tok-12345678ABCDEFGH", "tok-1234...EFGH"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ObfuscateToken(tt.input)
			if got != tt.output {
				t.Errorf("ObfuscateToken(%q) = %q, want %q", tt.input, got, tt.output)
			}
		})
	}
}

func TestNewAPIClient(t *testing.T) {
	baseURL := "http://localhost:1234"
	token := "test-token"
	c := NewAPIClient(baseURL, token)
	if c.baseURL != baseURL {
		t.Errorf("baseURL = %q, want %q", c.baseURL, baseURL)
	}
	if c.token != token {
		t.Errorf("token = %q, want %q", c.token, token)
	}
	if c.httpClient == nil {
		t.Error("httpClient is nil")
	}
}

func TestAPIClient_GetDashboardData(t *testing.T) {
	// Mock server that returns projects and tokens
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Check authorization header
		if auth := r.Header.Get("Authorization"); !strings.HasPrefix(auth, "Bearer ") {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}

		switch r.URL.Path {
		case "/manage/projects":
			projects := []Project{
				{ID: "1", Name: "Test Project", CreatedAt: time.Now()},
				{ID: "2", Name: "Another Project", CreatedAt: time.Now()},
			}
			if err := json.NewEncoder(w).Encode(projects); err != nil {
				t.Errorf("failed to encode projects: %v", err)
			}
		case "/manage/tokens":
			now := time.Now().Add(time.Hour) // Future time to ensure active
			expired := time.Now().Add(-time.Hour)
			lastUsed := time.Now().Add(-time.Hour)
			tokens := []Token{
				{ProjectID: "1", IsActive: true, ExpiresAt: &now, RequestCount: 10, LastUsedAt: &lastUsed},
				{ProjectID: "2", IsActive: false, ExpiresAt: &expired, RequestCount: 5},
			}
			if err := json.NewEncoder(w).Encode(tokens); err != nil {
				t.Errorf("failed to encode tokens: %v", err)
			}
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	client := NewAPIClient(server.URL, "test-token")
	ctx := context.Background()

	dataAny, err := client.GetDashboardData(ctx)
	if err != nil {
		t.Fatalf("GetDashboardData failed: %v", err)
	}
	data := dataAny.(*DashboardData)

	if data.TotalProjects != 2 {
		t.Errorf("TotalProjects = %d, want 2", data.TotalProjects)
	}
	if data.TotalTokens != 2 {
		t.Errorf("TotalTokens = %d, want 2", data.TotalTokens)
	}
	if data.ActiveTokens != 1 {
		t.Errorf("ActiveTokens = %d, want 1", data.ActiveTokens)
	}
	if data.ExpiredTokens != 1 {
		t.Errorf("ExpiredTokens = %d, want 1", data.ExpiredTokens)
	}
	if data.TotalRequests != 15 {
		t.Errorf("TotalRequests = %d, want 15", data.TotalRequests)
	}
}

func TestAPIClient_GetProjects(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		projects := []Project{
			{ID: "1", Name: "Project 1", CreatedAt: time.Now()},
			{ID: "2", Name: "Project 2", CreatedAt: time.Now()},
			{ID: "3", Name: "Project 3", CreatedAt: time.Now()},
		}
		if err := json.NewEncoder(w).Encode(projects); err != nil {
			t.Errorf("failed to encode projects: %v", err)
		}
	}))
	defer server.Close()

	client := NewAPIClient(server.URL, "test-token")
	ctx := context.Background()

	// Test pagination
	projects, pagination, err := client.GetProjects(ctx, 1, 2)
	if err != nil {
		t.Fatalf("GetProjects failed: %v", err)
	}

	if len(projects) != 2 {
		t.Errorf("len(projects) = %d, want 2", len(projects))
	}
	if pagination.TotalItems != 3 {
		t.Errorf("TotalItems = %d, want 3", pagination.TotalItems)
	}
	if pagination.TotalPages != 2 {
		t.Errorf("TotalPages = %d, want 2", pagination.TotalPages)
	}
	if !pagination.HasNext {
		t.Error("HasNext should be true")
	}
	if pagination.HasPrev {
		t.Error("HasPrev should be false")
	}

	// Test page beyond available items
	projects, _, err = client.GetProjects(ctx, 10, 2)
	if err != nil {
		t.Fatalf("GetProjects failed: %v", err)
	}
	if len(projects) != 0 {
		t.Errorf("len(projects) = %d, want 0", len(projects))
	}
}

func TestAPIClient_GetProject(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/manage/projects/1" {
			project := Project{ID: "1", Name: "Test Project", CreatedAt: time.Now()}
			if err := json.NewEncoder(w).Encode(project); err != nil {
				t.Errorf("failed to encode project: %v", err)
			}
		} else {
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	client := NewAPIClient(server.URL, "test-token")
	ctx := context.Background()

	project, err := client.GetProject(ctx, "1")
	if err != nil {
		t.Fatalf("GetProject failed: %v", err)
	}

	if project.ID != "1" {
		t.Errorf("ID = %q, want %q", project.ID, "1")
	}
	if project.Name != "Test Project" {
		t.Errorf("Name = %q, want %q", project.Name, "Test Project")
	}
}

func TestAPIClient_CreateProject(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" || r.URL.Path != "/manage/projects" {
			http.Error(w, "invalid request", http.StatusBadRequest)
			return
		}

		var req map[string]string
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "invalid JSON", http.StatusBadRequest)
			return
		}

		project := Project{
			ID:           "new-id",
			Name:         req["name"],
			OpenAIAPIKey: req["openai_api_key"],
			CreatedAt:    time.Now(),
		}
		if err := json.NewEncoder(w).Encode(project); err != nil {
			t.Errorf("failed to encode project: %v", err)
		}
	}))
	defer server.Close()

	client := NewAPIClient(server.URL, "test-token")
	ctx := context.Background()

	project, err := client.CreateProject(ctx, "New Project", "sk-test-key")
	if err != nil {
		t.Fatalf("CreateProject failed: %v", err)
	}

	if project.Name != "New Project" {
		t.Errorf("Name = %q, want %q", project.Name, "New Project")
	}
	if project.OpenAIAPIKey != "sk-test-key" {
		t.Errorf("OpenAIAPIKey = %q, want %q", project.OpenAIAPIKey, "sk-test-key")
	}
}

func TestAPIClient_UpdateProject(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "PATCH" || r.URL.Path != "/manage/projects/1" {
			http.Error(w, "invalid request", http.StatusBadRequest)
			return
		}

		var req map[string]string
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "invalid JSON", http.StatusBadRequest)
			return
		}

		project := Project{
			ID:           "1",
			Name:         req["name"],
			OpenAIAPIKey: req["openai_api_key"],
			UpdatedAt:    time.Now(),
		}
		if err := json.NewEncoder(w).Encode(project); err != nil {
			t.Errorf("failed to encode project: %v", err)
		}
	}))
	defer server.Close()

	client := NewAPIClient(server.URL, "test-token")
	ctx := context.Background()

	project, err := client.UpdateProject(ctx, "1", "Updated Name", "sk-updated-key")
	if err != nil {
		t.Fatalf("UpdateProject failed: %v", err)
	}

	if project.Name != "Updated Name" {
		t.Errorf("Name = %q, want %q", project.Name, "Updated Name")
	}
	if project.OpenAIAPIKey != "sk-updated-key" {
		t.Errorf("OpenAIAPIKey = %q, want %q", project.OpenAIAPIKey, "sk-updated-key")
	}
}

func TestAPIClient_DeleteProject(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "DELETE" || r.URL.Path != "/manage/projects/1" {
			http.Error(w, "invalid request", http.StatusBadRequest)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	client := NewAPIClient(server.URL, "test-token")
	ctx := context.Background()

	err := client.DeleteProject(ctx, "1")
	if err != nil {
		t.Fatalf("DeleteProject failed: %v", err)
	}
}

func TestAPIClient_GetTokens(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tokens := []Token{
			{ProjectID: "1", IsActive: true, RequestCount: 10},
			{ProjectID: "2", IsActive: false, RequestCount: 5},
			{ProjectID: "1", IsActive: true, RequestCount: 3},
		}
		if err := json.NewEncoder(w).Encode(tokens); err != nil {
			t.Errorf("failed to encode tokens: %v", err)
		}
	}))
	defer server.Close()

	client := NewAPIClient(server.URL, "test-token")
	ctx := context.Background()

	// Test with project filter
	tokens, pagination, err := client.GetTokens(ctx, "project-1", 1, 2)
	if err != nil {
		t.Fatalf("GetTokens failed: %v", err)
	}

	if len(tokens) != 2 {
		t.Errorf("len(tokens) = %d, want 2", len(tokens))
	}
	if pagination.TotalItems != 3 {
		t.Errorf("TotalItems = %d, want 3", pagination.TotalItems)
	}

	// Test without project filter
	tokens, _, err = client.GetTokens(ctx, "", 1, 10)
	if err != nil {
		t.Fatalf("GetTokens failed: %v", err)
	}
	if len(tokens) != 3 {
		t.Errorf("len(tokens) = %d, want 3", len(tokens))
	}
}

func TestAPIClient_CreateToken(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" || r.URL.Path != "/manage/tokens" {
			http.Error(w, "invalid request", http.StatusBadRequest)
			return
		}

		var req map[string]any
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "invalid JSON", http.StatusBadRequest)
			return
		}

		response := TokenCreateResponse{
			Token:     "tok-abcd1234",
			ExpiresAt: time.Now().Add(time.Duration(req["duration_hours"].(float64)) * time.Hour),
		}
		if err := json.NewEncoder(w).Encode(response); err != nil {
			t.Errorf("failed to encode token create response: %v", err)
		}
	}))
	defer server.Close()

	client := NewAPIClient(server.URL, "test-token")
	ctx := context.Background()

	token, err := client.CreateToken(ctx, "project-1", 24)
	if err != nil {
		t.Fatalf("CreateToken failed: %v", err)
	}

	if token.Token != "tok-abcd1234" {
		t.Errorf("Token = %q, want %q", token.Token, "tok-abcd1234")
	}
}

func TestAPIClient_ErrorHandling(t *testing.T) {
	// Test server error
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		if err := json.NewEncoder(w).Encode(map[string]string{"error": "internal server error"}); err != nil {
			t.Errorf("failed to encode error response: %v", err)
		}
	}))
	defer server.Close()

	client := NewAPIClient(server.URL, "test-token")
	ctx := context.Background()

	_, err := client.GetProject(ctx, "1")
	if err == nil {
		t.Fatal("Expected error, got nil")
	}
	if !strings.Contains(err.Error(), "internal server error") {
		t.Errorf("Error message should contain 'internal server error', got: %v", err)
	}

	// Test unauthorized
	server2 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer server2.Close()

	client2 := NewAPIClient(server2.URL, "bad-token")
	_, err = client2.GetProject(ctx, "1")
	if err == nil {
		t.Fatal("Expected error, got nil")
	}
}

func TestAPIClient_RequestCreation(t *testing.T) {
	client := NewAPIClient("http://example.com", "test-token")
	ctx := context.Background()

	// Test request with body
	req, err := client.newRequest(ctx, "POST", "/test", map[string]string{"key": "value"})
	if err != nil {
		t.Fatalf("newRequest failed: %v", err)
	}

	if req.Header.Get("Authorization") != "Bearer test-token" {
		t.Error("Authorization header not set correctly")
	}
	if req.Header.Get("Content-Type") != "application/json" {
		t.Error("Content-Type header not set correctly")
	}

	// Test request without body
	req, err = client.newRequest(ctx, "GET", "/test", nil)
	if err != nil {
		t.Fatalf("newRequest failed: %v", err)
	}

	if req.Header.Get("Content-Type") != "" {
		t.Error("Content-Type header should not be set for GET request")
	}
}

func TestAPIClient_UpdateProjectPartial(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req map[string]string
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Errorf("failed to decode request: %v", err)
		}

		project := Project{ID: "1", CreatedAt: time.Now()}
		// Only update provided fields
		if name, ok := req["name"]; ok {
			project.Name = name
		}
		if key, ok := req["openai_api_key"]; ok {
			project.OpenAIAPIKey = key
		}
		if err := json.NewEncoder(w).Encode(project); err != nil {
			t.Errorf("failed to encode project: %v", err)
		}
	}))
	defer server.Close()

	client := NewAPIClient(server.URL, "test-token")
	ctx := context.Background()

	// Test updating only name
	project, err := client.UpdateProject(ctx, "1", "New Name", "")
	if err != nil {
		t.Fatalf("UpdateProject failed: %v", err)
	}
	if project.Name != "New Name" {
		t.Errorf("Name = %q, want %q", project.Name, "New Name")
	}
	if project.OpenAIAPIKey != "" {
		t.Errorf("OpenAIAPIKey should be empty, got %q", project.OpenAIAPIKey)
	}
}

func TestAPIClient_NetworkError(t *testing.T) {
	// Test with invalid URL that will cause network error
	client := NewAPIClient("http://invalid-host-that-does-not-exist:12345", "test-token")
	ctx := context.Background()

	_, err := client.GetProject(ctx, "1")
	if err == nil {
		t.Fatal("Expected network error, got nil")
	}
}

func TestAPIClient_InvalidJSON(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if _, err := w.Write([]byte("invalid json")); err != nil {
			t.Errorf("failed to write invalid json: %v", err)
		}
	}))
	defer server.Close()

	client := NewAPIClient(server.URL, "test-token")
	ctx := context.Background()

	_, err := client.GetProject(ctx, "1")
	if err == nil {
		t.Fatal("Expected JSON decode error, got nil")
	}
	if !strings.Contains(err.Error(), "failed to decode response") {
		t.Errorf("Expected decode error, got: %v", err)
	}
}

func TestAPIClient_DashboardDataError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/manage/projects" {
			w.WriteHeader(http.StatusInternalServerError)
		} else {
			tokens := []Token{}
			if err := json.NewEncoder(w).Encode(tokens); err != nil {
				t.Errorf("failed to encode tokens: %v", err)
			}
		}
	}))
	defer server.Close()

	client := NewAPIClient(server.URL, "test-token")
	ctx := context.Background()

	_, err := client.GetDashboardData(ctx)
	if err == nil {
		t.Fatal("Expected error from projects endpoint, got nil")
	}
}

func TestAPIClient_RequestMarshalError(t *testing.T) {
	client := NewAPIClient("http://example.com", "test-token")
	ctx := context.Background()

	// Use a map with unmarshalable content (channels can't be marshaled to JSON)
	invalidBody := map[string]interface{}{
		"channel": make(chan int),
	}

	_, err := client.newRequest(ctx, "POST", "/test", invalidBody)
	if err == nil {
		t.Fatal("Expected marshal error, got nil")
	}
	if !strings.Contains(err.Error(), "failed to marshal request body") {
		t.Errorf("Expected marshal error, got: %v", err)
	}
}

func TestAPIClient_ErrorResponseWithoutJSON(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		if _, err := w.Write([]byte("plain text error")); err != nil {
			t.Errorf("failed to write plain text error: %v", err)
		}
	}))
	defer server.Close()

	client := NewAPIClient(server.URL, "test-token")
	ctx := context.Background()

	_, err := client.GetProject(ctx, "1")
	if err == nil {
		t.Fatal("Expected error, got nil")
	}
	if !strings.Contains(err.Error(), "400") {
		t.Errorf("Expected status code in error, got: %v", err)
	}
}

func TestAPIClient_CreateProject_Errors(t *testing.T) {
	client := NewAPIClient("http://invalid-host", "token")
	ctx := context.Background()

	t.Run("network error", func(t *testing.T) {
		_, err := client.CreateProject(ctx, "foo", "bar")
		if err == nil {
			t.Error("expected network error, got nil")
		}
	})

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"error":"fail"}`))
	}))
	defer server.Close()
	client2 := NewAPIClient(server.URL, "token")
	_, err := client2.CreateProject(ctx, "foo", "bar")
	if err == nil || !strings.Contains(err.Error(), "fail") {
		t.Errorf("expected API error, got %v", err)
	}

	server2 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		if _, err := w.Write([]byte("not json")); err != nil {
			t.Errorf("failed to write not json: %v", err)
		}
	}))
	defer server2.Close()
	client3 := NewAPIClient(server2.URL, "token")
	_, err = client3.CreateProject(ctx, "foo", "bar")
	if err == nil || !strings.Contains(err.Error(), "decode") {
		t.Errorf("expected decode error, got %v", err)
	}
}

func TestAPIClient_UpdateProject_Errors(t *testing.T) {
	client := NewAPIClient("http://invalid-host", "token")
	ctx := context.Background()
	_, err := client.UpdateProject(ctx, "id", "foo", "bar")
	if err == nil {
		t.Error("expected network error, got nil")
	}
}

func TestAPIClient_DeleteProject_Errors(t *testing.T) {
	client := NewAPIClient("http://invalid-host", "token")
	ctx := context.Background()
	err := client.DeleteProject(ctx, "id")
	if err == nil {
		t.Error("expected network error, got nil")
	}
}

func TestAPIClient_CreateToken_Errors(t *testing.T) {
	client := NewAPIClient("http://invalid-host", "token")
	ctx := context.Background()
	_, err := client.CreateToken(ctx, "id", 1)
	if err == nil {
		t.Error("expected network error, got nil")
	}
}
