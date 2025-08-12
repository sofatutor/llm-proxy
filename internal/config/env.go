package config

import (
	"os"
	"strconv"
)

// envOrDefault returns the value of the environment variable if set, otherwise the fallback.
func EnvOrDefault(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

// EnvIntOrDefault returns the int value of the environment variable if set and valid, otherwise the fallback.
func EnvIntOrDefault(key string, fallback int) int {
	if v := os.Getenv(key); v != "" {
		if i, err := strconv.Atoi(v); err == nil {
			return i
		}
	}
	return fallback
}

// EnvBoolOrDefault returns the bool value of the environment variable if set and valid, otherwise the fallback.
func EnvBoolOrDefault(key string, fallback bool) bool {
	if v := os.Getenv(key); v != "" {
		if b, err := strconv.ParseBool(v); err == nil {
			return b
		}
	}
	return fallback
}

// EnvFloat64OrDefault returns the float64 value of the environment variable if set and valid, otherwise the fallback.
func EnvFloat64OrDefault(key string, fallback float64) float64 {
	if v := os.Getenv(key); v != "" {
		if f, err := strconv.ParseFloat(v, 64); err == nil {
			return f
		}
	}
	return fallback
}
