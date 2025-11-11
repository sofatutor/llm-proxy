-- +goose Up
-- Add deactivation support columns to projects and tokens tables
-- This migration adds is_active and deactivated_at columns for soft deactivation
-- Note: For databases created with initDatabase() that already have these columns,
-- this migration will be skipped by goose's version tracking. For new databases,
-- these columns will be added.

-- Add is_active column to projects (with default for existing rows)
-- SQLite will fail if column exists, but goose tracks applied migrations
-- so this only runs once per database
ALTER TABLE projects ADD COLUMN is_active BOOLEAN NOT NULL DEFAULT 1;

-- Add deactivated_at column to projects
ALTER TABLE projects ADD COLUMN deactivated_at DATETIME;

-- Add deactivated_at column to tokens
ALTER TABLE tokens ADD COLUMN deactivated_at DATETIME;

-- +goose Down
-- Rollback: Remove deactivation columns
-- Note: SQLite doesn't support DROP COLUMN directly, but goose handles this
-- For proper rollback, we would need to recreate tables, but for simplicity
-- we'll document that rollback requires manual intervention for SQLite
-- In production with PostgreSQL, DROP COLUMN works directly

-- SQLite limitation: Cannot DROP COLUMN directly
-- This migration cannot be fully rolled back in SQLite without recreating tables
-- For PostgreSQL, uncomment the following:
-- ALTER TABLE tokens DROP COLUMN deactivated_at;
-- ALTER TABLE projects DROP COLUMN deactivated_at;
-- ALTER TABLE projects DROP COLUMN is_active;

-- For SQLite, this is a no-op - manual rollback would require table recreation
-- In practice, deactivation columns are rarely removed, so this is acceptable

