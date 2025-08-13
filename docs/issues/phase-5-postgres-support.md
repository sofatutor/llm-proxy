# Phase 5: PostgreSQL Support (Opt-in over SQLite)

## Summary
Enable PostgreSQL as an optional, production-grade database backend while keeping SQLite as the default for local/dev and simple deployments. Add configuration switches to choose the backend, wire up Docker Compose for local Postgres, and provide docs and tests. No change to default behavior.

Tracking: to be created (GitHub Issue)

## Motivation
- SQLite is ideal for local/dev but not for high-concurrency production workloads
- Many environments standardize on managed PostgreSQL; offering a drop-in switch reduces friction
- We already plan for Postgres in architecture; this operationalizes it earlier

## Goals
- Opt-in Postgres support with minimal config changes
- Preserve SQLite as the default; zero breaking changes
- Provide Docker Compose service for Postgres and a quick-start
- Tests (unit + minimal integration) for both backends
- Documentation for configuration, migrations, and operations

## Non-Goals
- Full DB optimization and tuning (tracked separately in `phase-7-db-optimization.md`)
- Advanced features (multi-tenant schemas, read replicas, failover)

## Requirements
- Configurable backend: `DB_DRIVER=sqlite|postgres`
- SQLite default behavior remains unchanged
- Postgres connection via `DATABASE_URL` (preferred) or discrete envs
- Migration/initialization compatible with both backends
- Docker Compose service for Postgres; app can connect when configured

## Design / Approach
1) Configuration
- Add new envs:
  - `DB_DRIVER` (default: `sqlite`)
  - `DATABASE_URL` (Postgres), example: `postgres://llmproxy:llmproxy@postgres:5432/llmproxy?sslmode=disable`
  - Continue supporting `DATABASE_PATH` for SQLite
- CLI `server` flags mirror env overrides where applicable

2) Database Abstraction
- Introduce `internal/database/driver.go` with an interface:
  - `Open(config) (*DB, error)`
  - `Init(*DB) error`
  - `Close() error`
- Implement `sqlite` driver (adapt current logic)
- Implement `postgres` driver using `lib/pq` or `pgx` (pgx preferred)

3) Migrations
- Adapt existing `scripts/schema.sql` into portable SQL where possible
- If dialect differences arise, maintain two sets: `migrations/sqlite` and `migrations/postgres`
- Consider lightweight migration runner (embedded SQL + version table)

4) Wiring in App
- In `cmd/proxy/server.go`, pick driver based on `DB_DRIVER`
- For Postgres, use `DATABASE_URL`; validate connectivity; run bootstrap/migrations
- Ensure pooling settings and sensible defaults (max open/idle, lifetime)

5) Docker Compose
- Add a `postgres` service (already scaffolded) with healthcheck and volume
- Document a Postgres compose profile or env example:
  - Set `DB_DRIVER=postgres`
  - Set `DATABASE_URL=postgres://llmproxy:llmproxy@postgres:5432/llmproxy?sslmode=disable`

6) Testing
- Unit tests for config parsing and driver selection
- Minimal integration tests (tagged) that spin up Postgres via CI service or GitHub Actions service container
- Ensure existing tests pass against SQLite

7) Documentation
- README: Postgres quick start and env reference
- `docs/api-configuration.md` or new `docs/database.md`: detailed DB config and operations
- Update `docker-compose.yml` section in README

## Acceptance Criteria
- `DB_DRIVER` supports `sqlite` (default) and `postgres`
- App starts and operates against both backends
- Docker Compose includes a working Postgres setup; app connects when configured
- Docs updated with configuration and examples
- Tests pass locally and in CI for SQLite; Postgres integration tests run in CI or behind a flag

## Tasks
- [ ] Config: add `DB_DRIVER` and `DATABASE_URL`; defaults unchanged
- [ ] Abstraction: extract database interface; refactor SQLite path
- [ ] Implement Postgres driver (pgx), pooling config, and ping
- [ ] Migrations: port schema; handle dialect differences
- [ ] App wiring: driver selection in `cmd/proxy/server.go`
- [ ] docker-compose: finalize Postgres service; document env to enable
- [ ] Tests: unit + minimal integration for Postgres
- [ ] Docs: README and database docs; update compose instructions
- [ ] CI: optional Postgres service/integration job (matrix or separate workflow)

## Risks / Considerations
- Divergent SQL dialects may require separate migration sets
- Connection pool defaults need tuning for Postgres
- CI runtime for DB integration jobs adds time; can be optional initially


