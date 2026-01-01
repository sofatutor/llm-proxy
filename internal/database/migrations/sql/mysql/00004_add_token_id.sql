-- +goose Up
-- Add UUID id column to tokens table and change PRIMARY KEY (MySQL)
-- Security: Separates token secret (auth) from identifier (management URLs)

-- Step 1: Add id column with CHAR(36) type for UUID storage
ALTER TABLE tokens ADD COLUMN id CHAR(36) DEFAULT NULL;

-- Step 2: Populate id column with UUIDs for existing rows
UPDATE tokens SET id = UUID() WHERE id IS NULL;

-- Step 3: Make id NOT NULL
ALTER TABLE tokens MODIFY COLUMN id CHAR(36) NOT NULL;

-- Step 4: Drop old PRIMARY KEY constraint on token
ALTER TABLE tokens DROP PRIMARY KEY;

-- Step 5: Add PRIMARY KEY constraint on id
ALTER TABLE tokens ADD PRIMARY KEY (id);

-- Step 6: Add UNIQUE constraint on token (it's still unique, just not PK)
-- Note: UNIQUE constraint automatically creates an index for authentication lookups
ALTER TABLE tokens ADD CONSTRAINT tokens_token_unique UNIQUE (token);

-- +goose Down
-- Rollback: Revert to token as PRIMARY KEY (MySQL)
-- WARNING: This reverts to the security-problematic design where secrets are in URLs

-- Drop new constraints and indexes
ALTER TABLE tokens DROP INDEX tokens_token_unique;
ALTER TABLE tokens DROP PRIMARY KEY;

-- Restore token as PRIMARY KEY
ALTER TABLE tokens ADD PRIMARY KEY (token);

-- Drop the id column
ALTER TABLE tokens DROP COLUMN id;
