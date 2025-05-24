# AGENTS.md

# Contributor & Agent Guide

> **Note on Git & GitHub Management:**
> - **Always use standard `git` and GitHub CLI (`gh`) commands for all routine git and GitHub repository management (branching, committing, pushing, PR creation, merging, etc.).**
> - **The MCP tools should only be used for actions or data that are not easily accessible via standard commands, such as automated retrieval of review comments on a PR, or advanced API queries.**
> - **Do not use the MCP for basic git/GitHub operations that are well-supported by `git` or `gh`.**
> - **Create (or owerwrite) a temporary md file as NEW_PR.md or NEW_ISSUE.md and as for review. Then use this with gh as a body-file argument. After the issue/pr is created, delete the file.

This file provides essential context and rules for both human and AI contributors working in this repository. It is adapted from the core working agreement in CLAUDE.md and is the primary source of truth for agent-driven development.

---

## Repository Structure & Focus
- **Work in the `internal/`, `cmd/`, and root-level config/docs files.**
- **docs/issues/** markdown files are the primary source for project status, progress, and workflow. Each major task or feature is tracked as a self-contained issue doc.
- **PLAN.md** is always up to date and must be referenced for project architecture and objectives.
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
  - Update the relevant issue doc in docs/issues/ with every significant change.
  - Document rationale for changes and workflow updates in the issue doc.
- **Review process:**
  - All review comments must be addressed in code or docs before merging.
  - For performance/architecture feedback, implement the solution immediately (do not defer).
- **CI Monitoring and Enforcement:**
  - After any push to the repository, you **must** use `gh run list` to view the latest GitHub Actions runs for your commit/branch.
  - Once you spotted an active job run, attach to it using `watch`.
  - Use `gh run watch <run-id>` to monitor the status of each CI job (Lint, Build, Test, etc.) until completion.
  - **It is mandatory to wait for all CI jobs to complete and to fix any CI failures before proceeding with further work, review, or merging.**
  - No code is merged unless all CI checks pass for the latest commit.


---

## PR Instructions
- **Title format:** `[<area>] <Short Description>` (e.g., `[proxy] Add streaming support`)
- **Description:**
  - Reference related checklist items in the relevant issue doc and PLAN.md.
  - Summarize what changed, why, and how it was validated.
  - Note any new or updated tests and coverage impact.
  - Note which issue doc is being addressed.
- **Checklist before merging:**
  - [ ] All tests pass (`make test`)
  - [ ] All linters pass (`make lint`)
  - [ ] Coverage is 90%+
  - [ ] Issue doc and PLAN.md are current
  - [ ] No unresolved TODOs or review comments

---

## Agent-Specific Instructions
- **Always use `git` and `gh` for all standard git and GitHub management tasks.**
- **Only use the MCP for actions or data not easily accessible via `git` or `gh`, such as automated retrieval of review comments on a PR, or advanced API queries.**
- Always explore relevant context in the current issue doc, PLAN.md, and CLAUDE.md before making changes.
- Prefer small, reviewable increments and document every step in the relevant issue doc.
- When in doubt, update documentation and ask for clarification in PRs.
- Respect the most nested AGENTS.md if present in subfolders.

---

**For more details, see CLAUDE.md and the root Makefile.** 