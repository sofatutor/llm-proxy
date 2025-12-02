package encryption

import (
	"strings"
	"testing"

	"golang.org/x/crypto/bcrypt"
)

func TestNewTokenHasher(t *testing.T) {
	hasher := NewTokenHasher()
	if hasher == nil {
		t.Fatal("NewTokenHasher() returned nil")
	}
	if hasher.bcryptCost != DefaultBcryptCost {
		t.Errorf("bcryptCost = %d, want %d", hasher.bcryptCost, DefaultBcryptCost)
	}
}

func TestNewTokenHasherWithCost(t *testing.T) {
	tests := []struct {
		name    string
		cost    int
		wantErr bool
	}{
		{
			name:    "valid cost 10",
			cost:    10,
			wantErr: false,
		},
		{
			name:    "min cost",
			cost:    bcrypt.MinCost,
			wantErr: false,
		},
		{
			name:    "max cost",
			cost:    bcrypt.MaxCost,
			wantErr: false,
		},
		{
			name:    "cost too low",
			cost:    bcrypt.MinCost - 1,
			wantErr: true,
		},
		{
			name:    "cost too high",
			cost:    bcrypt.MaxCost + 1,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hasher, err := NewTokenHasherWithCost(tt.cost)
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
			if hasher == nil {
				t.Fatal("expected hasher, got nil")
			}
			if hasher.bcryptCost != tt.cost {
				t.Errorf("bcryptCost = %d, want %d", hasher.bcryptCost, tt.cost)
			}
		})
	}
}

func TestTokenHasherHashToken(t *testing.T) {
	hasher := NewTokenHasher()

	tests := []struct {
		name    string
		token   string
		wantErr bool
	}{
		{
			name:    "valid token",
			token:   "sk-1234567890abcdef",
			wantErr: false,
		},
		{
			name:    "long token (72 byte limit)",
			token:   strings.Repeat("a", 72),
			wantErr: false,
		},
		{
			name:    "very long token (pre-hashed)",
			token:   strings.Repeat("a", 1000),
			wantErr: false,
		},
		{
			name:    "unicode token",
			token:   "token-世界-🔐",
			wantErr: false,
		},
		{
			name:    "empty token",
			token:   "",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hash, err := hasher.HashToken(tt.token)
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

			// Check prefix
			if !strings.HasPrefix(hash, HashPrefix) {
				t.Errorf("hash should have prefix %q", HashPrefix)
			}

			// Verify the hash is valid
			if err := hasher.VerifyToken(tt.token, hash); err != nil {
				t.Errorf("hash verification failed: %v", err)
			}
		})
	}
}

func TestTokenHasherVerifyToken(t *testing.T) {
	hasher := NewTokenHasher()
	token := "sk-test-token-12345"

	// Create a valid hash
	hash, err := hasher.HashToken(token)
	if err != nil {
		t.Fatalf("failed to create hash: %v", err)
	}

	tests := []struct {
		name        string
		token       string
		hashedToken string
		wantErr     error
	}{
		{
			name:        "correct token",
			token:       token,
			hashedToken: hash,
			wantErr:     nil,
		},
		{
			name:        "wrong token",
			token:       "wrong-token",
			hashedToken: hash,
			wantErr:     ErrHashMismatch,
		},
		{
			name:        "empty token",
			token:       "",
			hashedToken: hash,
			wantErr:     ErrHashMismatch,
		},
		{
			name:        "empty hash",
			token:       token,
			hashedToken: "",
			wantErr:     ErrHashMismatch,
		},
		{
			name:        "both empty",
			token:       "",
			hashedToken: "",
			wantErr:     ErrHashMismatch,
		},
		{
			name:        "backward compat - plaintext match",
			token:       "plaintext-token",
			hashedToken: "plaintext-token",
			wantErr:     nil,
		},
		{
			name:        "backward compat - plaintext mismatch",
			token:       "token1",
			hashedToken: "token2",
			wantErr:     ErrHashMismatch,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := hasher.VerifyToken(tt.token, tt.hashedToken)
			if tt.wantErr == nil {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
			} else {
				if err == nil {
					t.Error("expected error, got nil")
				} else if err != tt.wantErr && err.Error() != tt.wantErr.Error() {
					t.Errorf("error = %v, want %v", err, tt.wantErr)
				}
			}
		})
	}
}

func TestTokenHasherHashUniqueness(t *testing.T) {
	hasher := NewTokenHasher()
	token := "test-token"

	// Hash the same token multiple times
	hashes := make(map[string]bool)
	for i := 0; i < 5; i++ {
		hash, err := hasher.HashToken(token)
		if err != nil {
			t.Fatalf("hash failed: %v", err)
		}

		// Each hash should be unique (due to salt)
		if hashes[hash] {
			t.Error("hash should be unique due to salt")
		}
		hashes[hash] = true

		// But all should verify correctly
		if err := hasher.VerifyToken(token, hash); err != nil {
			t.Errorf("verification failed: %v", err)
		}
	}
}

func TestTokenHasherCreateLookupKey(t *testing.T) {
	hasher := NewTokenHasher()

	tests := []struct {
		name  string
		token string
	}{
		{
			name:  "normal token",
			token: "sk-1234567890",
		},
		{
			name:  "empty token",
			token: "",
		},
		{
			name:  "unicode token",
			token: "token-世界",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			key1 := hasher.CreateLookupKey(tt.token)
			key2 := hasher.CreateLookupKey(tt.token)

			// Same token should produce same key
			if key1 != key2 {
				t.Errorf("lookup key not deterministic: %s != %s", key1, key2)
			}

			// Empty token should produce empty key
			if tt.token == "" && key1 != "" {
				t.Error("empty token should produce empty key")
			}

			// Non-empty token should produce hex key
			if tt.token != "" {
				// SHA-256 produces 32 bytes = 64 hex chars
				if len(key1) != 64 {
					t.Errorf("lookup key length = %d, want 64", len(key1))
				}
			}
		})
	}

	// Different tokens should produce different keys
	key1 := hasher.CreateLookupKey("token1")
	key2 := hasher.CreateLookupKey("token2")
	if key1 == key2 {
		t.Error("different tokens should produce different lookup keys")
	}
}

func TestIsHashed(t *testing.T) {
	tests := []struct {
		name  string
		value string
		want  bool
	}{
		{
			name:  "hashed value",
			value: HashPrefix + "$2a$10$somehashedcontent",
			want:  true,
		},
		{
			name:  "plain value",
			value: "plaintext",
			want:  false,
		},
		{
			name:  "empty",
			value: "",
			want:  false,
		},
		{
			name:  "only prefix",
			value: HashPrefix,
			want:  false,
		},
		{
			name:  "partial prefix",
			value: "hash:",
			want:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsHashed(tt.value)
			if got != tt.want {
				t.Errorf("IsHashed(%q) = %v, want %v", tt.value, got, tt.want)
			}
		})
	}
}

func TestNullTokenHasher(t *testing.T) {
	hasher := NewNullTokenHasher()

	t.Run("HashToken", func(t *testing.T) {
		token := "test-token"
		hash, err := hasher.HashToken(token)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if hash != token {
			t.Errorf("hash = %q, want %q", hash, token)
		}
	})

	t.Run("VerifyToken match", func(t *testing.T) {
		token := "test-token"
		err := hasher.VerifyToken(token, token)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("VerifyToken mismatch", func(t *testing.T) {
		err := hasher.VerifyToken("token1", "token2")
		if err != ErrHashMismatch {
			t.Errorf("err = %v, want %v", err, ErrHashMismatch)
		}
	})

	t.Run("CreateLookupKey", func(t *testing.T) {
		token := "test-token"
		key := hasher.CreateLookupKey(token)
		if key != token {
			t.Errorf("key = %q, want %q", key, token)
		}
	})
}

func TestTokenHasherConcurrency(t *testing.T) {
	hasher := NewTokenHasher()
	token := "concurrent-test-token"

	// Create a hash first
	hash, err := hasher.HashToken(token)
	if err != nil {
		t.Fatalf("failed to create hash: %v", err)
	}

	// Run concurrent verifications
	done := make(chan bool, 100)
	for i := 0; i < 100; i++ {
		go func() {
			if err := hasher.VerifyToken(token, hash); err != nil {
				t.Errorf("concurrent verification failed: %v", err)
			}

			_ = hasher.CreateLookupKey(token)

			done <- true
		}()
	}

	for i := 0; i < 100; i++ {
		<-done
	}
}

func BenchmarkTokenHasher_HashToken(b *testing.B) {
	hasher := NewTokenHasher()
	token := "sk-1234567890abcdef"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = hasher.HashToken(token)
	}
}

func BenchmarkTokenHasher_VerifyToken(b *testing.B) {
	hasher := NewTokenHasher()
	token := "sk-1234567890abcdef"
	hash, _ := hasher.HashToken(token)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = hasher.VerifyToken(token, hash)
	}
}

func BenchmarkTokenHasher_CreateLookupKey(b *testing.B) {
	hasher := NewTokenHasher()
	token := "sk-1234567890abcdef"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = hasher.CreateLookupKey(token)
	}
}
