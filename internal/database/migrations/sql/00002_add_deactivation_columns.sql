-- +goose Up
-- Add deactivation support columns to projects and tokens tables
-- This migration adds deactivated_at columns and creates the is_active index
-- Note: is_active columns should exist from migration 00001 (new databases) or
-- from original schema (existing databases). If missing, they need to be added
-- but SQLite doesn't support conditional ALTER TABLE, so we handle it via
-- ensuring columns exist in 00001 for new DBs, and accepting that existing DBs
-- without is_active will need manual intervention or a data migration script.

-- Add deactivated_at column to projects
ALTER TABLE projects ADD COLUMN deactivated_at DATETIME;

-- Add deactivated_at column to tokens
ALTER TABLE tokens ADD COLUMN deactivated_at DATETIME;

-- Note: idx_tokens_is_active index is created in migration 00001

-- +goose Down
-- Rollback: Remove deactivation columns
-- Note: SQLite doesn't support DROP COLUMN directly. Goose cannot automatically
-- generate the table recreation logic required for SQLite rollback, so manual
-- intervention is needed for SQLite environments. For proper rollback, you would
-- need to recreate the tables in SQLite. In production with PostgreSQL, DROP COLUMN
-- works directly.

-- SQLite limitation: Cannot DROP COLUMN directly
-- This migration cannot be fully rolled back in SQLite without recreating tables
-- For PostgreSQL, uncomment the following:
-- ALTER TABLE tokens DROP COLUMN deactivated_at;
-- ALTER TABLE projects DROP COLUMN deactivated_at;

-- For SQLite, this is a no-op - manual rollback would require table recreation
-- In practice, deactivation columns are rarely removed, so this is acceptable
