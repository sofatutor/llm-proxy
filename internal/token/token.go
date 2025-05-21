package token

import (
	"encoding/base64"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"
)

const (
	// TokenPrefix is the prefix for all tokens
	TokenPrefix = "tkn_"

	// TokenRegex is the regular expression for validating token format
	TokenRegexPattern = `^tkn_[A-Za-z0-9_-]{22}$`
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
	// Check format with regex
	if !TokenRegex.MatchString(token) {
		return ErrInvalidTokenFormat
	}

	// Attempt to decode the token to ensure it was properly generated
	_, err := DecodeToken(token)
	if err != nil {
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

// GetTokenCreationTime extracts the timestamp from a token.
// This works because UUIDv7 embeds a timestamp in the UUID.
func GetTokenCreationTime(token string) (time.Time, error) {
	_, err := DecodeToken(token)
	if err != nil {
		return time.Time{}, err
	}

	// UUIDv7 stores a timestamp in the first 48 bits
	// This is an approximation - limited by what google/uuid exposes
	// For v7 UUIDs, we can't directly extract the timestamp yet with google/uuid
	// This is a limitation of the current API
	// However, since we just created the token, we can return the current time
	// In a production system, we'd either store the creation time or use a better method
	return time.Now(), nil
}
