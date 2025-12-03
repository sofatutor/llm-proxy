---
title: Observability
nav_order: 6
has_children: true
---

# Observability

Monitoring, logging, and performance tracking for LLM Proxy.

## What's in this section

- **[Instrumentation](instrumentation.md)** - Event middleware, async event bus, and dispatcher integrations
- **[Distributed Rate Limiting](distributed-rate-limiting.md)** - Redis-backed rate limiting across instances
- **[HTTP Response Caching](caching-strategy.md)** - Cache configuration and performance
- **[Coverage Reports](coverage.md)** - Live test coverage report
- **[Coverage Setup](coverage-reports.md)** - Setting up coverage reporting

## Event Flow

```
Request → Proxy → Event Bus → Dispatcher → Backends
                     ↓
              [Lunary, Helicone, File]
```

For detailed event flow documentation, see the [Instrumentation Guide](instrumentation.md).

