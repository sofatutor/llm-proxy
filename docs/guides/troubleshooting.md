---
title: Troubleshooting & FAQ
parent: Guides
nav_order: 5
---

# Troubleshooting & FAQ

This guide covers common issues and their solutions when using LLM Proxy.

## Quick Diagnostics

Before diving into specific issues, run these quick checks:

```bash
# Check if proxy is running
curl http://localhost:8080/health

# Check logs for errors
docker logs llm-proxy | tail -50

# Verify configuration
docker inspect llm-proxy | grep -A 20 "Env"
```

## Installation Issues

### Docker Container Won't Start

**Symptom**: Container exits immediately after starting.

**Check logs**:
```bash
docker logs llm-proxy
```

**Common causes**:

1. **Missing MANAGEMENT_TOKEN**
   ```bash
   # Error: MANAGEMENT_TOKEN environment variable is required
   
   # Solution: Set the environment variable
   docker run -e MANAGEMENT_TOKEN=your-token ...
   ```

2. **Port already in use**
   ```bash
   # Error: bind: address already in use
   
   # Check what's using port 8080
   lsof -i :8080  # macOS/Linux
   netstat -an | findstr :8080  # Windows
   
   # Solution: Use a different port
   docker run -p 9000:8080 ...
   ```

3. **Volume permission issues**
   ```bash
   # Error: permission denied
   
   # Solution: Fix permissions
   sudo chown -R $(id -u):$(id -g) ./data
   ```

### Build from Source Fails

**Go version mismatch**:
```bash
# Error: requires go >= 1.23

# Check version
go version

# Solution: Update Go from https://go.dev/dl/
```

**Missing dependencies**:
```bash
# Solution: Download all dependencies
go mod download
go mod tidy
```

**Lint failures**:
```bash
# Run linter to see issues
make lint

# Fix formatting
make fmt
```

## Authentication Errors

### 401 Unauthorized - Invalid Token

**Symptom**: `{"error": "unauthorized"}`

**Causes & Solutions**:

1. **Token is expired**
   ```bash
   # Check token details
   curl -H "Authorization: Bearer $MANAGEMENT_TOKEN" \
     "http://localhost:8080/manage/tokens/<token-id>"
   
   # Look for expires_at - if in the past, generate a new token
   ```

2. **Token value is incorrect**
   - Verify you're using the full token value
   - Check for extra whitespace or newlines
   - Ensure correct Authorization header format: `Bearer <token>`

3. **Token is revoked**
   ```bash
   # Check is_active field
   curl -H "Authorization: Bearer $MANAGEMENT_TOKEN" \
     "http://localhost:8080/manage/tokens/<token-id>"
   
   # If is_active: false, generate a new token or reactivate
   ```

### 401 Unauthorized - Invalid Management Token

**Symptom**: Management API returns 401.

**Solutions**:

1. **Verify the management token is set**
   ```bash
   # Check environment
   echo $MANAGEMENT_TOKEN
   
   # In Docker
   docker inspect llm-proxy | grep MANAGEMENT_TOKEN
   ```

2. **Ensure correct header format**
   ```bash
   curl -H "Authorization: Bearer $MANAGEMENT_TOKEN" ...
   ```

3. **Token value mismatch** - The token in your request must match exactly what the server was started with.

### 403 Forbidden - Project Inactive

**Symptom**: `{"error": "project is inactive"}`

**Solution**: Activate the project:
```bash
curl -X PATCH http://localhost:8080/manage/projects/<project-id> \
  -H "Authorization: Bearer $MANAGEMENT_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"is_active": true}'
```

### 429 Too Many Requests

**Symptom**: Rate limit exceeded.

**Causes**:

1. **Token request limit reached**
   ```bash
   # Check token's request_count vs max_requests
   curl -H "Authorization: Bearer $MANAGEMENT_TOKEN" \
     "http://localhost:8080/manage/tokens/<token-id>"
   ```

2. **Global or IP rate limiting**
   - Wait for the rate limit window to reset
   - Reduce request frequency
   - Consider increasing rate limits if appropriate

**Solutions**:
- Generate a new token with higher limits
- Implement request batching
- Use caching to reduce upstream requests

## Database Issues

### SQLite Permission Denied

**Symptom**: `unable to open database file`

**Solutions**:

1. **Check directory permissions**
   ```bash
   # Ensure data directory exists and is writable
   mkdir -p ./data
   chmod 755 ./data
   ```

2. **Check file permissions**
   ```bash
   chmod 644 ./data/llm-proxy.db
   ```

3. **Docker volume issues**
   ```bash
   # Use named volume or fix host path permissions
   docker run -v llm-proxy-data:/app/data ...
   ```

### PostgreSQL Connection Issues

See [PostgreSQL Troubleshooting Guide](postgresql-troubleshooting.md) for detailed PostgreSQL issues.

**Quick checks**:

```bash
# Test connection
psql "$DATABASE_URL" -c "SELECT 1"

# Common issues:
# - Database not running
# - Wrong host/port
# - Password authentication failed
# - Database doesn't exist
# - SSL configuration mismatch
```

### Migration Errors

**Symptom**: Migrations fail to run.

```bash
# Check migration status
llm-proxy migrate status

# Run pending migrations
llm-proxy migrate up

# If stuck, check for lock
# PostgreSQL:
psql "$DATABASE_URL" -c "SELECT * FROM pg_locks WHERE locktype = 'advisory';"
```

## Cache Issues

### Redis Connection Failed

**Symptom**: `connection refused` to Redis.

**Solutions**:

1. **Verify Redis is running**
   ```bash
   docker ps | grep redis
   redis-cli ping  # Should return PONG
   ```

2. **Check connection settings**
   ```bash
   # Verify REDIS_ADDR format
   # Correct: hostname:6379 or localhost:6379
   # Optional: Set REDIS_DB for database selection (default: 0)
   ```

3. **Network issues in Docker**
   ```bash
   # Containers must be on same network
   docker network inspect bridge
   
   # Use container name as hostname
   REDIS_ADDR=redis:6379
   ```

### High Cache Miss Rate

**Symptom**: Low cache hit ratio in metrics.

**Causes & Solutions**:

1. **TTL too short**
   ```bash
   # Increase default TTL
   HTTP_CACHE_DEFAULT_TTL=600  # 10 minutes
   ```

2. **Cache disabled**
   ```bash
   # Verify caching is enabled
   HTTP_CACHE_ENABLED=true
   ```

3. **Unique requests** - Each unique prompt creates a separate cache entry. This is expected behavior.

4. **POST requests not opted-in** - POST requests require explicit `Cache-Control` header to be cached.

### Cache Not Clearing

**Symptom**: Old responses being served.

**Solution**: Purge cache manually:
```bash
llm-proxy manage cache purge \
  --method GET \
  --url "/v1/models" \
  --management-token $MANAGEMENT_TOKEN
```

## Proxy Errors

### 502 Bad Gateway

**Symptom**: Upstream API unreachable.

**Causes & Solutions**:

1. **OpenAI API down** - Check [OpenAI Status](https://status.openai.com/)

2. **Network connectivity**
   ```bash
   # Test from container
   docker exec llm-proxy wget -q -O- https://api.openai.com/v1/models
   ```

3. **Invalid API key** - Verify the project's OpenAI API key is valid

### 504 Gateway Timeout

**Symptom**: Upstream request timed out.

**Solutions**:

1. **Increase timeout**
   ```bash
   REQUEST_TIMEOUT=120s
   ```

2. **Reduce request complexity** - Use simpler prompts or smaller models

3. **Check upstream latency** - OpenAI may be experiencing high load

### 413 Request Too Large

**Symptom**: Request body exceeds limit.

**Solution**: Increase max request size:
```bash
MAX_REQUEST_SIZE=50MB
```

## Admin UI Issues

### Cannot Access Admin UI

**Symptom**: 404 or connection refused at `/admin/`.

**Solutions**:

1. **Verify UI is enabled**
   ```bash
   ADMIN_UI_ENABLED=true
   ```

2. **Check the path**
   - Default: `http://localhost:8080/admin/`
   - Custom: Check `ADMIN_UI_PATH` setting

3. **Separate Admin service** - If running admin as separate container, check the admin container logs

### Login Fails

**Symptom**: Cannot log into Admin UI.

**Solutions**:

1. **Use management token** - The Admin UI uses the same `MANAGEMENT_TOKEN`

2. **Clear browser cache** - Try incognito/private mode

3. **Check CORS** - If accessing from different domain, configure `CORS_ALLOWED_ORIGINS`

### Stale Data in Admin UI

**Symptom**: Changes not reflected immediately.

**Solutions**:

1. **Refresh the page** - Data is fetched on page load

2. **Clear browser cache**
   ```
   Ctrl+Shift+R (Windows/Linux)
   Cmd+Shift+R (macOS)
   ```

## Event Bus Issues

### Events Not Being Delivered

**Symptom**: Dispatcher not receiving events.

**Solutions**:

1. **In-memory vs Redis Streams** - For multi-process, use Redis Streams (default):
   ```bash
   LLM_PROXY_EVENT_BUS=redis-streams
   REDIS_ADDR=redis:6379
   ```

2. **Check dispatcher is running**
   ```bash
   docker ps | grep dispatcher
   docker logs llm-proxy-logger
   ```

3. **Buffer overflow** - Increase buffer size:
   ```bash
   OBSERVABILITY_BUFFER_SIZE=2000
   ```

### Event Loss

**Symptom**: Gaps in event log.

**Causes**:
- Redis TTL/trimming before dispatcher reads
- Dispatcher falling behind

**Solutions**:
- Increase Redis retention settings
- Scale dispatcher batch size
- Monitor dispatcher lag

See [Instrumentation Guide - Production Reliability](instrumentation.md#production-reliability-warning-event-retention--loss).

## Performance Issues

See [Performance Tuning Guide](performance.md) for detailed optimization.

### High Latency

**Quick checks**:
```bash
# Enable debug logging
LOG_LEVEL=debug

# Check cache headers
curl -v -H "Authorization: Bearer $TOKEN" \
  http://localhost:8080/v1/models
# Look for X-PROXY-CACHE: hit vs miss
```

### Memory Usage High

**Solutions**:
1. Reduce connection pool size
2. Reduce event buffer size
3. Enable caching to reduce upstream requests

## FAQ

### How do I rotate the management token?

1. Update the `MANAGEMENT_TOKEN` environment variable
2. Restart the proxy
3. Update any scripts/automation using the old token

### Can I use multiple API keys?

Yes, create multiple projects, each with its own API key. Tokens are project-specific.

### How do I backup the database?

**SQLite**:
```bash
cp ./data/llm-proxy.db ./backups/llm-proxy-$(date +%Y%m%d).db
```

**PostgreSQL**:
```bash
pg_dump -U llmproxy llmproxy > backup.sql
```

### How do I check token usage?

```bash
# Via API
curl -H "Authorization: Bearer $MANAGEMENT_TOKEN" \
  "http://localhost:8080/manage/tokens/<token-id>"

# Check request_count and cache_hit_count fields
```

### Can I extend a token's expiration?

No, tokens cannot be extended. Generate a new token before the old one expires.

### How do I completely remove a project?

Projects cannot be deleted (405 Method Not Allowed) for data safety. Instead:
1. Deactivate the project
2. Revoke all its tokens
3. The project remains in the database for audit purposes

### Why are my requests not being cached?

1. POST requests require explicit opt-in via `Cache-Control: public` header
2. Responses with `Cache-Control: no-store` are not cached
3. Responses larger than `HTTP_CACHE_MAX_OBJECT_BYTES` are not cached
4. Each unique request creates a separate cache entry

### How do I enable Prometheus metrics?

```bash
ENABLE_METRICS=true
METRICS_PATH=/metrics
```

Access at `http://localhost:8080/metrics`

### Can I run multiple proxy instances?

Yes, for horizontal scaling:
1. Use PostgreSQL (not SQLite) for shared database
2. Use Redis for distributed caching and rate limiting
3. Use Redis for distributed event bus
4. Use a load balancer in front

See [Performance Tuning - Horizontal Scaling](performance.md#horizontal-scaling).

## Getting Help

If you're still experiencing issues:

1. **Check logs** with `LOG_LEVEL=debug`
2. **Search [GitHub Issues](https://github.com/sofatutor/llm-proxy/issues)**
3. **Review documentation** for your specific use case
4. **Open a new issue** with:
   - Error message and stack trace
   - LLM Proxy version
   - Configuration (redact sensitive values)
   - Steps to reproduce

## Related Documentation

- [Installation Guide](installation.md)
- [Configuration Reference](configuration.md)
- [PostgreSQL Troubleshooting](postgresql-troubleshooting.md)
- [Performance Tuning Guide](performance.md)
- [Security Best Practices](security.md)
