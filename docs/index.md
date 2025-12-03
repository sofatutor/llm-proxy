---
title: Home
nav_order: 0
---

# LLM Proxy

Open-source reverse proxy for LLM APIs with withering tokens, management UI, and observability.

## Why LLM Proxy?

- Transparent proxying to OpenAI‑compatible endpoints
- Short‑lived withering tokens with project‑scoped access control
- Admin UI for projects, tokens, and audit trails
- Async event system with pluggable dispatcher integrations (file, Lunary, Helicone)

Get the big picture in the [Architecture](architecture/architecture.md), try the [Quickstart](getting-started/quickstart.md), and explore the [Admin UI](admin/).

## Documentation Sections

| Section | Description |
|---------|-------------|
| [Getting Started](getting-started/) | Installation, quickstart, configuration |
| [Architecture](architecture/) | System design, brownfield reality, code organization |
| [Admin UI](admin/) | Web interface for projects, tokens, and audit logs |
| [Guides](guides/) | CLI reference, API configuration, troubleshooting |
| [Database](database/) | Database selection, migrations, PostgreSQL setup |
| [Observability](observability/) | Instrumentation, rate limiting, caching, coverage |
| [Deployment](deployment/) | AWS ECS, performance tuning, security |
| [Development](development/) | Testing, contributing, GitHub setup |

## Quick Links

- **[Installation Guide](getting-started/installation.md)** - Get started in minutes
- **[AWS ECS Deployment](deployment/aws-ecs-cdk.md)** - Production deployment on AWS
- **[CLI Reference](guides/cli-reference.md)** - Complete command documentation
- **[Security Best Practices](deployment/security.md)** - Production security guidelines

## Contributors welcome

- Read the [Contributing guide](development/contributing.md)
- Pick a task: [good first issues](https://github.com/sofatutor/llm-proxy/issues?q=is%3Aissue+is%3Aopen+label%3A%22good+first+issue%22)
- Explore the [roadmap](https://github.com/sofatutor/llm-proxy/blob/main/PLAN.md)

## Status & Coverage

![Build](https://img.shields.io/github/actions/workflow/status/sofatutor/llm-proxy/pages.yml?branch=main)
![Coverage](https://img.shields.io/badge/coverage-90%25+-brightgreen)
[![GitHub](https://img.shields.io/badge/GitHub-Repo-black?logo=github)](https://github.com/sofatutor/llm-proxy)

View the live coverage report on the [Coverage page](observability/coverage.md).
