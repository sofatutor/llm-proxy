//go:build !mysql

package database

import "fmt"

// newMySQLDB is a stub that returns an error when MySQL support
// is not compiled in. The real implementation is in factory_mysql.go
// and requires the 'mysql' build tag.
//
// To enable MySQL support, build with: go build -tags mysql ./...
func newMySQLDB(_ FullConfig) (*DB, error) {
	return nil, fmt.Errorf("MySQL support not compiled in; build with -tags mysql to enable")
}
