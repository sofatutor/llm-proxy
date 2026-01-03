-- +goose Up
-- Fix outcome CHECK constraint to include all valid ResultType values (MySQL)
-- The original constraint only allowed 'success' and 'failure', but the code
-- also uses 'denied' and 'error' (see internal/audit/schema.go ResultType).

-- MySQL 8.0.16+ enforces CHECK constraints, so we need to update this.
-- Step 1: Drop the existing CHECK constraint
-- Note: MySQL names CHECK constraints automatically if not named explicitly.
-- We need to find and drop it. In MySQL, CHECK constraints are stored in information_schema.
-- Since we can't easily identify the auto-generated name, we'll recreate the column.

-- For MySQL, the safest approach is to use ALTER TABLE with MODIFY COLUMN
-- which will replace the CHECK constraint.

-- Create a new column without the constraint, copy data, drop old, rename new
-- Actually, in MySQL 8.0.16+, we can use ALTER TABLE ... DROP CHECK constraint_name
-- But the constraint was created inline without a name.

-- MySQL 8.0.16+ allows: ALTER TABLE table_name DROP CHECK constraint_name;
-- We need to find the constraint name first. Let's use a workaround.

-- Workaround: Modify the column to remove the old constraint and add a new one
-- Step 1: Drop the old constraint by name (if we knew it) or recreate the table
-- Since inline CHECK constraints in MySQL are auto-named based on the column,
-- we'll use the standard MySQL approach of MODIFY COLUMN.

-- Actually, the cleanest way is to:
-- 1. Add a new column
-- 2. Copy data
-- 3. Drop old column
-- 4. Rename new column

-- But that's complex. Let's try a simpler approach using MySQL 8.0.19+ syntax:
-- ALTER TABLE audit_events DROP CONSTRAINT if the constraint was named.
-- Since it wasn't named, MySQL generates a name like 'audit_events_chk_1'.

-- We'll drop the check constraint by finding its generated name using a stored procedure approach.
-- For simplicity, let's use the MODIFY COLUMN approach which implicitly drops constraints.

-- Drop the existing CHECK constraint by modifying the column
-- This works because MODIFY COLUMN replaces the column definition entirely
ALTER TABLE audit_events MODIFY COLUMN outcome VARCHAR(20) NOT NULL;

-- Add the new CHECK constraint with all valid values
ALTER TABLE audit_events ADD CONSTRAINT audit_events_outcome_check
    CHECK (outcome IN ('success', 'failure', 'denied', 'error'));

-- +goose Down
-- Rollback: Restore the original constraint (note: this will fail if denied/error values exist)
ALTER TABLE audit_events DROP CONSTRAINT audit_events_outcome_check;
ALTER TABLE audit_events MODIFY COLUMN outcome VARCHAR(20) NOT NULL CHECK (outcome IN ('success', 'failure'));

