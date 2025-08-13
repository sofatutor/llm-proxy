# Event Dispatcher Service (Pluggable CLI)

Status: Completed via [PR #41](https://github.com/sofatutor/llm-proxy/pull/41)

## Summary
Build a dedicated event dispatcher service as a CLI command. The service subscribes to the async event bus and pushes events to a configured backend (e.g., Helicone, Lunary, file, etc). Each backend is implemented as a pluggable service, selectable via a --service option.

This dispatcher is intended to be run as a dedicated process—such as in a sidecar Docker container or as a background service on a virtual server (e.g., EC2)—using the same Docker image as the main server and admin CLI commands. The CLI will support a `--detach` option to run the dispatcher in the background.

## Rationale
- Clean separation of event production and delivery
- Easy to add new backends (Helicone, Lunary, file, etc.) without changing core proxy code
- Enables independent scaling and deployment of dispatchers
- Simplifies deployment: all CLI commands (server, admin, dispatcher) are available in a single Docker image, allowing flexible orchestration (sidecar, separate service, background process, etc.)

## Requirements
- [x] CLI command with --service option (e.g., `event-dispatcher --service helicone`)
- [x] Support for `--detach` flag to run the dispatcher as a background process (for virtual server/EC2 deployment)
- [x] Pluggable backend architecture (Helicone, Lunary, file, etc)
- [x] Robust error handling and retry logic
- [x] Metrics and logging for delivery status
- [x] Configurable concurrency and batching

## Tasks
- [x] Design CLI and service plugin interface
- [x] Implement core dispatcher loop
- [x] Add Helicone, Lunary, and file backends
- [x] Support config/env for backend selection and credentials
- [x] Add metrics, logging, and error handling
- [x] Write tests for each backend
- [x] Document usage and extension

## Acceptance Criteria
- [x] Dispatcher runs as a CLI with pluggable backends
- [x] Reliable, async delivery to selected backend
- [x] Easy to add new backends
- [x] Supports `--detach` for background operation
- [x] Tests and documentation are complete


