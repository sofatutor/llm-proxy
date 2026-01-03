-- +goose Up
-- Fix outcome CHECK constraint to include all valid ResultType values (PostgreSQL)
-- The original constraint only allowed 'success' and 'failure', but the code
-- also uses 'denied' and 'error' (see internal/audit/schema.go ResultType).

-- PostgreSQL names inline CHECK constraints as: tablename_columnname_check
-- So the constraint is likely named: audit_events_outcome_check

-- Drop the existing constraint
ALTER TABLE audit_events DROP CONSTRAINT IF EXISTS audit_events_outcome_check;

-- Add the new CHECK constraint with all valid values
ALTER TABLE audit_events ADD CONSTRAINT audit_events_outcome_check
    CHECK (outcome IN ('success', 'failure', 'denied', 'error'));

-- +goose Down
-- Rollback: Restore the original constraint (note: this will fail if denied/error values exist)
ALTER TABLE audit_events DROP CONSTRAINT IF EXISTS audit_events_outcome_check;
ALTER TABLE audit_events ADD CONSTRAINT audit_events_outcome_check
    CHECK (outcome IN ('success', 'failure'));

