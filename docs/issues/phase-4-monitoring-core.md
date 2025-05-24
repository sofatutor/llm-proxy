# Monitoring Core

## Summary
Implement the core monitoring features for the LLM proxy. This includes health check endpoints, readiness and liveness probes, and basic system metrics. This issue is foundational for operational visibility and can be worked on in parallel with other logging and monitoring enhancements.

## Rationale
- Health checks and probes are required for orchestration, deployment, and uptime monitoring.
- Basic system metrics provide insight into the health and performance of the proxy.

## Tasks
- [ ] Implement a /health endpoint for health checks
- [ ] Add readiness and liveness probes for deployment environments (e.g., Kubernetes)
- [ ] Implement basic system metrics (e.g., uptime, request counts, error rates)
- [ ] Document health check and monitoring endpoints
- [ ] Add tests for health checks and probes

## Acceptance Criteria
- /health endpoint is available and returns status, timestamp, and version
- Readiness and liveness probes are implemented and documented
- Basic system metrics are available and tested
- Documentation and tests are updated accordingly 