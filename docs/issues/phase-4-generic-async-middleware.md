# Generic Asynchronous Observability Middleware (Streaming-Ready)

## Summary
Implement a generic, asynchronous observability middleware for the LLM proxy. The middleware must support both standard and streaming HTTP responses, capturing and forwarding events without blocking the main request/response path.

## Rationale
- Decouples observability from business logic and proxy latency.
- Supports both streaming and non-streaming LLM APIs.
- Enables flexible downstream event handling (file, bus, cloud, etc.).

## Requirements
- Capture all relevant request/response metadata, including full streamed responses (buffered asynchronously).
- Non-blocking: never delay the client response.
- Pluggable: can be enabled/disabled via config.
- Extensible: supports custom event schemas.

## Tasks
- [x] Design middleware interface and event schema
- [x] Implement async buffering for streaming responses
- [x] Integrate with event bus for downstream dispatch
- [x] Add configuration for enable/disable
- [x] Write tests for streaming and non-streaming cases
- [x] Document usage and extension points

## Acceptance Criteria
- Middleware is fully async and streaming-capable
- No impact on proxy latency
- All events are captured and forwarded to the event bus
- Tests and documentation are complete 