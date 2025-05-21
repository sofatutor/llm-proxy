# AGENTS.md

# Contributor & Agent Guide

This file provides essential context and rules for both human and AI contributors working in this repository. It is adapted from the core working agreement in CLAUDE.md and is the primary source of truth for agent-driven development.

---

## Repository Structure & Focus
- **Work in the `internal/`, `cmd/`, and root-level config/docs files.**
- **WIP.md** and **PLAN.md** are always up to date and must be referenced for project status and architecture.
- **CLAUDE.md** contains the canonical working agreement; this file summarizes and adapts it for agent use.

---

## Development Environment
- Use Go 1.21+ (see `.tool-versions` or Dockerfile for specifics).
- Install dependencies with `make deps` or `go mod tidy`.
- Use `make lint` to run all linters (golangci-lint, gofmt, etc.).
- Use `make test` to run all tests (unit, race, coverage).
- Use `make` to see all available targets.
- CI runs on Ubuntu with the same Makefile commands.

---

## Testing & Validation
- **Test-Driven Development (TDD) is mandatory:**
  - Write a failing test before implementing any feature or fix.
  - All code must be covered by tests (unit, integration, or e2e as appropriate).
  - Code coverage must remain above 90% (enforced in CI).
- Run `make lint` and `make test` before every commit and PR.
- No code is merged unless all tests and linters pass.
- Use table-driven tests and cover edge/error cases.

---

## Contribution & Style Guidelines
- **Go best practices:**
  - Idiomatic naming, clear error handling, and documentation for all exported types/functions.
  - Keep code DRY, simple, and maintainable.
  - No TODOs left unresolved in code or docs.
- **Documentation:**
  - Update `WIP.md` and `PLAN.md` with every significant change.
  - Document rationale for changes and workflow updates.
- **Review process:**
  - All review comments must be addressed in code or docs before merging.
  - For performance/architecture feedback, implement the solution immediately (do not defer).

---

## PR Instructions
- **Title format:** `[<area>] <Short Description>` (e.g., `[proxy] Add streaming support`)
- **Description:**
  - Reference related checklist items in `WIP.md` and `PLAN.md`.
  - Summarize what changed, why, and how it was validated.
  - Note any new or updated tests and coverage impact.
- **Checklist before merging:**
  - [ ] All tests pass (`make test`)
  - [ ] All linters pass (`make lint`)
  - [ ] Coverage is 90%+
  - [ ] WIP.md and PLAN.md are current
  - [ ] No unresolved TODOs or review comments

---

## Agent-Specific Instructions
- Always explore relevant context in `WIP.md`, `PLAN.md`, and `CLAUDE.md` before making changes.
- Prefer small, reviewable increments and document every step in `WIP.md`.
- When in doubt, update documentation and ask for clarification in PRs.
- Respect the most nested AGENTS.md if present in subfolders.

---

**For more details, see CLAUDE.md and the root Makefile.** 