// Package config handles application configuration loading and validation
// from environment variables, providing a type-safe configuration structure.
package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

// Config holds all application configuration values loaded from environment variables.
// It provides a centralized, type-safe way to access configuration throughout the application.
type Config struct {
	// Server configuration
	ListenAddr        string        // Address to listen on (e.g., ":8080")
	RequestTimeout    time.Duration // Timeout for upstream API requests
	MaxRequestSize    int64         // Maximum size of incoming requests in bytes
	MaxConcurrentReqs int           // Maximum number of concurrent requests

	// Database configuration
	DatabasePath     string // Path to the SQLite database file
	DatabasePoolSize int    // Number of connections in the database pool

	// Authentication
	ManagementToken string // Token for admin operations, used to access the management API

	// OpenAI API settings
	OpenAIAPIURL    string // Base URL for OpenAI API
	EnableStreaming bool   // Whether to enable streaming responses from OpenAI

	// Admin UI settings
	AdminUIEnabled bool   // Whether the admin UI is enabled
	AdminUIPath    string // Base path for the admin UI

	// Logging
	LogLevel  string // Log level (debug, info, warn, error)
	LogFormat string // Log format (json, text)
	LogFile   string // Path to log file (empty for stdout)

	// CORS settings
	CORSAllowedOrigins []string      // Allowed origins for CORS
	CORSAllowedMethods []string      // Allowed methods for CORS
	CORSAllowedHeaders []string      // Allowed headers for CORS
	CORSMaxAge         time.Duration // Max age for CORS preflight responses

	// Rate limiting
	GlobalRateLimit int // Maximum requests per minute globally
	IPRateLimit     int // Maximum requests per minute per IP

	// Monitoring
	EnableMetrics bool   // Whether to enable Prometheus metrics endpoint
	MetricsPath   string // Path for metrics endpoint

	// Cleanup
	TokenCleanupInterval time.Duration // Interval for cleaning up expired tokens
}

// New creates a new configuration with values from environment variables.
// It applies default values where environment variables are not set,
// and validates required configuration settings.
//
// Returns a populated Config struct and nil error on success,
// or nil and an error if validation fails.
func New() (*Config, error) {
	config := &Config{
		// Server defaults
		ListenAddr:        getEnvString("LISTEN_ADDR", ":8080"),
		RequestTimeout:    getEnvDuration("REQUEST_TIMEOUT", 30*time.Second),
		MaxRequestSize:    getEnvInt64("MAX_REQUEST_SIZE", 10*1024*1024), // 10MB
		MaxConcurrentReqs: getEnvInt("MAX_CONCURRENT_REQUESTS", 100),

		// Database defaults
		DatabasePath:     getEnvString("DATABASE_PATH", "./data/llm-proxy.db"),
		DatabasePoolSize: getEnvInt("DATABASE_POOL_SIZE", 10),

		// Authentication
		ManagementToken: getEnvString("MANAGEMENT_TOKEN", ""),

		// OpenAI API settings
		OpenAIAPIURL:    getEnvString("OPENAI_API_URL", "https://api.openai.com"),
		EnableStreaming: getEnvBool("ENABLE_STREAMING", true),

		// Admin UI settings
		AdminUIEnabled: getEnvBool("ADMIN_UI_ENABLED", true),
		AdminUIPath:    getEnvString("ADMIN_UI_PATH", "/admin"),

		// Logging defaults
		LogLevel:  getEnvString("LOG_LEVEL", "info"),
		LogFormat: getEnvString("LOG_FORMAT", "json"),
		LogFile:   getEnvString("LOG_FILE", ""),

		// CORS defaults
		CORSAllowedOrigins: getEnvStringSlice("CORS_ALLOWED_ORIGINS", []string{"*"}),
		CORSAllowedMethods: getEnvStringSlice("CORS_ALLOWED_METHODS", []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"}),
		CORSAllowedHeaders: getEnvStringSlice("CORS_ALLOWED_HEADERS", []string{"Authorization", "Content-Type"}),
		CORSMaxAge:         getEnvDuration("CORS_MAX_AGE", 24*time.Hour),

		// Rate limiting defaults
		GlobalRateLimit: getEnvInt("GLOBAL_RATE_LIMIT", 100),
		IPRateLimit:     getEnvInt("IP_RATE_LIMIT", 30),

		// Monitoring defaults
		EnableMetrics: getEnvBool("ENABLE_METRICS", true),
		MetricsPath:   getEnvString("METRICS_PATH", "/metrics"),

		// Cleanup defaults
		TokenCleanupInterval: getEnvDuration("TOKEN_CLEANUP_INTERVAL", time.Hour),
	}

	// Validate required settings
	if config.ManagementToken == "" {
		return nil, fmt.Errorf("MANAGEMENT_TOKEN environment variable is required")
	}

	return config, nil
}

// getEnvString retrieves a string value from an environment variable,
// falling back to the provided default value if the variable is not set.
func getEnvString(key, defaultValue string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}
	return defaultValue
}

// getEnvBool retrieves a boolean value from an environment variable,
// falling back to the provided default value if the variable is not set
// or cannot be parsed as a boolean.
func getEnvBool(key string, defaultValue bool) bool {
	if value, exists := os.LookupEnv(key); exists {
		parsedValue, err := strconv.ParseBool(value)
		if err == nil {
			return parsedValue
		}
	}
	return defaultValue
}

// getEnvInt retrieves an integer value from an environment variable,
// falling back to the provided default value if the variable is not set
// or cannot be parsed as an integer.
func getEnvInt(key string, defaultValue int) int {
	if value, exists := os.LookupEnv(key); exists {
		parsedValue, err := strconv.Atoi(value)
		if err == nil {
			return parsedValue
		}
	}
	return defaultValue
}

// getEnvInt64 retrieves a 64-bit integer value from an environment variable,
// falling back to the provided default value if the variable is not set
// or cannot be parsed as a 64-bit integer.
func getEnvInt64(key string, defaultValue int64) int64 {
	if value, exists := os.LookupEnv(key); exists {
		parsedValue, err := strconv.ParseInt(value, 10, 64)
		if err == nil {
			return parsedValue
		}
	}
	return defaultValue
}

// getEnvDuration retrieves a duration value from an environment variable,
// falling back to the provided default value if the variable is not set
// or cannot be parsed as a duration.
func getEnvDuration(key string, defaultValue time.Duration) time.Duration {
	if value, exists := os.LookupEnv(key); exists {
		parsedValue, err := time.ParseDuration(value)
		if err == nil {
			return parsedValue
		}
	}
	return defaultValue
}

// getEnvStringSlice retrieves a comma-separated string value from an environment variable
// and splits it into a slice of strings, falling back to the provided default value
// if the variable is not set or is empty.
func getEnvStringSlice(key string, defaultValue []string) []string {
	if value, exists := os.LookupEnv(key); exists && value != "" {
		parts := strings.Split(value, ",")
		for i := range parts {
			parts[i] = strings.TrimSpace(parts[i])
		}
		return parts
	}
	return defaultValue
}
