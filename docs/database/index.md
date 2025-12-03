---
title: Database
nav_order: 5
has_children: true
---

# Database

Database configuration, migrations, and troubleshooting.

## What's in this section

- **[Database Selection](database-selection.md)** - Choosing between SQLite and PostgreSQL
- **[Migrations Guide](migrations.md)** - Database schema migrations with goose
- **[Migration Concurrency](migrations-concurrency.md)** - Handling migrations in distributed systems
- **[PostgreSQL Setup](docker-compose-postgres.md)** - Docker Compose setup for PostgreSQL
- **[PostgreSQL Troubleshooting](postgresql-troubleshooting.md)** - Common PostgreSQL issues

## Quick Decision

| Use Case | Recommended Database |
|----------|---------------------|
| Development/Testing | SQLite (default) |
| Single instance production | SQLite or PostgreSQL |
| Multi-instance production | PostgreSQL |
| AWS ECS deployment | Aurora PostgreSQL Serverless v2 |

For production deployments, see the [Database Selection Guide](database-selection.md).

