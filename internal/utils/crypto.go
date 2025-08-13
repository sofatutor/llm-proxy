// Package utils provides common utility functions.
package utils

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"

	"github.com/sofatutor/llm-proxy/internal/obfuscate"
)

// GenerateSecureToken generates a secure random token of the given length
func GenerateSecureToken(length int) (string, error) {
	if length <= 0 {
		return "", fmt.Errorf("length must be positive")
	}

	b := make([]byte, length)
	_, err := rand.Read(b)
	if err != nil {
		return "", fmt.Errorf("failed to generate secure token: %w", err)
	}
	return hex.EncodeToString(b), nil
}

// GenerateSecureTokenMustSucceed generates a secure random token or panics
// This is useful for initialization code where failure is unrecoverable
func GenerateSecureTokenMustSucceed(length int) string {
	token, err := GenerateSecureToken(length)
	if err != nil {
		panic(err)
	}
	return token
}

// ObfuscateToken obfuscates a token for display purposes
// Shows first 8 characters followed by dots and last 4 characters
func ObfuscateToken(token string) string { return obfuscate.ObfuscateTokenGeneric(token) }
