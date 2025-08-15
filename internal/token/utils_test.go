package token

import (
	"net/http"
	"strings"
	"testing"
	"time"
)

func TestTokenGenerator(t *testing.T) {
	// Create a token generator
	generator := NewTokenGenerator()

	// Test generating a token with default options
	token, err := generator.Generate()
	if err != nil {
		t.Fatalf("Failed to generate token: %v", err)
	}

	// Check that the token format is valid
	if err := ValidateTokenFormat(token); err != nil {
		t.Errorf("Generated token has invalid format: %v", err)
	}

	// Test generating a token with custom expiration
	customExpiration := 1 * time.Hour
	customGenerator := generator.WithExpiration(customExpiration)

	token2, expiresAt, maxRequests, err := customGenerator.GenerateWithOptions(0, nil)
	if err != nil {
		t.Fatalf("Failed to generate token with options: %v", err)
	}

	// Check that the token format is valid
	if err := ValidateTokenFormat(token2); err != nil {
		t.Errorf("Generated token has invalid format: %v", err)
	}

	// Check that the expiration time is set correctly
	if expiresAt == nil {
		t.Errorf("Expected expiration time, got nil")
	} else {
		expectedExpiry := time.Now().Add(customExpiration)
		timeDiff := expectedExpiry.Sub(*expiresAt)
		if timeDiff < -1*time.Second || timeDiff > 1*time.Second {
			t.Errorf("Expiration time is not close to expected: got %v, want %v (diff: %v)",
				*expiresAt, expectedExpiry, timeDiff)
		}
	}

	// Check that the max requests is still nil (unlimited)
	if maxRequests != nil {
		t.Errorf("Expected nil max requests, got %v", *maxRequests)
	}

	// Test with custom max requests
	maxReq := 100
	customGenerator = generator.WithMaxRequests(maxReq)

	_, _, maxRequests, err = customGenerator.GenerateWithOptions(0, nil)
	if err != nil {
		t.Fatalf("Failed to generate token with options: %v", err)
	}

	// Check that the max requests is set correctly
	if maxRequests == nil {
		t.Errorf("Expected max requests to be set, got nil")
	} else if *maxRequests != maxReq {
		t.Errorf("Expected max requests to be %d, got %d", maxReq, *maxRequests)
	}
}

func TestExtractTokenFromHeader(t *testing.T) {
	validToken, _ := GenerateToken()
	tests := []struct {
		name        string
		header      string
		wantToken   string
		wantSuccess bool
	}{
		{
			name:        "Valid Bearer token",
			header:      "Bearer " + validToken,
			wantToken:   validToken,
			wantSuccess: true,
		},
		{
			name:        "Invalid token format",
			header:      "Bearer invalidtoken",
			wantToken:   "",
			wantSuccess: false,
		},
		{
			name:        "Missing Bearer prefix",
			header:      validToken,
			wantToken:   "",
			wantSuccess: false,
		},
		{
			name:        "Empty header",
			header:      "",
			wantToken:   "",
			wantSuccess: false,
		},
		{
			name:        "Wrong scheme",
			header:      "Basic dXNlcjpwYXNz",
			wantToken:   "",
			wantSuccess: false,
		},
		{
			name:        "Empty token",
			header:      "Bearer ",
			wantToken:   "",
			wantSuccess: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotToken, gotSuccess := ExtractTokenFromHeader(tt.header)
			if gotSuccess != tt.wantSuccess {
				t.Errorf("ExtractTokenFromHeader() success = %v, want %v", gotSuccess, tt.wantSuccess)
			}
			if gotToken != tt.wantToken {
				t.Errorf("ExtractTokenFromHeader() token = %v, want %v", gotToken, tt.wantToken)
			}
		})
	}
}

func TestGenerateWithOptions_EdgeCases(t *testing.T) {
	tests := []struct {
		name           string
		setupGen       func() *TokenGenerator
		expiration     time.Duration
		maxRequests    *int
		expectExpiry   bool
		expectMaxReq   bool
		expectedMaxReq *int
	}{
		{
			name:         "Zero expiration with generator default",
			setupGen:     func() *TokenGenerator { return NewTokenGenerator().WithExpiration(time.Hour) },
			expiration:   0,
			maxRequests:  nil,
			expectExpiry: true, // Uses generator default
			expectMaxReq: false,
		},
		{
			name:         "Positive expiration overrides generator default",
			setupGen:     func() *TokenGenerator { return NewTokenGenerator().WithExpiration(time.Hour) },
			expiration:   2 * time.Hour, // Should override the 1 hour default
			maxRequests:  nil,
			expectExpiry: true,
			expectMaxReq: false,
		},
		{
			name:           "With max requests override",
			setupGen:       func() *TokenGenerator { return NewTokenGenerator().WithMaxRequests(50) },
			expiration:     time.Hour,
			maxRequests:    &[]int{100}[0],
			expectExpiry:   true,
			expectMaxReq:   true,
			expectedMaxReq: &[]int{100}[0],
		},
		{
			name:           "Use generator default max requests",
			setupGen:       func() *TokenGenerator { return NewTokenGenerator().WithMaxRequests(75) },
			expiration:     time.Hour,
			maxRequests:    nil,
			expectExpiry:   true,
			expectMaxReq:   true,
			expectedMaxReq: &[]int{75}[0],
		},
		{
			name:         "No expiration and no max requests",
			setupGen:     func() *TokenGenerator { return NewTokenGenerator() }, // Default generator
			expiration:   0,
			maxRequests:  nil,
			expectExpiry: true, // Will use DefaultTokenExpiration (30 days)
			expectMaxReq: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			generator := tt.setupGen()

			token, expiresAt, maxRequests, err := generator.GenerateWithOptions(tt.expiration, tt.maxRequests)
			if err != nil {
				t.Fatalf("GenerateWithOptions() error = %v", err)
			}

			// Validate token format
			if err := ValidateTokenFormat(token); err != nil {
				t.Errorf("Generated token has invalid format: %v", err)
			}

			// Check expiration
			if tt.expectExpiry && expiresAt == nil {
				t.Errorf("Expected expiration time, got nil")
			}
			if !tt.expectExpiry && expiresAt != nil {
				t.Errorf("Expected no expiration time, got %v", *expiresAt)
			}

			// Check max requests
			if tt.expectMaxReq && maxRequests == nil {
				t.Errorf("Expected max requests, got nil")
			}
			if !tt.expectMaxReq && maxRequests != nil {
				t.Errorf("Expected no max requests, got %v", *maxRequests)
			}

			// Verify specific values when expected
			if tt.expectedMaxReq != nil && maxRequests != nil && *maxRequests != *tt.expectedMaxReq {
				t.Errorf("Expected max requests %d, got %d", *tt.expectedMaxReq, *maxRequests)
			}
		})
	}
}

func TestExtractTokenFromRequest(t *testing.T) {
	// Generate a valid token for testing
	token, err := GenerateToken()
	if err != nil {
		t.Fatalf("Failed to generate token: %v", err)
	}

	// Create test request with Authorization header
	authHeaderReq, _ := http.NewRequest("GET", "https://example.com", nil)
	authHeaderReq.Header.Set("Authorization", "Bearer "+token)

	// Create test request with X-API-Key header
	apiKeyReq, _ := http.NewRequest("GET", "https://example.com", nil)
	apiKeyReq.Header.Set("X-API-Key", token)

	// Create test request with query parameter
	queryParamReq, _ := http.NewRequest("GET", "https://example.com?token="+token, nil)

	// Create test request with no token
	noTokenReq, _ := http.NewRequest("GET", "https://example.com", nil)

	// Create test request with invalid token
	invalidTokenReq, _ := http.NewRequest("GET", "https://example.com", nil)
	invalidTokenReq.Header.Set("Authorization", "Bearer invalidtoken")

	tests := []struct {
		name        string
		request     *http.Request
		wantToken   string
		wantSuccess bool
	}{
		{
			name:        "Auth header token",
			request:     authHeaderReq,
			wantToken:   token,
			wantSuccess: true,
		},
		{
			name:        "X-API-Key header token",
			request:     apiKeyReq,
			wantToken:   token,
			wantSuccess: true,
		},
		{
			name:        "Query parameter token",
			request:     queryParamReq,
			wantToken:   token,
			wantSuccess: true,
		},
		{
			name:        "No token",
			request:     noTokenReq,
			wantToken:   "",
			wantSuccess: false,
		},
		{
			name:        "Invalid token",
			request:     invalidTokenReq,
			wantToken:   "",
			wantSuccess: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotToken, gotSuccess := ExtractTokenFromRequest(tt.request)
			if gotSuccess != tt.wantSuccess {
				t.Errorf("ExtractTokenFromRequest() success = %v, want %v", gotSuccess, tt.wantSuccess)
			}
			if gotToken != tt.wantToken {
				t.Errorf("ExtractTokenFromRequest() token = %v, want %v", gotToken, tt.wantToken)
			}
		})
	}
}

func TestGenerateRandomKey(t *testing.T) {
	tests := []struct {
		name       string
		length     int
		wantMinLen int
		wantErr    bool
	}{
		{
			name:       "Standard length",
			length:     32,
			wantMinLen: 32,
			wantErr:    false,
		},
		{
			name:       "Short length (should use minimum)",
			length:     5,
			wantMinLen: MinTokenLength,
			wantErr:    false,
		},
		{
			name:       "Zero length (should use minimum)",
			length:     0,
			wantMinLen: MinTokenLength,
			wantErr:    false,
		},
		{
			name:       "Long length",
			length:     100,
			wantMinLen: 100,
			wantErr:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := GenerateRandomKey(tt.length)
			if (err != nil) != tt.wantErr {
				t.Errorf("GenerateRandomKey() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if len(got) < tt.wantMinLen {
				t.Errorf("GenerateRandomKey() returned key length %v, want at least %v", len(got), tt.wantMinLen)
			}

			// Check that the key only contains expected characters
			validChars := "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
			for _, c := range got {
				if !strings.ContainsRune(validChars, c) {
					t.Errorf("GenerateRandomKey() returned key with invalid character: %c", c)
				}
			}
		})
	}
}

func TestGenerateRandomKey_EdgeCases(t *testing.T) {
	key, err := GenerateRandomKey(0)
	if err != nil {
		t.Fatalf("GenerateRandomKey(0) error: %v", err)
	}
	if len(key) < MinTokenLength {
		t.Errorf("key too short: %d", len(key))
	}

	key2, err := GenerateRandomKey(32)
	if err != nil {
		t.Fatalf("GenerateRandomKey(32) error: %v", err)
	}
	if len(key2) != 32 {
		t.Errorf("key length: got %d, want 32", len(key2))
	}

	set := map[string]struct{}{key: {}, key2: {}}
	for i := 0; i < 10; i++ {
		k, err := GenerateRandomKey(24)
		if err != nil {
			t.Fatalf("GenerateRandomKey(24) error: %v", err)
		}
		if _, exists := set[k]; exists {
			t.Errorf("duplicate key generated: %s", k)
		}
		set[k] = struct{}{}
	}
}

func TestTruncateToken(t *testing.T) {
	validToken, _ := GenerateToken()
	showChars := 4
	shortToken := validToken[:showChars*2]
	longExpected := validToken[:showChars] + "..." + validToken[len(validToken)-showChars:]
	tests := []struct {
		name      string
		token     string
		showChars int
		want      string
	}{
		{"Short token (no truncation)", shortToken, showChars, shortToken},
		{"Long token", validToken, showChars, longExpected},
		{"Empty token", "", showChars, ""},
		{"Zero show chars", validToken, 0, "..."},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := TruncateToken(tt.token, tt.showChars)
			if got != tt.want {
				t.Errorf("TruncateToken() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestObfuscateToken(t *testing.T) {
	tests := []struct {
		name  string
		token string
		want  string
	}{
		{
			name:  "Standard token",
			token: "sk-abcdefghijklmnopqrst",
			want:  "sk-abcd************qrst",
		},
		{
			name:  "Short token (no obfuscation)",
			token: "sk-abcdefgh",
			want:  "sk-abcdefgh",
		},
		{
			name:  "Without prefix",
			token: "notprefix12345678901234",
			want:  "notprefix12345678901234", // No obfuscation for tokens without prefix
		},
		{
			name:  "Empty token",
			token: "",
			want:  "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ObfuscateToken(tt.token)
			if got != tt.want {
				t.Errorf("ObfuscateToken() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGetTokenInfo(t *testing.T) {
	// Generate a token for testing
	token, err := GenerateToken()
	if err != nil {
		t.Fatalf("Failed to generate token: %v", err)
	}

	now := time.Now()
	future := now.Add(1 * time.Hour)
	past := now.Add(-1 * time.Hour)
	lastUsed := now.Add(-30 * time.Minute)

	maxReq := 100

	tests := []struct {
		name      string
		tokenData TokenData
		wantValid bool
	}{
		{
			name: "Valid active token",
			tokenData: TokenData{
				Token:        token,
				ProjectID:    "project1",
				ExpiresAt:    &future,
				IsActive:     true,
				RequestCount: 50,
				MaxRequests:  &maxReq,
				CreatedAt:    now,
				LastUsedAt:   &lastUsed,
			},
			wantValid: true,
		},
		{
			name: "Expired token",
			tokenData: TokenData{
				Token:        token,
				ProjectID:    "project1",
				ExpiresAt:    &past,
				IsActive:     true,
				RequestCount: 50,
				MaxRequests:  &maxReq,
				CreatedAt:    now,
				LastUsedAt:   &lastUsed,
			},
			wantValid: false,
		},
		{
			name: "Inactive token",
			tokenData: TokenData{
				Token:        token,
				ProjectID:    "project1",
				ExpiresAt:    &future,
				IsActive:     false,
				RequestCount: 50,
				MaxRequests:  &maxReq,
				CreatedAt:    now,
				LastUsedAt:   &lastUsed,
			},
			wantValid: false,
		},
		{
			name: "Rate limited token",
			tokenData: TokenData{
				Token:        token,
				ProjectID:    "project1",
				ExpiresAt:    &future,
				IsActive:     true,
				RequestCount: 100,
				MaxRequests:  &maxReq,
				CreatedAt:    now,
				LastUsedAt:   &lastUsed,
			},
			wantValid: false,
		},
		{
			name: "No expiration token",
			tokenData: TokenData{
				Token:        token,
				ProjectID:    "project1",
				ExpiresAt:    nil,
				IsActive:     true,
				RequestCount: 50,
				MaxRequests:  &maxReq,
				CreatedAt:    now,
				LastUsedAt:   &lastUsed,
			},
			wantValid: true,
		},
		{
			name: "No max requests token",
			tokenData: TokenData{
				Token:        token,
				ProjectID:    "project1",
				ExpiresAt:    &future,
				IsActive:     true,
				RequestCount: 1000,
				MaxRequests:  nil,
				CreatedAt:    now,
				LastUsedAt:   &lastUsed,
			},
			wantValid: true,
		},
		{
			name: "Never used token",
			tokenData: TokenData{
				Token:        token,
				ProjectID:    "project1",
				ExpiresAt:    &future,
				IsActive:     true,
				RequestCount: 0,
				MaxRequests:  &maxReq,
				CreatedAt:    now,
				LastUsedAt:   nil,
			},
			wantValid: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			info, err := GetTokenInfo(tt.tokenData)
			if err != nil {
				t.Fatalf("GetTokenInfo() error = %v", err)
			}

			if info.IsValid != tt.wantValid {
				t.Errorf("GetTokenInfo().IsValid = %v, want %v", info.IsValid, tt.wantValid)
			}

			// Check that the obfuscated token is different from the original
			if info.Token == info.ObfuscatedToken && len(info.Token) > 12 {
				t.Errorf("GetTokenInfo() did not obfuscate the token")
			}

			// Check time remaining for tokens with expiration
			if tt.tokenData.ExpiresAt != nil && !IsExpired(tt.tokenData.ExpiresAt) {
				if info.TimeRemaining == "" {
					t.Errorf("GetTokenInfo() time remaining is empty for valid token")
				}
			}
		})
	}
}

func TestFormatTokenInfo(t *testing.T) {
	// Generate a token for testing
	token, err := GenerateToken()
	if err != nil {
		t.Fatalf("Failed to generate token: %v", err)
	}

	now := time.Now()
	future := now.Add(1 * time.Hour)
	lastUsed := now.Add(-30 * time.Minute)
	maxReq := 100

	// Create a token data
	tokenData := TokenData{
		Token:        token,
		ProjectID:    "project1",
		ExpiresAt:    &future,
		IsActive:     true,
		RequestCount: 50,
		MaxRequests:  &maxReq,
		CreatedAt:    now,
		LastUsedAt:   &lastUsed,
	}

	// Format the token info
	formatted := FormatTokenInfo(tokenData)

	// Check that the formatted string contains all the expected information
	expectedParts := []string{
		"Token:",
		"Created:",
		"Expires:",
		"Active: true",
		"Requests: 50 / 100",
		"Last Used:",
		"Valid: true",
	}

	for _, part := range expectedParts {
		if !strings.Contains(formatted, part) {
			t.Errorf("FormatTokenInfo() does not contain expected part: %s", part)
		}
	}

	// Test with different token to make sure each field is formatted correctly
	noExpiry := TokenData{
		Token:        token,
		ProjectID:    "project1",
		ExpiresAt:    nil,
		IsActive:     false,
		RequestCount: 200,
		MaxRequests:  nil,
		CreatedAt:    now,
		LastUsedAt:   nil,
	}

	formatted = FormatTokenInfo(noExpiry)

	// Check specific parts for the no-expiry token
	expectedParts = []string{
		"Expires: Never",
		"Active: false",
		"Requests: 200 / ∞",
		"Last Used: Never",
		"Valid: false",
	}

	for _, part := range expectedParts {
		if !strings.Contains(formatted, part) {
			t.Errorf("FormatTokenInfo() does not contain expected part: %s", part)
		}
	}
}

func TestFormatTokenInfo_EdgeCases(t *testing.T) {
	now := time.Now()
	future := now.Add(time.Hour)
	past := now.Add(-time.Hour)
	max := 2

	tokens := []TokenData{
		{Token: "sk-abc", ProjectID: "p", ExpiresAt: nil, IsActive: true, RequestCount: 0, MaxRequests: nil, CreatedAt: now, LastUsedAt: nil},
		{Token: "sk-def", ProjectID: "p", ExpiresAt: &future, IsActive: false, RequestCount: 0, MaxRequests: &max, CreatedAt: now, LastUsedAt: nil},
		{Token: "sk-ghi", ProjectID: "p", ExpiresAt: &past, IsActive: true, RequestCount: 0, MaxRequests: &max, CreatedAt: now, LastUsedAt: nil},
		{Token: "sk-jkl", ProjectID: "p", ExpiresAt: &future, IsActive: true, RequestCount: 2, MaxRequests: &max, CreatedAt: now, LastUsedAt: nil},
	}
	for _, td := range tokens {
		out := FormatTokenInfo(td)
		if out == "" {
			t.Errorf("FormatTokenInfo() returned empty string for %+v", td)
		}
		if !strings.Contains(out, "Token: ") {
			t.Errorf("FormatTokenInfo() missing Token: for %+v", td)
		}
	}
}

func TestFormatDuration(t *testing.T) {
	tests := []struct {
		dur      time.Duration
		expected string
	}{
		{30 * time.Second, "30 seconds"},
		{90 * time.Second, "1 minutes"},
		{2 * time.Hour, "2 hours"},
		{48 * time.Hour, "2 days"},
		{40 * 24 * time.Hour, "1 months"},
		{400 * 24 * time.Hour, "1 years"},
	}
	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			got := formatDuration(tt.dur)
			if got != tt.expected {
				t.Errorf("formatDuration(%v) = %q, want %q", tt.dur, got, tt.expected)
			}
		})
	}
}
