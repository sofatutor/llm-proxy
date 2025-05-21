package token

import (
	"errors"
	"time"
)

var (
	// ErrInvalidDuration is returned when an expiration duration is invalid
	ErrInvalidDuration = errors.New("invalid duration")

	// ErrExpirationInPast is returned when an expiration time is in the past
	ErrExpirationInPast = errors.New("expiration time is in the past")
)

// Common expiration durations
const (
	OneHour      = time.Hour
	OneDay       = 24 * time.Hour
	OneWeek      = 7 * 24 * time.Hour
	ThirtyDays   = 30 * 24 * time.Hour
	NinetyDays   = 90 * 24 * time.Hour
	NoExpiration = time.Duration(0)
)

// CalculateExpiration returns an expiration time based on the current time and the provided duration.
// If duration is 0 or negative, it returns nil (no expiration).
func CalculateExpiration(duration time.Duration) *time.Time {
	if duration <= 0 {
		return nil // No expiration
	}

	expiry := time.Now().Add(duration)
	return &expiry
}

// CalculateExpirationFrom returns an expiration time based on the provided start time and duration.
// If duration is 0 or negative, it returns nil (no expiration).
func CalculateExpirationFrom(startTime time.Time, duration time.Duration) *time.Time {
	if duration <= 0 {
		return nil // No expiration
	}

	expiry := startTime.Add(duration)
	return &expiry
}

// ValidateExpiration checks if the given expiration time is valid (nil or in the future).
func ValidateExpiration(expiresAt *time.Time) error {
	if expiresAt == nil {
		return nil // No expiration is valid
	}

	if expiresAt.Before(time.Now()) {
		return ErrExpirationInPast
	}

	return nil
}

// IsExpired checks if a token with the given expiration time is expired.
// If expiresAt is nil, the token never expires.
func IsExpired(expiresAt *time.Time) bool {
	if expiresAt == nil {
		return false // No expiration
	}
	return time.Now().After(*expiresAt)
}

// TimeUntilExpiration returns the duration until the token expires.
// If expiresAt is nil, it returns the max possible duration (effectively "never").
func TimeUntilExpiration(expiresAt *time.Time) time.Duration {
	if expiresAt == nil {
		return 1<<63 - 1 // Max duration, practically "never"
	}
	
	until := expiresAt.Sub(time.Now())
	if until < 0 {
		return 0 // Already expired
	}
	
	return until
}

// ExpiresWithin checks if the token will expire within the given duration.
// If expiresAt is nil, it returns false (never expires).
func ExpiresWithin(expiresAt *time.Time, duration time.Duration) bool {
	if expiresAt == nil {
		return false // No expiration
	}
	
	expiresIn := expiresAt.Sub(time.Now())
	return expiresIn >= 0 && expiresIn <= duration
}

// FormatExpirationTime returns a human-readable string for the expiration time.
// If expiresAt is nil, it returns "Never expires".
func FormatExpirationTime(expiresAt *time.Time) string {
	if expiresAt == nil {
		return "Never expires"
	}
	
	if expiresAt.Before(time.Now()) {
		return "Expired"
	}
	
	return expiresAt.Format(time.RFC3339)
}