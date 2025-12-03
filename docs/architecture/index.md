---
title: Architecture
nav_order: 2
has_children: true
---

# Architecture

Understanding the LLM Proxy system design and architecture.

## What's in this section

- **[Architecture Overview](architecture.md)** - System design, components, and data flow
- **[Brownfield Architecture](brownfield-architecture.md)** - Current implementation state and technical reality
- **[Proxy Design Decisions](proxy-design.md)** - Design rationale for the transparent proxy
- **[Technical Debt Register](technical-debt.md)** - Known issues and improvement plans
- **[Code Organization](code-organization.md)** - Package structure and module organization

## Quick Overview

The LLM Proxy is a transparent reverse proxy that:
- Replaces short-lived "withering" tokens with real API keys
- Provides project-based access control
- Publishes async events for observability
- Caches HTTP responses for efficiency

For the complete system design, see the [Architecture Overview](architecture.md).

