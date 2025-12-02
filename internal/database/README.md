# Database Package

This package manages SQLite database operations for the LLM Proxy:

- Database initialization and migrations
- CRUD operations for projects and tokens
- Index management and query optimization
- Connection pool management and transactions

## Migrations

The `migrations/` subpackage provides database migration functionality using goose.
For setup instructions, run: `go run scripts/complete_migration_install.go`

See migrations/README.md (after installation) for usage details.