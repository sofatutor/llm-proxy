# Logging System Core

## Summary
Implement the core local logging system for the LLM proxy. This includes researching best practices, defining a comprehensive log format, implementing JSON Lines local logging, and setting up log rotation and configuration. This issue is foundational for observability and compliance, and can be worked on in parallel with other logging/monitoring enhancements.

## Rationale
- Local logging is required for compliance, debugging, and as a fallback for all observability features.
- JSON Lines format enables easy parsing and integration with external tools.
- Log rotation and configuration are necessary for production readiness.

## Tasks
- [ ] Research logging best practices for Go applications
- [ ] Define a comprehensive log format (fields: timestamp, level, message, endpoint, method, status, duration, token counts, errors, etc.)
- [ ] Implement JSON Lines local logging
- [ ] Set up log file creation and rotation
- [ ] Add configuration options for log file location, rotation policy, and log levels
- [ ] Add basic documentation for the logging system
- [ ] Add unit tests for logging functionality

## Acceptance Criteria
- Local logging is implemented in JSON Lines format
- Log files are created and rotated according to configuration
- Log format includes all required fields
- Logging system is covered by unit tests
- Documentation is updated to describe logging configuration and usage 