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
			token:   "sk-dGhpcyBpcyBhIHZhbGlkIHRva2Vu", // "this is a valid token" encoded
			wantErr: true,                              // Will fail decode step
		},
		{
			name:    "Invalid prefix",
			token:   "invalid_dGhpcyBpcyBhIHRva2Vu",
			wantErr: true,
		},
		{
			name:    "Too short",
			token:   "sk-short",
			wantErr: true,
		},
		{
			name:    "Invalid characters",
			token:   "sk-this!has@invalid#chars$",
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
			token:   "sk-not-valid-base64!@#$",
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

func TestDecodeToken_EdgeCases(t *testing.T) {
	valid, _ := GenerateToken()
	tests := []struct {
		name    string
		token   string
		wantErr bool
	}{
		{"missing prefix", "notasktoken", true},
		{"invalid base64", "sk-!@#$%^&*()", true},
		{"invalid uuid bytes", "sk-AAAAAAAAAA", true}, // not a valid UUID (too short)
		{"valid token", valid, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := DecodeToken(tt.token)
			if (err != nil) != tt.wantErr {
				t.Errorf("DecodeToken() error = %v, wantErr %v", err, tt.wantErr)
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

func TestValidateTokenFormat_EdgeCases(t *testing.T) {
	tests := []struct {
		name    string
		token   string
		wantErr bool
	}{
		{
			name:    "Token with invalid base64 but correct regex format",
			token:   "sk-invalidBase64WithValidLength123456",
			wantErr: true,
		},
		{
			name:    "Token too short for regex",
			token:   "sk-short",
			wantErr: true,
		},
		{
			name:    "Token with special characters in base64 part",
			token:   "sk-some!@#$invalidChars",
			wantErr: true,
		},
		{
			name:    "Token without prefix",
			token:   "abcdefghijklmnopqrstuvwxyz123456",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateTokenFormat(tt.token)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateTokenFormat() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

// Test coverage for error paths and edge cases
func TestGenerateToken_Coverage(t *testing.T) {
	// Test multiple generations to increase coverage confidence
	tokens := make(map[string]bool)
	for i := 0; i < 50; i++ {
		token, err := GenerateToken()
		if err != nil {
			t.Fatalf("GenerateToken() iteration %d failed: %v", i, err)
		}

		if tokens[token] {
			t.Errorf("Duplicate token generated: %s", token)
		}
		tokens[token] = true

		// Verify each token can be decoded
		uuid, err := DecodeToken(token)
		if err != nil {
			t.Errorf("Failed to decode generated token %s: %v", token, err)
		}

		// Verify UUID is not empty
		if uuid.String() == "00000000-0000-0000-0000-000000000000" {
			t.Errorf("Generated token decoded to empty UUID")
		}

		// Verify token format
		if err := ValidateTokenFormat(token); err != nil {
			t.Errorf("Generated token failed format validation: %v", err)
		}
	}
}
