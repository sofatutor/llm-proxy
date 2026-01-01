# AGENTS.md

# LLM Proxy - Agent Guide & Project Context

> **Note on Git & GitHub Management:**
> - **Prefer GitHub MCP for GitHub management tasks**: PR and issue creation/update, fetching and posting review comments, summarizing changes, status checks, and mapping branch↔PR.
> - **Use standard `git` for local VCS operations**: branching, committing, rebasing, and pushing.
> - **Use `gh` as a fallback** when MCP is unavailable or for convenience workflows (e.g., `gh run watch` for CI monitoring). MCP can also fetch PR statuses.
> - When using `gh` for PR/issue bodies, you may create a temporary `NEW_PR.md`/`NEW_ISSUE.md` and delete it after use; with MCP this is not required.

This file provides essential context and rules for both human and AI contributors working in this repository. It serves as the primary source of truth for agent-driven development and includes a Sparse Prime representation of the project documentation.

---

## 🎯 Project Overview: LLM Proxy

**What:** A transparent, secure proxy for OpenAI's API with token management, rate limiting, logging, and admin UI.

**Key Features:**
- **Transparent Proxying** - Minimal request/response transformation, authorization header replacement only
- **Withering Tokens** - Short-lived tokens with expiration, revocation, and rate limiting
- **Project-based Access Control** - Multi-tenant architecture with isolated API keys
- **Async Event System** - Non-blocking instrumentation and observability
- **Admin Management** - CLI and web interface for project/token management

**Technology Stack:**
- **Language:** Go 1.23+
- **Database:** SQLite (production: PostgreSQL)
- **Architecture:** Reverse proxy using `httputil.ReverseProxy`
- **Deployment:** Docker, single binary, or container orchestration

---

## Development Environment
- Use Go 1.23+ (see `.tool-versions` or Dockerfile for specifics).
- Install dependencies with `make deps` or `go mod tidy`.
- Use `make lint` to run all linters (golangci-lint, gofmt, etc.).
- Use `make test` to run all tests (unit, race, coverage).
- Use `make` to see all available targets.
- CI runs on Ubuntu with the same Makefile commands.

## 📚 Sparse Prime Documentation Map

### **Core Documentation**
```
README.md                    → Quick start, overview, basic API usage
docs/README.md              → Complete documentation index with quick reference
├── docs/guides/cli-reference.md   → Complete CLI command reference and workflows
├── docs/development/index.md      → Developer documentation index
├── docs/architecture/index.md     → System design, data flow, and components
├── docs/guides/api-configuration.md → Advanced API provider configuration
├── docs/deployment/security.md    → Production security and best practices
└── docs/observability/instrumentation.md → Event system and observability
```

### **API Structure (Sparse Prime)**
```
Health Endpoints:
  GET /health, /ready, /live  → Service status and monitoring

Management API (requires MANAGEMENT_TOKEN):
  GET    /manage/projects     → List all projects
  POST   /manage/projects     → Create project
  GET    /manage/projects/{id} → Get project details
  PATCH  /manage/projects/{id} → Update project
  DELETE /manage/projects/{id} → Delete project
  
  GET    /manage/tokens       → List tokens (filter: ?projectId=X&activeOnly=true)
  POST   /manage/tokens       → Create token
  GET    /manage/tokens/{id}  → Get token details
  DELETE /manage/tokens/{id}  → Revoke token

Proxy API (requires withering token):
  GET|POST /v1/*              → Proxied to OpenAI (transparent)
```

### **CLI Structure (Sparse Prime)**
```
llm-proxy server              → Start HTTP server
llm-proxy setup [--interactive] → Configure proxy
llm-proxy manage project <cmd> → Project CRUD operations
llm-proxy manage token <cmd>   → Token management
llm-proxy dispatcher          → Event dispatcher service
llm-proxy openai chat         → Interactive chat interface
```

### **Go Package Structure (Sparse Prime)**
```
internal/config     → Configuration management (env vars, validation)
internal/server     → HTTP server, routing, lifecycle management
internal/token      → Token generation, validation, expiration, rate limiting
internal/proxy      → Transparent reverse proxy with auth middleware
internal/database   → SQLite/PostgreSQL storage with interfaces
internal/eventbus   → Async event publishing/subscription (in-memory/Redis)
internal/utils      → Cryptographic utilities and helpers
```

### **Key Environment Variables**
```
MANAGEMENT_TOKEN=<required>     → Admin API access
LISTEN_ADDR=:8080              → Server listen address
DATABASE_PATH=./data/proxy.db  → SQLite database location
LOG_LEVEL=info                 → Logging verbosity
OBSERVABILITY_BUFFER_SIZE=1000 → Event bus buffer size
```

### **Encryption at Rest (CRITICAL for Production)**

```bash
ENCRYPTION_KEY=<base64-encoded-32-bytes>
```

**Purpose**: Protects sensitive data in the database
- **API Keys**: Encrypted with AES-256-GCM (reversible for upstream calls)
- **Tokens**: Hashed with SHA-256 (irreversible, secure lookups)

**Setup**:
```bash
# Generate a secure key
export ENCRYPTION_KEY=$(openssl rand -base64 32)

# Encrypt existing plaintext data (idempotent)
llm-proxy migrate encrypt

# Verify encryption status
llm-proxy migrate encrypt-status
```

**⚠️ Security Notes**:
- **REQUIRED** for production deployments
- Store key securely (KMS/Vault recommended)
- Never commit to version control
- Backup separately from database backups
- See `docs/security/encryption.md` for full details

---

## 🏗️ Repository Structure & Focus

**Primary Working Areas:**
- **`internal/`** - Core application logic (token, proxy, server, database)
- **`cmd/`** - Entry points (proxy server, eventdispatcher)
- **`docs/`** - Comprehensive documentation
- **`api/`** - OpenAPI specifications
- **Root config files** - Docker, Makefile, go.mod, .env

**Key Project Files:**
- **`PLAN.md`** - Always current project architecture and objectives
- **`docs/issues/`** - Primary source for project status and workflow tracking (each task as self-contained issue doc)
- **`working-agreement.mdc`** - Core development workflow rules
- **`Makefile`** - All build, test, and development commands

**Working Agreement Principles:**
1. **Issue Docs as Source of Truth** - Each major task tracked in `docs/issues/` with GitHub issue link
2. **TDD Mandate** - Failing test first, 90%+ coverage enforced, no merges without tests
3. **Immediate Resolution** - All review comments addressed in code/docs, no TODOs left unresolved
4. **Transparency** - Every change documented in issue docs with rationale and process
5. **Best Practices** - Go idioms, clear naming, documentation for all exports

---

## �️ Database & Migration Rules (CRITICAL)

**These rules are MANDATORY and must NEVER be violated:**

### 1. **NEVER use secrets as PRIMARY KEYs**
- ❌ **WRONG:** `token TEXT PRIMARY KEY` (secret in URLs, logs, cannot be obfuscated)
- ✅ **CORRECT:** `id TEXT PRIMARY KEY, token TEXT UNIQUE` (UUID for management, secret for auth)
- **Rationale:** Secrets must never appear in URLs, logs, or admin UIs. Always use UUIDs for identifiers.
- **Pattern:** Same as projects table: `id` (UUID) for management operations, separate field for secrets

### 2. **Always use goose for PostgreSQL migrations**
- ❌ **WRONG:** Creating raw SQL scripts in `/scripts/` directory
- ✅ **CORRECT:** Create versioned migration in `internal/database/migrations/sql/postgres/00XXX_name.sql`
- **Location:** `internal/database/migrations/sql/postgres/` (dialect-specific directory)
- **Format:** goose format with `-- +goose Up` and `-- +goose Down` markers
- **Versioning:** Sequential numbering: `00001_initial.sql`, `00002_feature.sql`, etc.
- **Execution:** Migrations run automatically via `migrations.NewMigrationRunner(db, migrationsPath).Up()`
- **Library:** Uses [pressly/goose](https://github.com/pressly/goose) - proper Go migration framework
- **Dialect Isolation:** The migration runner only scans the dialect-specific directory (`postgres/`), so goose only runs migrations built for that engine. This ensures PostgreSQL migrations never accidentally run against SQLite and vice versa.

### 3. **SQLite: Only maintain current schema, NO migrations**
- ❌ **WRONG:** Creating migration files for SQLite
- ✅ **CORRECT:** Update base schema in `/scripts/schema.sql` only
- **Rationale:** SQLite is for testing/dev purposes only. Production uses PostgreSQL.
- **Development:** Run `make clean-db && make build && make run` to recreate from current schema
- **Location:** `/scripts/schema.sql` contains the authoritative current schema for SQLite

### 4. **Migration Development Workflow (PostgreSQL)**
```bash
# 1. Create new goose migration file
# Location: internal/database/migrations/sql/postgres/00XXX_description.sql
# Format:
# -- +goose Up
# ALTER TABLE foo ADD COLUMN bar TEXT;
#
# -- +goose Down  
# ALTER TABLE foo DROP COLUMN bar;

# 2. Test migration locally (PostgreSQL only)
docker compose --profile postgres up --build --force-recreate

# 3. Verify migration ran successfully
docker compose logs llm-proxy-postgres | grep migration

# 4. NEVER create manual SQL scripts for migrations
# The goose runner handles execution automatically
```

---

## �🔧 Development Environment

**Setup:**
```bash
# Dependencies
make deps           # Install Go dependencies
go mod tidy         # Clean up modules

# Development
make lint          # Run all linters (golangci-lint, gofmt, etc.)
make test          # Run all tests (unit, race, coverage)
make test-coverage # Generate coverage reports
make build         # Build binaries

# See all targets
make help
```

**Requirements:**
- Go 1.23+ (see `.tool-versions` or Dockerfile)
- SQLite for local development
- Docker for containerized deployment

---

## ✅ Testing & Validation (TDD Mandatory)

**Test-Driven Development Rules:**
1. **Write failing test first** - Before implementing any feature or fix
2. **90%+ coverage required** - Enforced in CI, no exceptions
3. **All tests must pass** - Before every commit and PR
4. **Cover edge cases** - Use table-driven tests, test error conditions

**Validation Commands:**
```bash
make test          # Run all tests
make test-race     # Run with race detection
make test-coverage # Generate coverage reports
make lint          # Run all linters
```

**CI Monitoring (Mandatory):**
```bash
# After every push, monitor CI completion
gh run list        # View latest GitHub Actions runs
gh run watch <id>  # Monitor specific run until completion
```

---

## 📝 Contribution & Style Guidelines

**Go Best Practices:**
- Idiomatic naming, clear error handling
- Document all exported types/functions
- Keep code DRY, simple, maintainable
- No unresolved TODOs in code or docs

**Documentation Updates:**
- Update relevant `docs/issues/` file with every significant change
- Document rationale and workflow updates
- Keep `PLAN.md` current with architecture changes

**Review Process:**
- Address all review comments in code or docs before merging
- Implement performance/architecture feedback immediately (no deferral)
- Validate changes with tests and linters

---

## 🚀 PR Instructions

**Title Format:** `[<area>] <Short Description>`
Examples: `[proxy] Add streaming support`, `[token] Implement rate limiting`

**Description Template:**
```markdown
## Summary
Brief description of changes and motivation

## Changes
- List of specific changes made
- Reference to issue docs/checklist items
- Related PLAN.md updates

## Testing
- New tests added
- Coverage impact
- Validation performed

## Documentation
- Updated issue doc: docs/issues/xxx.md
- Other documentation changes

Fixes #issue-number
```

**Pre-merge Checklist:**
- [ ] All tests pass (`make test`)
- [ ] All linters pass (`make lint`)
- [ ] Coverage ≥ 90%
- [ ] Issue doc and PLAN.md updated
- [ ] No unresolved TODOs or review comments
- [ ] All CI jobs passed (`gh run list`/`gh run watch`)

---

## 🤖 Agent-Specific Instructions

**Git & GitHub Management:**
- **Always use `git` and `gh`** for standard operations
- **Use MCP tools only** for data not accessible via standard commands
- Create temporary files (NEW_PR.md, NEW_ISSUE.md) for body content, delete after use

**Context Gathering:**
1. **Check current issue doc** in `docs/issues/` for task context
2. **Review PLAN.md** for architecture and objectives
3. **Reference documentation** using the Sparse Prime map above
4. **Extend context** by reading specific docs as needed

**Development Workflow:**
1. **CRITICAL: Create feature branch from epic/main** - NEVER work directly on epic/main branches
   - Check current branch: `git branch --show-current`
   - If on epic/main, create feature branch: `git checkout -b <feature-branch-name>`
   - Verify branch name TWICE before starting work
2. Understand the task from issue doc and PLAN.md
3. Write failing tests first (TDD)
4. Implement minimal solution
5. Ensure tests pass and coverage ≥ 90%
6. Run linters and fix issues
7. **VERIFY BRANCH AGAIN before commit** - Check `git branch --show-current` one more time
8. Update documentation (issue doc, relevant docs)
9. Create PR with proper format (feature branch → epic branch)
10. Monitor CI completion

**Quality Standards:**
- Prefer small, reviewable increments
- Document every significant step in issue docs
- Respect nested AGENTS.md files if present in subfolders
- When in doubt, update documentation and ask for clarification

### Global agent workflow rules (model-agnostic)
- Control eagerness explicitly per task:
  - Less eagerness: parallelize a single discovery batch, stop as soon as precise edits are known; avoid over-searching; escalate once if signals conflict.
  - More eagerness: persist until the task is fully resolved; proceed under reasonable assumptions and document them.
- Tool preambles: restate the goal, outline a short plan, provide brief progress notes during execution, and end with a short summary of changes and validation status.
- Stop conditions: tests green (including `-race`), coverage ≥ 90%, linters clean, no unrelated formatting, minimal diffs, no unresolved review items.
- Safe vs risky actions: small edits and tests are safe; deleting/renaming files, changing public APIs, or adding heavy dependencies are risky and require explicit rationale and tests.
- Context gathering and parallelization: batch semantic/code searches in parallel; cache results; avoid repeated reads; stop early once specific edits are identified.

---

## 🔗 Quick Links for Context Extension

**Project Understanding:**
- [Architecture Overview](docs/architecture/index.md) - Complete system design
- [Project Plan](PLAN.md) - Current objectives and roadmap
- [Working Agreement](working-agreement.mdc) - Core development rules

**Implementation Details:**
- [CLI Reference](docs/guides/cli-reference.md) - Complete command documentation
- [Developer Docs](docs/development/index.md) - Developer documentation index
- [API Configuration](docs/guides/api-configuration.md) - Advanced proxy configuration

**Production & Security:**
- [Security Guide](docs/deployment/security.md) - Production security practices
- [OpenAPI Spec](api/openapi.yaml) - Machine-readable API definitions

**Development Process:**
- [Issues Directory](docs/issues/) - Active task tracking and context
- [Contributing Guide](CONTRIBUTING.md) - Detailed contribution process

---

**For complete details, see the [Documentation Index](docs/README.md) and individual documentation files.** 