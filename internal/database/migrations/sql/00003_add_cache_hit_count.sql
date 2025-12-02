-- +goose Up
-- Add cache_hit_count column to tokens table for tracking per-token cache hits
-- This supports the async cache stats aggregator feature

-- Add cache_hit_count column to tokens
ALTER TABLE tokens ADD COLUMN cache_hit_count INTEGER NOT NULL DEFAULT 0;

-- +goose Down
-- Rollback: Remove cache_hit_count column
-- Note: SQLite doesn't support DROP COLUMN directly. For SQLite, this is a no-op.
-- For PostgreSQL, use the postgres-specific migration.
-- In practice, this column is rarely removed, so this is acceptable.
