# Logging Package

This package implements structured logging for the application:

- JSON formatted logs
- Log levels and filtering
- Request/response logging
- Audit logging and security events
- External logging integration with an asynchronous worker

## Configuration Options

The external logging integration supports the following configuration options:

- **Buffer Size**: The maximum number of log entries that can be stored in the buffer before being sent. Default: `100`.
- **Batch Size**: The number of log entries sent in a single batch to the external logging system. Default: `10`.
- **Retry Policy (maxAttempts)**: The maximum number of attempts to send a batch before falling back to local logging. Default: `3`.
- **Retry Interval**: The interval between retry attempts. Default: `5s`.
- **Fallback to Local Logging**: If enabled, failed batches after all attempts are sent to the local logger.
