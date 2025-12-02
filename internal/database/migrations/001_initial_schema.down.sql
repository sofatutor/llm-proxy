-- Rollback initial schema migration for LLM Proxy
-- Drops tables and indexes in reverse order of creation

-- Drop audit_events indexes first
DROP INDEX IF EXISTS idx_audit_ip_action;
DROP INDEX IF EXISTS idx_audit_outcome;
DROP INDEX IF EXISTS idx_audit_request_id;
DROP INDEX IF EXISTS idx_audit_client_ip;
DROP INDEX IF EXISTS idx_audit_project_id;
DROP INDEX IF EXISTS idx_audit_action;
DROP INDEX IF EXISTS idx_audit_timestamp;

-- Drop audit_events table
DROP TABLE IF EXISTS audit_events;

-- Drop tokens indexes
DROP INDEX IF EXISTS idx_tokens_is_active;
DROP INDEX IF EXISTS idx_tokens_expires_at;
DROP INDEX IF EXISTS idx_tokens_project_id;

-- Drop tokens table
DROP TABLE IF EXISTS tokens;

-- Drop projects index
DROP INDEX IF EXISTS idx_projects_name;

-- Drop projects table
DROP TABLE IF EXISTS projects;
