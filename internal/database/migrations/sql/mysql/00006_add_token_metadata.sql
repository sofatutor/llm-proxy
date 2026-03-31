-- +goose Up
ALTER TABLE tokens ADD COLUMN metadata TEXT;

-- +goose Down
ALTER TABLE tokens DROP COLUMN metadata;