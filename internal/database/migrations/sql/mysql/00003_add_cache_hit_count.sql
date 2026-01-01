-- +goose Up
-- Add cache_hit_count column to tokens table for tracking per-token cache hits (MySQL)

-- Add cache_hit_count column to tokens
ALTER TABLE tokens ADD COLUMN cache_hit_count INTEGER NOT NULL DEFAULT 0;

-- +goose Down
-- Rollback: Remove cache_hit_count column
ALTER TABLE tokens DROP COLUMN cache_hit_count;
