---
applyTo: "**/*.go"
---

# Go-specific instructions

Coding standards:
- Idiomatic Go 1.23+. Prefer standard library. Clear names; no 1–2 character identifiers.
- Use guard clauses; handle errors explicitly; return wrapped errors with context.
- Avoid deep nesting; keep functions small and composable.
- Do not add TODOs—implement immediately.
- Preserve existing indentation and do not reformat unrelated code.

Testing:
- TDD: write failing tests first. Use table-driven tests in `_test.go`.
- Cover error paths and edge cases. Run with `-race`.
- Maintain coverage ≥ 90% across `./internal/...` (CI-style aggregation command is provided in docs).

Concurrency and safety:
- Avoid global mutable state. Inject dependencies with constructors.
- Ensure thread-safety; keep request handlers non-blocking, especially around event publishing.

Project specifics:
- Reverse proxy: minimal transformation; replace/augment auth headers; support streaming.
- Tokens: withering tokens with expiration, revocation, and rate limiting; project-based access.
- Event bus: async, non-blocking. Publishing must not block request handling.
- Storage: SQLite locally; design abstractions compatible with PostgreSQL in production.

When proposing edits:
- Keep diffs small; include imports; ensure files compile.
- Add tests alongside changes; run `make test` and `make lint`.

 
