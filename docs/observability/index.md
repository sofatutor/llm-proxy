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

## Grafana Dashboards

Ready-to-import Grafana dashboards are available for visualizing LLM Proxy metrics:

- **Location**: [`deploy/helm/llm-proxy/dashboards/`](../../deploy/helm/llm-proxy/dashboards/)
- **Documentation**: See the [dashboards README](../../deploy/helm/llm-proxy/dashboards/README.md) for import instructions
- **Datasource**: Prometheus

The operational dashboard (`llm-proxy.json`) includes:
- Request rate, error rate, and uptime monitoring
- Cache performance metrics (hits, misses, bypass, stores)
- Memory usage and Go runtime metrics
- Garbage collection statistics

## Event Flow

```
Request → Proxy → Event Bus → Dispatcher → Backends
                     ↓
              [Lunary, Helicone, File]
```

For detailed event flow documentation, see the [Instrumentation Guide](instrumentation.md).

