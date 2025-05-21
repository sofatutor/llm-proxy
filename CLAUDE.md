# Working Agreement & Process Documentation

## Core Workflow Rules (as followed in this PR)

### 1. WIP and PLAN as Source of Truth
- **WIP.markdown** is updated immediately with any project progress, task completion, or workflow change.
- **PLAN.markdown** is the central reference for project architecture, objectives, and implementation steps. Any major change in direction or scope is reflected there.
- Before any PR is approved or merged, WIP and PLAN are checked for consistency and completeness.

### 2. Test-Driven Development (TDD) Mandate
- All features and changes begin with a failing unit test.
- Implementation is only written after the test is in place.
- No code is merged unless it is covered by tests.
- Code coverage is enforced at 90%+ at all times.
- CI blocks merges if coverage drops below threshold.

### 3. Review and Feedback Process
- All review comments are addressed directly in code or documentation—no TODOs are left unresolved.
- For performance or architectural feedback (e.g., cache eviction), the solution is implemented immediately, not deferred.
- All changes are validated with tests (including `-race` for concurrency) and linters before considering a PR resolved.

### 4. Transparency and Traceability
- Every significant change, fix, or workflow update is reflected in WIP.markdown.
- The process and rationale for changes are documented for future reference.
- The project is managed in small, reviewable increments, with clear status tracking in WIP.

### 5. Coding Best Practices
- Go best practices are followed: idiomatic style, clear naming, error handling, and documentation.
- Code is kept DRY, simple, and maintainable.
- All exported types and functions are documented.

---

## Example: How This PR Was Handled
- Review comments were fetched and addressed one by one.
- For performance feedback (cache eviction), a min-heap was implemented immediately.
- All changes were tested (`go test -race ./...`) and linted.
- WIP.markdown is updated to reflect the current state and process.

---

## Summary Table
| Rule/Practice                | Enforced in this PR? |
|------------------------------|:--------------------:|
| WIP.markdown always current  |          ✅           |
| PLAN.markdown as reference   |          ✅           |
| TDD-first, 90%+ coverage     |          ✅           |
| All review comments resolved |          ✅           |
| No TODOs left in code        |          ✅           |
| All changes tested/linted    |          ✅           |
| Go best practices            |          ✅           |

---

**Always follow these rules for every PR and workflow step.**