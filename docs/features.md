<!-- Minima handles nav; removing inline nav to avoid duplicate menus. -->

## Features

### Transparent Proxying
Minimal transformation reverse proxy built on `httputil.ReverseProxy`. Requests flow through with header replacement and optional instrumentation.

### Withering Tokens
Short-lived, project-scoped tokens with expiration and rate-limiting support. Rotate often; fits least‑privilege.

### Project-based Access Control
Multi-tenant isolation: each project binds to its own upstream API key and token space.

### Admin Management UI
Web UI for creating projects and generating withering tokens, including audit views and useful UX.

### Async Event System
Non-blocking instrumentation with a dispatcher layer (file, Lunary, Helicone) and transform pipeline for analytics.

### Observability Hooks
Latency headers and structured logs for proxy vs upstream timings; integration-ready metrics.

See [Architecture](./architecture.md) and [API configuration](./api-configuration.md) for details.


