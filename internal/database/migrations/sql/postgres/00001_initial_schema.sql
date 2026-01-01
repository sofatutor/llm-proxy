-- +goose Up
-- Initial database schema for llm-proxy (PostgreSQL)
-- This migration creates the base tables: projects, tokens, and audit_events

-- Projects table
CREATE TABLE IF NOT EXISTS projects (
	id TEXT PRIMARY KEY, -- Using TEXT for compatibility with SQLite; consider UUID type for PostgreSQL optimization
	name TEXT NOT NULL UNIQUE,
	api_key TEXT NOT NULL, -- NOTE: Encrypted when ENCRYPTION_KEY is set (AES-256-GCM). Provider-agnostic naming.
	is_active BOOLEAN NOT NULL DEFAULT TRUE,
	created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
	updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- Create index on project name
CREATE INDEX IF NOT EXISTS idx_projects_name ON projects(name);

-- Tokens table
CREATE TABLE IF NOT EXISTS tokens (
	token TEXT PRIMARY KEY, -- Using TEXT for compatibility with SQLite; consider VARCHAR with length constraints for PostgreSQL optimization
	project_id TEXT NOT NULL,
	expires_at TIMESTAMP,
	is_active BOOLEAN NOT NULL DEFAULT TRUE,
	request_count INTEGER NOT NULL DEFAULT 0,
	max_requests INTEGER,
	created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
	last_used_at TIMESTAMP,
	FOREIGN KEY (project_id) REFERENCES projects(id) ON DELETE CASCADE
);

-- Create indexes on tokens
CREATE INDEX IF NOT EXISTS idx_tokens_project_id ON tokens(project_id);
CREATE INDEX IF NOT EXISTS idx_tokens_expires_at ON tokens(expires_at);
CREATE INDEX IF NOT EXISTS idx_tokens_is_active ON tokens(is_active);

-- Audit events table for security logging and firewall rule derivation
CREATE TABLE IF NOT EXISTS audit_events (
	id TEXT PRIMARY KEY,
	timestamp TIMESTAMP NOT NULL,
	action TEXT NOT NULL,
	actor TEXT NOT NULL,
	project_id TEXT,
	request_id TEXT,
	correlation_id TEXT,
	client_ip TEXT,
	method TEXT,
	path TEXT,
	user_agent TEXT,
	outcome TEXT NOT NULL CHECK (outcome IN ('success', 'failure')),
	reason TEXT,
	token_id TEXT,
	metadata TEXT
);

-- Create indexes on audit events for performance and firewall rule queries
CREATE INDEX IF NOT EXISTS idx_audit_timestamp ON audit_events(timestamp);
CREATE INDEX IF NOT EXISTS idx_audit_action ON audit_events(action);
CREATE INDEX IF NOT EXISTS idx_audit_project_id ON audit_events(project_id);
CREATE INDEX IF NOT EXISTS idx_audit_client_ip ON audit_events(client_ip);
CREATE INDEX IF NOT EXISTS idx_audit_request_id ON audit_events(request_id);
CREATE INDEX IF NOT EXISTS idx_audit_outcome ON audit_events(outcome);
CREATE INDEX IF NOT EXISTS idx_audit_ip_action ON audit_events(client_ip, action);

-- +goose Down
-- Rollback: Drop all tables and indexes
DROP INDEX IF EXISTS idx_audit_ip_action;
DROP INDEX IF EXISTS idx_audit_outcome;
DROP INDEX IF EXISTS idx_audit_request_id;
DROP INDEX IF EXISTS idx_audit_client_ip;
DROP INDEX IF EXISTS idx_audit_project_id;
DROP INDEX IF EXISTS idx_audit_action;
DROP INDEX IF EXISTS idx_audit_timestamp;
DROP TABLE IF EXISTS audit_events;

DROP INDEX IF EXISTS idx_tokens_is_active;
DROP INDEX IF EXISTS idx_tokens_expires_at;
DROP INDEX IF EXISTS idx_tokens_project_id;
DROP TABLE IF EXISTS tokens;

DROP INDEX IF EXISTS idx_projects_name;
DROP TABLE IF EXISTS projects;
