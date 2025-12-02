# PostgreSQL Docker Compose Setup

This guide covers how to run llm-proxy with PostgreSQL using Docker Compose.

## Overview

The llm-proxy supports both SQLite (default) and PostgreSQL as database backends. This guide explains how to run the PostgreSQL setup using Docker Compose.

## Quick Start

### Prerequisites

- Docker and Docker Compose installed
- Go 1.23+ (for running integration tests)

### Start PostgreSQL Only

To start just the PostgreSQL database (useful for development):

```bash
# Start PostgreSQL with the postgres profile
docker compose --profile postgres up -d postgres

# Wait for PostgreSQL to be ready
docker compose --profile postgres exec postgres pg_isready -U llmproxy -d llmproxy
```

### Start Full Stack with PostgreSQL

To start the full llm-proxy stack with PostgreSQL backend:

```bash
# Set required environment variables
export POSTGRES_PASSWORD="your-secure-password"
export OPENAI_API_KEY="your-openai-key"
export MANAGEMENT_TOKEN="your-management-token"

# Start all services with PostgreSQL
docker compose --profile postgres up -d
```

### Stop and Clean Up

```bash
# Stop all containers
docker compose --profile postgres down

# Stop and remove volumes (deletes all data)
docker compose --profile postgres down -v
```

## Configuration

### Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `POSTGRES_PASSWORD` | `secret` | PostgreSQL password (required for production) |
| `DB_DRIVER` | `sqlite` | Database driver (`sqlite` or `postgres`) |
| `DATABASE_URL` | - | PostgreSQL connection string |
| `DATABASE_POOL_SIZE` | `10` | Maximum database connections |
| `DATABASE_MAX_IDLE_CONNS` | `5` | Maximum idle connections |
| `DATABASE_CONN_MAX_LIFETIME` | `1h` | Connection max lifetime |

### Connection String Format

The PostgreSQL connection string follows this format:

```
postgres://user:password@host:port/database?sslmode=MODE
```

**Example:**
```
postgres://llmproxy:secret@postgres:5432/llmproxy?sslmode=disable
```

### SSL Mode Options

| Mode | Description |
|------|-------------|
| `disable` | No SSL (development only) |
| `require` | SSL required, no certificate verification |
| `verify-ca` | SSL with CA certificate verification |
| `verify-full` | SSL with full certificate and hostname verification |

**Production Recommendation:** Use `sslmode=require` or `sslmode=verify-full`.

## Docker Compose Services

The PostgreSQL profile adds these services:

### postgres

The PostgreSQL 15 database server.

- **Image:** `postgres:15`
- **Port:** `5432`
- **Healthcheck:** `pg_isready`
- **Volume:** `postgres_data` (persistent)

### llm-proxy-postgres

The llm-proxy server configured to use PostgreSQL.

- **Port:** `8082` (to avoid conflict with SQLite version on `8080`)
- **Depends on:** `postgres` (healthy), `redis` (started)

## Integration Tests

### Run PostgreSQL Integration Tests

Use the provided script to run integration tests against a real PostgreSQL instance:

```bash
# Full test run (starts PostgreSQL, runs tests, stops PostgreSQL)
./scripts/run-postgres-integration.sh

# Start PostgreSQL and keep it running
./scripts/run-postgres-integration.sh start

# Run tests only (PostgreSQL must be running)
./scripts/run-postgres-integration.sh test

# Stop and clean up
./scripts/run-postgres-integration.sh teardown
```

### Manual Test Run

```bash
# Start PostgreSQL
docker compose --profile postgres up -d postgres

# Wait for PostgreSQL
docker compose --profile postgres exec postgres pg_isready -U llmproxy -d llmproxy

# Run tests with postgres and integration tags
export TEST_POSTGRES_URL="postgres://llmproxy:secret@localhost:5432/llmproxy?sslmode=disable"
go test -v -race -tags=postgres,integration ./internal/database/...

# Clean up
docker compose --profile postgres down -v
```

## Data Persistence

PostgreSQL data is persisted in a Docker volume named `llm-proxy-postgres-data`.

### View Data Volume

```bash
docker volume inspect llm-proxy-postgres-data
```

### Backup Database

```bash
docker compose --profile postgres exec postgres \
  pg_dump -U llmproxy llmproxy > backup.sql
```

### Restore Database

```bash
cat backup.sql | docker compose --profile postgres exec -T postgres \
  psql -U llmproxy llmproxy
```

## Migration from SQLite

To migrate from SQLite to PostgreSQL:

1. Export data from SQLite:
   ```bash
   # Export tokens and projects (adapt as needed)
   sqlite3 data/llm-proxy.db ".dump tokens" > tokens.sql
   sqlite3 data/llm-proxy.db ".dump projects" > projects.sql
   ```

2. Start PostgreSQL:
   ```bash
   docker compose --profile postgres up -d postgres
   ```

3. Migrations run automatically when the application starts.

4. Import data (you may need to adapt SQL syntax):
   ```bash
   # Note: SQLite and PostgreSQL have different SQL dialects
   # Manual data migration may be required
   ```

## Troubleshooting

### PostgreSQL Won't Start

Check the logs:
```bash
docker compose --profile postgres logs postgres
```

Common issues:
- Volume permission problems
- Port 5432 already in use

### Connection Refused

Ensure PostgreSQL is healthy:
```bash
docker compose --profile postgres exec postgres pg_isready -U llmproxy
```

### Migration Failures

Check application logs:
```bash
docker compose --profile postgres logs llm-proxy-postgres
```

Common issues:
- Database doesn't exist (should be created by PostgreSQL container)
- Invalid migration files

### Reset Database

To completely reset the PostgreSQL database:

```bash
# Stop and remove containers and volumes
docker compose --profile postgres down -v

# Start fresh
docker compose --profile postgres up -d postgres
```

## Security Considerations

### Production Checklist

- [ ] Change `POSTGRES_PASSWORD` from default
- [ ] Use `sslmode=require` or `sslmode=verify-full`
- [ ] Limit network access to PostgreSQL port
- [ ] Enable PostgreSQL audit logging
- [ ] Regular database backups
- [ ] Consider encrypting data at rest

### API Key Storage

API keys are stored in plaintext in the database. For production:
- Use a secret manager for sensitive configuration
- Consider database-level encryption
- Implement key rotation policies

## Related Documentation

- [Migrations Guide](migrations.md) - Database migration system
- [Migration Concurrency Guide](migrations-concurrency.md) - Concurrent migration handling
- [Configuration Reference](.env.example) - All environment variables
