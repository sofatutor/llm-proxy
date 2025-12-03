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

// TestEnvironmentFunctions tests remaining env helper functions
func TestEnvironmentFunctions(t *testing.T) {
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

	// Test getEnvString
	t.Run("getEnvString", func(t *testing.T) {
		os.Clearenv()

		// Test with no env var set
		result := getEnvString("TEST_STRING", "default")
		if result != "default" {
			t.Errorf("Expected default value 'default', got %s", result)
		}

		// Test with env var set
		if err := os.Setenv("TEST_STRING", "custom"); err != nil {
			t.Fatalf("Failed to set environment variable: %v", err)
		}
		result = getEnvString("TEST_STRING", "default")
		if result != "custom" {
			t.Errorf("Expected value 'custom', got %s", result)
		}
	})

	// Test getEnvBool
	t.Run("getEnvBool", func(t *testing.T) {
		os.Clearenv()

		// Test with no env var set
		result := getEnvBool("TEST_BOOL", true)
		if result != true {
			t.Errorf("Expected default value true, got %v", result)
		}

		// Test with valid bool env vars (only what strconv.ParseBool supports)
		testCases := []struct {
			value    string
			expected bool
		}{
			{"true", true},
			{"false", false},
			{"1", true},
			{"0", false},
		}

		for _, tc := range testCases {
			if err := os.Setenv("TEST_BOOL", tc.value); err != nil {
				t.Fatalf("Failed to set environment variable: %v", err)
			}
			result = getEnvBool("TEST_BOOL", false)
			if result != tc.expected {
				t.Errorf("Expected value %v for input '%s', got %v", tc.expected, tc.value, result)
			}
		}

		// Test with invalid bool env var
		if err := os.Setenv("TEST_BOOL", "invalid"); err != nil {
			t.Fatalf("Failed to set environment variable: %v", err)
		}
		result = getEnvBool("TEST_BOOL", true)
		if result != true {
			t.Errorf("Expected default value true for invalid input, got %v", result)
		}
	})

	// Test getEnvInt64
	t.Run("getEnvInt64", func(t *testing.T) {
		os.Clearenv()

		// Test with no env var set
		result := getEnvInt64("TEST_INT64", 42)
		if result != 42 {
			t.Errorf("Expected default value 42, got %d", result)
		}

		// Test with valid int64 env var
		if err := os.Setenv("TEST_INT64", "9223372036854775807"); err != nil { // max int64
			t.Fatalf("Failed to set environment variable: %v", err)
		}
		result = getEnvInt64("TEST_INT64", 42)
		if result != 9223372036854775807 {
			t.Errorf("Expected value 9223372036854775807, got %d", result)
		}

		// Test with invalid int64 env var
		if err := os.Setenv("TEST_INT64", "not-an-int"); err != nil {
			t.Fatalf("Failed to set environment variable: %v", err)
		}
		result = getEnvInt64("TEST_INT64", 42)
		if result != 42 {
			t.Errorf("Expected default value 42 for invalid input, got %d", result)
		}
	})

	// Test getEnvDuration
	t.Run("getEnvDuration", func(t *testing.T) {
		os.Clearenv()

		// Test with no env var set
		defaultDuration := 30 * time.Second
		result := getEnvDuration("TEST_DURATION", defaultDuration)
		if result != defaultDuration {
			t.Errorf("Expected default value %v, got %v", defaultDuration, result)
		}

		// Test with valid duration env var
		if err := os.Setenv("TEST_DURATION", "45s"); err != nil {
			t.Fatalf("Failed to set environment variable: %v", err)
		}
		result = getEnvDuration("TEST_DURATION", defaultDuration)
		if result != 45*time.Second {
			t.Errorf("Expected value 45s, got %v", result)
		}

		// Test with different duration formats
		testCases := []struct {
			value    string
			expected time.Duration
		}{
			{"1m", time.Minute},
			{"2h", 2 * time.Hour},
			{"500ms", 500 * time.Millisecond},
		}

		for _, tc := range testCases {
			if err := os.Setenv("TEST_DURATION", tc.value); err != nil {
				t.Fatalf("Failed to set environment variable: %v", err)
			}
			result = getEnvDuration("TEST_DURATION", defaultDuration)
			if result != tc.expected {
				t.Errorf("Expected value %v for input '%s', got %v", tc.expected, tc.value, result)
			}
		}

		// Test with invalid duration env var
		if err := os.Setenv("TEST_DURATION", "invalid"); err != nil {
			t.Fatalf("Failed to set environment variable: %v", err)
		}
		result = getEnvDuration("TEST_DURATION", defaultDuration)
		if result != defaultDuration {
			t.Errorf("Expected default value %v for invalid input, got %v", defaultDuration, result)
		}
	})
}

func TestDefaultConfig(t *testing.T) {
	config := DefaultConfig()

	// Check default values are set correctly
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
	if config.LogLevel != "info" {
		t.Errorf("Expected LogLevel to be info, got %s", config.LogLevel)
	}
	if config.EnableMetrics != true {
		t.Errorf("Expected EnableMetrics to be true, got %v", config.EnableMetrics)
	}
}

func TestLoadFromFile(t *testing.T) {
	// Test loading from file (currently just returns DefaultConfig)
	config, err := LoadFromFile("any-file.yaml")
	if err != nil {
		t.Fatalf("Expected no error loading config file, got %v", err)
	}

	// Since LoadFromFile currently returns DefaultConfig(), check default values
	defaultConfig := DefaultConfig()
	if config.ListenAddr != defaultConfig.ListenAddr {
		t.Errorf("Expected ListenAddr to be %s, got %s", defaultConfig.ListenAddr, config.ListenAddr)
	}
	if config.RequestTimeout != defaultConfig.RequestTimeout {
		t.Errorf("Expected RequestTimeout to be %s, got %s", defaultConfig.RequestTimeout, config.RequestTimeout)
	}
	if config.MaxRequestSize != defaultConfig.MaxRequestSize {
		t.Errorf("Expected MaxRequestSize to be %d, got %d", defaultConfig.MaxRequestSize, config.MaxRequestSize)
	}
}

func TestConfig_ProjectActiveGuard(t *testing.T) {
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

	// Clear environment
	os.Clearenv()

	// Set required fields
	if err := os.Setenv("MANAGEMENT_TOKEN", "test-token"); err != nil {
		t.Fatalf("Failed to set environment variable: %v", err)
	}

	// Test project active guard configuration
	if err := os.Setenv("LLM_PROXY_ENFORCE_PROJECT_ACTIVE", "false"); err != nil {
		t.Fatalf("Failed to set environment variable: %v", err)
	}
	if err := os.Setenv("LLM_PROXY_ACTIVE_CACHE_TTL", "10s"); err != nil {
		t.Fatalf("Failed to set environment variable: %v", err)
	}
	if err := os.Setenv("LLM_PROXY_ACTIVE_CACHE_MAX", "5000"); err != nil {
		t.Fatalf("Failed to set environment variable: %v", err)
	}

	config, err := New()
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}
	if config.EnforceProjectActive != false {
		t.Errorf("Expected EnforceProjectActive to be false, got %v", config.EnforceProjectActive)
	}
	if config.ActiveCacheTTL != 10*time.Second {
		t.Errorf("Expected ActiveCacheTTL to be 10s, got %v", config.ActiveCacheTTL)
	}
	if config.ActiveCacheMax != 5000 {
		t.Errorf("Expected ActiveCacheMax to be 5000, got %d", config.ActiveCacheMax)
	}
}

func TestConfig_ProjectActiveGuardDefaults(t *testing.T) {
	// Test defaults for project active guard
	config := DefaultConfig()
	if config.EnforceProjectActive != true {
		t.Errorf("Expected EnforceProjectActive to be true, got %v", config.EnforceProjectActive)
	}
	if config.ActiveCacheTTL != 5*time.Second {
		t.Errorf("Expected ActiveCacheTTL to be 5s, got %v", config.ActiveCacheTTL)
	}
	if config.ActiveCacheMax != 10000 {
		t.Errorf("Expected ActiveCacheMax to be 10000, got %d", config.ActiveCacheMax)
	}
}

func TestConfig_DistributedRateLimit(t *testing.T) {
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

	// Clear environment
	os.Clearenv()

	// Set required fields
	if err := os.Setenv("MANAGEMENT_TOKEN", "test-token"); err != nil {
		t.Fatalf("Failed to set environment variable: %v", err)
	}

	// Test distributed rate limiting configuration
	if err := os.Setenv("DISTRIBUTED_RATE_LIMIT_ENABLED", "true"); err != nil {
		t.Fatalf("Failed to set environment variable: %v", err)
	}
	if err := os.Setenv("DISTRIBUTED_RATE_LIMIT_PREFIX", "myapp:ratelimit:"); err != nil {
		t.Fatalf("Failed to set environment variable: %v", err)
	}
	if err := os.Setenv("DISTRIBUTED_RATE_LIMIT_WINDOW", "30s"); err != nil {
		t.Fatalf("Failed to set environment variable: %v", err)
	}
	if err := os.Setenv("DISTRIBUTED_RATE_LIMIT_MAX", "100"); err != nil {
		t.Fatalf("Failed to set environment variable: %v", err)
	}
	if err := os.Setenv("DISTRIBUTED_RATE_LIMIT_FALLBACK", "false"); err != nil {
		t.Fatalf("Failed to set environment variable: %v", err)
	}
	if err := os.Setenv("DISTRIBUTED_RATE_LIMIT_KEY_SECRET", "my-secret-key"); err != nil {
		t.Fatalf("Failed to set environment variable: %v", err)
	}

	config, err := New()
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}
	if !config.DistributedRateLimitEnabled {
		t.Errorf("Expected DistributedRateLimitEnabled to be true, got %v", config.DistributedRateLimitEnabled)
	}
	if config.DistributedRateLimitPrefix != "myapp:ratelimit:" {
		t.Errorf("Expected DistributedRateLimitPrefix to be 'myapp:ratelimit:', got %s", config.DistributedRateLimitPrefix)
	}
	if config.DistributedRateLimitKeySecret != "my-secret-key" {
		t.Errorf("Expected DistributedRateLimitKeySecret to be 'my-secret-key', got %s", config.DistributedRateLimitKeySecret)
	}
	if config.DistributedRateLimitWindow != 30*time.Second {
		t.Errorf("Expected DistributedRateLimitWindow to be 30s, got %v", config.DistributedRateLimitWindow)
	}
	if config.DistributedRateLimitMax != 100 {
		t.Errorf("Expected DistributedRateLimitMax to be 100, got %d", config.DistributedRateLimitMax)
	}
	if config.DistributedRateLimitFallback {
		t.Errorf("Expected DistributedRateLimitFallback to be false, got %v", config.DistributedRateLimitFallback)
	}
}

func TestConfig_DistributedRateLimitDefaults(t *testing.T) {
	// Test defaults for distributed rate limiting
	config := DefaultConfig()
	if config.DistributedRateLimitEnabled {
		t.Errorf("Expected DistributedRateLimitEnabled to be false, got %v", config.DistributedRateLimitEnabled)
	}
	if config.DistributedRateLimitPrefix != "ratelimit:" {
		t.Errorf("Expected DistributedRateLimitPrefix to be 'ratelimit:', got %s", config.DistributedRateLimitPrefix)
	}
	if config.DistributedRateLimitKeySecret != "" {
		t.Errorf("Expected DistributedRateLimitKeySecret to be empty, got %s", config.DistributedRateLimitKeySecret)
	}
	if config.DistributedRateLimitWindow != time.Minute {
		t.Errorf("Expected DistributedRateLimitWindow to be 1m, got %v", config.DistributedRateLimitWindow)
	}
	if config.DistributedRateLimitMax != 60 {
		t.Errorf("Expected DistributedRateLimitMax to be 60, got %d", config.DistributedRateLimitMax)
	}
	if !config.DistributedRateLimitFallback {
		t.Errorf("Expected DistributedRateLimitFallback to be true, got %v", config.DistributedRateLimitFallback)
	}
}

func TestConfig_RedisStreamsDefaults(t *testing.T) {
	config := DefaultConfig()

	if config.RedisStreamKey != "llm-proxy-events" {
		t.Errorf("Expected RedisStreamKey to be 'llm-proxy-events', got %s", config.RedisStreamKey)
	}
	if config.RedisConsumerGroup != "llm-proxy-dispatchers" {
		t.Errorf("Expected RedisConsumerGroup to be 'llm-proxy-dispatchers', got %s", config.RedisConsumerGroup)
	}
	if config.RedisConsumerName != "" {
		t.Errorf("Expected RedisConsumerName to be empty, got %s", config.RedisConsumerName)
	}
	if config.RedisStreamMaxLen != 10000 {
		t.Errorf("Expected RedisStreamMaxLen to be 10000, got %d", config.RedisStreamMaxLen)
	}
	if config.RedisStreamBlockTime != 5*time.Second {
		t.Errorf("Expected RedisStreamBlockTime to be 5s, got %v", config.RedisStreamBlockTime)
	}
	if config.RedisStreamClaimTime != 30*time.Second {
		t.Errorf("Expected RedisStreamClaimTime to be 30s, got %v", config.RedisStreamClaimTime)
	}
	if config.RedisStreamBatchSize != 100 {
		t.Errorf("Expected RedisStreamBatchSize to be 100, got %d", config.RedisStreamBatchSize)
	}
}

func TestConfig_RedisStreamsFromEnv(t *testing.T) {
	// Clean up environment after test
	defer func() {
		os.Unsetenv("MANAGEMENT_TOKEN")
		os.Unsetenv("REDIS_STREAM_KEY")
		os.Unsetenv("REDIS_CONSUMER_GROUP")
		os.Unsetenv("REDIS_CONSUMER_NAME")
		os.Unsetenv("REDIS_STREAM_MAX_LEN")
		os.Unsetenv("REDIS_STREAM_BLOCK_TIME")
		os.Unsetenv("REDIS_STREAM_CLAIM_TIME")
		os.Unsetenv("REDIS_STREAM_BATCH_SIZE")
	}()

	if err := os.Setenv("MANAGEMENT_TOKEN", "test-token"); err != nil {
		t.Fatalf("Failed to set MANAGEMENT_TOKEN: %v", err)
	}
	if err := os.Setenv("REDIS_STREAM_KEY", "custom-events"); err != nil {
		t.Fatalf("Failed to set REDIS_STREAM_KEY: %v", err)
	}
	if err := os.Setenv("REDIS_CONSUMER_GROUP", "custom-group"); err != nil {
		t.Fatalf("Failed to set REDIS_CONSUMER_GROUP: %v", err)
	}
	if err := os.Setenv("REDIS_CONSUMER_NAME", "custom-consumer"); err != nil {
		t.Fatalf("Failed to set REDIS_CONSUMER_NAME: %v", err)
	}
	if err := os.Setenv("REDIS_STREAM_MAX_LEN", "50000"); err != nil {
		t.Fatalf("Failed to set REDIS_STREAM_MAX_LEN: %v", err)
	}
	if err := os.Setenv("REDIS_STREAM_BLOCK_TIME", "10s"); err != nil {
		t.Fatalf("Failed to set REDIS_STREAM_BLOCK_TIME: %v", err)
	}
	if err := os.Setenv("REDIS_STREAM_CLAIM_TIME", "1m"); err != nil {
		t.Fatalf("Failed to set REDIS_STREAM_CLAIM_TIME: %v", err)
	}
	if err := os.Setenv("REDIS_STREAM_BATCH_SIZE", "200"); err != nil {
		t.Fatalf("Failed to set REDIS_STREAM_BATCH_SIZE: %v", err)
	}

	config, err := New()
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if config.RedisStreamKey != "custom-events" {
		t.Errorf("Expected RedisStreamKey to be 'custom-events', got %s", config.RedisStreamKey)
	}
	if config.RedisConsumerGroup != "custom-group" {
		t.Errorf("Expected RedisConsumerGroup to be 'custom-group', got %s", config.RedisConsumerGroup)
	}
	if config.RedisConsumerName != "custom-consumer" {
		t.Errorf("Expected RedisConsumerName to be 'custom-consumer', got %s", config.RedisConsumerName)
	}
	if config.RedisStreamMaxLen != 50000 {
		t.Errorf("Expected RedisStreamMaxLen to be 50000, got %d", config.RedisStreamMaxLen)
	}
	if config.RedisStreamBlockTime != 10*time.Second {
		t.Errorf("Expected RedisStreamBlockTime to be 10s, got %v", config.RedisStreamBlockTime)
	}
	if config.RedisStreamClaimTime != time.Minute {
		t.Errorf("Expected RedisStreamClaimTime to be 1m, got %v", config.RedisStreamClaimTime)
	}
	if config.RedisStreamBatchSize != 200 {
		t.Errorf("Expected RedisStreamBatchSize to be 200, got %d", config.RedisStreamBatchSize)
	}
}
