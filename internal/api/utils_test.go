package api

import (
	"os"
	"testing"
	"time"

	"github.com/spf13/cobra"
)

func TestObfuscateKey(t *testing.T) {
	// Short keys fully obfuscated
	if got := ObfuscateKey("abcd"); got != "****" {
		t.Fatalf("short key: got %q, want ****", got)
	}
	if got := ObfuscateKey("12345678"); got != "****" {
		t.Fatalf("8-char key: got %q, want ****", got)
	}
	// Long key shows first 4 and last 4 with stars in between
	if got := ObfuscateKey("abcdefghijkl"); got != "abcd****ijkl" {
		t.Fatalf("long key: got %q, want abcd****ijkl", got)
	}
}

func TestParseTimeHeader(t *testing.T) {
	if !ParseTimeHeader("").IsZero() {
		t.Fatal("empty header should produce zero time")
	}
	if !ParseTimeHeader("not-a-time").IsZero() {
		t.Fatal("invalid time should produce zero time")
	}
	ts := time.Now().UTC().Format(time.RFC3339Nano)
	parsed := ParseTimeHeader(ts)
	if parsed.IsZero() {
		t.Fatal("valid RFC3339Nano should parse")
	}
}

func TestGetManagementToken(t *testing.T) {
	// Helper to make a command with the flag
	newCmd := func() *cobra.Command {
		cmd := &cobra.Command{Use: "test"}
		cmd.Flags().String("management-token", "", "")
		return cmd
	}

	// 1) from flag
	cmd := newCmd()
	_ = cmd.Flags().Set("management-token", "from-flag")
	tok, err := GetManagementToken(cmd)
	if err != nil || tok != "from-flag" {
		t.Fatalf("flag token: tok=%q err=%v", tok, err)
	}

	// 2) from env
	cmd = newCmd()
	t.Setenv("MANAGEMENT_TOKEN", "from-env")
	tok, err = GetManagementToken(cmd)
	if err != nil || tok != "from-env" {
		t.Fatalf("env token: tok=%q err=%v", tok, err)
	}
	os.Unsetenv("MANAGEMENT_TOKEN")

	// 3) missing
	cmd = newCmd()
	tok, err = GetManagementToken(cmd)
	if err == nil || tok != "" {
		t.Fatalf("missing token should error, got tok=%q err=%v", tok, err)
	}
}
