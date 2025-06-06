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
	"strings"
	"time"
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
func ObfuscateAPIKey(apiKey string) string {
	if len(apiKey) <= 12 {
		// For short keys, show first few chars + dots
		if len(apiKey) <= 4 {
			return strings.Repeat("*", len(apiKey))
		}
		return apiKey[:2] + strings.Repeat("*", len(apiKey)-2)
	}

	// For longer keys (like OpenAI keys), show first 8 and last 4
	return apiKey[:8] + "..." + apiKey[len(apiKey)-4:]
}

// ObfuscateToken obfuscates a token for display purposes
// Shows first 8 characters followed by dots and last 4 characters
func ObfuscateToken(token string) string {
	if len(token) <= 12 {
		// For short tokens, show first few chars + dots
		if len(token) <= 4 {
			return strings.Repeat("*", len(token))
		}
		return token[:2] + strings.Repeat("*", len(token)-2)
	}

	// For longer tokens, show first 8 and last 4
	return token[:8] + "..." + token[len(token)-4:]
}

// Project represents a project from the Management API
type Project struct {
	ID           string    `json:"id"`
	Name         string    `json:"name"`
	OpenAIAPIKey string    `json:"openai_api_key"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

// Token represents a token from the Management API (sanitized)
type Token struct {
	ProjectID    string     `json:"project_id"`
	ExpiresAt    *time.Time `json:"expires_at,omitempty"`
	IsActive     bool       `json:"is_active"`
	RequestCount int        `json:"request_count"`
	MaxRequests  *int       `json:"max_requests,omitempty"`
	CreatedAt    time.Time  `json:"created_at"`
	LastUsedAt   *time.Time `json:"last_used_at,omitempty"`
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
func (c *APIClient) UpdateProject(ctx context.Context, id, name, openaiAPIKey string) (*Project, error) {
	payload := map[string]string{}
	if name != "" {
		payload["name"] = name
	}
	if openaiAPIKey != "" {
		payload["openai_api_key"] = openaiAPIKey
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
