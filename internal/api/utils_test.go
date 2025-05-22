package api

import (
	"os"
	"testing"
	"time"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
)

func TestObfuscateKey(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name   string
		key    string
		expect string
	}{
		{"short key", "1234567", "****"},
		{"exact 8", "12345678", "****"},
		{"long key", "1234567890abcdef", "1234********cdef"},
		{"empty", "", "****"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.expect, ObfuscateKey(tc.key))
		})
	}
}

func TestParseTimeHeader(t *testing.T) {
	t.Parallel()
	now := time.Now().UTC().Truncate(time.Millisecond)
	str := now.Format(time.RFC3339Nano)
	parsed := ParseTimeHeader(str)
	assert.WithinDuration(t, now, parsed, time.Millisecond)

	assert.True(t, ParseTimeHeader("").IsZero())
	assert.True(t, ParseTimeHeader("not-a-time").IsZero())
}

func TestGetManagementToken(t *testing.T) {
	t.Parallel()
	cmd := &cobra.Command{}
	cmd.Flags().String("management-token", "", "")

	// No flag, no env
	os.Unsetenv("MANAGEMENT_TOKEN")
	tok, err := GetManagementToken(cmd)
	assert.Error(t, err)
	assert.Empty(t, tok)

	// Env only
	os.Setenv("MANAGEMENT_TOKEN", "env-token")
	tok, err = GetManagementToken(cmd)
	assert.NoError(t, err)
	assert.Equal(t, "env-token", tok)
	os.Unsetenv("MANAGEMENT_TOKEN")

	// Flag only
	cmd.Flags().Set("management-token", "flag-token")
	tok, err = GetManagementToken(cmd)
	assert.NoError(t, err)
	assert.Equal(t, "flag-token", tok)
}
