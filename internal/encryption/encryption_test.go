package encryption

import (
	"encoding/base64"
	"strings"
	"testing"
)

func TestNewEncryptor(t *testing.T) {
	tests := []struct {
		name    string
		keyLen  int
		wantErr bool
		errType error
	}{
		{
			name:    "valid 32-byte key",
			keyLen:  32,
			wantErr: false,
		},
		{
			name:    "too short key",
			keyLen:  16,
			wantErr: true,
			errType: ErrInvalidKeySize,
		},
		{
			name:    "too long key",
			keyLen:  64,
			wantErr: true,
			errType: ErrInvalidKeySize,
		},
		{
			name:    "empty key",
			keyLen:  0,
			wantErr: true,
			errType: ErrInvalidKeySize,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			key := make([]byte, tt.keyLen)
			for i := range key {
				key[i] = byte(i % 256)
			}

			enc, err := NewEncryptor(key)
			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				}
				if tt.errType != nil && err != tt.errType {
					t.Errorf("expected error %v, got %v", tt.errType, err)
				}
				return
			}

			if err != nil {
				t.Errorf("unexpected error: %v", err)
			}
			if enc == nil {
				t.Error("expected encryptor, got nil")
			}
		})
	}
}

func TestNewEncryptorFromBase64Key(t *testing.T) {
	// Generate a valid key
	key, err := GenerateKey()
	if err != nil {
		t.Fatalf("failed to generate key: %v", err)
	}
	validBase64Key := base64.StdEncoding.EncodeToString(key)

	tests := []struct {
		name    string
		key     string
		wantErr bool
	}{
		{
			name:    "valid base64 key",
			key:     validBase64Key,
			wantErr: false,
		},
		{
			name:    "empty key",
			key:     "",
			wantErr: true,
		},
		{
			name:    "invalid base64",
			key:     "not-valid-base64!!!",
			wantErr: true,
		},
		{
			name:    "valid base64 but wrong size",
			key:     base64.StdEncoding.EncodeToString([]byte("short")),
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			enc, err := NewEncryptorFromBase64Key(tt.key)
			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Errorf("unexpected error: %v", err)
			}
			if enc == nil {
				t.Error("expected encryptor, got nil")
			}
		})
	}
}

func TestEncryptorEncryptDecrypt(t *testing.T) {
	key, err := GenerateKey()
	if err != nil {
		t.Fatalf("failed to generate key: %v", err)
	}

	enc, err := NewEncryptor(key)
	if err != nil {
		t.Fatalf("failed to create encryptor: %v", err)
	}

	tests := []struct {
		name      string
		plaintext string
	}{
		{
			name:      "simple text",
			plaintext: "hello world",
		},
		{
			name:      "api key format",
			plaintext: "sk-1234567890abcdefghijklmnopqrstuvwxyz",
		},
		{
			name:      "unicode text",
			plaintext: "Hello, 世界! 🔐",
		},
		{
			name:      "long text",
			plaintext: strings.Repeat("a", 10000),
		},
		{
			name:      "empty string",
			plaintext: "",
		},
		{
			name:      "special characters",
			plaintext: "!@#$%^&*()_+-=[]{}|;':\",./<>?",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Encrypt
			ciphertext, err := enc.Encrypt(tt.plaintext)
			if err != nil {
				t.Fatalf("encrypt failed: %v", err)
			}

			// Check prefix for non-empty strings
			if tt.plaintext != "" {
				if !strings.HasPrefix(ciphertext, EncryptedPrefix) {
					t.Errorf("ciphertext should have prefix %q", EncryptedPrefix)
				}
			}

			// Decrypt
			decrypted, err := enc.Decrypt(ciphertext)
			if err != nil {
				t.Fatalf("decrypt failed: %v", err)
			}

			// Verify
			if decrypted != tt.plaintext {
				t.Errorf("decrypted = %q, want %q", decrypted, tt.plaintext)
			}
		})
	}
}

func TestEncryptorDeterminism(t *testing.T) {
	key, err := GenerateKey()
	if err != nil {
		t.Fatalf("failed to generate key: %v", err)
	}

	enc, err := NewEncryptor(key)
	if err != nil {
		t.Fatalf("failed to create encryptor: %v", err)
	}

	plaintext := "test data"

	// Encrypt the same plaintext multiple times
	ciphertexts := make(map[string]bool)
	for i := 0; i < 10; i++ {
		ct, err := enc.Encrypt(plaintext)
		if err != nil {
			t.Fatalf("encrypt failed: %v", err)
		}

		// Each ciphertext should be unique (due to random nonce)
		if ciphertexts[ct] {
			t.Error("ciphertext should be unique due to random nonce")
		}
		ciphertexts[ct] = true

		// But all should decrypt to the same plaintext
		decrypted, err := enc.Decrypt(ct)
		if err != nil {
			t.Fatalf("decrypt failed: %v", err)
		}
		if decrypted != plaintext {
			t.Errorf("decrypted = %q, want %q", decrypted, plaintext)
		}
	}
}

func TestEncryptorWrongKey(t *testing.T) {
	key1, _ := GenerateKey()
	key2, _ := GenerateKey()

	enc1, _ := NewEncryptor(key1)
	enc2, _ := NewEncryptor(key2)

	plaintext := "secret data"
	ciphertext, _ := enc1.Encrypt(plaintext)

	// Try to decrypt with wrong key
	_, err := enc2.Decrypt(ciphertext)
	if err == nil {
		t.Error("expected error when decrypting with wrong key")
	}
	if err != ErrDecryptionFailed {
		t.Errorf("expected ErrDecryptionFailed, got %v", err)
	}
}

func TestEncryptorInvalidCiphertext(t *testing.T) {
	key, _ := GenerateKey()
	enc, _ := NewEncryptor(key)

	tests := []struct {
		name       string
		ciphertext string
		wantErr    bool
	}{
		{
			name:       "empty",
			ciphertext: "",
			wantErr:    false, // Empty returns empty
		},
		{
			name:       "no prefix (backward compat)",
			ciphertext: "plaintext-without-prefix",
			wantErr:    false, // Returned as-is for backward compatibility
		},
		{
			name:       "prefix but invalid base64",
			ciphertext: EncryptedPrefix + "not-valid-base64!!!",
			wantErr:    true,
		},
		{
			name:       "prefix but too short",
			ciphertext: EncryptedPrefix + base64.StdEncoding.EncodeToString([]byte("short")),
			wantErr:    true,
		},
		{
			name:       "prefix but corrupted data",
			ciphertext: EncryptedPrefix + base64.StdEncoding.EncodeToString(make([]byte, 100)),
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := enc.Decrypt(tt.ciphertext)
			if tt.wantErr && err == nil {
				t.Error("expected error, got nil")
			}
			if !tt.wantErr && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}

func TestIsEncrypted(t *testing.T) {
	tests := []struct {
		name  string
		value string
		want  bool
	}{
		{
			name:  "encrypted value",
			value: EncryptedPrefix + "someciphertext",
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
			value: EncryptedPrefix,
			want:  false, // Need more than just prefix
		},
		{
			name:  "partial prefix",
			value: "enc:",
			want:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsEncrypted(tt.value)
			if got != tt.want {
				t.Errorf("IsEncrypted(%q) = %v, want %v", tt.value, got, tt.want)
			}
		})
	}
}

func TestGenerateKey(t *testing.T) {
	// Generate multiple keys and verify they're unique and correct size
	keys := make(map[string]bool)
	for i := 0; i < 10; i++ {
		key, err := GenerateKey()
		if err != nil {
			t.Fatalf("GenerateKey() error: %v", err)
		}

		if len(key) != KeySize {
			t.Errorf("key length = %d, want %d", len(key), KeySize)
		}

		keyStr := string(key)
		if keys[keyStr] {
			t.Error("duplicate key generated")
		}
		keys[keyStr] = true
	}
}

func TestGenerateKeyBase64(t *testing.T) {
	keyBase64, err := GenerateKeyBase64()
	if err != nil {
		t.Fatalf("GenerateKeyBase64() error: %v", err)
	}

	// Verify it can be decoded
	key, err := base64.StdEncoding.DecodeString(keyBase64)
	if err != nil {
		t.Fatalf("failed to decode base64 key: %v", err)
	}

	if len(key) != KeySize {
		t.Errorf("decoded key length = %d, want %d", len(key), KeySize)
	}

	// Verify we can create an encryptor with it
	enc, err := NewEncryptorFromBase64Key(keyBase64)
	if err != nil {
		t.Fatalf("failed to create encryptor from generated key: %v", err)
	}

	// Test encryption/decryption
	plaintext := "test data"
	ciphertext, err := enc.Encrypt(plaintext)
	if err != nil {
		t.Fatalf("encrypt failed: %v", err)
	}

	decrypted, err := enc.Decrypt(ciphertext)
	if err != nil {
		t.Fatalf("decrypt failed: %v", err)
	}

	if decrypted != plaintext {
		t.Errorf("decrypted = %q, want %q", decrypted, plaintext)
	}
}

func TestNullEncryptor(t *testing.T) {
	enc := NewNullEncryptor()

	tests := []struct {
		name  string
		value string
	}{
		{name: "simple text", value: "hello"},
		{name: "empty", value: ""},
		{name: "special chars", value: "!@#$%"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Encrypt should return as-is
			encrypted, err := enc.Encrypt(tt.value)
			if err != nil {
				t.Errorf("Encrypt error: %v", err)
			}
			if encrypted != tt.value {
				t.Errorf("Encrypt(%q) = %q, want %q", tt.value, encrypted, tt.value)
			}

			// Decrypt should return as-is
			decrypted, err := enc.Decrypt(tt.value)
			if err != nil {
				t.Errorf("Decrypt error: %v", err)
			}
			if decrypted != tt.value {
				t.Errorf("Decrypt(%q) = %q, want %q", tt.value, decrypted, tt.value)
			}
		})
	}
}

func TestEncryptorConcurrency(t *testing.T) {
	key, _ := GenerateKey()
	enc, _ := NewEncryptor(key)

	// Run concurrent encrypt/decrypt operations
	done := make(chan bool, 100)
	for i := 0; i < 100; i++ {
		go func(id int) {
			plaintext := strings.Repeat("x", id)
			ct, err := enc.Encrypt(plaintext)
			if err != nil {
				t.Errorf("concurrent encrypt error: %v", err)
			}

			pt, err := enc.Decrypt(ct)
			if err != nil {
				t.Errorf("concurrent decrypt error: %v", err)
			}

			if pt != plaintext {
				t.Errorf("concurrent roundtrip failed: got %q, want %q", pt, plaintext)
			}

			done <- true
		}(i)
	}

	// Wait for all goroutines
	for i := 0; i < 100; i++ {
		<-done
	}
}

func TestBackwardCompatibility(t *testing.T) {
	key, _ := GenerateKey()
	enc, _ := NewEncryptor(key)

	// Unencrypted values (legacy data) should be returned as-is
	legacyValues := []string{
		"sk-abcdefghijklmnop",
		"plaintext-api-key",
		"another-legacy-value",
	}

	for _, legacy := range legacyValues {
		decrypted, err := enc.Decrypt(legacy)
		if err != nil {
			t.Errorf("Decrypt(%q) error: %v", legacy, err)
		}
		if decrypted != legacy {
			t.Errorf("Decrypt(%q) = %q, want %q (backward compat)", legacy, decrypted, legacy)
		}
	}
}
