// Package admin provides HTTP client functionality for communicating
// with the Management API from the Admin UI server.
package admin

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"time"

	"github.com/sofatutor/llm-proxy/internal/obfuscate"
)

// context keys used to forward browser metadata from Admin UI → Management API
type forwardedCtxKey string

const (
	ctxKeyForwardedUA      forwardedCtxKey = "forwarded_user_agent"
	ctxKeyForwardedReferer forwardedCtxKey = "forwarded_referer"
	ctxKeyForwardedIP      forwardedCtxKey = "forwarded_ip"
)

// APIClient handles communication with the Management API
type APIClient struct {
	baseURL    string
	token      string
	httpClient *http.Client
}

// NewAPIClient creates a new Management API client
func NewAPIClient(baseURL, token string) *APIClient {
	return &APIClient{
		baseURL: baseURL,
		token:   token,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// ObfuscateAPIKey obfuscates an API key for display purposes
// Shows first 8 characters followed by dots and last 4 characters
func ObfuscateAPIKey(apiKey string) string { return obfuscate.ObfuscateTokenGeneric(apiKey) }

// ObfuscateToken obfuscates a token for display purposes.
// Use centralized helper to avoid linter warning on deprecated wrapper.
func ObfuscateToken(token string) string { return obfuscate.ObfuscateTokenGeneric(token) }

// Project represents a project from the Management API
type Project struct {
	ID           string    `json:"id"`
	Name         string    `json:"name"`
	OpenAIAPIKey string    `json:"openai_api_key"`
	IsActive     bool      `json:"is_active"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

// Token represents a token from the Management API (sanitized)
type Token struct {
	TokenID       string     `json:"token_id"` // Added for Admin UI support
	ProjectID     string     `json:"project_id"`
	ExpiresAt     *time.Time `json:"expires_at,omitempty"`
	IsActive      bool       `json:"is_active"`
	RequestCount  int        `json:"request_count"`
	MaxRequests   *int       `json:"max_requests,omitempty"`
	CreatedAt     time.Time  `json:"created_at"`
	LastUsedAt    *time.Time `json:"last_used_at,omitempty"`
	CacheHitCount int        `json:"cache_hit_count"`
}

// TokenCreateResponse represents the response when creating a token
type TokenCreateResponse struct {
	Token     string    `json:"token"`
	ExpiresAt time.Time `json:"expires_at"`
}

// Pagination represents pagination metadata
type Pagination struct {
	Page       int  `json:"page"`
	PageSize   int  `json:"page_size"`
	TotalItems int  `json:"total_items"`
	TotalPages int  `json:"total_pages"`
	HasNext    bool `json:"has_next"`
	HasPrev    bool `json:"has_prev"`
}

// DashboardData represents dashboard statistics
type DashboardData struct {
	TotalProjects    int `json:"total_projects"`
	TotalTokens      int `json:"total_tokens"`
	ActiveTokens     int `json:"active_tokens"`
	ExpiredTokens    int `json:"expired_tokens"`
	TotalRequests    int `json:"total_requests"`
	RequestsToday    int `json:"requests_today"`
	RequestsThisWeek int `json:"requests_this_week"`
}

// AuditEvent represents an audit log entry for the admin UI
type AuditEvent struct {
	ID            string         `json:"id"`
	Timestamp     time.Time      `json:"timestamp"`
	Action        string         `json:"action"`
	Actor         string         `json:"actor"`
	ProjectID     *string        `json:"project_id,omitempty"`
	RequestID     *string        `json:"request_id,omitempty"`
	CorrelationID *string        `json:"correlation_id,omitempty"`
	ClientIP      *string        `json:"client_ip,omitempty"`
	Method        *string        `json:"method,omitempty"`
	Path          *string        `json:"path,omitempty"`
	UserAgent     *string        `json:"user_agent,omitempty"`
	Outcome       string         `json:"outcome"`
	Reason        *string        `json:"reason,omitempty"`
	TokenID       *string        `json:"token_id,omitempty"`
	Metadata      *string        `json:"metadata,omitempty"` // JSON string
	ParsedMeta    *AuditMetadata `json:"-"`                  // Parsed metadata for template use
}

// AuditMetadata represents parsed metadata for easier template access
type AuditMetadata map[string]interface{}

// AuditEventsResponse represents the API response for audit events listing
type AuditEventsResponse struct {
	Events     []AuditEvent `json:"events"`
	Pagination Pagination   `json:"pagination"`
}

// GetDashboardData retrieves dashboard statistics
func (c *APIClient) GetDashboardData(ctx context.Context) (*DashboardData, error) {
	// For now, calculate from projects and tokens lists
	// In the future, this could be a dedicated dashboard endpoint
	projects, _, err := c.GetProjects(ctx, 1, 1000) // Get all projects
	if err != nil {
		return nil, err
	}

	tokens, _, err := c.GetTokens(ctx, "", 1, 1000) // Get all tokens
	if err != nil {
		return nil, err
	}

	data := &DashboardData{
		TotalProjects: len(projects),
		TotalTokens:   len(tokens),
	}

	// Calculate active/expired tokens and request counts
	now := time.Now()
	for _, token := range tokens {
		if token.IsActive && token.ExpiresAt != nil && token.ExpiresAt.After(now) {
			data.ActiveTokens++
		} else {
			data.ExpiredTokens++
		}
		data.TotalRequests += token.RequestCount

		// Calculate today's requests (approximation)
		if token.LastUsedAt != nil && token.LastUsedAt.After(now.AddDate(0, 0, -1)) {
			data.RequestsToday += token.RequestCount
		}

		// Calculate this week's requests (approximation)
		if token.LastUsedAt != nil && token.LastUsedAt.After(now.AddDate(0, 0, -7)) {
			data.RequestsThisWeek += token.RequestCount
		}
	}

	return data, nil
}

// GetProjects retrieves a paginated list of projects
func (c *APIClient) GetProjects(ctx context.Context, page, pageSize int) ([]Project, *Pagination, error) {
	// Since the Management API doesn't currently support pagination,
	// we'll get all projects and simulate pagination
	req, err := c.newRequest(ctx, "GET", "/manage/projects", nil)
	if err != nil {
		return nil, nil, err
	}

	var projects []Project
	if err := c.doRequest(req, &projects); err != nil {
		return nil, nil, err
	}

	// Simulate pagination
	totalItems := len(projects)
	totalPages := (totalItems + pageSize - 1) / pageSize
	start := (page - 1) * pageSize
	end := start + pageSize

	if start >= totalItems {
		projects = []Project{}
	} else {
		if end > totalItems {
			end = totalItems
		}
		projects = projects[start:end]
	}

	pagination := &Pagination{
		Page:       page,
		PageSize:   pageSize,
		TotalItems: totalItems,
		TotalPages: totalPages,
		HasNext:    page < totalPages,
		HasPrev:    page > 1,
	}

	return projects, pagination, nil
}

// GetProject retrieves a single project by ID
func (c *APIClient) GetProject(ctx context.Context, id string) (*Project, error) {
	req, err := c.newRequest(ctx, "GET", fmt.Sprintf("/manage/projects/%s", id), nil)
	if err != nil {
		return nil, err
	}

	var project Project
	if err := c.doRequest(req, &project); err != nil {
		return nil, err
	}

	return &project, nil
}

// CreateProject creates a new project
func (c *APIClient) CreateProject(ctx context.Context, name, openaiAPIKey string) (*Project, error) {
	payload := map[string]string{
		"name":           name,
		"openai_api_key": openaiAPIKey,
	}

	req, err := c.newRequest(ctx, "POST", "/manage/projects", payload)
	if err != nil {
		return nil, err
	}

	var project Project
	if err := c.doRequest(req, &project); err != nil {
		return nil, err
	}

	return &project, nil
}

// UpdateProject updates an existing project
func (c *APIClient) UpdateProject(ctx context.Context, id, name, openaiAPIKey string, isActive *bool) (*Project, error) {
	payload := map[string]interface{}{}
	if name != "" {
		payload["name"] = name
	}
	if openaiAPIKey != "" {
		payload["openai_api_key"] = openaiAPIKey
	}
	if isActive != nil {
		payload["is_active"] = *isActive
	}

	req, err := c.newRequest(ctx, "PATCH", fmt.Sprintf("/manage/projects/%s", id), payload)
	if err != nil {
		return nil, err
	}

	var project Project
	if err := c.doRequest(req, &project); err != nil {
		return nil, err
	}

	return &project, nil
}

// DeleteProject deletes a project
func (c *APIClient) DeleteProject(ctx context.Context, id string) error {
	req, err := c.newRequest(ctx, "DELETE", fmt.Sprintf("/manage/projects/%s", id), nil)
	if err != nil {
		return err
	}

	return c.doRequest(req, nil)
}

// GetTokens retrieves a paginated list of tokens
func (c *APIClient) GetTokens(ctx context.Context, projectID string, page, pageSize int) ([]Token, *Pagination, error) {
	path := "/manage/tokens"
	if projectID != "" {
		path += "?projectId=" + url.QueryEscape(projectID)
	}

	req, err := c.newRequest(ctx, "GET", path, nil)
	if err != nil {
		return nil, nil, err
	}

	var tokens []Token
	if err := c.doRequest(req, &tokens); err != nil {
		return nil, nil, err
	}

	// Simulate pagination (similar to projects)
	totalItems := len(tokens)
	totalPages := (totalItems + pageSize - 1) / pageSize
	start := (page - 1) * pageSize
	end := start + pageSize

	if start >= totalItems {
		tokens = []Token{}
	} else {
		if end > totalItems {
			end = totalItems
		}
		tokens = tokens[start:end]
	}

	pagination := &Pagination{
		Page:       page,
		PageSize:   pageSize,
		TotalItems: totalItems,
		TotalPages: totalPages,
		HasNext:    page < totalPages,
		HasPrev:    page > 1,
	}

	return tokens, pagination, nil
}

// CreateToken creates a new token for a project with a given duration in minutes
func (c *APIClient) CreateToken(ctx context.Context, projectID string, durationMinutes int) (*TokenCreateResponse, error) {
	payload := map[string]interface{}{
		"project_id":       projectID,
		"duration_minutes": durationMinutes,
	}
	// Use newRequest and doRequest for consistent error handling
	req, err := c.newRequest(ctx, "POST", "/manage/tokens", payload)
	if err != nil {
		return nil, err
	}
	var result TokenCreateResponse
	if err := c.doRequest(req, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// newRequest creates a new HTTP request with authentication
func (c *APIClient) newRequest(ctx context.Context, method, path string, body any) (*http.Request, error) {
	var reqBody []byte
	var err error

	if body != nil {
		reqBody, err = json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal request body: %w", err)
		}
	}

	url := c.baseURL + path
	req, err := http.NewRequestWithContext(ctx, method, url, bytes.NewBuffer(reqBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Set headers
	req.Header.Set("Authorization", "Bearer "+c.token)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	// Forward browser context when present
	if v := ctx.Value(ctxKeyForwardedUA); v != nil {
		if ua, ok := v.(string); ok && ua != "" {
			req.Header.Set("X-Forwarded-User-Agent", ua)
			req.Header.Set("X-Admin-Origin", "1")
		}
	}
	if v := ctx.Value(ctxKeyForwardedReferer); v != nil {
		if ref, ok := v.(string); ok && ref != "" {
			req.Header.Set("X-Forwarded-Referer", ref)
			req.Header.Set("X-Admin-Origin", "1")
		}
	}
	if v := ctx.Value(ctxKeyForwardedIP); v != nil {
		if ip, ok := v.(string); ok && ip != "" {
			// Provide original browser IP for backend audit logging
			req.Header.Set("X-Forwarded-For", ip)
			req.Header.Set("X-Admin-Origin", "1")
		}
	}

	return req, nil
}

// doRequest executes an HTTP request and handles the response
func (c *APIClient) doRequest(req *http.Request, result any) error {
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer func() {
		err := resp.Body.Close()
		if err != nil {
			// Log or handle the error as appropriate
			// For now, just log to standard error
			fmt.Fprintf(os.Stderr, "failed to close response body: %v\n", err)
		}
	}()

	if resp.StatusCode >= 400 {
		var errorResp map[string]any
		if err := json.NewDecoder(resp.Body).Decode(&errorResp); err == nil {
			if msg, ok := errorResp["error"].(string); ok {
				return fmt.Errorf("API error (%d): %s", resp.StatusCode, msg)
			}
		}
		return fmt.Errorf("API error: %d %s", resp.StatusCode, resp.Status)
	}

	if result != nil && resp.StatusCode != http.StatusNoContent {
		if err := json.NewDecoder(resp.Body).Decode(result); err != nil {
			return fmt.Errorf("failed to decode response: %w", err)
		}
	}

	return nil
}

// GetAuditEvents retrieves a paginated list of audit events with optional filters
func (c *APIClient) GetAuditEvents(ctx context.Context, filters map[string]string, page, pageSize int) ([]AuditEvent, *Pagination, error) {
	// Build query parameters
	params := url.Values{}
	for key, value := range filters {
		if value != "" {
			params.Set(key, value)
		}
	}
	params.Set("page", fmt.Sprintf("%d", page))
	params.Set("page_size", fmt.Sprintf("%d", pageSize))

	endpoint := "/manage/audit"
	if len(params) > 0 {
		endpoint += "?" + params.Encode()
	}

	req, err := c.newRequest(ctx, "GET", endpoint, nil)
	if err != nil {
		return nil, nil, err
	}

	var response AuditEventsResponse
	if err := c.doRequest(req, &response); err != nil {
		return nil, nil, err
	}

	// Parse metadata for each event
	for i := range response.Events {
		if response.Events[i].Metadata != nil && *response.Events[i].Metadata != "" {
			var meta AuditMetadata
			if err := json.Unmarshal([]byte(*response.Events[i].Metadata), &meta); err == nil {
				response.Events[i].ParsedMeta = &meta
			}
		}
	}

	return response.Events, &response.Pagination, nil
}

// GetAuditEvent retrieves a specific audit event by ID
func (c *APIClient) GetAuditEvent(ctx context.Context, id string) (*AuditEvent, error) {
	req, err := c.newRequest(ctx, "GET", "/manage/audit/"+id, nil)
	if err != nil {
		return nil, err
	}

	var event AuditEvent
	if err := c.doRequest(req, &event); err != nil {
		return nil, err
	}

	// Parse metadata
	if event.Metadata != nil && *event.Metadata != "" {
		var meta AuditMetadata
		if err := json.Unmarshal([]byte(*event.Metadata), &meta); err == nil {
			event.ParsedMeta = &meta
		}
	}

	return &event, nil
}

// GetToken retrieves a single token by ID
func (c *APIClient) GetToken(ctx context.Context, tokenID string) (*Token, error) {
	req, err := c.newRequest(ctx, "GET", fmt.Sprintf("/manage/tokens/%s", tokenID), nil)
	if err != nil {
		return nil, err
	}

	var token Token
	if err := c.doRequest(req, &token); err != nil {
		return nil, err
	}

	return &token, nil
}

// UpdateToken updates an existing token
func (c *APIClient) UpdateToken(ctx context.Context, tokenID string, isActive *bool, maxRequests *int) (*Token, error) {
	payload := map[string]interface{}{}
	if isActive != nil {
		payload["is_active"] = *isActive
	}
	if maxRequests != nil {
		payload["max_requests"] = *maxRequests
	}

	req, err := c.newRequest(ctx, "PATCH", fmt.Sprintf("/manage/tokens/%s", tokenID), payload)
	if err != nil {
		return nil, err
	}

	var token Token
	if err := c.doRequest(req, &token); err != nil {
		return nil, err
	}

	return &token, nil
}

// RevokeToken revokes a single token by setting is_active to false
func (c *APIClient) RevokeToken(ctx context.Context, tokenID string) error {
	req, err := c.newRequest(ctx, "DELETE", fmt.Sprintf("/manage/tokens/%s", tokenID), nil)
	if err != nil {
		return err
	}

	return c.doRequest(req, nil)
}

// RevokeProjectTokens revokes all tokens for a project in bulk
func (c *APIClient) RevokeProjectTokens(ctx context.Context, projectID string) error {
	req, err := c.newRequest(ctx, "POST", fmt.Sprintf("/manage/projects/%s/tokens/revoke", projectID), nil)
	if err != nil {
		return err
	}

	return c.doRequest(req, nil)
}
