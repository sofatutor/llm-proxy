---
title: Security Best Practices
parent: Deployment
nav_order: 3
---

# Security Best Practices

This document outlines security best practices for deploying, configuring, and using the LLM Proxy.

## Secrets Management

### API Keys

- **Never hardcode** API keys or sensitive credentials in source code
- Store API keys in environment variables or secure secrets management systems
- Rotate API keys periodically (recommended: every 30-90 days)
- Use different API keys for development, testing, and production environments
- Consider implementing API key encryption at rest in the database

### Environment Variables

- Store sensitive configuration in `.env` files for local development
- **Never commit** `.env` files to version control
- Use secrets management services in production (AWS Secrets Manager, HashiCorp Vault, etc.)
- Restrict environment variable access to only the necessary processes
- Implement validation for required environment variables before application startup

## Token Security

### Management Token

- Use a cryptographically secure random generator for the management token
  ```bash
  # Generate a secure random token
  openssl rand -base64 32
  ```
- Rotate the management token periodically
- Store the management token securely and limit access to authorized personnel
- Consider IP restrictions for management API access

### Access Tokens

- Implement appropriate lifetimes for access tokens (recommended: 30 days or less)
- Enforce token expiration and provide secure refresh mechanisms
- Store token hashes rather than the tokens themselves when possible
- Implement rate limiting per token to prevent abuse
- Enable token revocation and maintain a blocklist for revoked tokens
- Track token usage and implement anomaly detection

## Container Security

### Non-Root Container Execution

The Dockerfile is configured to run as a non-root user:
- Application runs as `appuser` with restricted permissions
- File permissions are set to minimal required access
- Volumes are owned by the non-root user

### Container Hardening

- Use the latest base images and keep them updated
- Remove unnecessary packages and utilities from the container
- Set appropriate file permissions (principle of least privilege)
- Enable security scanning in CI/CD pipelines
- Use security-focused linters for Dockerfiles
- Implement container runtime security (seccomp, AppArmor, SELinux)

### Docker Recommendations

- Run containers with read-only filesystem where possible
- Limit container resources (CPU, memory)
- Use user namespaces to further isolate container processes
- Implement container-level network policies
- Scan images for vulnerabilities before deployment

## Network Security

### TLS Configuration

- Always enable HTTPS in production with proper TLS certificates
- Use TLS 1.2 or higher (TLS 1.3 preferred)
- Configure secure cipher suites
- Implement HTTP Strict Transport Security (HSTS)
- Consider using Let's Encrypt for certificate automation

### API Security

- Implement strict CORS policies (avoid wildcard `*` origins in production)
- Use rate limiting to prevent abuse
- Validate and sanitize all inputs
- Return appropriate error codes without leaking sensitive information
- Implement request timeout to prevent DoS attacks

## Logging and Monitoring

### Secure Logging

- Mask sensitive data in logs (API keys, tokens, personal information)
- Implement structured logging for better analysis
- Set appropriate log levels (avoid DEBUG in production)
- Secure log storage and transmission
- Implement log rotation and retention policies

### Audit Logging

The LLM Proxy provides comprehensive audit logging for security-sensitive operations, designed for compliance and security investigations.

#### Configuration

Audit logging is controlled by these environment variables:

- **`AUDIT_ENABLED`** (default: `true`): Enable/disable audit logging entirely
- **`AUDIT_LOG_FILE`** (default: `./data/audit.log`): Path to audit log file
- **`AUDIT_CREATE_DIR`** (default: `true`): Create parent directories if they don't exist
- **`AUDIT_STORE_IN_DB`** (default: `true`): Store audit events in database for analytics

#### Enabling and Disabling

```bash
# Enable audit logging (default)
AUDIT_ENABLED=true
AUDIT_LOG_FILE=./data/audit.log
AUDIT_STORE_IN_DB=true

# Disable audit logging
AUDIT_ENABLED=false

# File-only audit logging (no database storage)
AUDIT_ENABLED=true
AUDIT_LOG_FILE=./data/audit.log
AUDIT_STORE_IN_DB=false

# Database-only audit logging (no file)
AUDIT_ENABLED=true
AUDIT_LOG_FILE=""
AUDIT_STORE_IN_DB=true
```

#### Privacy and Data Protection

**Token Obfuscation Guarantees:**
- All tokens in audit logs are automatically obfuscated using a consistent pattern
- Original tokens are never stored in audit logs in plaintext
- Obfuscation preserves prefix (e.g., `tok-`) and shows partial content for identification
- For tokens with known prefixes (such as `tok-`, `sk-`, `api_`, `Bearer `, or `ghp_`): the obfuscation logic shows the first 4 and last 4 characters of the token, with asterisks in the middle.
- Example: `tok-1234567890abcdef` becomes `tok-1234****cdef`
- For tokens without a recognized prefix, the obfuscation may show only the first and last 2 characters, or fully mask the value depending on configuration.
- Example: `abcd1234efgh5678` (no known prefix) becomes `ab********78`

**No-Secrets Policy:**
- API keys are never logged in audit events
- Request/response bodies containing sensitive data are not logged
- Only metadata (method, endpoint, status, duration) is captured
- Personal Identifiable Information (PII) is not logged

**Data Minimization:**
- Audit events contain only necessary information for security investigations
- Client IP addresses are logged but can be masked via configuration
- User-Agent strings are logged for security analysis
- Request IDs enable correlation without exposing sensitive data

#### Retention Guidance

**File Backend Retention:**
- Implement log rotation using standard tools (logrotate, Docker volume rotation)
- Recommended retention: 90 days minimum for compliance, 1 year for enhanced security
- Secure storage with appropriate file permissions (600 or 640)
- Consider encryption at rest for sensitive environments

```bash
# Example logrotate configuration
./data/audit.log {
    daily
    rotate 90
    compress
    delaycompress
    missingok
    notifempty
    copytruncate
}
```

**Database Backend Retention:**
- Database storage enables structured queries and analytics
- Implement automated cleanup of old audit events
- Consider partitioning by date for large deployments
- Recommended indexes on timestamp, action, and project_id fields

```sql
-- Example cleanup query (run via scheduled job)
-- For databases that support LIMIT in DELETE (e.g., MySQL, SQLite):
DELETE FROM audit_events 
WHERE timestamp < datetime('now', '-90 days')
LIMIT 1000;

-- Repeat the cleanup job until no more rows match the condition.

-- For databases that do not support LIMIT in DELETE (e.g., PostgreSQL), consider using a CTE:
-- WITH del AS (
--   SELECT ctid FROM audit_events WHERE timestamp < NOW() - INTERVAL '90 days' LIMIT 1000
-- )
-- DELETE FROM audit_events WHERE ctid IN (SELECT ctid FROM del);

-- Recommended indexes
CREATE INDEX idx_audit_timestamp ON audit_events(timestamp);
CREATE INDEX idx_audit_action ON audit_events(action);
CREATE INDEX idx_audit_project ON audit_events(project_id);
```

#### Event Format

Audit events are stored in JSONL format (file backend) or structured database records:

```json
{
  "timestamp": "2023-12-01T10:00:00Z",
  "action": "token.create",
  "actor": "management",
  "project_id": "proj-123",
  "request_id": "req-456",
  "client_ip": "192.168.1.100",
  "result": "success",
  "details": {
    "http_method": "POST",
    "endpoint": "/manage/tokens",
    "duration_minutes": 60,
    "token_id": "tok-1234****cdef"
  }
}
```

#### Audited Events

The following operations generate audit events:

- **Token Operations**: create, revoke, delete, validate, access
- **Project Operations**: create, read, update, delete
- **Authentication**: successful and failed authentication attempts
- **Management API**: all administrative operations

#### Usage Examples

**Basic Audit Logger Setup**:
```go
// File-only audit logging
auditConfig := audit.LoggerConfig{
    FilePath:  "./data/audit.log",
    CreateDir: true,
}
auditLogger, err := audit.NewLogger(auditConfig)
if err != nil {
    log.Fatal(err)
}
defer auditLogger.Close()

// Log a token creation event
event := audit.NewEvent(audit.ActionTokenCreate, audit.ActorManagement, audit.ResultSuccess).
    WithProjectID("proj-123").
    WithRequestID("req-456").
    WithTokenID("tok-1234567890abcdef").
    WithDetail("duration_minutes", 60)

err = auditLogger.Log(event)
if err != nil {
    log.Printf("Failed to log audit event: %v", err)
}
```

**Dual Storage (File + Database)**:
```go
// Enable both file and database storage
auditConfig := audit.LoggerConfig{
    FilePath:       "./data/audit.log",
    CreateDir:      true,
    DatabaseStore:  db,
    EnableDatabase: true,
}
auditLogger, err := audit.NewLogger(auditConfig)
```

**Audit Event Correlation**:
```go
// Create audit event with request context
func (s *Server) auditTokenAccess(r *http.Request, tokenID, projectID string, result audit.ResultType) {
    requestID, _ := logging.GetRequestID(r.Context())
    clientIP := s.getClientIP(r)
    
    event := audit.NewEvent(audit.ActionTokenAccess, audit.ActorClient, result).
        WithRequestID(requestID).
        WithProjectID(projectID).
        WithTokenID(tokenID).
        WithClientIP(clientIP).
        WithHTTPMethod(r.Method).
        WithEndpoint(r.URL.Path)
    
    _ = s.auditLogger.Log(event)
}
```

**Query Audit Events**:
```bash
# Search audit log file for token events
grep "token\." /data/audit.log | jq '.timestamp, .action, .result'

# Find all failed authentication attempts
grep '"result":"failure"' /data/audit.log | grep "token.access"

# Extract events for specific project
grep '"project_id":"proj-123"' /data/audit.log
```

### Security Monitoring

- Monitor for unusual access patterns
- Set up alerts for potential security incidents
- Regularly review audit logs for security investigations
- Consider implementing a Web Application Firewall (WAF)
- Use audit logs for compliance reporting and forensic analysis

## Database Security

### SQLite Security

- **File Permissions**: Restrict database file access to the application user only
  ```bash
  chmod 600 ./data/llm-proxy.db
  ```
- **Directory Permissions**: Secure the data directory
  ```bash
  chmod 700 ./data
  ```
- **Encryption**: Consider SQLite encryption extensions for sensitive deployments
- **Backup Security**: Encrypt database backups and secure backup storage

### PostgreSQL Security

- **Connection Security**: Always use SSL in production
  ```bash
  DATABASE_URL=postgres://user:pass@host:5432/db?sslmode=require
  ```
- **Authentication**: Use strong passwords and consider certificate-based authentication
- **Network Security**: Limit PostgreSQL port access to application servers only
- **User Privileges**: Create a dedicated database user with minimal required privileges
  ```sql
  CREATE USER llmproxy WITH PASSWORD 'secure_password';
  GRANT CONNECT ON DATABASE llmproxy TO llmproxy;
  GRANT USAGE ON SCHEMA public TO llmproxy;
  GRANT SELECT, INSERT, UPDATE, DELETE ON ALL TABLES IN SCHEMA public TO llmproxy;
  GRANT USAGE, SELECT ON ALL SEQUENCES IN SCHEMA public TO llmproxy;
  -- Ensure future tables/sequences also get privileges:
  ALTER DEFAULT PRIVILEGES IN SCHEMA public GRANT SELECT, INSERT, UPDATE, DELETE ON TABLES TO llmproxy;
  ALTER DEFAULT PRIVILEGES IN SCHEMA public GRANT USAGE, SELECT ON SEQUENCES TO llmproxy;
  ```
- **Encryption at Rest**: Enable PostgreSQL data encryption for sensitive data
- **Audit Logging**: Enable PostgreSQL audit logging for compliance
  ```sql
  ALTER SYSTEM SET log_statement = 'ddl';
  ALTER SYSTEM SET log_connections = 'on';
  ALTER SYSTEM SET log_disconnections = 'on';
  ```

### Connection Pool Security

- **Limit Pool Size**: Set appropriate connection limits to prevent resource exhaustion
  ```bash
  DATABASE_POOL_SIZE=10
  DATABASE_MAX_IDLE_CONNS=5
  ```
- **Connection Lifetime**: Rotate connections periodically
  ```bash
  DATABASE_CONN_MAX_LIFETIME=30m
  ```

### Data Protection

#### Encryption at Rest

The LLM Proxy supports encryption of sensitive data at rest. When enabled:

- **API Keys**: Encrypted using AES-256-GCM with random nonces
- **Tokens**: Hashed using SHA-256 for lookup, with backward compatibility

**Enabling Encryption:**

```bash
# Generate a 32-byte encryption key
export ENCRYPTION_KEY=$(openssl rand -base64 32)

# Start the server - encryption is enabled automatically
llm-proxy server
```

**Migrating Existing Data:**

```bash
# Check current encryption status
llm-proxy migrate encrypt-status

# Encrypt existing plaintext data (idempotent - skips already encrypted data)
llm-proxy migrate encrypt --db ./data/llm-proxy.db
```

**Important:**
- Store the `ENCRYPTION_KEY` securely - loss of this key means data cannot be decrypted
- Back up the key separately from the database
- Use a secrets manager in production (AWS Secrets Manager, HashiCorp Vault, etc.)
- The encryption is backward compatible - unencrypted data is read transparently

#### API Key Storage

With encryption enabled:
- API keys are encrypted before storage using AES-256-GCM
- Each encryption uses a unique random nonce
- Encrypted values are prefixed with `enc:v1:` for identification
- Decryption happens transparently when reading

Without encryption (not recommended for production):
- API keys are stored in plaintext
- A warning is logged at server startup

#### Token Storage

With hashing enabled (automatic when `ENCRYPTION_KEY` is set):
- Tokens are hashed using SHA-256 for database lookup
- Original tokens are never stored in the database
- Token validation uses the hash for lookup

- **PII Handling**: Minimize storage of personally identifiable information
- **Data Retention**: Implement automated data cleanup policies

## Regular Security Practices

- Update dependencies regularly to patch security vulnerabilities
- Conduct security code reviews
- Implement automated security scanning in CI/CD
- Follow the principle of least privilege for all components
- Document and test incident response procedures

## Development Security

- Validate and sanitize all inputs
- Use prepared statements for database queries
- Implement proper error handling without leaking sensitive information
- Follow secure coding guidelines
- Keep security dependencies updated