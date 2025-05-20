package config

import (
	"os"
	"testing"
	"time"
)

// TestGetEnvFunctions tests the various getEnv helper functions directly
func TestGetEnvFunctions(t *testing.T) {
	// Save original environment
	originalEnv := make(map[string]string)
	for _, env := range os.Environ() {
		for i := 0; i < len(env); i++ {
			if env[i] == '=' {
				originalEnv[env[:i]] = env[i+1:]
				break
			}
		}
	}

	// Restore environment after test
	defer func() {
		os.Clearenv()
		for k, v := range originalEnv {
			if err := os.Setenv(k, v); err != nil {
				t.Fatalf("Failed to restore environment variable %s: %v", k, err)
			}
		}
	}()

	// Test getEnvInt with different values
	t.Run("getEnvInt", func(t *testing.T) {
		os.Clearenv()
		
		// Test with no env var set
		result := getEnvInt("TEST_INT", 42)
		if result != 42 {
			t.Errorf("Expected default value 42, got %d", result)
		}
		
		// Test with valid int env var
		if err := os.Setenv("TEST_INT", "123"); err != nil {
			t.Fatalf("Failed to set environment variable: %v", err)
		}
		result = getEnvInt("TEST_INT", 42)
		if result != 123 {
			t.Errorf("Expected value 123, got %d", result)
		}
		
		// Test with invalid int env var
		if err := os.Setenv("TEST_INT", "not-an-int"); err != nil {
			t.Fatalf("Failed to set environment variable: %v", err)
		}
		result = getEnvInt("TEST_INT", 42)
		if result != 42 {
			t.Errorf("Expected default value 42 for invalid input, got %d", result)
		}
	})
	
	// Test getEnvStringSlice with different values
	t.Run("getEnvStringSlice", func(t *testing.T) {
		os.Clearenv()
		
		// Test with no env var set
		defaultSlice := []string{"a", "b", "c"}
		result := getEnvStringSlice("TEST_SLICE", defaultSlice)
		if len(result) != len(defaultSlice) {
			t.Errorf("Expected default slice of length %d, got %d", len(defaultSlice), len(result))
		}
		
		// Test with empty env var
		if err := os.Setenv("TEST_SLICE", ""); err != nil {
			t.Fatalf("Failed to set environment variable: %v", err)
		}
		result = getEnvStringSlice("TEST_SLICE", defaultSlice)
		if len(result) != len(defaultSlice) {
			t.Errorf("Expected default slice for empty input, got %v", result)
		}
		
		// Test with single value
		if err := os.Setenv("TEST_SLICE", "single"); err != nil {
			t.Fatalf("Failed to set environment variable: %v", err)
		}
		result = getEnvStringSlice("TEST_SLICE", defaultSlice)
		if len(result) != 1 || result[0] != "single" {
			t.Errorf("Expected slice with single value 'single', got %v", result)
		}
		
		// Test with multiple values and spacing
		if err := os.Setenv("TEST_SLICE", "one, two,three , four"); err != nil {
			t.Fatalf("Failed to set environment variable: %v", err)
		}
		result = getEnvStringSlice("TEST_SLICE", defaultSlice)
		if len(result) != 4 {
			t.Errorf("Expected slice with 4 values, got %v", result)
		}
		if result[0] != "one" || result[1] != "two" || result[2] != "three" || result[3] != "four" {
			t.Errorf("Expected slice with proper trimming, got %v", result)
		}
	})
}

func TestNew(t *testing.T) {
	// Save original environment
	originalEnv := make(map[string]string)
	for _, env := range os.Environ() {
		for i := 0; i < len(env); i++ {
			if env[i] == '=' {
				originalEnv[env[:i]] = env[i+1:]
				break
			}
		}
	}

	// Restore environment after test
	defer func() {
		os.Clearenv()
		for k, v := range originalEnv {
			if err := os.Setenv(k, v); err != nil {
				t.Fatalf("Failed to restore environment variable %s: %v", k, err)
			}
		}
	}()

	t.Run("DefaultValues", func(t *testing.T) {
		// Clear environment
		os.Clearenv()
		
		// Set required fields
		if err := os.Setenv("MANAGEMENT_TOKEN", "test-token"); err != nil {
			t.Fatalf("Failed to set environment variable: %v", err)
		}
		
		config, err := New()
		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}
		
		// Check default values
		if config.ListenAddr != ":8080" {
			t.Errorf("Expected ListenAddr to be :8080, got %s", config.ListenAddr)
		}
		if config.RequestTimeout != 30*time.Second {
			t.Errorf("Expected RequestTimeout to be 30s, got %s", config.RequestTimeout)
		}
		if config.MaxRequestSize != 10*1024*1024 {
			t.Errorf("Expected MaxRequestSize to be 10MB, got %d", config.MaxRequestSize)
		}
		if config.DatabasePath != "./data/llm-proxy.db" {
			t.Errorf("Expected DatabasePath to be ./data/llm-proxy.db, got %s", config.DatabasePath)
		}
		if config.ManagementToken != "test-token" {
			t.Errorf("Expected ManagementToken to be test-token, got %s", config.ManagementToken)
		}
	})

	t.Run("CustomValues", func(t *testing.T) {
		// Clear environment
		os.Clearenv()
		
		// Set custom values
		if err := os.Setenv("LISTEN_ADDR", ":9090"); err != nil {
			t.Fatalf("Failed to set environment variable: %v", err)
		}
		if err := os.Setenv("REQUEST_TIMEOUT", "45s"); err != nil {
			t.Fatalf("Failed to set environment variable: %v", err)
		}
		if err := os.Setenv("MAX_REQUEST_SIZE", "5242880"); err != nil { // 5MB
			t.Fatalf("Failed to set environment variable: %v", err)
		}
		if err := os.Setenv("DATABASE_PATH", "./test.db"); err != nil {
			t.Fatalf("Failed to set environment variable: %v", err)
		}
		if err := os.Setenv("MANAGEMENT_TOKEN", "custom-token"); err != nil {
			t.Fatalf("Failed to set environment variable: %v", err)
		}
		if err := os.Setenv("LOG_LEVEL", "debug"); err != nil {
			t.Fatalf("Failed to set environment variable: %v", err)
		}
		if err := os.Setenv("ENABLE_METRICS", "false"); err != nil {
			t.Fatalf("Failed to set environment variable: %v", err)
		}
		
		config, err := New()
		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}
		
		// Check custom values
		if config.ListenAddr != ":9090" {
			t.Errorf("Expected ListenAddr to be :9090, got %s", config.ListenAddr)
		}
		if config.RequestTimeout != 45*time.Second {
			t.Errorf("Expected RequestTimeout to be 45s, got %s", config.RequestTimeout)
		}
		if config.MaxRequestSize != 5*1024*1024 {
			t.Errorf("Expected MaxRequestSize to be 5MB, got %d", config.MaxRequestSize)
		}
		if config.DatabasePath != "./test.db" {
			t.Errorf("Expected DatabasePath to be ./test.db, got %s", config.DatabasePath)
		}
		if config.ManagementToken != "custom-token" {
			t.Errorf("Expected ManagementToken to be custom-token, got %s", config.ManagementToken)
		}
		if config.LogLevel != "debug" {
			t.Errorf("Expected LogLevel to be debug, got %s", config.LogLevel)
		}
		if config.EnableMetrics != false {
			t.Errorf("Expected EnableMetrics to be false, got %v", config.EnableMetrics)
		}
	})

	t.Run("MissingRequiredValues", func(t *testing.T) {
		// Clear environment
		os.Clearenv()
		
		// Don't set required fields
		config, err := New()
		if err == nil {
			t.Fatalf("Expected error for missing MANAGEMENT_TOKEN, got none")
		}
		if config != nil {
			t.Errorf("Expected nil config when validation fails, got %+v", config)
		}
	})

	t.Run("InvalidValues", func(t *testing.T) {
		// Clear environment
		os.Clearenv()
		
		// Set required fields
		if err := os.Setenv("MANAGEMENT_TOKEN", "test-token"); err != nil {
			t.Fatalf("Failed to set environment variable: %v", err)
		}
		
		// Set invalid values that should be replaced with defaults
		if err := os.Setenv("REQUEST_TIMEOUT", "invalid"); err != nil {
			t.Fatalf("Failed to set environment variable: %v", err)
		}
		if err := os.Setenv("MAX_REQUEST_SIZE", "invalid"); err != nil {
			t.Fatalf("Failed to set environment variable: %v", err)
		}
		if err := os.Setenv("ENABLE_METRICS", "invalid"); err != nil {
			t.Fatalf("Failed to set environment variable: %v", err)
		}
		
		config, err := New()
		if err != nil {
			t.Fatalf("Expected no error despite invalid values, got %v", err)
		}
		
		// Should use defaults for invalid values
		if config.RequestTimeout != 30*time.Second {
			t.Errorf("Expected default RequestTimeout (30s) for invalid input, got %s", config.RequestTimeout)
		}
		if config.MaxRequestSize != 10*1024*1024 {
			t.Errorf("Expected default MaxRequestSize (10MB) for invalid input, got %d", config.MaxRequestSize)
		}
		if config.EnableMetrics != true {
			t.Errorf("Expected default EnableMetrics (true) for invalid input, got %v", config.EnableMetrics)
		}
	})
}