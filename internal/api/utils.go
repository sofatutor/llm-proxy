// Package api provides shared utility functions for CLI and API clients.
package api

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

// ObfuscateKey obfuscates a sensitive key for display.
func ObfuscateKey(key string) string {
	if len(key) <= 8 {
		return "****"
	}
	return key[:4] + strings.Repeat("*", len(key)-8) + key[len(key)-4:]
}

// ParseTimeHeader parses a time header in RFC3339Nano format.
func ParseTimeHeader(val string) time.Time {
	if val == "" {
		return time.Time{}
	}
	t, err := time.Parse(time.RFC3339Nano, val)
	if err != nil {
		return time.Time{}
	}
	return t
}

// GetManagementToken gets the management token from a cobra command or environment.
func GetManagementToken(cmd *cobra.Command) (string, error) {
	mgmtToken, _ := cmd.Flags().GetString("management-token")
	if mgmtToken == "" {
		mgmtToken = os.Getenv("MANAGEMENT_TOKEN")
	}
	if mgmtToken == "" {
		return "", fmt.Errorf("management token is required (set MANAGEMENT_TOKEN env or use --management-token)")
	}
	return mgmtToken, nil
}
