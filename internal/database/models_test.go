package database

import (
	"testing"
	"time"
)

func TestToken_IsExpired(t *testing.T) {
	now := time.Now()
	past := now.Add(-time.Hour)
	future := now.Add(time.Hour)
	tests := []struct {
		name    string
		token   Token
		expired bool
	}{
		{"no expiry", Token{ExpiresAt: nil}, false},
		{"expired", Token{ExpiresAt: &past}, true},
		{"not expired", Token{ExpiresAt: &future}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.token.IsExpired(); got != tt.expired {
				t.Errorf("IsExpired() = %v, want %v", got, tt.expired)
			}
		})
	}
}

func TestToken_IsRateLimited(t *testing.T) {
	max := 5
	tests := []struct {
		name        string
		token       Token
		rateLimited bool
	}{
		{"no max", Token{RequestCount: 10, MaxRequests: nil}, false},
		{"not limited", Token{RequestCount: 3, MaxRequests: &max}, false},
		{"at limit", Token{RequestCount: 5, MaxRequests: &max}, true},
		{"over limit", Token{RequestCount: 6, MaxRequests: &max}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.token.IsRateLimited(); got != tt.rateLimited {
				t.Errorf("IsRateLimited() = %v, want %v", got, tt.rateLimited)
			}
		})
	}
}

func TestToken_IsValid(t *testing.T) {
	now := time.Now()
	future := now.Add(time.Hour)
	past := now.Add(-time.Hour)
	max := 2
	tests := []struct {
		name  string
		token Token
		valid bool
	}{
		{"active, not expired, not limited", Token{IsActive: true, ExpiresAt: &future, RequestCount: 1, MaxRequests: &max}, true},
		{"inactive", Token{IsActive: false, ExpiresAt: &future, RequestCount: 1, MaxRequests: &max}, false},
		{"expired", Token{IsActive: true, ExpiresAt: &past, RequestCount: 1, MaxRequests: &max}, false},
		{"rate limited", Token{IsActive: true, ExpiresAt: &future, RequestCount: 2, MaxRequests: &max}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.token.IsValid(); got != tt.valid {
				t.Errorf("IsValid() = %v, want %v", got, tt.valid)
			}
		})
	}
}
