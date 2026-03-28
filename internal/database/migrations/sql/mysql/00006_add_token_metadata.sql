-- +goose Up
ALTER TABLE tokens ADD COLUMN IF NOT EXISTS metadata TEXT;

-- +goose Down
ALTER TABLE tokens DROP COLUMN IF EXISTS metadata;