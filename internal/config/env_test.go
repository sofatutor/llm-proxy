package config

import (
	"os"
	"testing"
)

func TestEnvHelpers(t *testing.T) {
	t.Setenv("TEST_STR", "value")
	t.Setenv("TEST_INT", "42")
	t.Setenv("TEST_BOOL_TRUE", "true")
	t.Setenv("TEST_BOOL_FALSE", "false")
	t.Setenv("TEST_FLOAT", "3.14")

	if got := EnvOrDefault("TEST_STR", "fallback"); got != "value" {
		t.Fatalf("EnvOrDefault: got %q, want value", got)
	}
	if got := EnvOrDefault("MISSING", "fallback"); got != "fallback" {
		t.Fatalf("EnvOrDefault missing: got %q, want fallback", got)
	}

	if got := EnvIntOrDefault("TEST_INT", 0); got != 42 {
		t.Fatalf("EnvIntOrDefault: got %d, want 42", got)
	}
	if got := EnvIntOrDefault("BAD_INT", 7); got != 7 {
		os.Setenv("BAD_INT", "oops")
		// fallback when not parseable or not set
		got2 := EnvIntOrDefault("BAD_INT", 7)
		if got2 != 7 {
			t.Fatalf("EnvIntOrDefault bad: got %d, want 7", got2)
		}
	}

	if got := EnvBoolOrDefault("TEST_BOOL_TRUE", false); got != true {
		t.Fatalf("EnvBoolOrDefault true: got %v, want true", got)
	}
	if got := EnvBoolOrDefault("TEST_BOOL_FALSE", true); got != false {
		t.Fatalf("EnvBoolOrDefault false: got %v, want false", got)
	}
	if got := EnvBoolOrDefault("BAD_BOOL", true); got != true {
		os.Setenv("BAD_BOOL", "oops")
		if EnvBoolOrDefault("BAD_BOOL", true) != true {
			t.Fatalf("EnvBoolOrDefault bad: expected fallback true")
		}
	}

	if got := EnvFloat64OrDefault("TEST_FLOAT", 0.0); got != 3.14 {
		t.Fatalf("EnvFloat64OrDefault: got %v, want 3.14", got)
	}
	if got := EnvFloat64OrDefault("BAD_FLOAT", 1.23); got != 1.23 {
		os.Setenv("BAD_FLOAT", "oops")
		if EnvFloat64OrDefault("BAD_FLOAT", 1.23) != 1.23 {
			t.Fatalf("EnvFloat64OrDefault bad: expected fallback 1.23")
		}
	}
}
