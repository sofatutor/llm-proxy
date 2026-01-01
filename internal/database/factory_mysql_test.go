//go:build mysql

package database

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestNewFromConfig_MySQLMissingURL tests that when MySQL
// support is compiled in but the DATABASE_URL is not provided, we get a proper error.
func TestNewFromConfig_MySQLMissingURL(t *testing.T) {
	config := FullConfig{
		Driver:      DriverMySQL,
		DatabaseURL: "",
	}

	db, err := NewFromConfig(config)
	assert.Error(t, err)
	assert.Nil(t, db)
	assert.Contains(t, err.Error(), "DATABASE_URL is required")
}

// TestNewFromConfig_MySQLInvalidURL tests that when MySQL
// support is compiled in but the DATABASE_URL is invalid, we get a connection error.
func TestNewFromConfig_MySQLInvalidURL(t *testing.T) {
	config := FullConfig{
		Driver:      DriverMySQL,
		DatabaseURL: "invalid-not-a-url",
	}

	// This should fail when trying to ping the database
	db, err := NewFromConfig(config)
	assert.Error(t, err)
	assert.Nil(t, db)
}
