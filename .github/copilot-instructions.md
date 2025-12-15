# Copilot Repository Instructions

These instructions tailor GitHub Copilot (Chat, code review, and coding agent) to the llm-proxy repository. They summarize our existing rules and expectations so Copilot can propose diffs and reviews that merge cleanly.

## Project Context
- Language: Go 1.23+
- Focus: Transparent OpenAI proxy with withering tokens, auth, HTTP caching, async instrumentation, and admin API.
- Architecture modules live under `internal/` (e.g., `proxy`, `server`, `token`, `database`).
- Tests must be fast, deterministic, and comprehensive; CI enforces coverage.

## Ground Rules (Must Follow)
- Testing and Quality
  - Write tests first where feasible (TDD preference).
  - CI-style total coverage across `./internal/...` must remain ≥ 90%.
  - All tests must pass locally: `make test` and `make test-coverage-ci`.
  - Run `make lint`; fix all lints. Do not introduce formatting drift.

- Go Best Practices (see repository guidelines)
  - Clear naming; no 1–2 character names. Prefer meaningful identifiers.
  - Guard clauses; shallow control flow; explicit error handling.
  - Add short doc comments for exported symbols; avoid TODOs in code.
  - Keep changes minimal and focused; avoid unrelated refactors.

- HTTP Caching & Metrics
  - Cache key generation must honor upstream `Vary` headers.
  - Use helper functions to avoid duplicated logic (e.g., `isVaryCompatible`, `storageKeyForResponse`).
  - Cache metrics are provider-agnostic (hits, misses, bypass, stores) and exposed via the JSON metrics endpoint.

- Provider-Agnostic Metrics Wording
  - Documentation and config must refer to a lightweight, provider-agnostic metrics endpoint. Prometheus scraping/export is optional.

- Security & Admin
  - Management endpoints require `MANAGEMENT_TOKEN`.
  - Do not log secrets. Obfuscate tokens in logs.

## What “Good” Contributions Look Like
- Include or update unit tests alongside code changes.
- Maintain or increase coverage (≥ 90%). If coverage would drop, add tests.
- Keep diffs small; update only the relevant modules and tests.
- When touching complex branches, prefer table-driven tests and explicit edge cases (errors, timeouts, auth, cache miss/hit/bypass/conditional-hit).
- For PR descriptions, include: Summary, Changes, Testing (with coverage impact), and any docs updates.

## Copilot Code Review Guidance
When reviewing PRs, Copilot should:
- Verify tests exist for new logic and that coverage likely remains ≥ 90%.
- Flag duplicated logic—suggest extracting helpers (e.g., Vary handling, storage key selection).
- Call out shadowed variables and potential confusion (rename or reuse).
- Prefer provider-agnostic terminology in docs/config/comments.
- Ensure handlers set appropriate headers and avoid leaking secrets.
- Check concurrency safety (mutexes, race conditions) and HTTP caching semantics.

## Commands and Tools (reference)
- Lint: `make lint`
- Tests (unit + coverage): `make test-coverage-ci`
- Format: `make fmt` or `gofmt -w -s .`

## Mandatory Pre-commit Checks
- Build: `make build` must succeed locally
- Lint: `make lint` must return 0
- Tests: `make test` must pass and `make test-coverage-ci` must show ≥ 90% total coverage
- Formatting: `gofmt -l .` must print nothing (run `make fmt` if needed)

Copilot should treat these as required before suggesting a commit is ready and call out any missing step in reviews.

## Documentation Updates
- Update `docs/` and issue docs under `docs/issues/` when behavior changes.
- Keep `PLAN.md` consistent with notable architectural changes.

## PR Discipline
- Do not merge with failing tests, lint errors, or reduced coverage.
- No unresolved review comments or TODOs.
- Minimal diffs, focused scope, and clear rationale.

---

Reference: GitHub Copilot custom instructions for repositories: https://docs.github.com/en/copilot/customizing-copilot/adding-repository-custom-instructions-for-github-copilot

