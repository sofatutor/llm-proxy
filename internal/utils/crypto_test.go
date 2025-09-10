package utils

import (
	"encoding/hex"
	"strings"
	"testing"
)

func TestGenerateSecureToken(t *testing.T) {
	tests := []struct {
		name    string
		length  int
		wantErr bool
	}{
		{"normal length", 16, false},
		{"zero length", 0, true},
		{"negative length", -1, true},
		{"large length", 64, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			token, err := GenerateSecureToken(tt.length)

			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			// Check that token is hex encoded (length should be 2x the input)
			expectedLen := tt.length * 2
			if len(token) != expectedLen {
				t.Errorf("token length = %d, want %d", len(token), expectedLen)
			}

			// Check that token is valid hex
			if _, err := hex.DecodeString(token); err != nil {
				t.Errorf("token is not valid hex: %v", err)
			}

			// Check that token doesn't contain invalid characters
			if strings.ContainsAny(token, "ghijklmnopqrstuvwxyzGHIJKLMNOPQRSTUVWXYZ") {
				t.Error("token contains non-hex characters")
			}
		})
	}
}

func TestGenerateSecureTokenMustSucceed(t *testing.T) {
	t.Run("success case", func(t *testing.T) {
		token := GenerateSecureTokenMustSucceed(16)
		if len(token) != 32 { // 16 bytes = 32 hex chars
			t.Errorf("token length = %d, want 32", len(token))
		}
	})

	t.Run("panic case", func(t *testing.T) {
		defer func() {
			if r := recover(); r == nil {
				t.Error("expected panic, got none")
			}
		}()
		GenerateSecureTokenMustSucceed(-1)
	})
}

func TestGenerateSecureTokenUniqueness(t *testing.T) {
	// Generate multiple tokens and ensure they're different
	tokens := make(map[string]bool)
	for i := 0; i < 100; i++ {
		token, err := GenerateSecureToken(16)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if tokens[token] {
			t.Error("generated duplicate token")
		}
		tokens[token] = true
	}
}

func TestObfuscateToken(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "empty token",
			input:    "",
			expected: "",
		},
		{
			name:     "very short token",
			input:    "abc",
			expected: "***",
		},
		{
			name:     "short token",
			input:    "abcdef",
			expected: "ab****",
		},
		{
			name:     "medium token",
			input:    "tk_1234567890",
			expected: "tk_12345...7890",
		},
		{
			name:     "long token",
			input:    "sk-1234567890abcdefghijklmnop",
			expected: "sk-12345...mnop",
		},
		{
			name:     "exactly 12 chars",
			input:    "123456789012",
			expected: "12**********",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ObfuscateToken(tt.input)
			if result != tt.expected {
				t.Errorf("ObfuscateToken(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

// Test that coverage statistics are accessible
func TestGenerateSecureToken_CoverageEdgeCases(t *testing.T) {
	// Test very small length case that should still work
	token, err := GenerateSecureToken(1)
	if err != nil {
		t.Errorf("GenerateSecureToken(1) error = %v", err)
	}
	if len(token) != 2 { // 1 byte = 2 hex chars
		t.Errorf("GenerateSecureToken(1) length = %d, want 2", len(token))
	}

	// Test very large length to ensure it works
	token, err = GenerateSecureToken(1000)
	if err != nil {
		t.Errorf("GenerateSecureToken(1000) error = %v", err)
	}
	if len(token) != 2000 { // 1000 bytes = 2000 hex chars
		t.Errorf("GenerateSecureToken(1000) length = %d, want 2000", len(token))
	}
}
