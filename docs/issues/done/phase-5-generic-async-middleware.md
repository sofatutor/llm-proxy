# Generic Instrumentation Middleware (Streaming-Ready)

Status: Completed via [PR #41](https://github.com/sofatutor/llm-proxy/pull/41)

## Summary
Implement a generic, asynchronous instrumentation middleware for the LLM proxy. The middleware supports both standard and streaming HTTP responses, capturing and forwarding events without blocking the main request/response path.

## Rationale
- Decouples instrumentation from business logic and proxy latency.
- Supports both streaming and non-streaming LLM APIs.
- Enables flexible downstream event handling (file, bus, cloud, etc.).

## Requirements
- [x] Capture all relevant request/response metadata, including full streamed responses (buffered asynchronously).
- [x] Non-blocking: never delay the client response.
- [x] Pluggable: can be enabled/disabled via config.
- [x] Extensible: supports custom event schemas.

## Tasks
- [x] Design middleware interface and event schema
- [x] Implement async buffering for streaming responses
- [x] Integrate with event bus for downstream dispatch
- [x] Add configuration for enable/disable
- [x] Write tests for streaming and non-streaming cases
- [x] Document usage and extension points

## Acceptance Criteria
- [x] Middleware is fully async and streaming-capable
- [x] No impact on proxy latency
- [x] All events are captured and forwarded to the event bus
- [x] Tests and documentation are complete
