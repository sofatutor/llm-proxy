-- +goose Up
-- Add deactivation support columns to projects and tokens tables (MySQL)

-- Add deactivated_at column to projects
ALTER TABLE projects ADD COLUMN deactivated_at DATETIME(6) DEFAULT NULL;

-- Add deactivated_at column to tokens
ALTER TABLE tokens ADD COLUMN deactivated_at DATETIME(6) DEFAULT NULL;

-- +goose Down
-- Rollback: Remove deactivation columns
ALTER TABLE tokens DROP COLUMN deactivated_at;
ALTER TABLE projects DROP COLUMN deactivated_at;
