package admin

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"
)

// Test Admin API client token methods: GetToken, UpdateToken, RevokeToken, RevokeProjectTokens
func TestAPIClient_TokenMethods(t *testing.T) {
	mux := http.NewServeMux()

	// GET /manage/tokens/:id
	mux.HandleFunc("/manage/tokens/tok-1", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method", http.StatusMethodNotAllowed)
			return
		}
		_ = json.NewEncoder(w).Encode(Token{ID: "tok-1", ProjectID: "p1", IsActive: true})
	})

	// PATCH /manage/tokens/:id
	mux.HandleFunc("/manage/tokens/tok-2", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPatch {
			http.Error(w, "method", http.StatusMethodNotAllowed)
			return
		}
		_ = json.NewEncoder(w).Encode(Token{ID: "tok-2", ProjectID: "p1", IsActive: false})
	})

	// DELETE /manage/tokens/:id
	mux.HandleFunc("/manage/tokens/tok-3", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			http.Error(w, "method", http.StatusMethodNotAllowed)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	})

	// POST /manage/projects/:id/tokens/revoke
	mux.HandleFunc("/manage/projects/p1/tokens/revoke", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method", http.StatusMethodNotAllowed)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	})

	ts := httptest.NewServer(mux)
	defer ts.Close()

	c := NewAPIClient(ts.URL, "mgmt-token")

	// GetToken
	tok, err := c.GetToken(context.Background(), "tok-1")
	if err != nil {
		t.Fatalf("GetToken error: %v", err)
	}
	if tok == nil || tok.ID != "tok-1" || tok.ProjectID != "p1" {
		t.Fatalf("GetToken unexpected: %+v", tok)
	}

	// UpdateToken (payload covered by doRequest decoding)
	updated, err := c.UpdateToken(context.Background(), "tok-2", nil, nil)
	if err != nil {
		t.Fatalf("UpdateToken error: %v", err)
	}
	if updated == nil || updated.ID != "tok-2" || updated.IsActive != false {
		t.Fatalf("UpdateToken unexpected: %+v", updated)
	}

	// RevokeToken (204 branch)
	if err := c.RevokeToken(context.Background(), "tok-3"); err != nil {
		t.Fatalf("RevokeToken error: %v", err)
	}

	// RevokeProjectTokens (204 branch)
	if err := c.RevokeProjectTokens(context.Background(), "p1"); err != nil {
		t.Fatalf("RevokeProjectTokens error: %v", err)
	}
}

func TestAPIClient_TokenMethods_ErrorBranches(t *testing.T) {
	// Server returns 400 with JSON body for GetToken
	srvJSON := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"error":"bad token"}`))
	}))
	defer srvJSON.Close()

	c1 := NewAPIClient(srvJSON.URL, "tkn")
	if _, err := c1.GetToken(context.Background(), "tok-x"); err == nil {
		t.Fatalf("expected error on 400 JSON, got nil")
	}

	// Server returns 400 with non-JSON body for UpdateToken
	srvText := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte("plain error"))
	}))
	defer srvText.Close()

	c2 := NewAPIClient(srvText.URL, "tkn")
	if _, err := c2.UpdateToken(context.Background(), "tok-y", nil, nil); err == nil {
		t.Fatalf("expected error on 400 text, got nil")
	}
}

func TestAPIClient_UpdateToken_SendsBothFields(t *testing.T) {
	// Capture request payload and return updated token
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/manage/tokens/tok-99" || r.Method != http.MethodPatch {
			http.Error(w, "bad route", http.StatusNotFound)
			return
		}
		var body map[string]interface{}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decode payload: %v", err)
		}
		if _, ok := body["is_active"]; !ok {
			t.Fatalf("missing is_active in payload: %#v", body)
		}
		if _, ok := body["max_requests"]; !ok {
			t.Fatalf("missing max_requests in payload: %#v", body)
		}
		_ = json.NewEncoder(w).Encode(Token{ID: "tok-99", ProjectID: "p1", IsActive: true})
	}))
	defer srv.Close()

	c := NewAPIClient(srv.URL, "tkn")
	active := true
	maxReq := 42
	if _, err := c.UpdateToken(context.Background(), "tok-99", &active, &maxReq); err != nil {
		t.Fatalf("UpdateToken err: %v", err)
	}
}

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

func TestAPIClient_NewRequestAndDoRequest_ErrorBranches(t *testing.T) {
	t.Parallel()
	// Create client with fake base and token
	c := NewAPIClient("http://example", "tkn")

	// newRequest with body sets headers and JSON marshals
	req, err := c.newRequest(context.Background(), http.MethodPost, "/x", map[string]string{"a": "b"})
	if err != nil {
		t.Fatalf("newRequest err: %v", err)
	}
	if req.Header.Get("Authorization") == "" || req.Header.Get("Content-Type") != "application/json" {
		t.Fatalf("headers not set as expected")
	}

	// doRequest with 400 + JSON error payload
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"error":"bad"}`))
	}))
	defer srv.Close()
	c.baseURL = srv.URL
	req.URL = mustParseURL(t, srv.URL+"/x")
	err = c.doRequest(req, nil)
	if err == nil || !strings.Contains(err.Error(), "API error (400): bad") {
		t.Fatalf("expected structured API error, got %v", err)
	}

	// doRequest with 500 and non-JSON body (use a fresh request so body isn't exhausted)
	srv2 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte("oops"))
	}))
	defer srv2.Close()
	req2, err := c.newRequest(context.Background(), http.MethodPost, "/x", map[string]string{"a": "b"})
	if err != nil {
		t.Fatalf("newRequest err: %v", err)
	}
	req2.URL = mustParseURL(t, srv2.URL+"/x")
	err = c.doRequest(req2, nil)
	if err == nil || !strings.Contains(err.Error(), "API error: 500") {
		t.Fatalf("expected generic API error, got %v", err)
	}
}

func mustParseURL(t *testing.T, u string) *url.URL {
	t.Helper()
	pu, err := url.Parse(u)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	return pu
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

	data, err := client.GetDashboardData(ctx)
	if err != nil {
		t.Fatalf("GetDashboardData failed: %v", err)
	}

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
			APIKey: req["api_key"],
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
	if project.APIKey != "sk-test-key" {
		t.Errorf("APIKey = %q, want %q", project.APIKey, "sk-test-key")
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
			APIKey: req["api_key"],
			UpdatedAt:    time.Now(),
		}
		if err := json.NewEncoder(w).Encode(project); err != nil {
			t.Errorf("failed to encode project: %v", err)
		}
	}))
	defer server.Close()

	client := NewAPIClient(server.URL, "test-token")
	ctx := context.Background()

	project, err := client.UpdateProject(ctx, "1", "Updated Name", "sk-updated-key", nil)
	if err != nil {
		t.Fatalf("UpdateProject failed: %v", err)
	}

	if project.Name != "Updated Name" {
		t.Errorf("Name = %q, want %q", project.Name, "Updated Name")
	}
	if project.APIKey != "sk-updated-key" {
		t.Errorf("APIKey = %q, want %q", project.APIKey, "sk-updated-key")
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

func TestAPIClient_DeleteProject_Error(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "internal server error", http.StatusInternalServerError)
	}))
	defer server.Close()

	client := NewAPIClient(server.URL, "test-token")
	ctx := context.Background()

	err := client.DeleteProject(ctx, "1")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestAPIClient_DeleteProject_InvalidURL(t *testing.T) {
	// Test with malformed baseURL to trigger newRequest error
	client := NewAPIClient("://invalid-url", "test-token")
	ctx := context.Background()

	err := client.DeleteProject(ctx, "1")
	if err == nil {
		t.Fatal("expected error for invalid URL, got nil")
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
	intPtr := func(v int) *int { return &v }

	tests := []struct {
		name          string
		maxRequests   *int
		expectMaxKey  bool
		expectedValue int
	}{
		{name: "without max", maxRequests: nil, expectMaxKey: false},
		{name: "with max", maxRequests: intPtr(42), expectMaxKey: true, expectedValue: 42},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
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

				if tt.expectMaxKey {
					val, ok := req["max_requests"].(float64)
					if !ok {
						t.Fatalf("missing max_requests in payload: %#v", req)
					}
					if int(val) != tt.expectedValue {
						t.Fatalf("max_requests = %v, want %d", val, tt.expectedValue)
					}
				} else if _, ok := req["max_requests"]; ok {
					t.Fatalf("did not expect max_requests in payload: %#v", req)
				}

				response := TokenCreateResponse{
					Token:     "tok-abcd1234",
					ExpiresAt: time.Now().Add(time.Duration(req["duration_minutes"].(float64)) * time.Minute),
				}
				if tt.maxRequests != nil {
					response.MaxRequests = tt.maxRequests
				}
				if err := json.NewEncoder(w).Encode(response); err != nil {
					t.Errorf("failed to encode token create response: %v", err)
				}
			}))
			defer server.Close()

			client := NewAPIClient(server.URL, "test-token")
			ctx := context.Background()

			token, err := client.CreateToken(ctx, "project-1", 24, tt.maxRequests)
			if err != nil {
				t.Fatalf("CreateToken failed: %v", err)
			}

			if token.Token != "tok-abcd1234" {
				t.Errorf("Token = %q, want %q", token.Token, "tok-abcd1234")
			}
			if tt.maxRequests == nil {
				if token.MaxRequests != nil {
					t.Fatalf("MaxRequests = %v, want nil", *token.MaxRequests)
				}
			} else {
				if token.MaxRequests == nil {
					t.Fatalf("MaxRequests = nil, want %d", *tt.maxRequests)
				}
				if *token.MaxRequests != *tt.maxRequests {
					t.Fatalf("MaxRequests = %d, want %d", *token.MaxRequests, *tt.maxRequests)
				}
			}
		})
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

func TestAPIClient_newRequest_ForwardsBrowserContext(t *testing.T) {
	client := NewAPIClient("http://example.com", "test-token")
	ctx := context.Background()
	ctx = context.WithValue(ctx, ctxKeyForwardedUA, "UA-123")
	ctx = context.WithValue(ctx, ctxKeyForwardedReferer, "http://admin.local/page")
	ctx = context.WithValue(ctx, ctxKeyForwardedIP, "203.0.113.7")

	req, err := client.newRequest(ctx, http.MethodGet, "/test", nil)
	if err != nil {
		t.Fatalf("newRequest failed: %v", err)
	}

	if got := req.Header.Get("X-Forwarded-User-Agent"); got != "UA-123" {
		t.Fatalf("X-Forwarded-User-Agent=%q, want %q", got, "UA-123")
	}
	if got := req.Header.Get("X-Forwarded-Referer"); got != "http://admin.local/page" {
		t.Fatalf("X-Forwarded-Referer=%q, want %q", got, "http://admin.local/page")
	}
	if got := req.Header.Get("X-Forwarded-For"); got != "203.0.113.7" {
		t.Fatalf("X-Forwarded-For=%q, want %q", got, "203.0.113.7")
	}
	if got := req.Header.Get("X-Admin-Origin"); got != "1" {
		t.Fatalf("X-Admin-Origin=%q, want %q", got, "1")
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
		if key, ok := req["api_key"]; ok {
			project.APIKey = key
		}
		if err := json.NewEncoder(w).Encode(project); err != nil {
			t.Errorf("failed to encode project: %v", err)
		}
	}))
	defer server.Close()

	client := NewAPIClient(server.URL, "test-token")
	ctx := context.Background()

	// Test updating only name
	project, err := client.UpdateProject(ctx, "1", "New Name", "", nil)
	if err != nil {
		t.Fatalf("UpdateProject failed: %v", err)
	}
	if project.Name != "New Name" {
		t.Errorf("Name = %q, want %q", project.Name, "New Name")
	}
	if project.APIKey != "" {
		t.Errorf("APIKey should be empty, got %q", project.APIKey)
	}
}

func TestAPIClient_UpdateProject_WithIsActive(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req map[string]interface{}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Errorf("failed to decode request: %v", err)
		}

		project := Project{ID: "1", CreatedAt: time.Now()}
		if name, ok := req["name"].(string); ok {
			project.Name = name
		}
		if isActive, ok := req["is_active"].(bool); ok {
			project.IsActive = isActive
		}
		if err := json.NewEncoder(w).Encode(project); err != nil {
			t.Errorf("failed to encode project: %v", err)
		}
	}))
	defer server.Close()

	client := NewAPIClient(server.URL, "test-token")
	ctx := context.Background()

	// Test updating with isActive = true
	active := true
	project, err := client.UpdateProject(ctx, "1", "Updated", "", &active)
	if err != nil {
		t.Fatalf("UpdateProject failed: %v", err)
	}
	if !project.IsActive {
		t.Errorf("IsActive = %v, want true", project.IsActive)
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
	_, err := client.UpdateProject(ctx, "id", "foo", "bar", nil)
	if err == nil {
		t.Error("expected network error, got nil")
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"error":"fail"}`))
	}))
	defer server.Close()
	client2 := NewAPIClient(server.URL, "token")
	_, err = client2.UpdateProject(ctx, "id", "foo", "bar", nil)
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
	_, err = client3.UpdateProject(ctx, "id", "foo", "bar", nil)
	if err == nil || !strings.Contains(err.Error(), "decode") {
		t.Errorf("expected decode error, got %v", err)
	}
}

func TestAPIClient_DeleteProject_Errors(t *testing.T) {
	client := NewAPIClient("http://invalid-host", "token")
	ctx := context.Background()
	err := client.DeleteProject(ctx, "id")
	if err == nil {
		t.Error("expected network error, got nil")
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"error":"fail"}`))
	}))
	defer server.Close()
	client2 := NewAPIClient(server.URL, "token")
	err = client2.DeleteProject(ctx, "id")
	if err == nil || !strings.Contains(err.Error(), "fail") {
		t.Errorf("expected API error, got %v", err)
	}
}

func TestAPIClient_GetTokens_Errors(t *testing.T) {
	client := NewAPIClient("http://invalid-host", "token")
	ctx := context.Background()
	_, _, err := client.GetTokens(ctx, "id", 1, 1)
	if err == nil {
		t.Error("expected network error, got nil")
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"error":"fail"}`))
	}))
	defer server.Close()
	client2 := NewAPIClient(server.URL, "token")
	_, _, err = client2.GetTokens(ctx, "id", 1, 1)
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
	_, _, err = client3.GetTokens(ctx, "id", 1, 1)
	if err == nil || !strings.Contains(err.Error(), "decode") {
		t.Errorf("expected decode error, got %v", err)
	}
}

func TestAPIClient_CreateToken_Errors(t *testing.T) {
	client := NewAPIClient("http://invalid-host", "token")
	ctx := context.Background()
	_, err := client.CreateToken(ctx, "id", 1, nil)
	if err == nil {
		t.Error("expected network error, got nil")
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"error":"fail"}`))
	}))
	defer server.Close()
	client2 := NewAPIClient(server.URL, "token")
	_, err = client2.CreateToken(ctx, "id", 1, nil)
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
	_, err = client3.CreateToken(ctx, "id", 1, nil)
	if err == nil || !strings.Contains(err.Error(), "decode") {
		t.Errorf("expected decode error, got %v", err)
	}
}

func TestAPIClient_GetAuditEvents_and_GetAuditEvent(t *testing.T) {
	// Mock management API
	var lastQuery string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/manage/audit":
			lastQuery = r.URL.RawQuery
			if _, err := io.WriteString(w, `{
                "events": [
                    {"id":"evt-1","outcome":"success","metadata":"{\"k\":\"v\"}"}
                ],
                "pagination": {"page":1, "page_size":20, "total_items":1, "total_pages":1}
            }`); err != nil {
				t.Errorf("failed to write response: %v", err)
			}
		case r.Method == http.MethodGet && strings.HasPrefix(r.URL.Path, "/manage/audit/"):
			if _, err := io.WriteString(w, `{"id":"evt-2","outcome":"failure","metadata":"{\"a\":\"b\"}"}`); err != nil {
				t.Errorf("failed to write response: %v", err)
			}
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()

	c := &APIClient{baseURL: srv.URL, httpClient: srv.Client()}

	// list
	events, p, err := c.GetAuditEvents(context.Background(), map[string]string{"search": "req-123"}, 1, 20)
	if err != nil {
		t.Fatalf("GetAuditEvents error: %v", err)
	}
	if p == nil || p.Page != 1 || len(events) != 1 {
		t.Fatalf("unexpected pagination/events: %+v len=%d", p, len(events))
	}
	if events[0].ParsedMeta == nil || (*events[0].ParsedMeta)["k"] != "v" {
		t.Fatalf("ParsedMeta not decoded: %#v", events[0].ParsedMeta)
	}
	if !strings.Contains(lastQuery, "search=req-123") {
		t.Fatalf("filters were not forwarded: %q", lastQuery)
	}

	// show
	ev, err := c.GetAuditEvent(context.Background(), "evt-2")
	if err != nil {
		t.Fatalf("GetAuditEvent error: %v", err)
	}
	if ev.ID != "evt-2" || ev.Outcome != "failure" {
		t.Fatalf("unexpected event: %+v", ev)
	}
	if ev.ParsedMeta == nil || (*ev.ParsedMeta)["a"] != "b" {
		t.Fatalf("ParsedMeta not decoded on show: %#v", ev.ParsedMeta)
	}
}

func TestAPIClient_GetAuditEvents_Error(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		if _, err := io.WriteString(w, `{"error":"boom"}`); err != nil {
			t.Errorf("failed to write response: %v", err)
		}
	}))
	defer srv.Close()

	c := &APIClient{baseURL: srv.URL, httpClient: srv.Client()}
	_, _, err := c.GetAuditEvents(context.Background(), nil, 1, 20)
	if err == nil {
		t.Fatal("expected error for 500 response")
	}
}
