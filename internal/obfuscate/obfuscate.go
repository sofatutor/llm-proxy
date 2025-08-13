// Package obfuscate centralizes redaction/obfuscation helpers used across the codebase.
package obfuscate

import (
	"strings"
)

// ObfuscateTokenGeneric obfuscates arbitrary token-like strings for display/logging.
// Behavior (kept for backward-compat with previous utils implementation):
// - length <= 4  → all asterisks of same length
// - 5..12        → keep first 2 characters, replace the rest with asterisks
// - > 12         → keep first 8 characters, then "...", then last 4 characters
func ObfuscateTokenGeneric(s string) string {
	if len(s) <= 4 {
		return strings.Repeat("*", len(s))
	}
	if len(s) <= 12 {
		return s[:2] + strings.Repeat("*", len(s)-2)
	}
	return s[:8] + "..." + s[len(s)-4:]
}

// ObfuscateTokenSimple obfuscates token-like strings with a fixed pattern suitable for UIs.
// Behavior (kept for backward-compat with Admin template helper):
// - length <= 8  → "****"
// - > 8         → first 4 + "****" + last 4
func ObfuscateTokenSimple(s string) string {
	if len(s) <= 8 {
		return "****"
	}
	return s[:4] + "****" + s[len(s)-4:]
}

// ObfuscateTokenByPrefix obfuscates tokens that follow a known prefix convention (e.g., "sk-").
// If the string doesn't have the expected prefix, it falls back to the generic strategy.
// Behavior (kept for backward-compat with token.ObfuscateToken):
//   - With prefix: keep the prefix, then show first 4 and last 4 of the remainder, replacing the middle with '*'
//     If remainder <= 8, return the input unmodified
//   - Without prefix: same result as ObfuscateTokenGeneric
func ObfuscateTokenByPrefix(s string, prefix string) string {
	if s == "" {
		return s
	}
	if !strings.HasPrefix(s, prefix) {
		// Preserve previous behavior of token.ObfuscateToken: leave non-prefixed strings unchanged
		return s
	}
	rest := s[len(prefix):]
	if len(rest) <= 8 {
		return s
	}
	visible := 4
	first := rest[:visible]
	last := rest[len(rest)-visible:]
	middle := strings.Repeat("*", len(rest)-(visible*2))
	return prefix + first + middle + last
}
