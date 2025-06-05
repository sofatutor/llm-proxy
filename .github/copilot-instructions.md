# GitHub Copilot Agent Working Agreement

## 1. Planning & Sequential Thinking
- **Thorough Planning Required:**
  - Reference `PLAN.md` for architecture, objectives, and implementation steps before starting any work.
  - Break down tasks into clear, sequential steps. Use issue docs in `docs/issues/` to track progress and dependencies.
  - Always update `WIP.md` and relevant issue docs with status, rationale, and workflow changes.
  - Make use of the sequential thinking tool in order to verify your plans against the original request, before starting to write any line of code!

## 2. Test-Driven Development (TDD)
- **TDD is Mandatory:**
  - Every feature or fix begins with a failing test.
  - Only implement code after the test is in place.
  - No code is merged unless it is covered by tests.
  - Maintain 90%+ code coverage at all times (enforced by CI).

## 3. Review, Feedback, and Best Practices
- **Review Process:**
  - Address all review comments directly in code or docs—no TODOs left unresolved.
  - Implement performance or architectural feedback immediately.
  - All changes must pass tests (including `-race` for concurrency) and linters before merging.
- **Coding Standards:**
  - Follow Go best practices: idiomatic naming, clear error handling, and documentation for all exported types/functions.
  - Keep code DRY, simple, and maintainable.

## 4. Transparency & Traceability
- **Document Every Step:**
  - Reflect all significant changes, fixes, and workflow updates in `WIP.md` and issue docs.
  - Ensure all work is traceable and managed in small, reviewable increments.

## 5. Agent-Specific Instructions
- Use `git` and `gh` for all standard git and GitHub management tasks.
- Only use advanced automation for actions or data not easily accessible via standard tools (e.g., automated review comment retrieval).
- When in doubt, update documentation and ask for clarification in PRs.

---

**Always follow this agreement for every PR and workflow step.** 