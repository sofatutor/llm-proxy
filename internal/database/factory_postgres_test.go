//go:build postgres

package database

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestNewFromConfig_PostgresMissingURL_WithPostgresTag tests that when PostgreSQL
// support is compiled in but the DATABASE_URL is not provided, we get a proper error.
func TestNewFromConfig_PostgresMissingURL_WithPostgresTag(t *testing.T) {
	config := FullConfig{
		Driver:      DriverPostgres,
		DatabaseURL: "",
	}

	db, err := NewFromConfig(config)
	assert.Error(t, err)
	assert.Nil(t, db)
	assert.Contains(t, err.Error(), "DATABASE_URL is required")
}

// TestNewFromConfig_PostgresInvalidURL_WithPostgresTag tests that when PostgreSQL
// support is compiled in but the DATABASE_URL is invalid, we get a connection error.
func TestNewFromConfig_PostgresInvalidURL_WithPostgresTag(t *testing.T) {
	config := FullConfig{
		Driver:      DriverPostgres,
		DatabaseURL: "invalid-not-a-url",
	}

	// This should fail when trying to ping the database
	db, err := NewFromConfig(config)
	assert.Error(t, err)
	assert.Nil(t, db)
}
