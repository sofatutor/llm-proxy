package token

import (
	"context"
	"errors"
	"fmt"
	"time"
)

// Errors related to token validation
var (
	ErrTokenNotFound  = errors.New("token not found")
	ErrTokenInactive  = errors.New("token is inactive")
	ErrTokenExpired   = errors.New("token has expired")
	ErrTokenRateLimit = errors.New("token has reached rate limit")
)

// TokenValidator defines the interface for token validation
type TokenValidator interface {
	// ValidateToken validates a token and returns the associated project ID
	ValidateToken(ctx context.Context, token string) (string, error)

	// ValidateTokenWithTracking validates a token, returns the project ID, and tracks usage
	ValidateTokenWithTracking(ctx context.Context, token string) (string, error)
}

// TokenStore defines the interface for token storage and retrieval
type TokenStore interface {
	// GetTokenByID retrieves a token by its UUID (for management operations)
	GetTokenByID(ctx context.Context, id string) (TokenData, error)

	// GetTokenByToken retrieves a token by its token string (for authentication)
	GetTokenByToken(ctx context.Context, tokenString string) (TokenData, error)

	// IncrementTokenUsage increments the usage count for a token by its token string
	IncrementTokenUsage(ctx context.Context, tokenString string) error

	// CreateToken creates a new token in the store
	CreateToken(ctx context.Context, token TokenData) error

	// UpdateToken updates an existing token
	UpdateToken(ctx context.Context, token TokenData) error

	// ListTokens retrieves all tokens from the store
	ListTokens(ctx context.Context) ([]TokenData, error)

	// GetTokensByProjectID retrieves all tokens for a project
	GetTokensByProjectID(ctx context.Context, projectID string) ([]TokenData, error)
}

// TokenData represents the data associated with a token
type TokenData struct {
	ID            string     // The token ID (UUID) - used for management operations
	Token         string     // The token string (sk-...) - used for authentication
	ProjectID     string     // The associated project ID
	ExpiresAt     *time.Time // When the token expires (nil for no expiration)
	IsActive      bool       // Whether the token is active
	DeactivatedAt *time.Time // When the token was deactivated (nil if not deactivated)
	RequestCount  int        // Number of requests made with this token
	MaxRequests   *int       // Maximum number of requests allowed (nil for unlimited)
	CreatedAt     time.Time  // When the token was created
	LastUsedAt    *time.Time // When the token was last used (nil if never used)
	CacheHitCount int        // Number of cache hits for this token
}

// IsValid returns true if the token is active, not expired, and not rate limited
func (t *TokenData) IsValid() bool {
	return t.IsActive && !IsExpired(t.ExpiresAt) && !t.IsRateLimited()
}

// IsRateLimited returns true if the token has reached its maximum number of requests
func (t *TokenData) IsRateLimited() bool {
	if t.MaxRequests == nil {
		return false
	}
	return t.RequestCount >= *t.MaxRequests
}

// ValidateTokenFormat checks if a token has the correct format
func (t *TokenData) ValidateFormat() error {
	return ValidateTokenFormat(t.Token)
}

// StandardValidator is a validator that uses a TokenStore for validation
type StandardValidator struct {
	store TokenStore
}

// NewValidator creates a new StandardValidator with the given TokenStore
func NewValidator(store TokenStore) *StandardValidator {
	return &StandardValidator{
		store: store,
	}
}

// ValidateToken validates a token without incrementing usage
func (v *StandardValidator) ValidateToken(ctx context.Context, tokenString string) (string, error) {
	// First validate the token format
	if err := ValidateTokenFormat(tokenString); err != nil {
		return "", fmt.Errorf("invalid token format: %w", err)
	}

	// Retrieve the token from the store by token string
	tokenData, err := v.store.GetTokenByToken(ctx, tokenString)
	if err != nil {
		if errors.Is(err, ErrTokenNotFound) {
			return "", ErrTokenNotFound
		}
		return "", fmt.Errorf("failed to retrieve token: %w", err)
	}

	// Check if the token is active
	if !tokenData.IsActive {
		return "", ErrTokenInactive
	}

	// Check if the token has expired
	if IsExpired(tokenData.ExpiresAt) {
		return "", ErrTokenExpired
	}

	// Check if the token has reached its rate limit
	if tokenData.IsRateLimited() {
		return "", ErrTokenRateLimit
	}

	// Token is valid, return the project ID
	return tokenData.ProjectID, nil
}

// ValidateTokenWithTracking validates a token and increments its usage count
func (v *StandardValidator) ValidateTokenWithTracking(ctx context.Context, tokenString string) (string, error) {
	// Validate the token first
	projectID, err := v.ValidateToken(ctx, tokenString)
	if err != nil {
		return "", err
	}

	// Increment the token usage by token string
	if err := v.store.IncrementTokenUsage(ctx, tokenString); err != nil {
		return "", fmt.Errorf("failed to track token usage: %w", err)
	}

	return projectID, nil
}

// ValidateTokenFormat checks if a token string has the correct format
func ValidateToken(ctx context.Context, validator TokenValidator, tokenID string) (string, error) {
	return validator.ValidateToken(ctx, tokenID)
}

// ValidateTokenWithTracking validates a token and tracks its usage
func ValidateTokenWithTracking(ctx context.Context, validator TokenValidator, tokenID string) (string, error) {
	return validator.ValidateTokenWithTracking(ctx, tokenID)
}
