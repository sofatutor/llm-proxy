-- +goose Up
-- Rename openai_api_key column to api_key for provider-agnostic naming (PostgreSQL)
-- Security: This column stores encrypted API keys when ENCRYPTION_KEY is set

-- Step 1: Rename the column (idempotent).
-- Some environments may already have `api_key` (e.g., when the base schema was updated).
DO $$
BEGIN
  IF EXISTS (
    SELECT 1
    FROM information_schema.columns
    WHERE table_schema = 'public'
      AND table_name = 'projects'
      AND column_name = 'openai_api_key'
  )
  AND NOT EXISTS (
    SELECT 1
    FROM information_schema.columns
    WHERE table_schema = 'public'
      AND table_name = 'projects'
      AND column_name = 'api_key'
  )
  THEN
    ALTER TABLE projects RENAME COLUMN openai_api_key TO api_key;
  END IF;
END
$$;

-- +goose Down
-- Rollback: Rename api_key back to openai_api_key (PostgreSQL)

DO $$
BEGIN
  IF EXISTS (
    SELECT 1
    FROM information_schema.columns
    WHERE table_schema = 'public'
      AND table_name = 'projects'
      AND column_name = 'api_key'
  )
  AND NOT EXISTS (
    SELECT 1
    FROM information_schema.columns
    WHERE table_schema = 'public'
      AND table_name = 'projects'
      AND column_name = 'openai_api_key'
  )
  THEN
    ALTER TABLE projects RENAME COLUMN api_key TO openai_api_key;
  END IF;
END
$$;
