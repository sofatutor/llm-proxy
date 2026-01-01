-- +goose Up
-- Initial database schema for llm-proxy (MySQL)
-- This migration creates the base tables: projects, tokens, and audit_events

-- Projects table
CREATE TABLE IF NOT EXISTS projects (
	id VARCHAR(191) PRIMARY KEY, -- VARCHAR for compatibility; 191 chars max for utf8mb4 indexes
	name VARCHAR(255) NOT NULL UNIQUE,
	openai_api_key TEXT NOT NULL, -- NOTE: Stored as plaintext. For production, consider encryption at rest or secret manager integration (see review comment #2580739426)
	is_active BOOLEAN NOT NULL DEFAULT TRUE,
	created_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
	updated_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6) ON UPDATE CURRENT_TIMESTAMP(6)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- Create index on project name
CREATE INDEX idx_projects_name ON projects(name);

-- Tokens table
CREATE TABLE IF NOT EXISTS tokens (
	token VARCHAR(191) PRIMARY KEY, -- VARCHAR for compatibility; 191 chars max for utf8mb4 indexes
	project_id VARCHAR(191) NOT NULL,
	expires_at DATETIME(6),
	is_active BOOLEAN NOT NULL DEFAULT TRUE,
	request_count INTEGER NOT NULL DEFAULT 0,
	max_requests INTEGER,
	created_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
	last_used_at DATETIME(6),
	FOREIGN KEY (project_id) REFERENCES projects(id) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- Create indexes on tokens
CREATE INDEX idx_tokens_project_id ON tokens(project_id);
CREATE INDEX idx_tokens_expires_at ON tokens(expires_at);
CREATE INDEX idx_tokens_is_active ON tokens(is_active);

-- Audit events table for security logging and firewall rule derivation
CREATE TABLE IF NOT EXISTS audit_events (
	id VARCHAR(191) PRIMARY KEY,
	timestamp DATETIME(6) NOT NULL,
	action VARCHAR(255) NOT NULL,
	actor VARCHAR(255) NOT NULL,
	project_id VARCHAR(191),
	request_id VARCHAR(191),
	correlation_id VARCHAR(191),
	client_ip VARCHAR(45), -- IPv6 max length is 45 chars
	method VARCHAR(10),
	path TEXT,
	user_agent TEXT,
	outcome VARCHAR(20) NOT NULL CHECK (outcome IN ('success', 'failure')),
	reason TEXT,
	token_id VARCHAR(191),
	metadata TEXT
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- Create indexes on audit events for performance and firewall rule queries
CREATE INDEX idx_audit_timestamp ON audit_events(timestamp);
CREATE INDEX idx_audit_action ON audit_events(action);
CREATE INDEX idx_audit_project_id ON audit_events(project_id);
CREATE INDEX idx_audit_client_ip ON audit_events(client_ip);
CREATE INDEX idx_audit_request_id ON audit_events(request_id);
CREATE INDEX idx_audit_outcome ON audit_events(outcome);
CREATE INDEX idx_audit_ip_action ON audit_events(client_ip, action);

-- +goose Down
-- Rollback: Drop all tables and indexes
DROP INDEX IF EXISTS idx_audit_ip_action ON audit_events;
DROP INDEX IF EXISTS idx_audit_outcome ON audit_events;
DROP INDEX IF EXISTS idx_audit_request_id ON audit_events;
DROP INDEX IF EXISTS idx_audit_client_ip ON audit_events;
DROP INDEX IF EXISTS idx_audit_project_id ON audit_events;
DROP INDEX IF EXISTS idx_audit_action ON audit_events;
DROP INDEX IF EXISTS idx_audit_timestamp ON audit_events;
DROP TABLE IF EXISTS audit_events;

DROP INDEX IF EXISTS idx_tokens_is_active ON tokens;
DROP INDEX IF EXISTS idx_tokens_expires_at ON tokens;
DROP INDEX IF EXISTS idx_tokens_project_id ON tokens;
DROP TABLE IF EXISTS tokens;

DROP INDEX IF EXISTS idx_projects_name ON projects;
DROP TABLE IF EXISTS projects;
