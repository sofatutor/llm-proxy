-- Projects table
CREATE TABLE IF NOT EXISTS projects (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL UNIQUE,
    openai_api_key TEXT NOT NULL,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- Create index on project name
CREATE INDEX IF NOT EXISTS idx_projects_name ON projects(name);

-- Tokens table
CREATE TABLE IF NOT EXISTS tokens (
    token TEXT PRIMARY KEY,
    project_id TEXT NOT NULL,
    expires_at DATETIME,
    is_active BOOLEAN NOT NULL DEFAULT 1,
    request_count INTEGER NOT NULL DEFAULT 0,
    max_requests INTEGER,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    last_used_at DATETIME,
    FOREIGN KEY (project_id) REFERENCES projects(id) ON DELETE CASCADE
);

-- Create indexes on tokens
CREATE INDEX IF NOT EXISTS idx_tokens_project_id ON tokens(project_id);
CREATE INDEX IF NOT EXISTS idx_tokens_expires_at ON tokens(expires_at);
CREATE INDEX IF NOT EXISTS idx_tokens_is_active ON tokens(is_active);

-- Enable foreign key support
PRAGMA foreign_keys = ON;

-- Use Write-Ahead Logging for better concurrency
PRAGMA journal_mode = WAL;