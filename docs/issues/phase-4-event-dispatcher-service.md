# Event Dispatcher Service (Pluggable CLI)

## Summary
Build a dedicated event dispatcher service as a CLI command. The service subscribes to the async event bus and pushes events to a configured backend (e.g., Helicone, AWS CloudWatch, file, etc). Each backend is implemented as a pluggable service, selectable via a --service option.

## Rationale
- Clean separation of event production and delivery
- Easy to add new backends without changing core proxy code
- Enables independent scaling and deployment of dispatchers

## Requirements
- CLI command with --service option (e.g., `event-dispatcher --service helicone`)
- Pluggable backend architecture (Helicone, CloudWatch, file, etc)
- Robust error handling and retry logic
- Metrics and logging for delivery status
- Configurable concurrency and batching

## Tasks
- [ ] Design CLI and service plugin interface
- [ ] Implement core dispatcher loop
- [ ] Add Helicone, Lunary, CloudWatch, and file backends
- [ ] Support config/env for backend selection and credentials
- [ ] Add metrics, logging, and error handling
- [ ] Write tests for each backend
- [ ] Document usage and extension

## Acceptance Criteria
- Dispatcher runs as a CLI with pluggable backends
- Reliable, async delivery to selected backend
- Easy to add new backends
- Tests and documentation are complete 