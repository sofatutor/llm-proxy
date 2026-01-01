//go:build mysql

package migrations

import (
	"database/sql"
	"fmt"
	"log"
	"time"
)

// acquireMySQLLock acquires a named lock using MySQL's GET_LOCK function.
// This prevents concurrent migrations when multiple instances start simultaneously.
// The lock is automatically released when the connection closes.
//
// NOTE: This function requires a real MySQL connection and is tested via
// Docker Compose integration tests. It is excluded from default coverage
// calculations using the mysql build tag.
func (m *MigrationRunner) acquireMySQLLock() (func(), error) {
	// Use a fixed lock name for migrations
	const lockName = "llm-proxy-migrations"
	const lockTimeout = 10 // seconds to wait for lock acquisition

	maxRetries := 10
	retryDelay := 100 * time.Millisecond

	for i := 0; i < maxRetries; i++ {
		// Try to acquire the named lock
		// GET_LOCK returns:
		//   1 if lock was acquired
		//   0 if timeout occurred
		//   NULL if an error occurred
		var result sql.NullInt64
		err := m.db.QueryRow("SELECT GET_LOCK(?, ?)", lockName, lockTimeout).Scan(&result)
		if err != nil {
			return nil, fmt.Errorf("failed to try MySQL named lock: %w", err)
		}

		if !result.Valid {
			return nil, fmt.Errorf("MySQL GET_LOCK returned NULL (error occurred)")
		}

		if result.Int64 == 1 {
			// Lock acquired successfully
			release := func() {
				_, err := m.db.Exec("SELECT RELEASE_LOCK(?)", lockName)
				if err != nil {
					// Log warning but don't fail - connection close will release the lock anyway
					log.Printf("Warning: failed to release MySQL named lock: %v", err)
				}
			}
			return release, nil
		}

		// Lock not acquired (timeout or already held), wait and retry
		if i < maxRetries-1 {
			time.Sleep(retryDelay)
		}
	}

	return nil, fmt.Errorf("failed to acquire MySQL named lock after %d retries", maxRetries)
}
