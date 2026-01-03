-- +goose Up
-- Fix outcome CHECK constraint to include all valid ResultType values (MySQL)
-- The original constraint only allowed 'success' and 'failure', but the code
-- also uses 'denied' and 'error' (see internal/audit/schema.go ResultType).
--
-- Approach: drop the old inline CHECK by redefining the column, then add a named
-- constraint that permits all valid ResultType values ('success', 'failure',
-- 'denied', 'error').

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
