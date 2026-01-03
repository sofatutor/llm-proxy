---
title: MySQL Docker Setup
parent: Database
nav_order: 5
---

# MySQL Docker Compose Setup

This guide covers how to run llm-proxy with MySQL using Docker Compose.

## Build Tags

The llm-proxy uses Go build tags to conditionally compile MySQL support. This allows for smaller binaries when MySQL support is not needed.

### Default Build (SQLite Only)

By default, the binary is built without MySQL support:

```bash
# Build without MySQL support (smaller binary)
go build ./cmd/proxy

# Or with explicit flags
go build -o llm-proxy ./cmd/proxy
```

### Build with MySQL Support

To enable MySQL support, use the `mysql` build tag:

```bash
# Build with MySQL support
go build -tags mysql ./cmd/proxy

# Or with explicit flags
go build -tags mysql -o llm-proxy ./cmd/proxy
```

### Docker Build Variants

The Dockerfile supports MySQL via the `MYSQL_SUPPORT` build argument:

```bash
# Build with MySQL support
docker build --build-arg MYSQL_SUPPORT=true -t llm-proxy:mysql .

# Build without MySQL support (smaller image)
docker build --build-arg MYSQL_SUPPORT=false -t llm-proxy:sqlite .
```

### Binary Size Comparison

| Variant | Approximate Size |
|---------|-----------------|
| SQLite only | ~31 MB |
| With MySQL | ~37 MB |
| With PostgreSQL | ~37 MB |

### Error Handling

If you try to use MySQL with a binary built without the `mysql` tag, you'll receive this error:
```
MySQL named locking requires the 'mysql' build tag
```

## Overview

The llm-proxy supports SQLite (default), PostgreSQL, and MySQL as database backends. This guide explains how to run the MySQL setup using Docker Compose.

## Quick Start

### Prerequisites

- Docker and Docker Compose installed
- Go 1.23+ (for running integration tests)

### Start MySQL Only

To start just the MySQL database (useful for development):

```bash
# Start MySQL with the mysql profile
docker compose --profile mysql up -d mysql

# Wait for MySQL to be ready
docker compose --profile mysql exec mysql mysql -ullmproxy -psecret -e "SELECT 1"
```

### Start Full Stack with MySQL

To start the full llm-proxy stack with MySQL backend:

```bash
# Set required environment variables
export MYSQL_ROOT_PASSWORD="your-secure-password"
export MYSQL_PASSWORD="your-secure-password"
export OPENAI_API_KEY="your-openai-key"
export MANAGEMENT_TOKEN="your-management-token"

# Start all services with MySQL
docker compose --profile mysql up -d
```

### Stop and Clean Up

```bash
# Stop all containers
docker compose --profile mysql down

# Stop and remove volumes (deletes all data)
docker compose --profile mysql down -v
```

## Configuration

### Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `MYSQL_ROOT_PASSWORD` | `secret` | MySQL root password (required for production) |
| `MYSQL_PASSWORD` | `secret` | MySQL user password (required for production) |
| `MYSQL_DATABASE` | `llmproxy` | MySQL database name |
| `DB_DRIVER` | `sqlite` | Database driver (`sqlite`, `postgres`, or `mysql`) |
| `DATABASE_URL` | - | MySQL connection string |
| `DATABASE_POOL_SIZE` | `10` | Maximum database connections |
| `DATABASE_MAX_IDLE_CONNS` | `5` | Maximum idle connections |
| `DATABASE_CONN_MAX_LIFETIME` | `1h` | Connection max lifetime |

### Connection String Format

The MySQL connection string follows this format:

```
user:password@tcp(host:port)/database?parseTime=true
```

**Example:**
```
llmproxy:secret@tcp(mysql:3306)/llmproxy?parseTime=true
```

### TLS/SSL Options

| Parameter | Description |
|-----------|-------------|
| `tls=false` | No TLS (development only) |
| `tls=true` | TLS with system CA verification |
| `tls=skip-verify` | TLS without certificate verification |
| `tls=custom` | TLS with custom CA (requires additional config) |

**Production Recommendation:** Use `tls=true` with proper certificates.

**Example with TLS:**
```
llmproxy:secret@tcp(mysql:3306)/llmproxy?parseTime=true&tls=true
```

## Docker Compose Services

The MySQL profile adds these services:

### mysql

The MySQL 8.4 database server.

- **Image:** `mysql:8.4.5`
- **Port:** `3306`
- **Healthcheck:** `mysqladmin ping`
- **Volume:** `mysql_data` (persistent)
- **Container:** `llm-proxy-mysql-db`

### mysql-test

The MySQL test instance (with `mysql-test` profile).

- **Image:** `mysql:8.4.5`
- **Port:** `33306` (to avoid conflict with development instance)
- **Healthcheck:** `mysqladmin ping`
- **Volume:** `mysql_test_data` (persistent)
- **Container:** `llm-proxy-mysql-test`

### llm-proxy-mysql

The llm-proxy server configured to use MySQL.

- **Port:** `8082` (to avoid conflict with SQLite version on `8080`)
- **Depends on:** `mysql` (healthy), `redis` (started)
- **Container:** `llm-proxy-mysql`

## Integration Tests

### Run MySQL Integration Tests

Similar to PostgreSQL, you can run integration tests against MySQL:

```bash
# Start MySQL test instance
docker compose --profile mysql-test up -d mysql-test

# Wait for MySQL
docker compose --profile mysql-test exec mysql-test mysqladmin ping -h localhost

# Run tests with mysql and integration tags
export TEST_MYSQL_URL="llmproxy:secret@tcp(localhost:33306)/llmproxy?parseTime=true"
go test -v -race -tags=mysql,integration ./internal/database/...

# Clean up
docker compose --profile mysql-test down -v
```

## Data Persistence

MySQL data is persisted in Docker volumes:
- `llm-proxy-mysql-data` - Development instance data
- `llm-proxy-mysql-test-data` - Test instance data

### View Data Volume

```bash
docker volume inspect llm-proxy-mysql-data
```

### Backup Database

```bash
docker compose --profile mysql exec mysql \
  mysqldump -ullmproxy -psecret llmproxy > backup.sql
```

### Restore Database

```bash
cat backup.sql | docker compose --profile mysql exec -T mysql \
  mysql -ullmproxy -psecret llmproxy
```

## Migration from SQLite

To migrate from SQLite to MySQL:

1. Export data from SQLite:
   ```bash
   # Export tokens and projects (adapt as needed)
   sqlite3 data/llm-proxy.db ".dump tokens" > tokens.sql
   sqlite3 data/llm-proxy.db ".dump projects" > projects.sql
   ```

2. Start MySQL:
   ```bash
   docker compose --profile mysql up -d mysql
   ```

3. Migrations run automatically when the application starts.

4. Import data (you may need to adapt SQL syntax):
   ```bash
   # Note: SQLite and MySQL have different SQL dialects
   # Manual data migration may be required
   ```

## Troubleshooting

### MySQL Won't Start

Check the logs:
```bash
docker compose --profile mysql logs mysql
```

Common issues:
- Volume permission problems
- Port 3306 already in use
- Insufficient memory

### Connection Refused

Ensure MySQL is healthy:
```bash
docker compose --profile mysql exec mysql mysqladmin ping -h localhost
```

### Migration Failures

Check application logs:
```bash
docker compose --profile mysql logs llm-proxy-mysql
```

Common issues:
- Database doesn't exist (should be created by MySQL container)
- Invalid migration files
- MySQL named lock errors (ensure `mysql` build tag is used)

### Reset Database

To completely reset the MySQL database:

```bash
# Stop and remove containers and volumes
docker compose --profile mysql down -v

# Start fresh
docker compose --profile mysql up -d mysql
```

## Port Allocation

| Service | Port | Usage |
|---------|------|-------|
| mysql (dev) | 3306 | Development database |
| mysql-test | 33306 | Test database (isolated) |
| llm-proxy-mysql | 8082 | Proxy with MySQL backend |

This allocation prevents conflicts between:
- Development and test MySQL instances
- Multiple database backend variants (SQLite, PostgreSQL, MySQL)

## Security Considerations

### Production Checklist

- [ ] Change `MYSQL_ROOT_PASSWORD` and `MYSQL_PASSWORD` from defaults
- [ ] Use TLS connections (`tls=true`)
- [ ] Limit network access to MySQL port
- [ ] Enable MySQL audit logging
- [ ] Regular database backups
- [ ] Consider encrypting data at rest
- [ ] Use strong passwords (minimum 16 characters)

### API Key Storage

API keys are stored encrypted in the database when `ENCRYPTION_KEY` is set:
- Use a secret manager for `ENCRYPTION_KEY`
- Generate with: `openssl rand -base64 32`
- Implement key rotation policies

## Performance Tuning

### MySQL Configuration

The default MySQL configuration is suitable for development. For production:

1. **Increase connection pool size** based on expected load:
   ```bash
   export DATABASE_POOL_SIZE=50
   export DATABASE_MAX_IDLE_CONNS=25
   ```

2. **Tune MySQL server** via custom configuration file:
   ```yaml
   # Add to docker-compose.yml mysql service
   volumes:
     - ./config/my.cnf:/etc/mysql/conf.d/custom.cnf
   ```

3. **Monitor connection usage**:
   ```bash
   docker compose --profile mysql exec mysql mysql -ullmproxy -psecret \
     -e "SHOW STATUS LIKE 'Threads_connected'"
   ```

## Comparison with PostgreSQL

| Feature | MySQL | PostgreSQL |
|---------|-------|------------|
| Port | 3306 | 5432 |
| Image | mysql:8.4.5 | postgres:15 |
| Health Check | mysqladmin ping | pg_isready |
| Lock Mechanism | Named locks | Advisory locks |
| Build Tag | mysql | postgres |
| Connection String | user:pass@tcp(host:port)/db | postgres://user:pass@host:port/db |

Both backends offer similar features and performance for llm-proxy workloads.

## Related Documentation

- [PostgreSQL Docker Setup](docker-compose-postgres.md) - PostgreSQL alternative
- [Migrations Guide](migrations.md) - Database migration system
- [Migration Concurrency Guide](migrations-concurrency.md) - Concurrent migration handling
- [Configuration Reference](.env.example) - All environment variables
