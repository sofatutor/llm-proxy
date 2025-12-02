# Async Event Bus (Redis/In-Memory Backend)

Status: Completed via [PR #41](https://github.com/sofatutor/llm-proxy/pull/41)

## Summary
Develop an asynchronous event bus component to buffer, batch, and deliver instrumentation events from the middleware to one or more downstream consumers. Support both in-memory and Redis backends for flexibility and scalability.

## Rationale
- Decouples event production from event consumption.
- Enables scalable, reliable delivery to multiple dispatchers/services.
- Supports both local development (in-memory) and production (Redis) use cases.

## Requirements
- [x] Support for both in-memory and Redis backends (configurable)
- [x] Thread-safe, non-blocking event ingestion
- [x] Batching and retry logic for delivery
- [x] Subscription model for multiple consumers
- [x] Metrics and health checks

## Tasks
- [x] Design event bus interface and backend abstraction
- [x] Implement in-memory backend
- [x] Implement Redis backend
- [x] Add batching, retry, and health check logic
- [x] Support multiple subscribers/dispatchers
- [x] Write tests for both backends
- [x] Document configuration and usage

## Acceptance Criteria
- [x] Event bus works with both in-memory and Redis
- [x] Supports multiple subscribers
- [x] Reliable, async delivery with batching and retries
- [x] Tests and documentation are complete
