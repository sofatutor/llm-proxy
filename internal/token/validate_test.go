package token

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"
)

// MockTokenStore implements TokenStore interface for testing
type MockTokenStore struct {
	tokens      map[string]TokenData
	tokenExists bool
	failOnGet   bool
	failOnIncr  bool
	mutex       sync.RWMutex // Add mutex for thread safety
}

func NewMockTokenStore() *MockTokenStore {
	return &MockTokenStore{
		tokens:      make(map[string]TokenData),
		tokenExists: true,
		failOnGet:   false,
		failOnIncr:  false,
	}
}

func (m *MockTokenStore) GetTokenByID(ctx context.Context, tokenID string) (TokenData, error) {
	m.mutex.RLock()
	defer m.mutex.RUnlock()
	if m.failOnGet {
		return TokenData{}, errors.New("mock store failure")
	}

	if !m.tokenExists {
		return TokenData{}, ErrTokenNotFound
	}

	if token, exists := m.tokens[tokenID]; exists {
		return token, nil
	}

	return TokenData{}, ErrTokenNotFound
}

func (m *MockTokenStore) IncrementTokenUsage(ctx context.Context, tokenID string) error {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	if m.failOnIncr {
		return errors.New("mock increment failure")
	}

	if !m.tokenExists {
		return ErrTokenNotFound
	}

	token, exists := m.tokens[tokenID]
	if exists {
		token.RequestCount++
		token.LastUsedAt = func() *time.Time { t := time.Now(); return &t }()
		m.tokens[tokenID] = token
	}

	return nil
}

func (m *MockTokenStore) AddToken(tokenID string, data TokenData) {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	m.tokens[tokenID] = data
}

func (m *MockTokenStore) CreateToken(ctx context.Context, td TokenData) error {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	m.tokens[td.Token] = td
	return nil
}

func (m *MockTokenStore) GetTokensByProjectID(ctx context.Context, projectID string) ([]TokenData, error) {
	m.mutex.RLock()
	defer m.mutex.RUnlock()
	var tokens []TokenData
	for _, t := range m.tokens {
		if t.ProjectID == projectID {
			tokens = append(tokens, t)
		}
	}
	return tokens, nil
}

func (m *MockTokenStore) ListTokens(ctx context.Context) ([]TokenData, error) {
	m.mutex.RLock()
	defer m.mutex.RUnlock()
	tokens := make([]TokenData, 0, len(m.tokens))
	for _, t := range m.tokens {
		tokens = append(tokens, t)
	}
	return tokens, nil
}

func TestTokenDataIsValid(t *testing.T) {
	now := time.Now()
	future := now.Add(1 * time.Hour)
	past := now.Add(-1 * time.Hour)

	requestLimit := 100

	validToken, _ := GenerateToken()
	inactiveToken, _ := GenerateToken()
	expiredToken, _ := GenerateToken()
	ratelimitedToken, _ := GenerateToken()
	validWithExpiryToken, _ := GenerateToken()

	tests := []struct {
		name        string
		token       TokenData
		expectValid bool
	}{
		{
			name: "Valid active token with no expiration or limits",
			token: TokenData{
				Token:        validToken,
				ProjectID:    "project1",
				IsActive:     true,
				RequestCount: 0,
				MaxRequests:  nil,
				ExpiresAt:    nil,
			},
			expectValid: true,
		},
		{
			name: "Inactive token",
			token: TokenData{
				Token:        inactiveToken,
				ProjectID:    "project1",
				IsActive:     false,
				RequestCount: 0,
			},
			expectValid: false,
		},
		{
			name: "Expired token",
			token: TokenData{
				Token:        expiredToken,
				ProjectID:    "project1",
				IsActive:     true,
				RequestCount: 0,
				ExpiresAt:    &past,
			},
			expectValid: false,
		},
		{
			name: "Rate-limited token",
			token: TokenData{
				Token:        ratelimitedToken,
				ProjectID:    "project1",
				IsActive:     true,
				RequestCount: 100,
				MaxRequests:  &requestLimit,
			},
			expectValid: false,
		},
		{
			name: "Valid token with future expiration and under limit",
			token: TokenData{
				Token:        validWithExpiryToken,
				ProjectID:    "project1",
				IsActive:     true,
				RequestCount: 50,
				MaxRequests:  &requestLimit,
				ExpiresAt:    &future,
			},
			expectValid: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if valid := tt.token.IsValid(); valid != tt.expectValid {
				t.Errorf("TokenData.IsValid() = %v, want %v", valid, tt.expectValid)
			}
		})
	}
}

func TestStandardValidator_ValidateToken(t *testing.T) {
	ctx := context.Background()
	store := NewMockTokenStore()
	validator := NewValidator(store)

	// Add some test tokens to the store
	now := time.Now()
	future := now.Add(1 * time.Hour)
	past := now.Add(-1 * time.Hour)
	requestLimit := 100

	validToken, _ := GenerateToken()
	inactiveToken, _ := GenerateToken()
	expiredToken, _ := GenerateToken()
	ratelimitedToken, _ := GenerateToken()

	// Valid token
	store.AddToken(validToken, TokenData{
		Token:        validToken,
		ProjectID:    "project1",
		IsActive:     true,
		RequestCount: 50,
		MaxRequests:  &requestLimit,
		ExpiresAt:    &future,
		CreatedAt:    now,
	})

	// Inactive token
	store.AddToken(inactiveToken, TokenData{
		Token:        inactiveToken,
		ProjectID:    "project2",
		IsActive:     false,
		RequestCount: 0,
		CreatedAt:    now,
	})

	// Expired token
	store.AddToken(expiredToken, TokenData{
		Token:        expiredToken,
		ProjectID:    "project3",
		IsActive:     true,
		RequestCount: 0,
		ExpiresAt:    &past,
		CreatedAt:    now,
	})

	// Rate-limited token
	store.AddToken(ratelimitedToken, TokenData{
		Token:        ratelimitedToken,
		ProjectID:    "project4",
		IsActive:     true,
		RequestCount: 100,
		MaxRequests:  &requestLimit,
		CreatedAt:    now,
	})

	tests := []struct {
		name      string
		tokenID   string
		wantErr   bool
		wantErrIs error
	}{
		{
			name:      "Valid token",
			tokenID:   validToken,
			wantErr:   false,
			wantErrIs: nil,
		},
		{
			name:      "Inactive token",
			tokenID:   inactiveToken,
			wantErr:   true,
			wantErrIs: ErrTokenInactive,
		},
		{
			name:      "Expired token",
			tokenID:   expiredToken,
			wantErr:   true,
			wantErrIs: ErrTokenExpired,
		},
		{
			name:      "Rate-limited token",
			tokenID:   ratelimitedToken,
			wantErr:   true,
			wantErrIs: ErrTokenRateLimit,
		},
		{
			name:      "Invalid format token",
			tokenID:   "invalid-token-format",
			wantErr:   true,
			wantErrIs: nil, // We don't check specific error type here
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			projectID, err := validator.ValidateToken(ctx, tt.tokenID)
			if (err != nil) != tt.wantErr {
				t.Errorf("StandardValidator.ValidateToken() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if err != nil && tt.wantErrIs != nil && !errors.Is(err, tt.wantErrIs) {
				t.Errorf("StandardValidator.ValidateToken() error = %v, want %v", err, tt.wantErrIs)
			}

			if err == nil && projectID == "" {
				t.Errorf("StandardValidator.ValidateToken() returned empty projectID for valid token")
			}
		})
	}

	// Test store failure
	t.Run("Store failure", func(t *testing.T) {
		store.failOnGet = true
		defer func() { store.failOnGet = false }()

		_, err := validator.ValidateToken(ctx, validToken)
		if err == nil {
			t.Errorf("StandardValidator.ValidateToken() expected error on store failure")
		}
	})

	// Test token not found
	t.Run("Token not found", func(t *testing.T) {
		store.tokenExists = false
		defer func() { store.tokenExists = true }()

		_, err := validator.ValidateToken(ctx, validToken)
		if !errors.Is(err, ErrTokenNotFound) {
			t.Errorf("StandardValidator.ValidateToken() error = %v, want %v", err, ErrTokenNotFound)
		}
	})
}

func TestStandardValidator_ValidateTokenWithTracking(t *testing.T) {
	ctx := context.Background()
	store := NewMockTokenStore()
	validator := NewValidator(store)

	// Add a valid token to the store
	now := time.Now()
	future := now.Add(1 * time.Hour)
	requestLimit := 100

	validToken, _ := GenerateToken()
	store.AddToken(validToken, TokenData{
		Token:        validToken,
		ProjectID:    "project1",
		IsActive:     true,
		RequestCount: 50,
		MaxRequests:  &requestLimit,
		ExpiresAt:    &future,
		CreatedAt:    now,
	})

	// Test successful tracking
	t.Run("Successful validation with tracking", func(t *testing.T) {
		projectID, err := validator.ValidateTokenWithTracking(ctx, validToken)
		if err != nil {
			t.Errorf("StandardValidator.ValidateTokenWithTracking() error = %v", err)
			return
		}

		if projectID != "project1" {
			t.Errorf("StandardValidator.ValidateTokenWithTracking() projectID = %v, want %v", projectID, "project1")
		}

		token, err := store.GetTokenByID(ctx, validToken)
		if err != nil {
			t.Errorf("GetTokenByID() error = %v", err)
			return
		}

		if token.RequestCount != 51 {
			t.Errorf("Token RequestCount = %v, want %v", token.RequestCount, 51)
		}

		if token.LastUsedAt == nil {
			t.Errorf("Token LastUsedAt is nil, expected non-nil")
		}
	})

	// Test increment failure
	t.Run("Increment failure", func(t *testing.T) {
		store.failOnIncr = true
		defer func() { store.failOnIncr = false }()

		_, err := validator.ValidateTokenWithTracking(ctx, validToken)
		if err == nil {
			t.Errorf("StandardValidator.ValidateTokenWithTracking() expected error on increment failure")
		}
	})

	// Test validation failure
	t.Run("Validation failure", func(t *testing.T) {
		_, err := validator.ValidateTokenWithTracking(ctx, "invalid-token-format")
		if err == nil {
			t.Errorf("StandardValidator.ValidateTokenWithTracking() expected error on validation failure")
		}
	})
}

func TestTokenData_ValidateFormat(t *testing.T) {
	validToken, _ := GenerateToken()
	invalidToken := "invalid-token-format"

	tests := []struct {
		token   TokenData
		wantErr bool
	}{
		{TokenData{Token: validToken}, false},
		{TokenData{Token: invalidToken}, true},
	}
	for _, tt := range tests {
		err := tt.token.ValidateFormat()
		if (err != nil) != tt.wantErr {
			t.Errorf("TokenData.ValidateFormat() error = %v, wantErr %v", err, tt.wantErr)
		}
	}
}

func TestStandardValidator_ValidateToken_Coverage(t *testing.T) {
	store := NewMockTokenStore()
	validator := NewValidator(store)
	ctx := context.Background()
	_, err := validator.ValidateToken(ctx, "dummy")
	if err == nil {
		t.Error("expected error for ValidateToken with dummy token")
	}
	_, err = validator.ValidateTokenWithTracking(ctx, "dummy")
	if err == nil {
		t.Error("expected error for ValidateTokenWithTracking with dummy token")
	}
}

func TestStandardValidator_ValidateToken_AllBranches(t *testing.T) {
	ctx := context.Background()
	store := NewMockTokenStore()
	validator := NewValidator(store)

	now := time.Now()
	future := now.Add(1 * time.Hour)
	past := now.Add(-1 * time.Hour)
	maxReq := 2

	validToken, _ := GenerateToken()
	inactiveToken, _ := GenerateToken()
	expiredToken, _ := GenerateToken()
	ratelimitedToken, _ := GenerateToken()
	badFormatToken := "badtoken"
	missingToken, _ := GenerateToken() // valid format, not in store

	store.AddToken(validToken, TokenData{
		Token:        validToken,
		ProjectID:    "p",
		IsActive:     true,
		ExpiresAt:    &future,
		RequestCount: 0,
		MaxRequests:  &maxReq,
		CreatedAt:    now,
	})
	store.AddToken(inactiveToken, TokenData{
		Token:     inactiveToken,
		ProjectID: "p",
		IsActive:  false,
		CreatedAt: now,
	})
	store.AddToken(expiredToken, TokenData{
		Token:     expiredToken,
		ProjectID: "p",
		IsActive:  true,
		ExpiresAt: &past,
		CreatedAt: now,
	})
	store.AddToken(ratelimitedToken, TokenData{
		Token:        ratelimitedToken,
		ProjectID:    "p",
		IsActive:     true,
		ExpiresAt:    &future,
		RequestCount: 2,
		MaxRequests:  &maxReq,
		CreatedAt:    now,
	})

	tests := []struct {
		name    string
		tokenID string
		wantErr error
	}{
		{"valid", validToken, nil},
		{"inactive", inactiveToken, ErrTokenInactive},
		{"expired", expiredToken, ErrTokenExpired},
		{"ratelimited", ratelimitedToken, ErrTokenRateLimit},
		{"notfound", missingToken, ErrTokenNotFound},
		{"bad format", badFormatToken, ErrInvalidTokenFormat},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := validator.ValidateToken(ctx, tt.tokenID)
			if tt.wantErr == nil && err != nil {
				t.Errorf("unexpected error: %v", err)
			} else if tt.wantErr != nil && !errors.Is(err, tt.wantErr) {
				t.Errorf("expected error %v, got %v", tt.wantErr, err)
			}
		})
	}

	t.Run("store error", func(t *testing.T) {
		store.failOnGet = true
		defer func() { store.failOnGet = false }()
		_, err := validator.ValidateToken(ctx, validToken)
		if err == nil || !strings.Contains(err.Error(), "mock store failure") {
			t.Errorf("expected store failure, got %v", err)
		}
	})
}

func TestStandardValidator_ValidateTokenWithTracking_AllBranches(t *testing.T) {
	ctx := context.Background()
	store := NewMockTokenStore()
	validator := NewValidator(store)

	now := time.Now()
	future := now.Add(1 * time.Hour)
	maxReq := 2
	validToken, _ := GenerateToken()
	missingToken, _ := GenerateToken() // valid format, not in store
	store.AddToken(validToken, TokenData{
		Token:        validToken,
		ProjectID:    "p",
		IsActive:     true,
		ExpiresAt:    &future,
		RequestCount: 0,
		MaxRequests:  &maxReq,
		CreatedAt:    now,
	})

	t.Run("happy path", func(t *testing.T) {
		_, err := validator.ValidateTokenWithTracking(ctx, validToken)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})
	t.Run("increment error", func(t *testing.T) {
		store.failOnIncr = true
		defer func() { store.failOnIncr = false }()
		_, err := validator.ValidateTokenWithTracking(ctx, validToken)
		if err == nil || !strings.Contains(err.Error(), "mock increment failure") {
			t.Errorf("expected increment failure, got %v", err)
		}
	})
	t.Run("validate error", func(t *testing.T) {
		_, err := validator.ValidateTokenWithTracking(ctx, missingToken)
		if !errors.Is(err, ErrTokenNotFound) {
			t.Errorf("expected notfound error, got %v", err)
		}
	})
}
