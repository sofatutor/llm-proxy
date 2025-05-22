package server

import "time"

// tokenListResponse matches the sanitized token response schema (shared for tests)
type TokenListResponse struct {
	ProjectID    string     `json:"project_id"`
	ExpiresAt    *time.Time `json:"expires_at"`
	IsActive     bool       `json:"is_active"`
	RequestCount int        `json:"request_count"`
	MaxRequests  *int       `json:"max_requests"`
	CreatedAt    time.Time  `json:"created_at"`
	LastUsedAt   *time.Time `json:"last_used_at"`
}
