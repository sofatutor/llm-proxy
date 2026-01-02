---
title: Database Selection
parent: Database
nav_order: 1
---

# Database Selection Guide

This guide helps you choose between SQLite and PostgreSQL for your LLM Proxy deployment.

## Overview

The LLM Proxy supports three database backends:

| Feature | SQLite | PostgreSQL | MySQL |
|---------|--------|------------|-------|
| Setup Complexity | Zero configuration | Requires server setup | Requires server setup |
| Scalability | Single instance | Multiple instances | Multiple instances |
| Concurrent Writes | Limited | Excellent | Excellent |
| Best For | Development, small deployments | Production, high-traffic | Production, web apps |
| Build Tag Required | No | Yes (`-tags postgres`) | Yes (`-tags mysql`) |
| Maintenance | None | Requires DBA knowledge | Requires DBA knowledge |
| Cloud Managed Options | N/A | RDS, Aurora, Cloud SQL, Azure | RDS, Aurora, Cloud SQL, Azure |
| In-Cluster Helm | N/A | Dev/Test only (single replica) | Dev/Test only (single replica) |

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

### Production Deployment

**⚠️ For production high-availability (HA) deployments, use an external managed PostgreSQL service:**
- Amazon Aurora PostgreSQL / Amazon RDS for PostgreSQL
- Google Cloud SQL for PostgreSQL
- Azure Database for PostgreSQL
- Self-managed PostgreSQL with replication

**Important:** The in-cluster PostgreSQL StatefulSet in the Helm chart is hardcoded to `replicas: 1` and is suitable for **development/testing only**. It does not provide high availability or automatic failover.

## MySQL

MySQL is recommended for production deployments, especially for teams already familiar with MySQL or using MySQL-based infrastructure.

### When to Use MySQL

- **Production deployments** - Excellent reliability and wide ecosystem
- **High traffic** - Handles thousands of concurrent connections
- **Multiple instances** - Required for horizontal scaling
- **MySQL familiarity** - Team has existing MySQL expertise
- **MySQL infrastructure** - Already using MySQL for other services

### Configuration

```bash
export DB_DRIVER=mysql
export DATABASE_URL=llmproxy:secret@tcp(localhost:3306)/llmproxy?parseTime=true&tls=true

# Optional: Connection pool settings
export DATABASE_POOL_SIZE=10
export DATABASE_MAX_IDLE_CONNS=5
export DATABASE_CONN_MAX_LIFETIME=1h
```

### Connection String Format

```
[user]:[password]@tcp([host]:[port])/[database]?parseTime=true&tls=[mode]
```

**TLS/SSL Options:**

| Parameter | Description | Use Case |
|-----------|-------------|----------|
| `tls=false` | No TLS | Development only |
| `tls=true` | TLS with system CA verification | Production (recommended) |
| `tls=skip-verify` | TLS without certificate verification | Not recommended |
| `tls=custom` | TLS with custom CA | Advanced configurations |

**Examples:**

```bash
# Local development (no TLS)
DATABASE_URL=llmproxy:secret@tcp(localhost:3306)/llmproxy?parseTime=true

# Production with TLS
DATABASE_URL=llmproxy:secret@tcp(localhost:3306)/llmproxy?parseTime=true&tls=true

# AWS RDS MySQL
DATABASE_URL=admin:password@tcp(mydb.xxx.rds.amazonaws.com:3306)/llmproxy?parseTime=true&tls=true

# Google Cloud SQL MySQL
DATABASE_URL=root:password@tcp(10.x.x.x:3306)/llmproxy?parseTime=true&tls=true

# Azure Database for MySQL
DATABASE_URL=admin@server:password@tcp(server.mysql.database.azure.com:3306)/llmproxy?parseTime=true&tls=true
```

### Performance Characteristics

- **Read performance**: Excellent with proper indexing
- **Write performance**: Excellent with connection pooling
- **Concurrent connections**: Thousands with proper configuration
- **Recommended**: 100+ requests/second, multiple instances

### Compatibility

- **Minimum Version**: MySQL 8.0+
- **MariaDB Support**: MariaDB 10.5+ is compatible
- **Build Requirement**: Binary must be built with `-tags mysql` flag

### Production Deployment

**⚠️ For production high-availability (HA) deployments, use an external managed MySQL service:**
- Amazon Aurora MySQL / Amazon RDS for MySQL
- Google Cloud SQL for MySQL
- Azure Database for MySQL
- Self-managed MySQL Group Replication or InnoDB Cluster

**Important:** The in-cluster MySQL StatefulSet in the Helm chart is hardcoded to `replicas: 1` and is suitable for **development/testing only**. It does not provide high availability or automatic failover.

## Migration Between Databases

### SQLite to PostgreSQL or MySQL

1. **Export data from SQLite:**

   ```bash
   sqlite3 data/llm-proxy.db ".dump projects" > projects.sql
   sqlite3 data/llm-proxy.db ".dump tokens" > tokens.sql
   ```

2. **Start target database:**

   ```bash
   # PostgreSQL
   docker compose --profile postgres up -d postgres
   
   # MySQL
   docker compose --profile mysql up -d mysql
   ```

3. **Update configuration:**

   ```bash
   # For PostgreSQL
   export DB_DRIVER=postgres
   export DATABASE_URL=postgres://llmproxy:secret@localhost:5432/llmproxy?sslmode=disable
   
   # For MySQL
   export DB_DRIVER=mysql
   export DATABASE_URL=llmproxy:secret@tcp(localhost:3306)/llmproxy?parseTime=true
   ```

4. **Start the proxy** (migrations run automatically):

   ```bash
   llm-proxy server
   ```

5. **Import data** (requires SQL syntax adaptation):

   ```bash
   # Note: SQLite, PostgreSQL, and MySQL SQL dialects differ
   # Manual adjustment of exported SQL may be required
   ```

### Considerations

- **Schema migrations**: Run automatically on startup for all databases
- **Data migration**: Manual process, requires SQL dialect conversion
- **Downtime**: Plan for maintenance window during migration
- **Build tags**: Ensure binary is built with appropriate tags (`-tags postgres` or `-tags mysql`)

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
- [Docker Compose MySQL Setup](docker-compose-mysql.md)
- [Database Migrations Guide](migrations.md)
- [Migration Concurrency](migrations-concurrency.md)
- [PostgreSQL Troubleshooting Guide](postgresql-troubleshooting.md)
