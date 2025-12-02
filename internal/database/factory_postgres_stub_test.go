//go:build !postgres

package database

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestNewFromConfig_PostgresNotCompiledIn tests that when PostgreSQL support
// is not compiled in, attempting to use postgres driver returns an appropriate error.
func TestNewFromConfig_PostgresNotCompiledIn(t *testing.T) {
	config := FullConfig{
		Driver:      DriverPostgres,
		DatabaseURL: "postgres://localhost:5432/testdb",
	}

	db, err := NewFromConfig(config)
	assert.Error(t, err)
	assert.Nil(t, db)
	assert.Contains(t, err.Error(), "PostgreSQL support not compiled in")
}
