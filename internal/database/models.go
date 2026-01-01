package database

import (
	"time"
)

// Project represents a project in the database.
type Project struct {
	ID            string     `json:"id"`
	Name          string     `json:"name"`
	APIKey        string     `json:"-"` // Sensitive data, not included in JSON. Encrypted when ENCRYPTION_KEY is set.
	IsActive      bool       `json:"is_active"`
	DeactivatedAt *time.Time `json:"deactivated_at,omitempty"`
	CreatedAt     time.Time  `json:"created_at"`
	UpdatedAt     time.Time  `json:"updated_at"`
}

// Token represents a token in the database.
type Token struct {
	ID            string     `json:"id"`
	Token         string     `json:"token"`
	ProjectID     string     `json:"project_id"`
	ExpiresAt     *time.Time `json:"expires_at,omitempty"`
	IsActive      bool       `json:"is_active"`
	DeactivatedAt *time.Time `json:"deactivated_at,omitempty"`
	RequestCount  int        `json:"request_count"`
	MaxRequests   *int       `json:"max_requests,omitempty"`
	CreatedAt     time.Time  `json:"created_at"`
	LastUsedAt    *time.Time `json:"last_used_at,omitempty"`
	CacheHitCount int        `json:"cache_hit_count"`
}

// AuditEvent represents an audit log entry in the database.
type AuditEvent struct {
	ID            string    `json:"id"`
	Timestamp     time.Time `json:"timestamp"`
	Action        string    `json:"action"`
	Actor         string    `json:"actor"`
	ProjectID     *string   `json:"project_id,omitempty"`
	RequestID     *string   `json:"request_id,omitempty"`
	CorrelationID *string   `json:"correlation_id,omitempty"`
	ClientIP      *string   `json:"client_ip,omitempty"`
	Method        *string   `json:"method,omitempty"`
	Path          *string   `json:"path,omitempty"`
	UserAgent     *string   `json:"user_agent,omitempty"`
	Outcome       string    `json:"outcome"`
	Reason        *string   `json:"reason,omitempty"`
	TokenID       *string   `json:"token_id,omitempty"`
	Metadata      *string   `json:"metadata,omitempty"` // JSON string
}

// IsExpired returns true if the token has expired.
func (t *Token) IsExpired() bool {
	if t.ExpiresAt == nil {
		return false
	}
	return time.Now().After(*t.ExpiresAt)
}

// IsRateLimited returns true if the token has reached its maximum number of requests.
func (t *Token) IsRateLimited() bool {
	if t.MaxRequests == nil {
		return false
	}
	return t.RequestCount >= *t.MaxRequests
}

// IsValid returns true if the token is active, not expired, and not rate limited.
func (t *Token) IsValid() bool {
	return t.IsActive && !t.IsExpired() && !t.IsRateLimited()
}

// IsDeactivated returns true if the token has been explicitly deactivated.
func (t *Token) IsDeactivated() bool {
	return t.DeactivatedAt != nil
}

// IsDeactivated returns true if the project has been explicitly deactivated.
func (p *Project) IsDeactivated() bool {
	return p.DeactivatedAt != nil
}
