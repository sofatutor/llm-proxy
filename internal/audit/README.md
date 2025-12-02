# Audit Package

This package provides audit logging functionality for security-sensitive events in the LLM proxy. It implements a separate audit sink with immutable semantics for compliance and security investigations.

## Purpose & Responsibilities

The `audit` package handles security audit logging:

- **Security Event Tracking**: Records security-sensitive operations (token lifecycle, project management, proxy access)
- **Immutable Audit Trail**: Write-once semantics for compliance requirements
- **Dual Backend Support**: Writes to both file (JSONL) and database
- **Standardized Event Schema**: Canonical fields for consistent audit analysis
- **Token Obfuscation**: Automatically redacts sensitive token data

## Relationship to Application Logging

The audit package is **separate from application logging**:

| Aspect | Application Logging | Audit Logging |
|--------|--------------------|--------------| 
| Purpose | Debugging, monitoring | Security, compliance |
| Events | All operations | Security-sensitive only |
| Retention | Rotated regularly | Long-term retention |
| Modification | Can be filtered/suppressed | Immutable records |
| Package | `internal/logging` | `internal/audit` |

## Configuration Options

| Environment Variable | Description | Default |
|---------------------|-------------|---------|
| `AUDIT_LOG_PATH` | Path to audit log file | Required |
| `AUDIT_DB_ENABLED` | Enable database audit storage | `false` |

## Event Schema

All audit events follow this structure:

```go
type Event struct {
    Timestamp     time.Time              `json:"timestamp"`
    Action        string                 `json:"action"`
    Actor         string                 `json:"actor"`
    ProjectID     string                 `json:"project_id,omitempty"`
    RequestID     string                 `json:"request_id,omitempty"`
    CorrelationID string                 `json:"correlation_id,omitempty"`
    ClientIP      string                 `json:"client_ip,omitempty"`
    Result        ResultType             `json:"result"`
    Details       map[string]interface{} `json:"details,omitempty"`
}
```

### Result Types

| Result | Description |
|--------|-------------|
| `success` | Operation completed successfully |
| `failure` | Operation failed |
| `denied` | Operation denied by policy |
| `error` | Operation encountered an error |

## Action Categories

### Token Lifecycle Actions

| Action | Description |
|--------|-------------|
| `token.create` | New token created |
| `token.read` | Token details accessed |
| `token.update` | Token modified |
| `token.revoke` | Single token revoked |
| `token.revoke_batch` | Multiple tokens revoked |
| `token.delete` | Token deleted |
| `token.list` | Token listing accessed |
| `token.validate` | Token validation attempted |
| `token.access` | Token used for API access |

### Project Lifecycle Actions

| Action | Description |
|--------|-------------|
| `project.create` | New project created |
| `project.read` | Project details accessed |
| `project.update` | Project modified (including `is_active` changes) |
| `project.delete` | Project deleted |
| `project.list` | Project listing accessed |

### Proxy Request Actions

| Action | Description |
|--------|-------------|
| `proxy_request` | API request proxied (with result: denied, error) |

### Admin Actions

| Action | Description |
|--------|-------------|
| `admin.login` | Admin UI login |
| `admin.logout` | Admin UI logout |
| `admin.access` | Admin resource accessed |
| `audit.list` | Audit log listing |
| `audit.show` | Audit event details |
| `cache.purge` | Cache purge operation |

## Creating an Audit Logger

### File-Only Logger

```go
package main

import (
    "github.com/sofatutor/llm-proxy/internal/audit"
)

func main() {
    logger, err := audit.NewLogger(audit.LoggerConfig{
        FilePath:  "/var/log/llm-proxy/audit.jsonl",
        CreateDir: true,  // Create parent directories if needed
    })
    if err != nil {
        panic(err)
    }
    defer logger.Close()
    
    // Log events...
}
```

### File + Database Logger

```go
logger, err := audit.NewLogger(audit.LoggerConfig{
    FilePath:       "/var/log/llm-proxy/audit.jsonl",
    CreateDir:      true,
    DatabaseStore:  dbStore,  // Implements audit.DatabaseStore
    EnableDatabase: true,
})
```

### Null Logger (for testing)

```go
logger := audit.NewNullLogger()  // Discards all events
```

## Creating and Logging Events

### Basic Event

```go
event := audit.NewEvent(audit.ActionTokenCreate, "admin", audit.ResultSuccess)
event.WithProjectID("proj-123")
event.WithRequestID("req-abc")

err := logger.Log(event)
```

### Event with Details

```go
event := audit.NewEvent(audit.ActionTokenRevoke, audit.ActorManagement, audit.ResultSuccess).
    WithProjectID("proj-123").
    WithRequestID("req-abc").
    WithClientIP("192.168.1.100").
    WithTokenID("sk-abc123...").  // Automatically obfuscated
    WithDetail("reason", "expired").
    WithUserAgent("Mozilla/5.0...")

err := logger.Log(event)
```

### Error Events

```go
event := audit.NewEvent(audit.ActionProxyRequest, tokenID, audit.ResultError).
    WithProjectID(projectID).
    WithError(err).
    WithReason("service_unavailable")

logger.Log(event)
```

### Denied Events

```go
event := audit.NewEvent(audit.ActionProxyRequest, tokenID, audit.ResultDenied).
    WithProjectID(projectID).
    WithReason("project_inactive").
    WithClientIP(clientIP).
    WithHTTPMethod("POST").
    WithEndpoint("/v1/chat/completions")

logger.Log(event)
```

## Event Builder Methods

All builder methods return the event for chaining:

```go
event := audit.NewEvent(action, actor, result).
    WithProjectID(projectID).
    WithRequestID(requestID).
    WithCorrelationID(correlationID).
    WithClientIP(clientIP).
    WithTokenID(token).      // Obfuscated
    WithError(err).
    WithUserAgent(userAgent).
    WithHTTPMethod(method).
    WithEndpoint(endpoint).
    WithDuration(duration).
    WithReason(reason).
    WithDetail("custom_key", customValue)
```

## Actor Types

| Actor | Description |
|-------|-------------|
| `system` | Automated system operations |
| `anonymous` | Unauthenticated operations |
| `admin` | Admin UI operations |
| `management_api` | Management API operations |
| Token ID | API operations (use obfuscated token) |

## Integration Patterns

### With HTTP Handlers

```go
func (h *Handler) CreateToken(c *gin.Context) {
    // ... create token ...
    
    event := audit.NewEvent(audit.ActionTokenCreate, audit.ActorManagement, audit.ResultSuccess).
        WithProjectID(projectID).
        WithRequestID(c.GetString("request_id")).
        WithClientIP(c.ClientIP()).
        WithTokenID(newToken.ID)
    
    h.auditLogger.Log(event)
}
```

### With Middleware

```go
func AuditMiddleware(logger *audit.Logger) gin.HandlerFunc {
    return func(c *gin.Context) {
        // For denied requests (e.g., inactive project)
        if projectInactive {
            event := audit.NewEvent(audit.ActionProxyRequest, token, audit.ResultDenied).
                WithProjectID(projectID).
                WithReason("project_inactive").
                WithClientIP(c.ClientIP())
            logger.Log(event)
            
            c.AbortWithStatus(403)
            return
        }
        
        c.Next()
    }
}
```

### With Services

```go
type TokenService struct {
    auditLogger *audit.Logger
}

func (s *TokenService) RevokeToken(ctx context.Context, tokenID string) error {
    err := s.store.RevokeToken(ctx, tokenID)
    
    result := audit.ResultSuccess
    if err != nil {
        result = audit.ResultFailure
    }
    
    event := audit.NewEvent(audit.ActionTokenRevoke, audit.ActorManagement, result).
        WithTokenID(tokenID).
        WithError(err)
    
    s.auditLogger.Log(event)
    return err
}
```

## Database Store Interface

To enable database storage, implement this interface:

```go
type DatabaseStore interface {
    StoreAuditEvent(ctx context.Context, event *Event) error
}
```

Example implementation:

```go
type SQLiteAuditStore struct {
    db *sql.DB
}

func (s *SQLiteAuditStore) StoreAuditEvent(ctx context.Context, event *audit.Event) error {
    details, _ := json.Marshal(event.Details)
    _, err := s.db.ExecContext(ctx,
        `INSERT INTO audit_events (timestamp, action, actor, project_id, request_id, result, details)
         VALUES (?, ?, ?, ?, ?, ?, ?)`,
        event.Timestamp, event.Action, event.Actor, event.ProjectID, event.RequestID, event.Result, details,
    )
    return err
}
```

## File Output Format

Audit events are written as JSON Lines (JSONL):

```json
{"timestamp":"2024-01-15T10:30:45.123Z","action":"token.create","actor":"management_api","project_id":"proj-123","request_id":"req-abc","result":"success","details":{"token_id":"sk-ab...89"}}
{"timestamp":"2024-01-15T10:31:00.456Z","action":"proxy_request","actor":"sk-ab...89","project_id":"proj-123","result":"denied","client_ip":"192.168.1.100","details":{"reason":"project_inactive"}}
```

## Testing Guidance

### Using Null Logger

```go
func TestHandler(t *testing.T) {
    logger := audit.NewNullLogger()
    handler := NewHandler(logger)
    
    // Events are discarded
    handler.CreateToken(ctx)
}
```

### Testing Event Content

```go
func TestAuditEvent(t *testing.T) {
    event := audit.NewEvent(audit.ActionTokenCreate, audit.ActorManagement, audit.ResultSuccess).
        WithProjectID("proj-123").
        WithTokenID("sk-secret-token-12345")
    
    assert.Equal(t, audit.ActionTokenCreate, event.Action)
    assert.Equal(t, "proj-123", event.ProjectID)
    // Token should be obfuscated
    assert.NotContains(t, event.Details["token_id"], "secret")
}
```

### Testing with File Output

```go
func TestAuditFileOutput(t *testing.T) {
    tmpFile := filepath.Join(t.TempDir(), "audit.jsonl")
    
    logger, err := audit.NewLogger(audit.LoggerConfig{
        FilePath:  tmpFile,
        CreateDir: false,
    })
    require.NoError(t, err)
    defer logger.Close()
    
    event := audit.NewEvent(audit.ActionTokenCreate, "admin", audit.ResultSuccess)
    logger.Log(event)
    
    // Verify file content
    data, _ := os.ReadFile(tmpFile)
    assert.Contains(t, string(data), "token.create")
}
```

## Security Considerations

1. **Token Obfuscation**: Always use `WithTokenID()` to ensure tokens are obfuscated
2. **No Secrets in Details**: Never add raw secrets to the `Details` map
3. **File Permissions**: Audit log files are created with `0644` permissions
4. **Sync on Write**: Each event triggers a file sync for durability

## Related Documentation

- [Logging Package](../logging/README.md) - Application logging (separate from audit)
- [Database Package](../database/README.md) - Database storage for audit events
- [Instrumentation Guide](../../docs/instrumentation.md) - Complete observability documentation

## Files

| File | Description |
|------|-------------|
| `logger.go` | Audit logger implementation with file and database backends |
| `schema.go` | Event struct, action constants, result types, and builder methods |
