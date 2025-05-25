// Package api provides types for API requests and responses shared between CLI and server.
package api

import "time"

// ProjectCreateRequest is the request body for creating a project.
type ProjectCreateRequest struct {
	Name         string `json:"name"`
	OpenAIAPIKey string `json:"openai_api_key"`
}

// ProjectCreateResponse is the response body for a created project.
type ProjectCreateResponse struct {
	ID           string    `json:"id"`
	Name         string    `json:"name"`
	OpenAIAPIKey string    `json:"openai_api_key"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

// TokenCreateRequest is the request body for creating a token.
type TokenCreateRequest struct {
	ProjectID       string `json:"project_id"`
	DurationMinutes int    `json:"duration_minutes"`
}

// TokenCreateResponse is the response body for a created token.
type TokenCreateResponse struct {
	Token     string    `json:"token"`
	ExpiresAt time.Time `json:"expires_at"`
}
