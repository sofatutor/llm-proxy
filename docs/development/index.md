---
title: Development
nav_order: 8
has_children: true
---

# Development

Contributing to and developing LLM Proxy.

## What's in this section

- **[Testing Guide](testing-guide.md)** - Running tests, coverage requirements, and TDD practices
- **[Contributing](contributing.md)** - How to contribute to the project
- **[GitHub Copilot Agent Setup](copilot-agent-setup.md)** - Setting up AI coding assistants
- **[GitHub Repository](github.md)** - Links and repository information

## Quick Start for Contributors

1. Fork and clone the repository
2. Run `make deps` to install dependencies
3. Run `make test` to verify setup
4. Run `make lint` to check code style
5. Read the [Contributing Guide](contributing.md)

## Quality Requirements

- **Test Coverage**: 90%+ on all `internal/` packages
- **Linting**: All code must pass `make lint`
- **TDD**: Write failing tests before implementing features

