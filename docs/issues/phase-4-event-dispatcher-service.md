# Event Dispatcher Service (Pluggable CLI)

## Summary
Build a dedicated event dispatcher service as a CLI command. The service subscribes to the async event bus and pushes events to a configured backend (e.g., Helicone, Lunary, file, etc). Each backend is implemented as a pluggable service, selectable via a --service option.

This dispatcher is intended to be run as a dedicated process—such as in a sidecar Docker container or as a background service on a virtual server (e.g., EC2)—using the same Docker image as the main server and admin CLI commands. The CLI will support a `--detach` option to run the dispatcher in the background.

## Rationale
- Clean separation of event production and delivery
- Easy to add new backends (Helicone, Lunary, file, etc.) without changing core proxy code
- Enables independent scaling and deployment of dispatchers
- Simplifies deployment: all CLI commands (server, admin, dispatcher) are available in a single Docker image, allowing flexible orchestration (sidecar, separate service, background process, etc.)

## Requirements
- CLI command with --service option (e.g., `event-dispatcher --service helicone`)
- Support for `--detach` flag to run the dispatcher as a background process (for virtual server/EC2 deployment)
- Pluggable backend architecture (Helicone, Lunary, file, etc)
- Robust error handling and retry logic
- Metrics and logging for delivery status
- Configurable concurrency and batching

## Tasks
- [ ] Design CLI and service plugin interface
- [ ] Implement core dispatcher loop
- [ ] Add Helicone, Lunary, and file backends
- [ ] Support config/env for backend selection and credentials
- [ ] Add metrics, logging, and error handling
- [ ] Write tests for each backend
- [ ] Document usage and extension

## Acceptance Criteria
- Dispatcher runs as a CLI with pluggable backends
- Reliable, async delivery to selected backend
- Easy to add new backends
- Supports `--detach` for background operation
- Tests and documentation are complete 