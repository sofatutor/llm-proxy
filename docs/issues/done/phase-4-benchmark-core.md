# Benchmark Tool Core

## Summary
Design and implement the core architecture of the benchmark tool for the LLM proxy. This includes CLI flag parsing, concurrent request handling, and initial request generators. This issue can be worked on in parallel with other benchmark and performance issues.

## Rationale
- Benchmarking is essential for measuring and optimizing proxy performance and latency.
- A flexible CLI and concurrent request handling are required for realistic load testing.

## Tasks
- [x] Design the architecture of the benchmark tool
- [x] Implement CLI with flag parsing (target URL, endpoint, token, request count, concurrency, etc.)
- [x] Add concurrent request handling (worker pool, request generation)
- [x] Implement initial request generators for supported endpoints
- [x] Document benchmark tool usage and architecture
- [x] Add tests for CLI and concurrency logic

## Acceptance Criteria
- Benchmark tool core is implemented and documented
- CLI supports all required flags and options
- Concurrent request handling is robust and tested
- Documentation and tests are updated accordingly 