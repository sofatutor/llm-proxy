package server

import "time"

// TokenListResponse matches the sanitized token response schema (shared for tests and production)
type TokenListResponse struct {
	TokenID       string     `json:"token_id"` // Obfuscated token for security
	ProjectID     string     `json:"project_id"`
	ExpiresAt     *time.Time `json:"expires_at"`
	IsActive      bool       `json:"is_active"`
	RequestCount  int        `json:"request_count"`
	MaxRequests   *int       `json:"max_requests"`
	CreatedAt     time.Time  `json:"created_at"`
	LastUsedAt    *time.Time `json:"last_used_at"`
	CacheHitCount int        `json:"cache_hit_count"`
}

// ProjectResponse is the sanitized project response with obfuscated API key
type ProjectResponse struct {
	ID            string     `json:"id"`
	Name          string     `json:"name"`
	OpenAIAPIKey  string     `json:"openai_api_key"` // Obfuscated for security
	IsActive      bool       `json:"is_active"`
	DeactivatedAt *time.Time `json:"deactivated_at,omitempty"`
	CreatedAt     time.Time  `json:"created_at"`
	UpdatedAt     time.Time  `json:"updated_at"`
}
