package token

import (
	"crypto/rand"
	"fmt"
	"math/big"
	"net/http"
	"strings"
	"time"

	"github.com/sofatutor/llm-proxy/internal/obfuscate"
)

// Constants for token options and generation
const (
	// MinTokenLength is the minimum acceptable token length
	MinTokenLength = 20

	// DefaultTokenExpiration is the default expiration for tokens (30 days)
	DefaultTokenExpiration = 30 * 24 * time.Hour

	// DefaultMaxRequests is the default maximum requests per token (unlimited)
	DefaultMaxRequests = 0 // 0 means unlimited
)

// TokenGenerator is a utility for generating tokens with specific options
type TokenGenerator struct {
	// Default expiration time for new tokens
	DefaultExpiration time.Duration

	// Default maximum requests for new tokens (nil means unlimited)
	DefaultMaxRequests *int
}

// NewTokenGenerator creates a new TokenGenerator with default options
func NewTokenGenerator() *TokenGenerator {
	return &TokenGenerator{
		DefaultExpiration:  DefaultTokenExpiration,
		DefaultMaxRequests: nil, // Unlimited by default
	}
}

// WithExpiration sets the default expiration for new tokens
func (g *TokenGenerator) WithExpiration(expiration time.Duration) *TokenGenerator {
	g.DefaultExpiration = expiration
	return g
}

// WithMaxRequests sets the default maximum requests for new tokens
func (g *TokenGenerator) WithMaxRequests(maxRequests int) *TokenGenerator {
	g.DefaultMaxRequests = &maxRequests
	return g
}

// Generate generates a new token with default options
func (g *TokenGenerator) Generate() (string, error) {
	return GenerateToken()
}

// GenerateWithOptions generates a new token with specific options
func (g *TokenGenerator) GenerateWithOptions(expiration time.Duration, maxRequests *int) (string, *time.Time, *int, error) {
	// Generate token
	token, err := GenerateToken()
	if err != nil {
		return "", nil, nil, err
	}

	// Calculate expiration
	var expiresAt *time.Time
	if expiration > 0 {
		exp := CalculateExpiration(expiration)
		expiresAt = exp
	} else if g.DefaultExpiration > 0 {
		exp := CalculateExpiration(g.DefaultExpiration)
		expiresAt = exp
	}

	// Determine max requests
	var maxReq *int
	if maxRequests != nil {
		maxReq = maxRequests
	} else {
		maxReq = g.DefaultMaxRequests
	}

	return token, expiresAt, maxReq, nil
}

// ExtractTokenFromHeader extracts a token from an HTTP Authorization header
func ExtractTokenFromHeader(header string) (string, bool) {
	if header == "" {
		return "", false
	}

	// Check for "Bearer" auth scheme
	parts := strings.Split(header, " ")
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
		return "", false
	}

	token := parts[1]
	if token == "" {
		return "", false
	}

	// Validate the token format
	if err := ValidateTokenFormat(token); err != nil {
		return "", false
	}

	return token, true
}

// ExtractTokenFromRequest extracts a token from an HTTP request
func ExtractTokenFromRequest(r *http.Request) (string, bool) {
	// Try Authorization header first
	token, ok := ExtractTokenFromHeader(r.Header.Get("Authorization"))
	if ok {
		return token, true
	}

	// Try X-API-Key header next
	apiKey := r.Header.Get("X-API-Key")
	if apiKey != "" {
		if err := ValidateTokenFormat(apiKey); err == nil {
			return apiKey, true
		}
	}

	// Try query parameter last
	queryToken := r.URL.Query().Get("token")
	if queryToken != "" {
		if err := ValidateTokenFormat(queryToken); err == nil {
			return queryToken, true
		}
	}

	return "", false
}

// GenerateRandomKey generates a random string suitable for use as an API key
func GenerateRandomKey(length int) (string, error) {
	if length < MinTokenLength {
		length = MinTokenLength
	}

	// Character set for random key
	const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	charsetLen := big.NewInt(int64(len(charset)))

	// Build the random string
	b := make([]byte, length)
	for i := 0; i < length; i++ {
		n, err := rand.Int(rand.Reader, charsetLen)
		if err != nil {
			return "", err
		}
		b[i] = charset[n.Int64()]
	}

	return string(b), nil
}

// TruncateToken truncates a token string for display, preserving the prefix and suffix
func TruncateToken(token string, showChars int) string {
	if token == "" || len(token) <= showChars*2 {
		return token
	}

	prefix := token[:showChars]
	suffix := token[len(token)-showChars:]

	return prefix + "..." + suffix
}

// ObfuscateToken partially obfuscates a token for display purposes
func ObfuscateToken(token string) string { return obfuscate.ObfuscateTokenByPrefix(token, TokenPrefix) }

// TokenInfo represents information about a token for display purposes
type TokenInfo struct {
	Token           string     `json:"token"`
	ObfuscatedToken string     `json:"obfuscated_token"`
	CreationTime    time.Time  `json:"creation_time"`
	ExpiresAt       *time.Time `json:"expires_at,omitempty"`
	IsActive        bool       `json:"is_active"`
	RequestCount    int        `json:"request_count"`
	MaxRequests     *int       `json:"max_requests,omitempty"`
	LastUsedAt      *time.Time `json:"last_used_at,omitempty"`
	TimeRemaining   string     `json:"time_remaining,omitempty"`
	IsValid         bool       `json:"is_valid"`
}

// GetTokenInfo creates a TokenInfo struct with token details
func GetTokenInfo(token TokenData) TokenInfo {
	// Use the canonical creation time from the database
	// Note: UUIDv7 embeds a timestamp, but the library does not expose it; CreatedAt is the source of truth
	creationTime := token.CreatedAt

	info := TokenInfo{
		Token:           token.Token,
		ObfuscatedToken: ObfuscateToken(token.Token),
		CreationTime:    creationTime,
		ExpiresAt:       token.ExpiresAt,
		IsActive:        token.IsActive,
		RequestCount:    token.RequestCount,
		MaxRequests:     token.MaxRequests,
		LastUsedAt:      token.LastUsedAt,
		IsValid:         token.IsActive && !IsExpired(token.ExpiresAt) && !token.IsRateLimited(),
	}

	// Calculate time remaining
	if token.ExpiresAt != nil && !IsExpired(token.ExpiresAt) {
		duration := time.Until(*token.ExpiresAt)
		info.TimeRemaining = formatDuration(duration)
	}

	return info
}

// FormatTokenInfo formats token information as a human-readable string
func FormatTokenInfo(token TokenData) string {
	info := GetTokenInfo(token)

	var sb strings.Builder
	sb.WriteString("Token: " + info.ObfuscatedToken + "\n")
	sb.WriteString("Created: " + info.CreationTime.Format(time.RFC3339) + "\n")

	if info.ExpiresAt != nil {
		sb.WriteString("Expires: " + info.ExpiresAt.Format(time.RFC3339))
		if info.TimeRemaining != "" {
			sb.WriteString(" (" + info.TimeRemaining + " remaining)")
		}
		sb.WriteString("\n")
	} else {
		sb.WriteString("Expires: Never\n")
	}

	sb.WriteString("Active: " + fmt.Sprintf("%t", info.IsActive) + "\n")

	if info.MaxRequests != nil {
		sb.WriteString(fmt.Sprintf("Requests: %d / %d\n", info.RequestCount, *info.MaxRequests))
	} else {
		sb.WriteString(fmt.Sprintf("Requests: %d / ∞\n", info.RequestCount))
	}

	if info.LastUsedAt != nil {
		sb.WriteString("Last Used: " + info.LastUsedAt.Format(time.RFC3339) + "\n")
	} else {
		sb.WriteString("Last Used: Never\n")
	}

	sb.WriteString("Valid: " + fmt.Sprintf("%t", info.IsValid) + "\n")

	return sb.String()
}

// formatDuration formats a duration in a human-readable way
func formatDuration(d time.Duration) string {
	if d < time.Minute {
		return fmt.Sprintf("%d seconds", int(d.Seconds()))
	} else if d < time.Hour {
		return fmt.Sprintf("%d minutes", int(d.Minutes()))
	} else if d < 24*time.Hour {
		return fmt.Sprintf("%d hours", int(d.Hours()))
	} else if d < 30*24*time.Hour {
		return fmt.Sprintf("%d days", int(d.Hours()/24))
	} else if d < 365*24*time.Hour {
		return fmt.Sprintf("%d months", int(d.Hours()/(24*30)))
	}
	return fmt.Sprintf("%d years", int(d.Hours()/(24*365)))
}
