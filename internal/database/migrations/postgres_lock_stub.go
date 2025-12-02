//go:build !postgres

package migrations

import "fmt"

// acquirePostgresLock is a stub that returns an error when PostgreSQL support
// is not compiled in. The real implementation is in postgres_lock.go and requires
// the 'postgres' build tag.
//
// PostgreSQL locking will be tested via Docker Compose integration tests (issue #139).
func (m *MigrationRunner) acquirePostgresLock() (func(), error) {
	return nil, fmt.Errorf("PostgreSQL advisory locking requires the 'postgres' build tag; see issue #139 for Docker Compose integration tests")
}
