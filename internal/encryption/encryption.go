// Package encryption provides utilities for encrypting and decrypting sensitive data.
// It uses AES-256-GCM for symmetric encryption of data at rest.
package encryption

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
)

const (
	// KeySize is the required size for AES-256 encryption keys (32 bytes).
	KeySize = 32

	// NonceSize is the size of the GCM nonce (12 bytes).
	NonceSize = 12

	// EncryptedPrefix is added to encrypted values to identify them.
	EncryptedPrefix = "enc:v1:"
)

var (
	// ErrInvalidKeySize is returned when the encryption key has an invalid size.
	ErrInvalidKeySize = errors.New("encryption key must be exactly 32 bytes")

	// ErrDecryptionFailed is returned when decryption fails.
	ErrDecryptionFailed = errors.New("decryption failed")

	// ErrNoEncryptionKey is returned when no encryption key is configured.
	ErrNoEncryptionKey = errors.New("no encryption key configured")

	// ErrInvalidCiphertext is returned when the ciphertext is invalid.
	ErrInvalidCiphertext = errors.New("invalid ciphertext format")
)

// Encryptor provides encryption and decryption operations.
// It is safe for concurrent use - cipher.AEAD implementations are thread-safe.
type Encryptor struct {
	gcm cipher.AEAD
}

// NewEncryptor creates a new Encryptor with the given 32-byte key.
// The key must be exactly 32 bytes for AES-256 encryption.
func NewEncryptor(key []byte) (*Encryptor, error) {
	if len(key) != KeySize {
		return nil, ErrInvalidKeySize
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("failed to create AES cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("failed to create GCM: %w", err)
	}

	return &Encryptor{gcm: gcm}, nil
}

// NewEncryptorFromBase64Key creates a new Encryptor from a base64-encoded key.
func NewEncryptorFromBase64Key(base64Key string) (*Encryptor, error) {
	if base64Key == "" {
		return nil, ErrNoEncryptionKey
	}

	key, err := base64.StdEncoding.DecodeString(base64Key)
	if err != nil {
		return nil, fmt.Errorf("failed to decode base64 key: %w", err)
	}

	return NewEncryptor(key)
}

// Encrypt encrypts plaintext and returns a base64-encoded ciphertext with prefix.
func (e *Encryptor) Encrypt(plaintext string) (string, error) {
	if plaintext == "" {
		return "", nil // Empty strings are not encrypted
	}

	// Generate a random nonce
	nonce := make([]byte, NonceSize)
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", fmt.Errorf("failed to generate nonce: %w", err)
	}

	// Encrypt the plaintext
	ciphertext := e.gcm.Seal(nonce, nonce, []byte(plaintext), nil)

	// Encode to base64 and add prefix
	encoded := base64.StdEncoding.EncodeToString(ciphertext)
	return EncryptedPrefix + encoded, nil
}

// Decrypt decrypts a base64-encoded ciphertext and returns the plaintext.
// If the value is not encrypted (no prefix), it returns the value as-is.
func (e *Encryptor) Decrypt(ciphertext string) (string, error) {
	if ciphertext == "" {
		return "", nil // Empty strings are returned as-is
	}

	// Check if the value is encrypted
	if !IsEncrypted(ciphertext) {
		return ciphertext, nil // Return unencrypted values as-is (backward compatibility)
	}

	// Remove the prefix
	encoded := ciphertext[len(EncryptedPrefix):]

	// Decode from base64
	data, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return "", fmt.Errorf("failed to decode ciphertext: %w", err)
	}

	// Validate minimum length (nonce + at least 1 byte of ciphertext + tag)
	if len(data) < NonceSize+e.gcm.Overhead()+1 {
		return "", ErrInvalidCiphertext
	}

	// Extract nonce and ciphertext
	nonce := data[:NonceSize]
	encryptedData := data[NonceSize:]

	// Decrypt
	plaintext, err := e.gcm.Open(nil, nonce, encryptedData, nil)
	if err != nil {
		return "", ErrDecryptionFailed
	}

	return string(plaintext), nil
}

// IsEncrypted checks if a value has the encryption prefix.
func IsEncrypted(value string) bool {
	return len(value) > len(EncryptedPrefix) && value[:len(EncryptedPrefix)] == EncryptedPrefix
}

// GenerateKey generates a new random 32-byte encryption key.
func GenerateKey() ([]byte, error) {
	key := make([]byte, KeySize)
	if _, err := io.ReadFull(rand.Reader, key); err != nil {
		return nil, fmt.Errorf("failed to generate key: %w", err)
	}
	return key, nil
}

// GenerateKeyBase64 generates a new random encryption key and returns it as base64.
func GenerateKeyBase64() (string, error) {
	key, err := GenerateKey()
	if err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(key), nil
}

// NullEncryptor is a no-op encryptor for when encryption is disabled.
type NullEncryptor struct{}

// NewNullEncryptor creates a new NullEncryptor.
func NewNullEncryptor() *NullEncryptor {
	return &NullEncryptor{}
}

// Encrypt returns the plaintext as-is (no encryption).
func (e *NullEncryptor) Encrypt(plaintext string) (string, error) {
	return plaintext, nil
}

// Decrypt returns the ciphertext as-is (no decryption).
func (e *NullEncryptor) Decrypt(ciphertext string) (string, error) {
	return ciphertext, nil
}

// FieldEncryptor is an interface for encrypting and decrypting field values.
type FieldEncryptor interface {
	Encrypt(plaintext string) (string, error)
	Decrypt(ciphertext string) (string, error)
}

// Compile-time interface checks
var (
	_ FieldEncryptor = (*Encryptor)(nil)
	_ FieldEncryptor = (*NullEncryptor)(nil)
)
