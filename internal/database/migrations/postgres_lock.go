//go:build postgres

package migrations

import (
	"fmt"
	"time"
)

// acquirePostgresLock acquires an advisory lock using PostgreSQL's pg_advisory_lock.
// This prevents concurrent migrations when multiple instances start simultaneously.
// The lock is automatically released when the connection closes.
//
// NOTE: This function requires a real PostgreSQL connection and is tested via
// Docker Compose integration tests (see issue #139). It is excluded from
// default coverage calculations using the postgres build tag.
func (m *MigrationRunner) acquirePostgresLock() (func(), error) {
	// Use a fixed lock ID for migrations (derived from "llm-proxy-migrations")
	// This ID is unique enough for our purposes and consistent across instances
	const lockID = 3141592653 // A fixed number to identify this application's migration lock

	maxRetries := 10
	retryDelay := 100 * time.Millisecond

	for i := 0; i < maxRetries; i++ {
		// Try to acquire the advisory lock (non-blocking)
		var acquired bool
		err := m.db.QueryRow("SELECT pg_try_advisory_lock($1)", lockID).Scan(&acquired)
		if err != nil {
			return nil, fmt.Errorf("failed to try advisory lock: %w", err)
		}

		if acquired {
			// Lock acquired successfully
			release := func() {
				_, _ = m.db.Exec("SELECT pg_advisory_unlock($1)", lockID)
			}
			return release, nil
		}

		// Lock not acquired, wait and retry
		if i < maxRetries-1 {
			time.Sleep(retryDelay)
		}
	}

	return nil, fmt.Errorf("failed to acquire PostgreSQL advisory lock after %d retries", maxRetries)
}
