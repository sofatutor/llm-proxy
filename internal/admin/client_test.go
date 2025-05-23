package admin

import (
	"testing"
)

func TestObfuscateAPIKey(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		output string
	}{
		{"short key", "abcd", "****"},
		{"medium key", "abcdefgh", "ab******"},
		{"long key", "sk-12345678ABCDEFGH", "sk-12345...EFGH"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ObfuscateAPIKey(tt.input)
			if got != tt.output {
				t.Errorf("ObfuscateAPIKey(%q) = %q, want %q", tt.input, got, tt.output)
			}
		})
	}
}

func TestObfuscateToken(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		output string
	}{
		{"short token", "abcd", "****"},
		{"medium token", "abcdefgh", "ab******"},
		{"long token", "tok-12345678ABCDEFGH", "tok-1234...EFGH"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ObfuscateToken(tt.input)
			if got != tt.output {
				t.Errorf("ObfuscateToken(%q) = %q, want %q", tt.input, got, tt.output)
			}
		})
	}
}

func TestNewAPIClient(t *testing.T) {
	baseURL := "http://localhost:1234"
	token := "test-token"
	c := NewAPIClient(baseURL, token)
	if c.baseURL != baseURL {
		t.Errorf("baseURL = %q, want %q", c.baseURL, baseURL)
	}
	if c.token != token {
		t.Errorf("token = %q, want %q", c.token, token)
	}
	if c.httpClient == nil {
		t.Error("httpClient is nil")
	}
}
