# LLM Proxy Documentation

This directory contains comprehensive documentation for the LLM Proxy project. Use this index to find the information you need.

## Getting Started

Start with the main [README](../README.md) for a quick overview, then follow these guides:

- **[Installation Guide](getting-started/installation.md)** - Install via Docker, Docker Compose, or from source
- **[Configuration Reference](getting-started/configuration.md)** - All environment variables and config options
- **[Admin UI Quickstart](admin/quickstart.md)** - Set up projects and tokens via web interface

## User Guides

- **[Token Management Guide](guides/token-management.md)** - Complete token lifecycle: create, use, revoke, expire
- **[Admin UI Guide](admin/index.md)** - Web interface for management
  - [Project Management](admin/projects.md) - Create and manage projects
  - [Token Management](admin/tokens.md) - Generate and manage tokens
  - [Screenshots](admin/screens.md) - Visual tour of all screens
- **[Troubleshooting & FAQ](troubleshooting.md)** - Common issues and solutions
- **[Performance Tuning](deployment/performance.md)** - Optimize caching, connections, and scaling

## Architecture & Design

- **[Architecture Overview](architecture/index.md)** - Complete system architecture, components, and data flow
- **[Code Organization Guide](code-organization.md)** - Package structure, layering, and dependency management  
- **[Proxy Design](proxy-design.md)** - Transparent proxy implementation details
- **[Caching Strategy](caching-strategy.md)** - HTTP response caching with Redis backend and streaming support

## Configuration & Setup

- **[Configuration Reference](getting-started/configuration.md)** - Complete environment variable reference
- **[API Configuration](guides/api-configuration.md)** - Configure API providers, endpoints, and security policies
- **[Security Best Practices](deployment/security.md)** - Production security, secrets management, and hardening
- **[Docker Compose PostgreSQL Setup](database/docker-compose-postgres.md)** - Run llm-proxy with PostgreSQL using Docker Compose
- **[Docker Compose MySQL Setup](database/docker-compose-mysql.md)** - Run llm-proxy with MySQL using Docker Compose
- **[Database Selection Guide](database/database-selection.md)** - Choose between SQLite, PostgreSQL, and MySQL
- **[Database Migrations Guide](database/migrations.md)** - Version-controlled schema changes

## Observability & Monitoring

- **[Log Search & Filter Guide](guides/logging.md)** - Structured log fields, jq queries, and observability backend examples
- **[Instrumentation Guide](observability/instrumentation.md)** - Event system, async middleware, and monitoring
- **[Security Best Practices](deployment/security.md)** - Production security, secrets management, audit logging, and hardening

## Troubleshooting

- **[Troubleshooting & FAQ](troubleshooting.md)** - Common issues and solutions for all components
- **[PostgreSQL Troubleshooting](database/postgresql-troubleshooting.md)** - PostgreSQL-specific issues and solutions

## API Reference

For complete API reference, see the main [README](../README.md) which covers:
- Management API endpoints (`/manage/projects`, `/manage/tokens`)
- Proxy endpoints (`/v1/*`)
- Health and monitoring endpoints (`/health`, `/metrics`)
- CLI commands and flags

**Detailed References:**
- **[CLI Reference](guides/cli-reference.md)** - Comprehensive command-line interface documentation
- **[Go Package Documentation](go-packages.md)** - Using LLM Proxy packages in your Go applications

The [OpenAPI specification](../api/openapi.yaml) provides machine-readable API definitions.

## Development & Contribution

- **[Contributing Guidelines](../CONTRIBUTING.md)** - How to contribute, TDD workflow, and PR process
- **[Testing Guide](testing-guide.md)** - Comprehensive testing practices, TDD workflow, and coverage requirements  
- **[Code Organization Guide](code-organization.md)** - Package structure, layering, and architectural boundaries
- **[Development Setup](copilot-agent-setup.md)** - Development environment and tooling

## Implementation Details

- **[Brownfield Architecture](brownfield-architecture.md)** - **ACTUAL** system state, technical debt, and constraints
- **[Technical Debt Register](technical-debt.md)** - Consolidated tracking of known issues and improvements
- **[Technical Debt GitHub Issues](TECHNICAL_DEBT_GITHUB_ISSUES.md)** - Summary of GitHub issues created for technical debt
- **[Epic Breakdown Summary](EPIC_BREAKDOWN_SUMMARY.md)** - Brownfield epic breakdown with 24 sub-issues for technical debt
- **[PostgreSQL Epic Breakdown](POSTGRESQL_EPIC_BREAKDOWN.md)** - Detailed breakdown of PostgreSQL support (#57)
- **[Issues](issues/)** - Design decisions, architectural discussions, and implementation notes
- **[Tasks](tasks/)** - Development tasks and tracking
- **[Done](done/)** - Completed features and implementation history

## Quick Reference

### Environment Variables

| Variable | Purpose | Default |
|----------|---------|---------|
| `MANAGEMENT_TOKEN` | Admin API access | **Required** |
| `LISTEN_ADDR` | Server address | `:8080` |
| `DB_DRIVER` | Database driver (`sqlite`, `postgres`, or `mysql`) | `sqlite` |
| `DATABASE_PATH` | SQLite database path | `./data/llm-proxy.db` |
| `DATABASE_URL` | PostgreSQL or MySQL connection string | - |
| `LOG_LEVEL` | Logging level | `info` |
| `HTTP_CACHE_ENABLED` | Enable response caching | `true` |
| `HTTP_CACHE_BACKEND` | Cache backend (`in-memory` or `redis`) | `in-memory` |
| `AUDIT_ENABLED` | Enable audit logging | `true` |
| `AUDIT_LOG_FILE` | Audit log file path | `./data/audit.log` |
| `AUDIT_STORE_IN_DB` | Store audit events in DB | `true` |
| `OBSERVABILITY_BUFFER_SIZE` | Event bus buffer | `1000` |

See [Configuration Reference](getting-started/configuration.md) for the complete list.

### Key Commands

```bash
# Start server
llm-proxy server

# Interactive setup
llm-proxy setup --interactive

# Create project
llm-proxy manage project create --name "My Project" --openai-key sk-...

# Generate token
llm-proxy manage token generate --project-id <id> --duration 24
```

### Key Endpoints

```bash
# Health check
GET /health

# List projects (requires management token)
GET /manage/projects

# Proxy OpenAI request (requires withering token)
POST /v1/chat/completions
```

## Documentation Map

| If you want to... | Read... |
|-------------------|---------|
| Install the proxy | [Installation Guide](getting-started/installation.md) |
| Configure settings | [Configuration Reference](getting-started/configuration.md) |
| Manage via web UI | [Admin UI Guide](admin/index.md) |
| Work with tokens | [Token Management Guide](guides/token-management.md) |
| Troubleshoot issues | [Troubleshooting & FAQ](troubleshooting.md) |
| Optimize performance | [Performance Tuning](deployment/performance.md) |
| Secure deployment | [Security Best Practices](deployment/security.md) |
| Use the CLI | [CLI Reference](guides/cli-reference.md) |
| Understand architecture | [Architecture Overview](architecture/index.md) |

## Support

- Check existing [issues](../PLAN.md) for known problems and solutions
- Review [architecture documentation](architecture/index.md) for system understanding
- See [security guidelines](deployment/security.md) for production deployment

---

**Note:** This documentation is continuously updated. If you find gaps or outdated information, please contribute updates or open an issue.