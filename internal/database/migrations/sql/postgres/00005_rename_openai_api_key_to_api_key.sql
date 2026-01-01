-- +goose Up
-- Rename openai_api_key column to api_key for provider-agnostic naming (PostgreSQL)
-- Security: This column stores encrypted API keys when ENCRYPTION_KEY is set

-- Step 1: Rename the column
ALTER TABLE projects RENAME COLUMN openai_api_key TO api_key;

-- +goose Down
-- Rollback: Rename api_key back to openai_api_key (PostgreSQL)

ALTER TABLE projects RENAME COLUMN api_key TO openai_api_key;
