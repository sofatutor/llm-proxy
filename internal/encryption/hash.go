// Package encryption provides utilities for hashing sensitive data.
// It uses bcrypt for secure password-like hashing of tokens.
package encryption

import (
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"errors"
	"fmt"

	"golang.org/x/crypto/bcrypt"
)

const (
	// HashPrefix is added to hashed values to identify them.
	HashPrefix = "hash:v1:"

	// DefaultBcryptCost is the default cost parameter for bcrypt.
	// A cost of 10 is a good balance between security and performance.
	DefaultBcryptCost = 10
)

var (
	// ErrHashMismatch is returned when a hash comparison fails.
	ErrHashMismatch = errors.New("hash does not match")

	// ErrInvalidHash is returned when the hash format is invalid.
	ErrInvalidHash = errors.New("invalid hash format")
)

// TokenHasher provides secure hashing for authentication tokens.
// It uses SHA-256 for creating lookup keys and bcrypt for secure storage.
type TokenHasher struct {
	bcryptCost int
}

// NewTokenHasher creates a new TokenHasher with the default bcrypt cost.
func NewTokenHasher() *TokenHasher {
	return &TokenHasher{bcryptCost: DefaultBcryptCost}
}

// NewTokenHasherWithCost creates a new TokenHasher with a custom bcrypt cost.
func NewTokenHasherWithCost(cost int) (*TokenHasher, error) {
	if cost < bcrypt.MinCost || cost > bcrypt.MaxCost {
		return nil, fmt.Errorf("bcrypt cost must be between %d and %d", bcrypt.MinCost, bcrypt.MaxCost)
	}
	return &TokenHasher{bcryptCost: cost}, nil
}

// HashToken creates a bcrypt hash of a token for secure storage.
// Returns a hash prefixed with HashPrefix for identification.
// For tokens longer than 72 bytes, a SHA-256 pre-hash is used since
// bcrypt has a 72-byte input limit.
func (h *TokenHasher) HashToken(token string) (string, error) {
	if token == "" {
		return "", errors.New("token cannot be empty")
	}

	// Pre-hash if token is too long for bcrypt (72 byte limit)
	input := []byte(token)
	if len(input) > 72 {
		sum := sha256.Sum256(input)
		input = sum[:]
	}

	hash, err := bcrypt.GenerateFromPassword(input, h.bcryptCost)
	if err != nil {
		return "", fmt.Errorf("failed to hash token: %w", err)
	}

	return HashPrefix + string(hash), nil
}

// VerifyToken compares a plaintext token against a stored hash.
// It returns nil if the token matches, or ErrHashMismatch if it doesn't.
func (h *TokenHasher) VerifyToken(token, hashedToken string) error {
	if token == "" || hashedToken == "" {
		return ErrHashMismatch
	}

	// Check if it's a hashed value
	if !IsHashed(hashedToken) {
		// For backward compatibility, do a constant-time comparison
		// if the stored value is not hashed
		if subtle.ConstantTimeCompare([]byte(token), []byte(hashedToken)) == 1 {
			return nil
		}
		return ErrHashMismatch
	}

	// Remove the prefix
	bcryptHash := hashedToken[len(HashPrefix):]

	// Pre-hash if token is too long for bcrypt (72 byte limit)
	input := []byte(token)
	if len(input) > 72 {
		sum := sha256.Sum256(input)
		input = sum[:]
	}

	// Verify with bcrypt
	err := bcrypt.CompareHashAndPassword([]byte(bcryptHash), input)
	if err != nil {
		if errors.Is(err, bcrypt.ErrMismatchedHashAndPassword) {
			return ErrHashMismatch
		}
		return fmt.Errorf("failed to verify token: %w", err)
	}

	return nil
}

// CreateLookupKey creates a deterministic hash for token lookup.
// This is used as an index key in the database for finding tokens.
// Uses SHA-256 which is fast and collision-resistant.
func (h *TokenHasher) CreateLookupKey(token string) string {
	if token == "" {
		return ""
	}
	hash := sha256.Sum256([]byte(token))
	return hex.EncodeToString(hash[:])
}

// IsHashed checks if a value has the hash prefix.
func IsHashed(value string) bool {
	return len(value) > len(HashPrefix) && value[:len(HashPrefix)] == HashPrefix
}

// NullTokenHasher is a no-op hasher for when hashing is disabled.
type NullTokenHasher struct{}

// NewNullTokenHasher creates a new NullTokenHasher.
func NewNullTokenHasher() *NullTokenHasher {
	return &NullTokenHasher{}
}

// HashToken returns the token as-is (no hashing).
func (h *NullTokenHasher) HashToken(token string) (string, error) {
	return token, nil
}

// VerifyToken performs a constant-time comparison of the tokens.
func (h *NullTokenHasher) VerifyToken(token, storedToken string) error {
	if subtle.ConstantTimeCompare([]byte(token), []byte(storedToken)) == 1 {
		return nil
	}
	return ErrHashMismatch
}

// CreateLookupKey returns the token as-is (it's already the lookup key).
func (h *NullTokenHasher) CreateLookupKey(token string) string {
	return token
}

// TokenHasherInterface defines the interface for token hashing operations.
type TokenHasherInterface interface {
	HashToken(token string) (string, error)
	VerifyToken(token, hashedToken string) error
	CreateLookupKey(token string) string
}

// Compile-time interface checks
var (
	_ TokenHasherInterface = (*TokenHasher)(nil)
	_ TokenHasherInterface = (*NullTokenHasher)(nil)
)
