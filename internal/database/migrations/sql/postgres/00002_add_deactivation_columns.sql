-- +goose Up
-- Add deactivation support columns to projects and tokens tables (PostgreSQL)

-- Add deactivated_at column to projects
ALTER TABLE projects ADD COLUMN IF NOT EXISTS deactivated_at TIMESTAMP;

-- Add deactivated_at column to tokens
ALTER TABLE tokens ADD COLUMN IF NOT EXISTS deactivated_at TIMESTAMP;

-- +goose Down
-- Rollback: Remove deactivation columns
ALTER TABLE tokens DROP COLUMN IF EXISTS deactivated_at;
ALTER TABLE projects DROP COLUMN IF EXISTS deactivated_at;
