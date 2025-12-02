//go:build !postgres

package database

import "fmt"

// newPostgresDB is a stub that returns an error when PostgreSQL support
// is not compiled in. The real implementation is in factory_postgres.go
// and requires the 'postgres' build tag.
//
// To enable PostgreSQL support, build with: go build -tags postgres ./...
func newPostgresDB(_ FullConfig) (*DB, error) {
	return nil, fmt.Errorf("PostgreSQL support not compiled in; build with -tags postgres to enable")
}
