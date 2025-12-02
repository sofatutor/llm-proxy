-- +goose Up
-- Add cache_hit_count column to tokens table for tracking per-token cache hits (PostgreSQL)

-- Add cache_hit_count column to tokens
ALTER TABLE tokens ADD COLUMN IF NOT EXISTS cache_hit_count INTEGER NOT NULL DEFAULT 0;

-- +goose Down
-- Rollback: Remove cache_hit_count column
ALTER TABLE tokens DROP COLUMN IF EXISTS cache_hit_count;
