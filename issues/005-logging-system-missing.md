# Logging system not implemented

PLAN.md specifies a logging system that records API calls locally and to external backends:
```
- **Logging**: Record API calls with metadata to local files and async backends
```
【F:PLAN.md†L24-L30】

The `internal/logging` package only contains a README with no actual code. Implement structured logging, file output and optional external backend support as described in the plan.
