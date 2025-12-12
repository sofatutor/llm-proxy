-- LLM Proxy SQLite Schema
-- This is the authoritative current schema for SQLite (dev/test only).
-- PostgreSQL uses goose migrations in internal/database/migrations/sql/postgres/
-- NO migrations for SQLite - just update this file for schema changes.

-- Projects table
CREATE TABLE IF NOT EXISTS projects (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL UNIQUE,
    openai_api_key TEXT NOT NULL,
    is_active BOOLEAN NOT NULL DEFAULT 1,
    deactivated_at DATETIME,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- Create index on project name
CREATE INDEX IF NOT EXISTS idx_projects_name ON projects(name);

-- Tokens table
CREATE TABLE IF NOT EXISTS tokens (
    id TEXT PRIMARY KEY,
    token TEXT NOT NULL UNIQUE,
    project_id TEXT NOT NULL,
    expires_at DATETIME,
    is_active BOOLEAN NOT NULL DEFAULT 1,
    deactivated_at DATETIME,
    request_count INTEGER NOT NULL DEFAULT 0,
    max_requests INTEGER,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    last_used_at DATETIME,
    cache_hit_count INTEGER NOT NULL DEFAULT 0,
    FOREIGN KEY (project_id) REFERENCES projects(id) ON DELETE CASCADE
);

-- Create indexes on tokens
CREATE INDEX IF NOT EXISTS idx_tokens_token ON tokens(token);
CREATE INDEX IF NOT EXISTS idx_tokens_project_id ON tokens(project_id);
CREATE INDEX IF NOT EXISTS idx_tokens_expires_at ON tokens(expires_at);
CREATE INDEX IF NOT EXISTS idx_tokens_is_active ON tokens(is_active);

-- Audit events table for security logging
CREATE TABLE IF NOT EXISTS audit_events (
    id TEXT PRIMARY KEY,
    timestamp DATETIME NOT NULL,
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

-- Create indexes on audit events
CREATE INDEX IF NOT EXISTS idx_audit_timestamp ON audit_events(timestamp);
CREATE INDEX IF NOT EXISTS idx_audit_action ON audit_events(action);
CREATE INDEX IF NOT EXISTS idx_audit_project_id ON audit_events(project_id);
CREATE INDEX IF NOT EXISTS idx_audit_client_ip ON audit_events(client_ip);
CREATE INDEX IF NOT EXISTS idx_audit_request_id ON audit_events(request_id);
CREATE INDEX IF NOT EXISTS idx_audit_outcome ON audit_events(outcome);
CREATE INDEX IF NOT EXISTS idx_audit_ip_action ON audit_events(client_ip, action);

-- Enable foreign key support
PRAGMA foreign_keys = ON;

-- Use Write-Ahead Logging for better concurrency
PRAGMA journal_mode = WAL;