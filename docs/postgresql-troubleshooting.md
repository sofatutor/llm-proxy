# PostgreSQL Troubleshooting Guide

This guide covers common PostgreSQL issues when running LLM Proxy and their solutions.

## Connection Issues

### "connection refused" Error

**Symptom:**
```
failed to ping PostgreSQL database: connection refused
```

**Causes & Solutions:**

1. **PostgreSQL not running:**
   ```bash
   # Check if PostgreSQL is running
   docker compose --profile postgres ps
   
   # Start PostgreSQL
   docker compose --profile postgres up -d postgres
   ```

2. **Wrong host/port:**
   ```bash
   # Verify connection string
   echo $DATABASE_URL
   
   # Test connection
   psql "$DATABASE_URL" -c "SELECT 1"
   ```

3. **Firewall blocking port 5432:**
   ```bash
   # Check if port is accessible
   nc -zv localhost 5432
   ```

### "password authentication failed"

**Symptom:**
```
FATAL: password authentication failed for user "llmproxy"
```

**Solutions:**

1. **Verify password:**
   ```bash
   # Check environment variable
   echo $POSTGRES_PASSWORD
   
   # Ensure URL is properly encoded
   # Special characters need URL encoding: @ -> %40, # -> %23
   ```

2. **Reset password:**
   ```bash
   docker compose --profile postgres exec postgres \
     psql -U postgres -c "ALTER USER llmproxy PASSWORD 'new_password';"
   ```

### "database does not exist"

**Symptom:**
```
FATAL: database "llmproxy" does not exist
```

**Solutions:**

1. **Create database manually:**
   ```bash
   docker compose --profile postgres exec postgres \
     createdb -U postgres llmproxy
   ```

2. **Or reset completely:**
   ```bash
   docker compose --profile postgres down -v
   docker compose --profile postgres up -d postgres
   ```

### SSL Connection Issues

**Symptom:**
```
SSL is not enabled on the server
```

**Solutions:**

1. **For development (disable SSL):**
   ```bash
   DATABASE_URL=postgres://user:pass@localhost:5432/db?sslmode=disable
   ```

2. **For production (require SSL):**
   ```bash
   DATABASE_URL=postgres://user:pass@host:5432/db?sslmode=require
   ```

3. **With certificate verification:**
   ```bash
   DATABASE_URL="postgres://user:pass@host:5432/db?sslmode=verify-full&sslrootcert=/path/to/ca.crt"
   ```

## Migration Issues

### "migration lock not acquired"

**Symptom:**
```
failed to acquire migration lock: timeout waiting for lock
```

**Causes:**
- Another instance is running migrations
- Previous migration crashed, leaving lock held

**Solutions:**

1. **Wait for other migrations:**
   ```bash
   # Check for running migrations
   docker compose --profile postgres exec postgres \
     psql -U llmproxy -c "SELECT * FROM pg_locks WHERE locktype = 'advisory';"
   ```

2. **Force release lock (use with caution):**
   ```bash
   docker compose --profile postgres exec postgres \
     psql -U llmproxy -c "SELECT pg_advisory_unlock_all();"
   ```

### "migration file not found"

**Symptom:**
```
failed to run PostgreSQL migrations: migrations directory not found
```

**Solutions:**

1. **Verify migrations directory exists:**
   ```bash
   ls -la internal/database/migrations/sql/postgres/
   ```

2. **Build with correct working directory:**
   ```bash
   # From project root
   make build
   ./bin/llm-proxy server
   ```

### "syntax error in migration"

**Symptom:**
```
ERROR: syntax error at or near "..."
```

**Solutions:**

1. **Check PostgreSQL-specific syntax:**
   - Use `SERIAL` instead of `INTEGER PRIMARY KEY AUTOINCREMENT`
   - Use `BOOLEAN` instead of `INTEGER` for booleans
   - Use `TIMESTAMP WITH TIME ZONE` for timestamps

2. **Verify migration file:**
   ```bash
   cat internal/database/migrations/sql/postgres/00001_initial_schema.sql
   ```

## Performance Issues

### Slow Queries

**Diagnosis:**
```bash
# Enable slow query logging in PostgreSQL
docker compose --profile postgres exec postgres \
  psql -U llmproxy -c "ALTER SYSTEM SET log_min_duration_statement = '100ms';"

# Check slow queries
docker compose --profile postgres exec postgres \
  psql -U llmproxy -c "SELECT * FROM pg_stat_statements ORDER BY total_time DESC LIMIT 10;"
```

**Solutions:**

1. **Add missing indexes:**
   ```sql
   CREATE INDEX CONCURRENTLY idx_tokens_project_id ON tokens(project_id);
   CREATE INDEX CONCURRENTLY idx_tokens_expires_at ON tokens(expires_at) WHERE is_active = true;
   ```

2. **Vacuum and analyze:**
   ```bash
   docker compose --profile postgres exec postgres \
     psql -U llmproxy -c "VACUUM ANALYZE;"
   ```

### Connection Pool Exhaustion

**Symptom:**
```
too many connections for role "llmproxy"
```

**Solutions:**

1. **Increase max connections in PostgreSQL:**
   ```bash
   docker compose --profile postgres exec postgres \
     psql -U postgres -c "ALTER SYSTEM SET max_connections = '200';"
   ```

2. **Tune application pool:**
   ```bash
   export DATABASE_POOL_SIZE=20
   export DATABASE_MAX_IDLE_CONNS=10
   ```

3. **Check for connection leaks:**
   ```bash
   docker compose --profile postgres exec postgres \
     psql -U llmproxy -c "SELECT * FROM pg_stat_activity WHERE datname = 'llmproxy';"
   ```

### High Memory Usage

**Solutions:**

1. **Tune shared buffers:**
   ```bash
   # Set to ~25% of system RAM
   docker compose --profile postgres exec postgres \
     psql -U postgres -c "ALTER SYSTEM SET shared_buffers = '256MB';"
   ```

2. **Tune work_mem:**
   ```bash
   docker compose --profile postgres exec postgres \
     psql -U postgres -c "ALTER SYSTEM SET work_mem = '16MB';"
   ```

## Data Recovery

### Backup Database

```bash
# Create backup
docker compose --profile postgres exec postgres \
  pg_dump -U llmproxy llmproxy > backup.sql

# With compression
docker compose --profile postgres exec postgres \
  pg_dump -U llmproxy llmproxy | gzip > backup.sql.gz
```

### Restore Database

```bash
# Restore from backup
cat backup.sql | docker compose --profile postgres exec -T postgres \
  psql -U llmproxy llmproxy

# From compressed backup
gunzip -c backup.sql.gz | docker compose --profile postgres exec -T postgres \
  psql -U llmproxy llmproxy
```

### Point-in-Time Recovery

For production systems, enable WAL archiving:

```bash
# In PostgreSQL configuration
archive_mode = on
archive_command = 'cp %p /path/to/archive/%f'
```

## Docker-Specific Issues

### Container Won't Start

**Check logs:**
```bash
docker compose --profile postgres logs postgres
```

**Common issues:**
- Volume permission problems
- Port already in use
- Insufficient disk space

### Volume Permission Issues

```bash
# Fix permissions
sudo chown -R 999:999 ./postgres_data

# Or use named volume
docker volume create llm-proxy-postgres-data
```

### Network Issues

```bash
# Check network connectivity
docker compose --profile postgres exec postgres \
  ping -c 1 llm-proxy-postgres

# Verify network
docker network inspect llm-proxy_default
```

## Logging and Debugging

### Enable Debug Logging

```bash
# In application
export LOG_LEVEL=debug

# In PostgreSQL
docker compose --profile postgres exec postgres \
  psql -U postgres -c "ALTER SYSTEM SET log_statement = 'all';"
```

### View PostgreSQL Logs

```bash
# Docker Compose logs
docker compose --profile postgres logs -f postgres

# Inside container
docker compose --profile postgres exec postgres \
  tail -f /var/log/postgresql/postgresql-15-main.log
```

## Health Checks

### Verify Database Health

```bash
# Check PostgreSQL is ready
docker compose --profile postgres exec postgres \
  pg_isready -U llmproxy -d llmproxy

# Check table counts
docker compose --profile postgres exec postgres \
  psql -U llmproxy -c "
    SELECT 
      (SELECT COUNT(*) FROM projects) as projects,
      (SELECT COUNT(*) FROM tokens) as tokens,
      (SELECT COUNT(*) FROM audit_events) as audit_events;
  "
```

### Verify Application Connection

```bash
# Health endpoint
curl http://localhost:8080/health

# Metrics (includes database stats)
curl http://localhost:8080/metrics | grep database
```

## Getting Help

If you're still experiencing issues:

1. Check the [GitHub Issues](https://github.com/sofatutor/llm-proxy/issues) for similar problems
2. Review the [Architecture Guide](architecture.md) for system understanding
3. Enable debug logging and collect relevant logs
4. Open a new issue with:
   - Error message and stack trace
   - PostgreSQL version and configuration
   - LLM Proxy version and configuration
   - Steps to reproduce

## Related Documentation

- [Database Selection Guide](database-selection.md)
- [Docker Compose PostgreSQL Setup](docker-compose-postgres.md)
- [Database Migrations Guide](migrations.md)
