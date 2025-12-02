# Log Search and Filter Guide

This guide documents the canonical log fields used by LLM Proxy and provides ready-to-use queries for debugging locally and in observability backends.

## Overview

The proxy uses structured JSON logging via `internal/logging/logger.go` with canonical field helpers. All logs are emitted in JSON format (default) or console format, with consistent field names across all components.

## Canonical Log Fields

| Field | Type | Description | Source |
|-------|------|-------------|--------|
| `ts` | string | ISO8601 timestamp | zap encoder |
| `level` | string | Log level (debug, info, warn, error) | zap encoder |
| `msg` | string | Log message | zap encoder |
| `caller` | string | Source file:line | zap encoder |
| `request_id` | string | Unique request identifier (UUIDv7) | `RequestFields()` |
| `correlation_id` | string | Cross-service trace ID | `CorrelationID()` |
| `project_id` | string | Project identifier | `ProjectID()` |
| `token_id` | string | Obfuscated token (e.g., `sk-abc1...xyz9`) | `TokenID()` |
| `client_ip` | string | Client IP address | `ClientIP()` |
| `method` | string | HTTP method (GET, POST, etc.) | `RequestFields()` |
| `path` | string | Request URL path | `RequestFields()` |
| `status_code` | int | HTTP response status code | `RequestFields()` |
| `duration_ms` | int | Request duration in milliseconds | `RequestFields()` |
| `duration` | string | Duration (Go format, e.g., "1.234s") | Proxy logs |
| `remote_addr` | string | Remote address with port | Proxy logs |
| `component` | string | Logger component name | `NewChildLogger()` |
| `error` | string | Error message (when applicable) | zap.Error() |

## Field Relationships

### request_id vs correlation_id

- **`request_id`**: A unique identifier generated for each incoming HTTP request. Every request gets its own `request_id`, making it ideal for tracing a single request through the system.

- **`correlation_id`**: An optional identifier passed from upstream services (via headers like `X-Correlation-ID` or `X-Request-ID`). Use this to trace requests across multiple services in a distributed system.

**When to use:**
- Use `request_id` to find all logs for a single proxy request
- Use `correlation_id` to follow a user action across multiple services

### Field Availability by Component

| Field | Proxy | Server | Admin |
|-------|:-----:|:------:|:-----:|
| `ts`, `level`, `msg`, `caller` | ✓ | ✓ | ✓ |
| `request_id` | ✓ | ✓ | ✓ |
| `correlation_id` | ✓ | - | - |
| `project_id` | ✓ | - | ✓ |
| `token_id` | ✓ | - | ✓ |
| `client_ip` | ✓ | ✓ | ✓ |
| `method`, `path` | ✓ | ✓ | ✓ |
| `status_code`, `duration_ms` | ✓ | ✓ | ✓ |
| `duration`, `remote_addr` | ✓ | - | - |
| `component` | ✓ | ✓ | ✓ |

## Token ID Obfuscation

The `token_id` field contains an obfuscated version of the authentication token for security. The obfuscation rules are:

| Token Length | Obfuscation Rule | Example |
|--------------|------------------|---------|
| ≤ 4 chars | All asterisks | `sk-a` → `****` |
| 5-12 chars | Keep first 2, asterisks for rest | `sk-abc123` → `sk********` |
| > 12 chars | Keep first 8, `...`, last 4 | `sk-abc123xyz789def456` → `sk-abc12...f456` |

**Note:** Never log raw tokens. Always use `logging.TokenID()` which automatically applies obfuscation.

## Example Log Output

```json
{
  "ts": "2025-12-02T21:30:00.123Z",
  "level": "info",
  "msg": "Request completed",
  "caller": "proxy/proxy.go:1033",
  "request_id": "01938a7c-4d5e-7f8a-9b0c-1d2e3f4a5b6c",
  "method": "POST",
  "path": "/v1/chat/completions",
  "status_code": 200,
  "duration": "1.234s",
  "project_id": "proj-abc123",
  "token_id": "sk-abc1...xyz9"
}
```

---

## Local JSON Log Queries (grep/jq)

These examples assume logs are written to a file in JSON format. Pipe your logs or read from a file:

```bash
# Read from log file
cat logs.json | jq '...'

# Stream logs in real-time
tail -f logs.json | jq '...'

# Read from Docker container logs
docker logs llm-proxy 2>&1 | jq '...'
```

### Filter by Request ID

Find all logs for a specific request:

```bash
jq 'select(.request_id == "01938a7c-4d5e-7f8a-9b0c-1d2e3f4a5b6c")'
```

### Filter by Project ID

Find all logs for a specific project:

```bash
jq 'select(.project_id == "proj-abc123")'
```

### Filter by Correlation ID

Trace requests across services:

```bash
jq 'select(.correlation_id == "corr-xyz789")'
```

### Find Error Logs

```bash
jq 'select(.level == "error")'
```

### Find Warnings and Errors

```bash
jq 'select(.level == "error" or .level == "warn")'
```

### Find Slow Requests

Requests taking longer than 1 second (1000ms):

```bash
jq 'select(.duration_ms > 1000)'
```

Requests taking longer than 500ms:

```bash
jq 'select(.duration_ms > 500)'
```

### Filter by HTTP Status Code

Find 5xx errors:

```bash
jq 'select(.status_code >= 500)'
```

Find 4xx client errors:

```bash
jq 'select(.status_code >= 400 and .status_code < 500)'
```

### Filter by Time Range

Find logs within a specific time window:

```bash
jq 'select(.ts >= "2025-12-02T10:00:00Z" and .ts <= "2025-12-02T11:00:00Z")'
```

### Filter by Endpoint

Find all chat completion requests:

```bash
jq 'select(.path == "/v1/chat/completions")'
```

Find all model listing requests:

```bash
jq 'select(.path == "/v1/models")'
```

### Filter by Client IP

```bash
jq 'select(.client_ip == "192.168.1.100")'
```

### Filter by Component

Find logs from a specific component:

```bash
jq 'select(.component == "proxy")'
```

### Aggregate by Status Code

Count requests by status code:

```bash
jq -s 'group_by(.status_code) | map({status: .[0].status_code, count: length}) | sort_by(.count) | reverse'
```

### Aggregate Errors by Endpoint

```bash
jq -s '[.[] | select(.level == "error")] | group_by(.path) | map({path: .[0].path, count: length}) | sort_by(.count) | reverse'
```

### Calculate Average Duration by Endpoint

```bash
jq -s 'group_by(.path) | map({path: .[0].path, avg_ms: (map(.duration_ms) | add / length)}) | sort_by(.avg_ms) | reverse'
```

### Combined Filters

Find slow error requests for a specific project:

```bash
jq 'select(.level == "error" and .duration_ms > 1000 and .project_id == "proj-abc123")'
```

---

## Loki / Grafana (LogQL)

Use these queries in Grafana's Explore view or Loki datasource.

### Filter by Request ID

```logql
{app="llm-proxy"} | json | request_id=`01938a7c-4d5e-7f8a-9b0c-1d2e3f4a5b6c`
```

### Filter by Project ID

```logql
{app="llm-proxy"} | json | project_id=`proj-abc123`
```

### Find Error Logs

```logql
{app="llm-proxy"} | json | level=`error`
```

### Find Warnings and Errors

```logql
{app="llm-proxy"} | json | level=~`error|warn`
```

### Filter by Status Code

5xx errors:

```logql
{app="llm-proxy"} | json | status_code >= 500
```

4xx client errors:

```logql
{app="llm-proxy"} | json | status_code >= 400 | status_code < 500
```

### Find Slow Requests

Requests over 1 second:

```logql
{app="llm-proxy"} | json | duration_ms > 1000
```

### Filter by Endpoint

```logql
{app="llm-proxy"} | json | path=`/v1/chat/completions`
```

### Error Rate (5xx per 5 minutes)

```logql
rate({app="llm-proxy"} | json | status_code >= 500 [5m])
```

### Request Rate by Status

```logql
sum by (status_code) (rate({app="llm-proxy"} | json [5m]))
```

### Average Duration by Endpoint

```logql
avg by (path) (
  avg_over_time({app="llm-proxy"} | json | unwrap duration_ms [5m])
)
```

### Count Requests by Project

```logql
sum by (project_id) (count_over_time({app="llm-proxy"} | json [1h]))
```

---

## Elasticsearch / Kibana (KQL)

Use these queries in Kibana's Discover or Dashboard views.

### Filter by Request ID

```kql
request_id: "01938a7c-4d5e-7f8a-9b0c-1d2e3f4a5b6c"
```

### Filter by Project ID

```kql
project_id: "proj-abc123"
```

Wildcard match:

```kql
project_id: proj-*
```

### Find Error Logs

```kql
level: "error"
```

### Find Errors for a Project

```kql
level: "error" AND project_id: "proj-abc123"
```

### Filter by Status Code

5xx errors:

```kql
status_code >= 500
```

4xx client errors:

```kql
status_code >= 400 AND status_code < 500
```

### Find Slow Requests

Over 1 second:

```kql
duration_ms > 1000
```

### Filter by Endpoint

```kql
path: "/v1/chat/completions"
```

### Filter by Client IP

```kql
client_ip: "192.168.1.100"
```

### Combined Filters

Slow errors for a project:

```kql
level: "error" AND duration_ms > 500 AND project_id: "proj-abc123"
```

### Time Range Filter

```kql
@timestamp >= "2025-12-02T10:00:00" AND @timestamp <= "2025-12-02T11:00:00"
```

---

## Datadog

Use these queries in Datadog Logs.

### Filter by Request ID

```
service:llm-proxy @request_id:01938a7c-4d5e-7f8a-9b0c-1d2e3f4a5b6c
```

### Filter by Project ID

```
service:llm-proxy @project_id:proj-abc123
```

Wildcard match:

```
service:llm-proxy @project_id:proj-*
```

### Find Error Logs

```
service:llm-proxy @level:error
```

Or use status facet:

```
service:llm-proxy status:error
```

### Find Errors for a Project

```
service:llm-proxy @level:error @project_id:proj-abc123
```

### Filter by Status Code

5xx errors:

```
service:llm-proxy @status_code:>=500
```

4xx client errors:

```
service:llm-proxy @status_code:[400 TO 499]
```

### Find Slow Requests

Over 1 second:

```
service:llm-proxy @duration_ms:>1000
```

### Filter by Endpoint

```
service:llm-proxy @path:"/v1/chat/completions"
```

### Filter by Client IP

```
service:llm-proxy @client_ip:192.168.1.100
```

### Combined Filters

Slow errors for a project:

```
service:llm-proxy @level:error @duration_ms:>500 @project_id:proj-abc123
```

### Aggregate Errors by Endpoint

Use Datadog's Analytics to group by `@path` and filter on `@level:error`.

---

## CLI Helper Script

A helper script is available at `scripts/log-search.sh` for common log searches.

### Usage

```bash
# Filter by request ID
./scripts/log-search.sh --request-id 01938a7c-4d5e-7f8a-9b0c-1d2e3f4a5b6c

# Find all errors
./scripts/log-search.sh --errors

# Find slow requests (>500ms)
./scripts/log-search.sh --slow 500

# Find logs for a project
./scripts/log-search.sh --project proj-abc123

# Combine filters
./scripts/log-search.sh --errors --project proj-abc123

# Specify input file (default: stdin)
./scripts/log-search.sh --file /var/log/llm-proxy.log --errors

# Real-time log monitoring
tail -f /var/log/llm-proxy.log | ./scripts/log-search.sh --errors
```

### Options

| Option | Description |
|--------|-------------|
| `--request-id ID` | Filter by request ID |
| `--correlation-id ID` | Filter by correlation ID |
| `--project ID` | Filter by project ID |
| `--errors` | Show only error logs |
| `--slow MS` | Show requests slower than MS milliseconds |
| `--status CODE` | Filter by HTTP status code |
| `--path PATH` | Filter by request path |
| `--file FILE` | Read from file instead of stdin |
| `--help` | Show help message |

---

## Related Documentation

- [Instrumentation Guide](instrumentation.md) - Event system, async middleware, and monitoring
- [Architecture Overview](architecture.md) - System architecture and components
- [Security Best Practices](security.md) - Token obfuscation and audit logging
