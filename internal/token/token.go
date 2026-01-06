package token

import (
	"encoding/base64"
	"errors"
	"fmt"
	"regexp"
	"strings"

	"github.com/google/uuid"
)

const (
	// TokenPrefix is the prefix for all tokens
	TokenPrefix = "sk-"

	// TokenRegex is the regular expression for validating token format
	TokenRegexPattern = `^sk-[A-Za-z0-9_-]{22}$`
)

var (
	// TokenRegex is the compiled regular expression for token format validation
	TokenRegex = regexp.MustCompile(TokenRegexPattern)

	// ErrInvalidTokenFormat is returned when the token format is invalid
	ErrInvalidTokenFormat = errors.New("invalid token format")

	// ErrTokenDecodingFailed is returned when the token cannot be decoded
	ErrTokenDecodingFailed = errors.New("token decoding failed")
)

// GenerateToken generates a new token with the provided prefix and a UUIDv7.
// The UUIDv7 includes the current timestamp, making tokens time-ordered.
func GenerateToken() (string, error) {
	// Generate a UUIDv7 which includes a timestamp component
	id, err := uuid.NewV7()
	if err != nil {
		return "", fmt.Errorf("failed to generate UUID: %w", err)
	}

	// Convert UUID to Base64 URL-safe encoding
	uuidBytes, err := id.MarshalBinary()
	if err != nil {
		return "", fmt.Errorf("failed to marshal UUID: %w", err)
	}

	// Use URL-safe base64 encoding without padding
	encoded := base64.RawURLEncoding.EncodeToString(uuidBytes)

	// Combine prefix with encoded UUID
	token := TokenPrefix + encoded

	return token, nil
}

// ValidateTokenFormat checks if the given token string follows the expected format.
// It does not check if the token exists or is valid in the database.
func ValidateTokenFormat(token string) error {
	// Fast-path structural validation (avoid regex + decode double work).
	// Tokens are: "sk-" + base64url(UUID bytes) where UUID is 16 bytes => 22 base64url chars.
	if len(token) != len(TokenPrefix)+22 {
		return ErrInvalidTokenFormat
	}
	if !strings.HasPrefix(token, TokenPrefix) {
		return ErrInvalidTokenFormat
	}

	// Attempt to decode the token to ensure it was properly generated.
	// This implicitly validates the base64url charset and length.
	if _, err := DecodeToken(token); err != nil {
		if errors.Is(err, ErrInvalidTokenFormat) {
			return ErrInvalidTokenFormat
		}
		return fmt.Errorf("%w: %v", ErrTokenDecodingFailed, err)
	}

	return nil
}

// DecodeToken extracts the UUID from a token string.
func DecodeToken(token string) (uuid.UUID, error) {
	// Check if the token has the correct prefix
	if !strings.HasPrefix(token, TokenPrefix) {
		return uuid.UUID{}, ErrInvalidTokenFormat
	}

	// Remove the prefix
	encodedPart := strings.TrimPrefix(token, TokenPrefix)

	// Decode from base64
	uuidBytes, err := base64.RawURLEncoding.DecodeString(encodedPart)
	if err != nil {
		return uuid.UUID{}, fmt.Errorf("failed to decode token: %w", err)
	}

	// Parse the UUID
	var id uuid.UUID
	if err := id.UnmarshalBinary(uuidBytes); err != nil {
		return uuid.UUID{}, fmt.Errorf("failed to unmarshal UUID: %w", err)
	}

	return id, nil
}
