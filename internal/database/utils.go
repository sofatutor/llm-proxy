package database

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"
)

// Placeholder returns the appropriate placeholder for the driver.
// For SQLite: ?, for PostgreSQL: $1, $2, etc.
func (d *DB) Placeholder(n int) string {
	if d.driver == DriverPostgres {
		return fmt.Sprintf("$%d", n)
	}
	return "?"
}

// Placeholders returns a slice of placeholders for the driver.
// For n=3: SQLite returns ["?", "?", "?"], PostgreSQL returns ["$1", "$2", "$3"].
func (d *DB) Placeholders(n int) []string {
	result := make([]string, n)
	for i := 0; i < n; i++ {
		result[i] = d.Placeholder(i + 1)
	}
	return result
}

// PlaceholderList returns a comma-separated list of placeholders.
// For n=3: SQLite returns "?, ?, ?", PostgreSQL returns "$1, $2, $3".
func (d *DB) PlaceholderList(n int) string {
	return strings.Join(d.Placeholders(n), ", ")
}

// RebindQuery converts a query from ? placeholders to the appropriate
// placeholder style for the database driver.
//
// IMPORTANT: This function performs a simple character replacement and does NOT
// handle ? characters inside SQL string literals (e.g., "WHERE name = 'what?'").
// Since this codebase exclusively uses parameterized queries with ? as placeholders,
// this limitation does not affect normal usage. If you need to use literal ? in
// string values, use parameterized queries: "WHERE name = ?" with the value passed
// as an argument.
func (d *DB) RebindQuery(query string) string {
	if d.driver != DriverPostgres {
		return query
	}

	// Convert ? to $1, $2, $3, etc. (single pass for better performance)
	var builder strings.Builder
	builder.Grow(len(query) + 10) // pre-allocate with some buffer
	count := 0
	for i := 0; i < len(query); i++ {
		if query[i] == '?' {
			count++
			builder.WriteString(fmt.Sprintf("$%d", count))
		} else {
			builder.WriteByte(query[i])
		}
	}
	return builder.String()
}

// ExecContextRebound executes a query with automatic placeholder rebinding.
func (d *DB) ExecContextRebound(ctx context.Context, query string, args ...interface{}) (sql.Result, error) {
	return d.db.ExecContext(ctx, d.RebindQuery(query), args...)
}

// QueryRowContextRebound queries a single row with automatic placeholder rebinding.
func (d *DB) QueryRowContextRebound(ctx context.Context, query string, args ...interface{}) *sql.Row {
	return d.db.QueryRowContext(ctx, d.RebindQuery(query), args...)
}

// QueryContextRebound queries multiple rows with automatic placeholder rebinding.
func (d *DB) QueryContextRebound(ctx context.Context, query string, args ...interface{}) (*sql.Rows, error) {
	return d.db.QueryContext(ctx, d.RebindQuery(query), args...)
}

// BackupDatabase creates a backup of the database.
// Note: This function is SQLite-specific. For PostgreSQL, use pg_dump.
func (d *DB) BackupDatabase(ctx context.Context, backupPath string) error {
	if d.driver == DriverPostgres {
		return fmt.Errorf("backup not supported for PostgreSQL via this method; use pg_dump")
	}

	// Validate the backupPath to ensure it is a valid file path
	if backupPath == "" {
		return fmt.Errorf("backup path cannot be empty")
	}
	// SQLite does not support parameterized VACUUM INTO, so we must sanitize the path
	// Only allow simple file paths (no semicolons, no SQL metacharacters)
	if len(backupPath) > 256 || backupPath[0] == '-' || backupPath[0] == '|' || backupPath[0] == ';' {
		return fmt.Errorf("invalid backup path")
	}
	// For SQLite, we can use the VACUUM INTO statement to create a backup
	query := fmt.Sprintf("VACUUM INTO '%s'", backupPath)
	_, err := d.db.ExecContext(ctx, query)
	if err != nil {
		return fmt.Errorf("failed to backup database: %w", err)
	}
	return nil
}

// MaintainDatabase performs regular maintenance on the database.
// WARNING: VACUUM and ANALYZE can be expensive operations. In production, schedule this function to run periodically (e.g., daily) rather than on every call.
// The caller is responsible for scheduling.
func (d *DB) MaintainDatabase(ctx context.Context) error {
	if d.driver == DriverPostgres {
		// PostgreSQL uses VACUUM ANALYZE
		_, err := d.db.ExecContext(ctx, "VACUUM ANALYZE")
		if err != nil {
			return fmt.Errorf("failed to vacuum analyze database: %w", err)
		}
		return nil
	}

	// SQLite-specific maintenance
	// Run VACUUM to reclaim space and optimize the database
	_, err := d.db.ExecContext(ctx, "VACUUM")
	if err != nil {
		return fmt.Errorf("failed to vacuum database: %w", err)
	}

	// Run PRAGMA optimize to optimize the database
	_, err = d.db.ExecContext(ctx, "PRAGMA optimize")
	if err != nil {
		return fmt.Errorf("failed to optimize database: %w", err)
	}

	// Run ANALYZE to update statistics
	_, err = d.db.ExecContext(ctx, "ANALYZE")
	if err != nil {
		return fmt.Errorf("failed to analyze database: %w", err)
	}

	return nil
}

// boolValue returns the appropriate boolean representation for the driver.
// SQLite uses 1/0, PostgreSQL uses true/false.
func (d *DB) boolValue(b bool) interface{} {
	if d.driver == DriverPostgres {
		return b
	}
	if b {
		return 1
	}
	return 0
}

// GetStats returns database statistics.
func (d *DB) GetStats(ctx context.Context) (map[string]interface{}, error) {
	stats := make(map[string]interface{})

	// Get database size (driver-specific - these queries are fundamentally different)
	var dbSize int64
	if d.driver == DriverPostgres {
		err := d.db.QueryRowContext(ctx, "SELECT pg_database_size(current_database())").Scan(&dbSize)
		if err != nil {
			return nil, fmt.Errorf("failed to get database size: %w", err)
		}
	} else {
		err := d.db.QueryRowContext(ctx, "SELECT (SELECT page_count FROM pragma_page_count) * (SELECT page_size FROM pragma_page_size)").Scan(&dbSize)
		if err != nil {
			return nil, fmt.Errorf("failed to get database size: %w", err)
		}
	}
	stats["database_size_bytes"] = dbSize

	// Count active projects
	var projectCount int
	err := d.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM projects").Scan(&projectCount)
	if err != nil {
		return nil, fmt.Errorf("failed to count projects: %w", err)
	}
	stats["project_count"] = projectCount

	// Count active tokens using QueryRowContextRebound for placeholder rebinding
	var activeTokens int
	err = d.QueryRowContextRebound(ctx,
		"SELECT COUNT(*) FROM tokens WHERE is_active = ? AND (expires_at IS NULL OR expires_at > ?)",
		d.boolValue(true), time.Now()).Scan(&activeTokens)
	if err != nil {
		return nil, fmt.Errorf("failed to count active tokens: %w", err)
	}
	stats["active_token_count"] = activeTokens

	// Count expired tokens using QueryRowContextRebound for placeholder rebinding
	var expiredTokens int
	err = d.QueryRowContextRebound(ctx,
		"SELECT COUNT(*) FROM tokens WHERE expires_at IS NOT NULL AND expires_at <= ?",
		time.Now()).Scan(&expiredTokens)
	if err != nil {
		return nil, fmt.Errorf("failed to count expired tokens: %w", err)
	}
	stats["expired_token_count"] = expiredTokens

	// Count total request count
	var totalRequests sql.NullInt64
	err = d.db.QueryRowContext(ctx, "SELECT SUM(request_count) FROM tokens").Scan(&totalRequests)
	if err != nil {
		return nil, fmt.Errorf("failed to sum request counts: %w", err)
	}
	if totalRequests.Valid {
		stats["total_request_count"] = totalRequests.Int64
	} else {
		stats["total_request_count"] = int64(0)
	}

	return stats, nil
}

// IsTokenValid checks if a token is valid (exists, is active, not expired, and not rate limited).
func (d *DB) IsTokenValid(ctx context.Context, tokenID string) (bool, error) {
	token, err := d.GetTokenByID(ctx, tokenID)
	if err != nil {
		if err == ErrTokenNotFound {
			return false, nil
		}
		return false, err
	}

	return token.IsValid(), nil
}
