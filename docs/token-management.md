# Token Management Guide

This guide covers the complete lifecycle of tokens in LLM Proxy, from creation to expiration and revocation.

## Overview

LLM Proxy uses **withering tokens** - short-lived tokens that automatically expire after a configurable duration. This provides enhanced security by limiting the exposure window if a token is compromised.

### Key Concepts

- **Project**: A logical grouping with an OpenAI API key. Tokens belong to projects.
- **Withering Token**: A time-limited access token that "withers" (expires) after a set duration.
- **Rate Limits**: Optional per-token request limits to prevent abuse.
- **Soft Deactivation**: Tokens are revoked (deactivated) rather than deleted, preserving audit history.

## Token Lifecycle

```
┌─────────────┐    ┌─────────────┐    ┌─────────────┐    ┌─────────────┐
│   Create    │───>│   Active    │───>│  Expired    │    │   Revoked   │
│   Token     │    │   (in use)  │    │(auto-wither)│    │(manual/bulk)│
└─────────────┘    └─────────────┘    └─────────────┘    └─────────────┘
                         │                                      ▲
                         │          Manual Revoke               │
                         └──────────────────────────────────────┘
```

### Token States

| State | Description | Can be used? |
|-------|-------------|--------------|
| **Active** | Valid token within its lifetime and request limits | ✅ Yes |
| **Expired** | Token has passed its expiration time | ❌ No |
| **Revoked** | Token manually deactivated | ❌ No |
| **Rate Limited** | Active but exceeded request limit | ❌ No |

## Token Properties

When creating a token, you can configure:

| Property | Description | Default |
|----------|-------------|---------|
| `project_id` | Project the token belongs to | Required |
| `duration_hours` | Hours until token expires | 24 |
| `max_requests` | Maximum requests allowed (0 = unlimited) | 0 |
| `name` | Optional descriptive name | - |

## Creating Tokens

### Using the Admin UI

1. Navigate to the Admin UI at http://localhost:8080/admin/
2. Select the project or go to **Tokens** section
3. Click **Generate Token**
4. Configure duration and request limits
5. Copy the generated token (shown only once)

See [Admin UI Token Management](admin/tokens.md) for detailed workflows.

### Using the CLI

```bash
# Generate token with default 24-hour expiration
llm-proxy manage token generate \
  --project-id <project-id> \
  --management-token $MANAGEMENT_TOKEN

# Generate token with custom duration (168 hours = 7 days)
llm-proxy manage token generate \
  --project-id <project-id> \
  --duration 168 \
  --management-token $MANAGEMENT_TOKEN

# Generate token with request limit
llm-proxy manage token generate \
  --project-id <project-id> \
  --duration 24 \
  --max-requests 1000 \
  --management-token $MANAGEMENT_TOKEN
```

### Using the API

```bash
# Generate a token
curl -X POST http://localhost:8080/manage/tokens \
  -H "Authorization: Bearer $MANAGEMENT_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "project_id": "<project-id>",
    "duration_hours": 24,
    "max_requests": 1000
  }'

# Response includes the token (shown only once)
{
  "id": "token-uuid",
  "token": "sk-wt-xxxxxxxxxxxxxxxxxxxxxxxx",
  "project_id": "<project-id>",
  "expires_at": "2024-01-02T10:00:00Z",
  "max_requests": 1000,
  "request_count": 0,
  "is_active": true
}
```

> **Important**: The token value is only returned once at creation time. Store it securely.

## Using Tokens

Tokens are used to authenticate proxy requests:

```bash
# Use token to proxy a request to OpenAI
curl -X POST http://localhost:8080/v1/chat/completions \
  -H "Authorization: Bearer sk-wt-your-token-here" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "gpt-4",
    "messages": [{"role": "user", "content": "Hello!"}]
  }'
```

### Token Validation

When a request is made, the proxy validates:

1. **Token exists** and is active
2. **Token not expired** (expiration time not passed)
3. **Project is active** (parent project not deactivated)
4. **Request limit not exceeded** (if max_requests is set)

If any validation fails, the request is rejected with an appropriate error.

## Listing Tokens

### Using the API

```bash
# List all tokens
curl -H "Authorization: Bearer $MANAGEMENT_TOKEN" \
  "http://localhost:8080/manage/tokens"

# List tokens for a specific project
curl -H "Authorization: Bearer $MANAGEMENT_TOKEN" \
  "http://localhost:8080/manage/tokens?project_id=<project-id>"

# List only active tokens
curl -H "Authorization: Bearer $MANAGEMENT_TOKEN" \
  "http://localhost:8080/manage/tokens?active_only=true"

# Combined filters
curl -H "Authorization: Bearer $MANAGEMENT_TOKEN" \
  "http://localhost:8080/manage/tokens?project_id=<project-id>&active_only=true"
```

### Query Parameters

| Parameter | Type | Description |
|-----------|------|-------------|
| `project_id` | string | Filter by project |
| `active_only` | bool | Only return active tokens |

## Getting Token Details

### Using the API

```bash
curl -H "Authorization: Bearer $MANAGEMENT_TOKEN" \
  "http://localhost:8080/manage/tokens/<token-id>"

# Response
{
  "id": "<token-id>",
  "project_id": "<project-id>",
  "expires_at": "2024-01-02T10:00:00Z",
  "max_requests": 1000,
  "request_count": 42,
  "cache_hit_count": 15,
  "is_active": true,
  "created_at": "2024-01-01T10:00:00Z"
}
```

### Token Fields

| Field | Description |
|-------|-------------|
| `id` | Unique token identifier |
| `project_id` | Parent project ID |
| `expires_at` | Token expiration timestamp |
| `max_requests` | Maximum allowed requests (0 = unlimited) |
| `request_count` | Current request count |
| `cache_hit_count` | Requests served from cache |
| `is_active` | Whether token is active |
| `created_at` | Token creation timestamp |

## Revoking Tokens

### Individual Token Revocation

#### Using the API

```bash
# Revoke (soft delete) a single token
curl -X DELETE \
  -H "Authorization: Bearer $MANAGEMENT_TOKEN" \
  "http://localhost:8080/manage/tokens/<token-id>"

# Response
{
  "message": "Token revoked successfully"
}
```

### Bulk Token Revocation

Revoke all tokens for a project:

#### Using the API

```bash
# Revoke all tokens for a project
curl -X POST \
  -H "Authorization: Bearer $MANAGEMENT_TOKEN" \
  "http://localhost:8080/manage/projects/<project-id>/tokens/revoke"

# Response
{
  "message": "Revoked 5 tokens for project",
  "revoked_count": 5
}
```

> **Note**: Bulk revocation is useful when rotating API keys or decommissioning a project.

## Updating Tokens

### Using the API

```bash
# Reactivate a revoked token
curl -X PATCH \
  -H "Authorization: Bearer $MANAGEMENT_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"is_active": true}' \
  "http://localhost:8080/manage/tokens/<token-id>"

# Deactivate a token (same as revoke but reversible)
curl -X PATCH \
  -H "Authorization: Bearer $MANAGEMENT_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"is_active": false}' \
  "http://localhost:8080/manage/tokens/<token-id>"
```

## Token Security Best Practices

### 1. Use Short Lifetimes

Set appropriate token durations based on use case:

| Use Case | Recommended Duration |
|----------|---------------------|
| CI/CD pipelines | 1-4 hours |
| Development | 24 hours |
| Weekly batch jobs | 168 hours (7 days) |
| Long-running services | 720 hours (30 days) max |

### 2. Set Request Limits

Prevent abuse by setting appropriate request limits:

```bash
# Create token with 1000 request limit
llm-proxy manage token generate \
  --project-id <project-id> \
  --duration 24 \
  --max-requests 1000 \
  --management-token $MANAGEMENT_TOKEN
```

### 3. Never Share Tokens

- Store tokens in environment variables or secret managers
- Never commit tokens to version control
- Use different tokens for different environments

### 4. Monitor Token Usage

Check token usage through the Admin UI or API:

```bash
# Get token details including usage
curl -H "Authorization: Bearer $MANAGEMENT_TOKEN" \
  "http://localhost:8080/manage/tokens/<token-id>"

# Check request_count vs max_requests
```

### 5. Rotate Tokens Regularly

Implement token rotation in your deployment:

```bash
# 1. Generate new token
NEW_TOKEN=$(curl -s -X POST http://localhost:8080/manage/tokens \
  -H "Authorization: Bearer $MANAGEMENT_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"project_id": "<project-id>", "duration_hours": 24}' \
  | jq -r '.token')

# 2. Update your application with new token
# 3. Revoke old token
curl -X DELETE \
  -H "Authorization: Bearer $MANAGEMENT_TOKEN" \
  "http://localhost:8080/manage/tokens/<old-token-id>"
```

### 6. Revoke Compromised Tokens Immediately

If a token is compromised:

```bash
# Immediately revoke the specific token
curl -X DELETE \
  -H "Authorization: Bearer $MANAGEMENT_TOKEN" \
  "http://localhost:8080/manage/tokens/<compromised-token-id>"

# Or revoke all tokens for the project if scope is uncertain
curl -X POST \
  -H "Authorization: Bearer $MANAGEMENT_TOKEN" \
  "http://localhost:8080/manage/projects/<project-id>/tokens/revoke"
```

## Integration with Projects

Tokens inherit behavior from their parent project:

### Project Deactivation

When a project is deactivated:
- All existing tokens for that project become unusable
- No new tokens can be generated for the project
- Tokens are NOT automatically revoked (can be reactivated when project is reactivated)

```bash
# Deactivate project
curl -X PATCH http://localhost:8080/manage/projects/<project-id> \
  -H "Authorization: Bearer $MANAGEMENT_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"is_active": false}'
```

### Creating Tokens for Projects

Tokens can only be created for active projects:

```bash
# This will fail if project is inactive
curl -X POST http://localhost:8080/manage/tokens \
  -H "Authorization: Bearer $MANAGEMENT_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"project_id": "<inactive-project-id>", "duration_hours": 24}'

# Response: 400 Bad Request
# {"error": "cannot generate tokens for inactive project"}
```

## Audit Trail

All token operations are logged for security:

- Token creation (actor, project, duration, limits)
- Token usage (obfuscated token ID, result)
- Token revocation (actor, reason)
- Bulk operations (project, count)

View audit logs via the Admin UI or directly in the audit log file.

See [Security Best Practices - Audit Logging](security.md#audit-logging) for details.

## Troubleshooting

### Token Authentication Fails

**Error**: `401 Unauthorized`

**Causes**:
1. Token is expired - generate a new token
2. Token is revoked - check if accidentally revoked
3. Token value is incorrect - verify the token string
4. Project is deactivated - check project status

**Debug**:
```bash
# Check token status
curl -H "Authorization: Bearer $MANAGEMENT_TOKEN" \
  "http://localhost:8080/manage/tokens/<token-id>"
```

### Rate Limit Exceeded

**Error**: `429 Too Many Requests`

**Causes**:
1. Token `max_requests` limit reached
2. Global or IP rate limiting

**Solutions**:
1. Generate a new token with higher limit
2. Use multiple tokens for high-volume applications
3. Implement request batching

### Cannot Generate Token

**Error**: `400 Bad Request` - cannot generate tokens for inactive project

**Solution**: Activate the project first:
```bash
curl -X PATCH http://localhost:8080/manage/projects/<project-id> \
  -H "Authorization: Bearer $MANAGEMENT_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"is_active": true}'
```

## Related Documentation

- [Admin UI Token Management](admin/tokens.md)
- [CLI Reference - Token Commands](cli-reference.md#token-management)
- [Security Best Practices](security.md)
- [Troubleshooting Guide](troubleshooting.md)
