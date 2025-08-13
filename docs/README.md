# LLM Proxy Documentation

This directory contains comprehensive documentation for the LLM Proxy project. Use this index to find the information you need.

## Getting Started

Start with the main [README](../README.md) for a quick overview, installation, and basic usage.

## Architecture & Design

- **[Architecture Overview](architecture.md)** - Complete system architecture, components, and data flow
- **[Proxy Design](proxy-design.md)** - Transparent proxy implementation details
- **[Caching Strategy](caching-strategy.md)** - Token caching and performance optimization

## Configuration & Setup

- **[API Configuration](api-configuration.md)** - Configure API providers, endpoints, and security policies
- **[Security Best Practices](security.md)** - Production security, secrets management, and hardening

## Observability & Monitoring

- **[Instrumentation Guide](instrumentation.md)** - Event system, async middleware, and monitoring
- **[Security Best Practices](security.md)** - Production security, secrets management, audit logging, and hardening

## API Reference

For complete API reference, see the main [README](../README.md) which covers:
- Management API endpoints (`/manage/projects`, `/manage/tokens`)
- Proxy endpoints (`/v1/*`)
- Health and monitoring endpoints (`/health`, `/metrics`)
- CLI commands and flags

**Detailed References:**
- **[CLI Reference](cli-reference.md)** - Comprehensive command-line interface documentation
- **[Go Package Documentation](go-packages.md)** - Using LLM Proxy packages in your Go applications

The [OpenAPI specification](../api/openapi.yaml) provides machine-readable API definitions.

## Development & Contribution

- **[Development Setup](copilot-agent-setup.md)** - Development environment and tooling
- **[Contributing Guidelines](../CONTRIBUTING.md)** - How to contribute to the project

## Implementation Details

- **[Issues](issues/)** - Design decisions, architectural discussions, and implementation notes
- **[Tasks](tasks/)** - Development tasks and tracking
- **[Done](done/)** - Completed features and implementation history

## Quick Reference

### Environment Variables

| Variable | Purpose | Default |
|----------|---------|---------|
| `MANAGEMENT_TOKEN` | Admin API access | **Required** |
| `LISTEN_ADDR` | Server address | `:8080` |
| `DATABASE_PATH` | SQLite database | `./data/llm-proxy.db` |
| `LOG_LEVEL` | Logging level | `info` |
| `AUDIT_ENABLED` | Enable audit logging | `true` |
| `AUDIT_LOG_FILE` | Audit log file path | `./data/audit.log` |
| `AUDIT_STORE_IN_DB` | Store audit events in DB | `true` |
| `OBSERVABILITY_BUFFER_SIZE` | Event bus buffer | `1000` |

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

## Support

- Check existing [issues](../PLAN.md) for known problems and solutions
- Review [architecture documentation](architecture.md) for system understanding
- See [security guidelines](security.md) for production deployment

---

**Note:** This documentation is continuously updated. If you find gaps or outdated information, please contribute updates or open an issue.