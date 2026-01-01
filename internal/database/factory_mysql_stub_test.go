//go:build !mysql

package database

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestNewFromConfig_MySQLNotCompiledIn tests that when MySQL support
// is not compiled in, attempting to use mysql driver returns an appropriate error.
func TestNewFromConfig_MySQLNotCompiledIn(t *testing.T) {
	config := FullConfig{
		Driver:      DriverMySQL,
		DatabaseURL: "user:password@tcp(localhost:3306)/dbname?parseTime=true",
	}

	db, err := NewFromConfig(config)
	assert.Error(t, err)
	assert.Nil(t, db)
	assert.Contains(t, err.Error(), "MySQL support not compiled in")
}
