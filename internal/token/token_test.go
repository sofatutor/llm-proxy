package token

import (
	"strings"
	"testing"
)

func TestGenerateToken(t *testing.T) {
	tests := []struct {
		name           string
		expectPrefix   string
		expectedFormat string
	}{
		{
			name:           "Generate standard token",
			expectPrefix:   TokenPrefix,
			expectedFormat: TokenRegexPattern,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			token, err := GenerateToken()
			if err != nil {
				t.Fatalf("GenerateToken() error = %v", err)
			}

			// Verify prefix
			if !strings.HasPrefix(token, tt.expectPrefix) {
				t.Errorf("GenerateToken() prefix = %v, want %v", token[:len(tt.expectPrefix)], tt.expectPrefix)
			}

			// Verify format with regex
			if !TokenRegex.MatchString(token) {
				t.Errorf("GenerateToken() format invalid, token = %v, expected format %v", token, tt.expectedFormat)
			}

			// Verify length is reasonable
			expectedLength := len(tt.expectPrefix) + 22 // Prefix + 22 chars of base64
			if len(token) < expectedLength-2 || len(token) > expectedLength+2 {
				t.Errorf("GenerateToken() length = %v, expected approximately %v", len(token), expectedLength)
			}
		})
	}
}

func TestValidateTokenFormat(t *testing.T) {
	tests := []struct {
		name    string
		token   string
		wantErr bool
	}{
		{
			name:    "Valid token format",
			token:   "tkn_dGhpcyBpcyBhIHZhbGlkIHRva2Vu", // "this is a valid token" encoded
			wantErr: true,                               // Will fail decode step
		},
		{
			name:    "Invalid prefix",
			token:   "invalid_dGhpcyBpcyBhIHRva2Vu",
			wantErr: true,
		},
		{
			name:    "Too short",
			token:   "tkn_short",
			wantErr: true,
		},
		{
			name:    "Invalid characters",
			token:   "tkn_this!has@invalid#chars$",
			wantErr: true,
		},
		{
			name:    "Empty string",
			token:   "",
			wantErr: true,
		},
	}

	// Also test with a real generated token
	validToken, err := GenerateToken()
	if err != nil {
		t.Fatalf("Failed to generate token for test: %v", err)
	}
	tests = append(tests, struct {
		name    string
		token   string
		wantErr bool
	}{
		"Real generated token",
		validToken,
		false,
	})

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateTokenFormat(tt.token)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateTokenFormat() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestDecodeToken(t *testing.T) {
	// Generate a token to decode
	token, err := GenerateToken()
	if err != nil {
		t.Fatalf("Failed to generate token for test: %v", err)
	}

	tests := []struct {
		name    string
		token   string
		wantErr bool
	}{
		{
			name:    "Valid token",
			token:   token,
			wantErr: false,
		},
		{
			name:    "Invalid token format",
			token:   "not_a_valid_token",
			wantErr: true,
		},
		{
			name:    "Token with correct prefix but invalid base64",
			token:   "tkn_not-valid-base64!@#$",
			wantErr: true,
		},
		{
			name:    "Empty string",
			token:   "",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			uuid, err := DecodeToken(tt.token)
			if (err != nil) != tt.wantErr {
				t.Errorf("DecodeToken() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			// For valid tokens, ensure UUID is not empty
			if !tt.wantErr && uuid.String() == "00000000-0000-0000-0000-000000000000" {
				t.Errorf("DecodeToken() = empty UUID, want non-empty")
			}
		})
	}
}

func TestTokenGeneration_Multiple(t *testing.T) {
	// Generate multiple tokens and ensure they're all unique
	tokenCount := 100
	tokens := make(map[string]bool)

	for i := 0; i < tokenCount; i++ {
		token, err := GenerateToken()
		if err != nil {
			t.Fatalf("GenerateToken() error on iteration %d: %v", i, err)
		}

		if tokens[token] {
			t.Errorf("Duplicate token generated: %s", token)
		}
		tokens[token] = true

		// Ensure each token passes validation
		if err := ValidateTokenFormat(token); err != nil {
			t.Errorf("Generated token failed validation: %v", err)
		}
	}
}
