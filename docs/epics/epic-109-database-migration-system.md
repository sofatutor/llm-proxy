# Epic 109: Database Migration System - Brownfield Enhancement

**GitHub Issue**: [#109](https://github.com/sofatutor/llm-proxy/issues/109)  
**Priority**: 🔴 **Critical** (Priority 1)  
**Status**: In Progress  
**Created**: 2025-11-11

---

## Epic Goal

Implement a robust database migration system to track schema changes, enable rollbacks, and prevent schema drift between environments, replacing the current manual schema management approach.

---

## Epic Description

### Existing System Context

**Current Relevant Functionality:**
- Database schema is defined inline in `internal/database/database.go` via `initDatabase()` function
- Manual schema migrations handled via `applyMigrations()` function with column existence checks
- Schema includes: `projects`, `tokens`, and `audit_events` tables
- SQLite with WAL mode for development, PostgreSQL planned for production
- Connection pooling and transaction support already implemented

**Technology Stack:**
- **Language**: Go 1.23+
- **Database**: SQLite (current), PostgreSQL (planned - Issue #57)
- **ORM**: None (using `database/sql` directly)
- **Current Migration Approach**: Manual `ALTER TABLE` statements with existence checks

**Integration Points:**
- `internal/database/database.go` - `initDatabase()` and `applyMigrations()` functions
- `cmd/proxy/main.go` - Database initialization on startup
- `llm-proxy setup` command - Initial database setup
- CI/CD pipeline - Schema validation

### Enhancement Details

**What's Being Added/Changed:**

1. **Migration Tool Integration**
   - Research and select between golang-migrate, goose, or custom solution
   - Evaluate based on: SQLite+PostgreSQL support, transaction handling, rollback capability, community support

2. **Migration Infrastructure**
   - Create `internal/database/migrations/` directory structure
   - Add migration tracking table (e.g., `schema_migrations`)
   - Convert existing schema to initial migration
   - Replace manual `applyMigrations()` with migration runner

3. **Migration Runner**
   - Implement migration runner in `internal/database/` package
   - Integrate with `llm-proxy setup` command
   - Support: up/down migrations, status queries, version targeting
   - Transaction-wrapped migrations for safety

4. **Tooling & Documentation**
   - CLI commands for migration management
   - Migration workflow documentation
   - CI validation for migration files

**How It Integrates:**
- Extends `internal/database/` package with new migration subpackage
- Replaces current `applyMigrations()` function with migration runner
- Maintains backward compatibility with existing database initialization
- Runs automatically during `llm-proxy setup` and server startup
- No changes to external APIs or user-facing functionality

**Success Criteria:**
- ✅ Migration tool selected and integrated
- ✅ All existing schema converted to migrations
- ✅ Migrations run automatically on startup
- ✅ Both SQLite and PostgreSQL supported
- ✅ Rollback works for failed migrations
- ✅ Migration status queryable via CLI
- ✅ 90%+ test coverage maintained
- ✅ CI validates migration integrity
- ✅ Zero downtime for existing databases

---

## Stories

### Story 1.1: Migration Tool Selection and Initial Setup
**GitHub Issue**: [#117](https://github.com/sofatutor/llm-proxy/issues/117)  
**Status**: ✅ **COMPLETE**  
**Estimated Effort**: 3-5 days  
**Actual Effort**: 1 day  
**PR**: [#143](https://github.com/sofatutor/llm-proxy/pull/143)

**Description**: Research migration tools (golang-migrate, goose, custom), document pros/cons, select tool, and create basic migration infrastructure.

**Tasks**:
- Research golang-migrate, goose, and custom solution options
- Document comparison matrix (SQLite/PostgreSQL support, transactions, rollback, community)
- Make selection with rationale
- Create `internal/database/migrations/` directory structure
- Add migration tracking table to schema
- Implement basic migration runner
- Add tests for migration runner

**Acceptance Criteria**:
- Migration tool selected and documented
- Basic migration runner implemented
- Migration tracking table created
- Tests pass for basic migration operations
- Documentation updated with tool selection rationale

---

### Story 1.2: Schema Migration Implementation
**GitHub Issue**: [#144](https://github.com/sofatutor/llm-proxy/issues/144)  
**Status**: Ready (unblocked - Story 1.1 complete)  
**Estimated Effort**: 2-3 days

**Description**: Convert existing schema to migrations, implement full migration runner, and add version tracking.

**Tasks**:
- Convert `initDatabase()` schema to initial migration
- Convert `applyMigrations()` logic to versioned migrations
- Implement migration runner with up/down support
- Add migration status query functionality
- Update database initialization to use migration runner
- Add comprehensive tests for migration scenarios

**Acceptance Criteria**:
- All existing schema converted to migrations
- Migration runner supports up/down/status operations
- Existing databases migrate seamlessly
- Tests cover success and failure scenarios
- 90%+ test coverage maintained

---

### Story 1.3: CLI Integration and Documentation
**GitHub Issue**: [#145](https://github.com/sofatutor/llm-proxy/issues/145)  
**Status**: Pending (blocked by Story 1.2)  
**Estimated Effort**: 1-2 days

**Description**: Integrate migration runner with CLI commands and create comprehensive documentation.

**Tasks**:
- Add migration commands to `llm-proxy` CLI
- Integrate migration runner with `llm-proxy setup` command
- Add migration validation to CI pipeline
- Create migration workflow documentation
- Document migration best practices
- Add troubleshooting guide

**Acceptance Criteria**:
- CLI commands for migration management available
- Migrations run automatically during setup
- CI validates migration integrity
- Documentation complete and clear
- Migration workflow tested end-to-end

---

## Compatibility Requirements

- ✅ **Existing APIs remain unchanged** - No external API changes
- ✅ **Database schema changes are backward compatible** - Existing databases migrate automatically
- ✅ **Follows existing patterns** - Uses same `database/sql` approach, maintains transaction support
- ✅ **Performance impact is minimal** - Migrations run only during setup/startup, not on every request
- ✅ **Zero downtime** - Existing databases continue working, migrations applied on next startup

---

## Risk Mitigation

### Primary Risks

1. **Risk**: Breaking existing database functionality during migration
   - **Probability**: Medium
   - **Impact**: HIGH
   - **Mitigation**: 
     - Comprehensive tests for existing and new schema
     - Database backup before migration
     - Rollback capability for failed migrations
     - Test migrations on copy of production data

2. **Risk**: SQLite/PostgreSQL compatibility issues
   - **Probability**: Medium
   - **Impact**: HIGH
   - **Mitigation**:
     - Tool selection prioritizes dual database support
     - Test migrations on both SQLite and PostgreSQL
     - Document database-specific migration patterns

3. **Risk**: Migration tool learning curve and integration complexity
   - **Probability**: Low
   - **Impact**: Medium
   - **Mitigation**:
     - Choose well-documented, actively maintained tool
     - Start with simple migrations
     - Document migration patterns and best practices

### Rollback Plan

- **Pre-Migration**: Automatic database backup via `VACUUM INTO`
- **Failed Migration**: Automatic rollback to previous version
- **Manual Rollback**: CLI command to revert to specific version
- **Emergency**: Restore from backup file

---

## Definition of Done

- ✅ All stories completed with acceptance criteria met
- ✅ Existing functionality verified through testing
- ✅ Integration points working correctly (setup command, startup, CI)
- ✅ Documentation updated appropriately (migration workflow, CLI reference, troubleshooting)
- ✅ No regression in existing features (all tests pass, 90%+ coverage maintained)
- ✅ Migration system validated on both SQLite and PostgreSQL
- ✅ CI pipeline includes migration validation
- ✅ Zero downtime migration path for existing databases

---

## Dependencies

- **Blocks**: Issue #57 (PostgreSQL support - requires migration system for dual database support)
- **Related**: Technical Debt Register (`docs/technical-debt.md` lines 46-72)
- **Related**: Brownfield Architecture (`docs/brownfield-architecture.md` "No Migrations" section)

---

## Technical Notes

### Current Schema Management Approach
```go
// internal/database/database.go
func initDatabase(db *sql.DB) error {
    // CREATE TABLE IF NOT EXISTS for projects, tokens, audit_events
    // Manual schema creation
}

func applyMigrations(db *sql.DB) error {
    // Manual column additions with existence checks
    // addColumnIfNotExists() for each new column
}
```

### Target Migration Approach
```go
// internal/database/migrations/runner.go
func (r *Runner) Up(ctx context.Context) error {
    // Apply pending migrations in order
    // Track applied migrations in schema_migrations table
    // Transaction-wrapped for safety
}

func (r *Runner) Down(ctx context.Context, steps int) error {
    // Rollback last N migrations
}
```

### Migration File Structure
```
internal/database/migrations/
├── 001_initial_schema.up.sql
├── 001_initial_schema.down.sql
├── 002_add_audit_events.up.sql
├── 002_add_audit_events.down.sql
└── ...
```

---

## References

- **Issue #109**: https://github.com/sofatutor/llm-proxy/issues/109
- **Story #117**: https://github.com/sofatutor/llm-proxy/issues/117
- **Technical Debt Register**: `docs/technical-debt.md`
- **Brownfield Architecture**: `docs/brownfield-architecture.md`
- **Current Schema**: `internal/database/database.go` lines 106-180

---

## Story Manager Handoff

**Story Manager Handoff:**

"Please develop detailed user stories for this brownfield epic. Key considerations:

- This is an enhancement to an existing system running **Go 1.23+, SQLite (dev), PostgreSQL (planned)**
- Integration points: 
  - `internal/database/database.go` - `initDatabase()` and `applyMigrations()` functions
  - `cmd/proxy/main.go` - Database initialization on startup
  - `llm-proxy setup` command - Initial setup flow
  - CI/CD pipeline - Schema validation
- Existing patterns to follow:
  - Direct `database/sql` usage (no ORM)
  - Transaction support for safety
  - Connection pooling configuration
  - Test-driven development (TDD) with 90%+ coverage
  - Go best practices and idiomatic code
- Critical compatibility requirements:
  - Zero downtime for existing databases
  - Automatic migration on startup
  - Rollback capability for failed migrations
  - Support for both SQLite and PostgreSQL
- Each story must include verification that existing functionality remains intact

The epic should maintain system integrity while delivering **a robust, production-ready database migration system**."

---

**Epic created by**: PM Agent (John)  
**Date**: 2025-11-11  
**Next Step**: Story Manager to create detailed story documents in `docs/stories/`

