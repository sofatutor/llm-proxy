---
title: Database Selection
parent: Database
nav_order: 1
---

# Database Selection Guide

This guide helps you choose between SQLite and PostgreSQL for your LLM Proxy deployment.

## Overview

The LLM Proxy supports two database backends:

| Feature | SQLite | PostgreSQL |
|---------|--------|------------|
| Setup Complexity | Zero configuration | Requires server setup |
| Scalability | Single instance | Multiple instances |
| Concurrent Writes | Limited | Excellent |
| Best For | Development, small deployments | Production, high-traffic |
| Maintenance | None | Requires DBA knowledge |

## SQLite (Default)

SQLite is the default database, requiring no additional setup.

### When to Use SQLite

- **Development and testing** - Zero configuration, runs anywhere
- **Single-instance deployments** - Up to moderate traffic
- **Edge deployments** - Minimal resource requirements
- **Prototyping** - Quick start without external dependencies

### Configuration

```bash
# SQLite is the default - no additional configuration needed
export DB_DRIVER=sqlite
export DATABASE_PATH=./data/llm-proxy.db
```

### Performance Characteristics

- **Read performance**: Excellent for moderate concurrent reads
- **Write performance**: Single-writer model, limited concurrent writes
- **Recommended limit**: ~100 requests/second per instance

### Limitations

- Cannot scale horizontally (single instance only)
- Write contention under heavy load
- Not suitable for distributed deployments

## PostgreSQL

PostgreSQL is recommended for production deployments with higher traffic or multiple instances.

### When to Use PostgreSQL

- **Production deployments** - Better reliability and monitoring
- **High traffic** - Handles thousands of concurrent connections
- **Multiple instances** - Required for horizontal scaling
- **Advanced features** - Full-text search, JSON queries, etc.

### Configuration

```bash
export DB_DRIVER=postgres
export DATABASE_URL=postgres://user:password@localhost:5432/llmproxy?sslmode=require

# Optional: Connection pool settings
export DATABASE_POOL_SIZE=10
export DATABASE_MAX_IDLE_CONNS=5
export DATABASE_CONN_MAX_LIFETIME=1h
```

### Connection String Format

```
postgres://[user]:[password]@[host]:[port]/[database]?sslmode=[mode]
```

**SSL Mode Options:**

| Mode | Description | Use Case |
|------|-------------|----------|
| `disable` | No SSL | Development only |
| `require` | SSL required, no verification | Cloud databases |
| `verify-ca` | SSL with CA verification | High security |
| `verify-full` | Full certificate verification | Maximum security |

**Examples:**

```bash
# Local development (no SSL)
DATABASE_URL=postgres://llmproxy:secret@localhost:5432/llmproxy?sslmode=disable

# AWS RDS
DATABASE_URL=postgres://user:pass@mydb.xxx.rds.amazonaws.com:5432/llmproxy?sslmode=require

# Google Cloud SQL
DATABASE_URL=postgres://user:pass@/llmproxy?host=/cloudsql/project:region:instance

# Azure PostgreSQL
DATABASE_URL=postgres://user@server:pass@server.postgres.database.azure.com:5432/llmproxy?sslmode=require
```

### Performance Characteristics

- **Read performance**: Excellent with proper indexing
- **Write performance**: Excellent with connection pooling
- **Concurrent connections**: Thousands with proper configuration
- **Recommended**: 100+ requests/second, multiple instances

## Migration Between Databases

### SQLite to PostgreSQL

1. **Export data from SQLite:**

   ```bash
   sqlite3 data/llm-proxy.db ".dump projects" > projects.sql
   sqlite3 data/llm-proxy.db ".dump tokens" > tokens.sql
   ```

2. **Start PostgreSQL:**

   ```bash
   docker compose --profile postgres up -d postgres
   ```

3. **Update configuration:**

   ```bash
   export DB_DRIVER=postgres
   export DATABASE_URL=postgres://llmproxy:secret@localhost:5432/llmproxy?sslmode=disable
   ```

4. **Start the proxy** (migrations run automatically):

   ```bash
   llm-proxy server
   ```

5. **Import data** (requires SQL syntax adaptation):

   ```bash
   # Note: SQLite and PostgreSQL SQL dialects differ
   # Manual adjustment of exported SQL may be required
   ```

### Considerations

- **Schema migrations**: Run automatically on startup for both databases
- **Data migration**: Manual process, requires SQL dialect conversion
- **Downtime**: Plan for maintenance window during migration

## Connection Pooling

Both databases support connection pooling to optimize performance.

### SQLite Pool Settings

SQLite uses a single-connection pool internally due to its single-writer architecture:

```bash
# SQLite automatically uses MaxOpenConns=1 for :memory: databases
# For file-based databases, pooling is configurable but limited
export DATABASE_POOL_SIZE=10
```

### PostgreSQL Pool Settings

PostgreSQL benefits significantly from connection pooling:

```bash
# Production settings
export DATABASE_POOL_SIZE=20
export DATABASE_MAX_IDLE_CONNS=10
export DATABASE_CONN_MAX_LIFETIME=30m
```

**Sizing Guidelines:**

| Deployment Size | Pool Size | Idle Conns | Max Lifetime |
|-----------------|-----------|------------|--------------|
| Small (<100 rps) | 10 | 5 | 1h |
| Medium (100-500 rps) | 20 | 10 | 30m |
| Large (500+ rps) | 50+ | 20 | 15m |

## Monitoring

### SQLite Monitoring

SQLite statistics are available via the metrics endpoint:

```bash
curl http://localhost:8080/metrics | grep database
```

### PostgreSQL Monitoring

PostgreSQL provides additional monitoring capabilities:

```sql
-- Connection count
SELECT count(*) FROM pg_stat_activity WHERE datname = 'llmproxy';

-- Table sizes
SELECT relname, pg_size_pretty(pg_total_relation_size(relid))
FROM pg_catalog.pg_statio_user_tables
ORDER BY pg_total_relation_size(relid) DESC;
```

## Troubleshooting

See the [PostgreSQL Troubleshooting Guide](postgresql-troubleshooting.md) for common issues and solutions.

## Related Documentation

- [Docker Compose PostgreSQL Setup](docker-compose-postgres.md)
- [Database Migrations Guide](migrations.md)
- [Migration Concurrency](migrations-concurrency.md)
