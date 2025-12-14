-- +goose Up
-- Add UUID id column to tokens table and change PRIMARY KEY (PostgreSQL)
-- Security: Separates token secret (auth) from identifier (management URLs)

-- Step 1: Add id column with UUID type and auto-generate values
ALTER TABLE tokens ADD COLUMN id UUID DEFAULT gen_random_uuid();

-- Step 2: Populate any NULL ids (shouldn't exist with DEFAULT, but be safe)
UPDATE tokens SET id = gen_random_uuid() WHERE id IS NULL;

-- Step 3: Make id NOT NULL
ALTER TABLE tokens ALTER COLUMN id SET NOT NULL;

-- Step 4: Drop old PRIMARY KEY constraint on token
ALTER TABLE tokens DROP CONSTRAINT tokens_pkey;

-- Step 5: Add PRIMARY KEY constraint on id
ALTER TABLE tokens ADD PRIMARY KEY (id);

-- Step 6: Add UNIQUE constraint on token (it's still unique, just not PK)
ALTER TABLE tokens ADD CONSTRAINT tokens_token_unique UNIQUE (token);

-- Step 7: Add index on token for fast authentication lookups
CREATE INDEX idx_tokens_token ON tokens(token);

-- +goose Down
-- Rollback: Revert to token as PRIMARY KEY (PostgreSQL)
-- WARNING: This reverts to the security-problematic design where secrets are in URLs

-- Drop new constraints and indexes
DROP INDEX IF EXISTS idx_tokens_token;
ALTER TABLE tokens DROP CONSTRAINT IF EXISTS tokens_token_unique;
ALTER TABLE tokens DROP CONSTRAINT IF EXISTS tokens_pkey;

-- Restore token as PRIMARY KEY
ALTER TABLE tokens ADD PRIMARY KEY (token);

-- Drop the id column
ALTER TABLE tokens DROP COLUMN id;