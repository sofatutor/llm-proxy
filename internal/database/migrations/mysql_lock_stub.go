//go:build !mysql

package migrations

import "fmt"

// acquireMySQLLock is a stub that returns an error when MySQL support
// is not compiled in. The real implementation is in mysql_lock.go and requires
// the 'mysql' build tag.
//
// MySQL locking will be tested via Docker Compose integration tests.
func (m *MigrationRunner) acquireMySQLLock() (func(), error) {
	return nil, fmt.Errorf("MySQL named locking requires the 'mysql' build tag")
}
