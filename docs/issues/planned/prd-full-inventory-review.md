# PRD: Initial Full Inventory & Review of the Codebase (One-off)

## 1. Introduction / Overview
This document defines a one-off, lightweight governance process to perform a comprehensive initial inventory and review of the LLM Proxy codebase. The goal is to proactively prevent architectural drift, remove dead code, and keep documentation, tests, and coverage aligned with the current implementation. This PRD focuses on maintainers and aligns with repository practices described in `AGENTS.md`, `README.md`, and the documentation under `docs/`.

This effort is a process and documentation deliverable with optional supporting automation (CLI + CI scaffolding) that produce a single Markdown report committed to the repository. No runtime behavior changes are in scope.

## 2. Goals
- Identify and mitigate architectural drift and dead/unused code across primary packages.
- Ensure docs, tests, and coverage remain consistent with implementation.
- Provide a clear, repeatable checklist and a Markdown report artifact for each review cycle.
- Minimize maintainer time through a guided workflow and optional automation entry points.

## 3. Target Users / Personas
- Maintainers (core developers responsible for code quality, docs, and release readiness).

## 4. User Stories
1. As a maintainer, I can run a documented checklist that verifies code health, docs alignment, tests, and coverage, producing a Markdown report for the repository.
2. As a maintainer, I can optionally trigger a CLI command to generate a templated review report, then fill in human findings.
3. As a maintainer, I can view a CI job (non-blocking) that confirms the presence/format of the report when the 100-PR cadence is reached and points reviewers to the result.
4. As a maintainer, I can quickly identify action items without changing runtime behavior or introducing heavy dependencies.

## 5. Functional Requirements
1) Execution
   - This is a one-off initial inventory to establish a baseline report for the repository.
   - The inventory can be repeated ad-hoc in the future as needed, but no cadence is defined here.

2) Scope of Review
   - Packages to review include (not exhaustive):
     - `internal/proxy` (routing, middleware)
     - `internal/token` (generation, validation, expiration, rate limiting)
     - Management API and admin UI: `internal/server`, `internal/admin`, `web/templates/*`
     - Eventing: `internal/eventbus`, `internal/dispatcher`, `internal/eventtransformer`
     - Database: `internal/database` (schema, usage patterns)
     - Logging/obfuscation/security: `internal/logging`, `internal/obfuscate`, `internal/audit`
   - CLI and entrypoints: `cmd/*`, `docs/guides/cli-reference.md`
   - Documentation: `docs/**`, `PLAN.md`, `docs/issues/archive/WIP.md`, issue docs under `docs/issues/`

3) Deliverable Artifact
   - A Markdown report committed under `docs/reviews/` with filename pattern: `review-PRs-<start>-to-<end>.md` (e.g., `review-PRs-201-to-300.md`).
   - The report contains a structured checklist (see Section 6) with pass/fail/notes and a short list of follow-ups (links to issues if created manually by maintainers).
   - No persistence or history beyond the Markdown files is required.

4) Workflow & Surfaces
   - Documentation-first checklist is the source of truth.
   - No automation is required for this initial inventory; maintainers generate and complete the report manually.

5) Security & Compliance Checks (in-scope for review, manual or scripted as feasible)
   - Secret scanning in repository history and config files as part of the checklist.
   - Access control review (e.g., `MANAGEMENT_TOKEN` usage paths, token scopes, admin endpoints protections) against `docs/deployment/security.md` and `AGENTS.md`.
   - Dependency and license review (Go modules) with notes on any flagged items.

6) Acceptance Gates
   - `make test` and `make lint` must be green at the time of the review.
   - CI-style coverage (≥ 90%) verified via:
     - `go test -v -race -parallel=4 -coverprofile=coverage_ci.txt -covermode=atomic -coverpkg=./internal/... ./...`
     - `go tool cover -func=coverage_ci.txt | tail -n 1`
   - The report explicitly lists pass/fail for key checks and highlights follow-ups. The process does not block merges by itself.

## 6. Non-Goals (Out of Scope)
- No automated large refactors or code rewriting.
- No database schema migrations.
- No changes to reverse proxy runtime behavior.
- No external SaaS integrations (e.g., uploading reports, third-party dashboards).

## 7. Design Considerations (References)
- See `docs/architecture/index.md` for system design and components.
- See `docs/observability/instrumentation.md` for event/dispatcher design (note: events are not emitted by this process).
- See `docs/guides/api-configuration.md` and `docs/deployment/security.md` for configuration and security practices.
- See `PLAN.md`, `docs/issues/archive/WIP.md`, and `docs/issues/*` for project plan and active tasks.

## 8. Technical Considerations
- Language/Env: Go 1.23+.
- Tooling: Reuse repository Make targets (`make test`, `make lint`, coverage commands) for measurements.
- CLI helper (if implemented) must avoid adding heavy dependencies; generate a Markdown skeleton and run existing repo commands.
- CI job (if implemented) should be lightweight and non-blocking; it only checks presence/format of the review artifact and optionally comments with a link.

## 9. Success Metrics
- A new review Markdown report exists in `docs/reviews/` for each 100-PR window.
- Tests and linters green at review time; CI-style coverage ≥ 90%.
- Clear, actionable follow-ups are documented (with manual links to issues when maintainers choose to create them).
- Time-to-complete for maintainers kept minimal (target ≤ 60 minutes per cycle).

## 10. Open Questions
1. Should we introduce light automation later (CLI or CI) to assist future ad-hoc inventories?
2. Do we want a standardized issue template for follow-ups to ensure consistent tagging and tracking?

## 11. Checklist Template (to be included in each report)
Copy this into `docs/reviews/review-PRs-<start>-to-<end>.md` and fill it out.

```markdown
# Codebase Inventory Review: PRs <start>–<end>

Date: <YYYY-MM-DD>
Reviewed by: <Maintainer>


## Summary
- Overall status: <pass|needs-attention>
- Key findings:
  - <bullet 1>
  - <bullet 2>

## Global Health
- Tests green (`make test`): <pass/fail>
- Linters green (`make lint`): <pass/fail>
- CI-style coverage ≥ 90%: <pass/fail> (Value: <xx.xx%>)

## Package Reviews
- internal/proxy: <notes>
- internal/token: <notes>
- internal/server & internal/admin (incl. web/templates): <notes>
- internal/eventbus, internal/dispatcher, internal/eventtransformer: <notes>
- internal/database: <notes>
- internal/logging, internal/obfuscate, internal/audit: <notes>
- cmd/* and docs/guides/cli-reference.md: <notes>
- Documentation (docs/**, PLAN.md, docs/issues/archive/WIP.md, docs/issues/*): <notes>

## Security & Compliance
- Secret scanning: <pass/fail> (notes)
- Access controls (MANAGEMENT_TOKEN, scopes, admin endpoints): <pass/fail> (notes)
- Dependencies/licenses: <pass/fail> (notes)

## Follow-ups
- Action items:
  - [ ] <item> (link to issue if created)
  - [ ] <item>

## Sign-off
- Maintainer lead approval: <name> (approved/date)
```

## 12. Ownership & Sign-off
- Maintainer lead owns the process and approves each review report.
- CI/automation (if present) is supportive and non-blocking.


