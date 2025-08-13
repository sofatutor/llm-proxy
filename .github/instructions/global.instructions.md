---
applyTo: "**/*"
---

# Repository-wide instructions

Project: Go-based LLM Proxy (transparent OpenAI-compatible reverse proxy) with token management, rate limiting, async events, and admin tooling.

Response style:
- Be concise and high-signal. Use headings and bullet points. Use backticks for file, directory, function, and class names.
- Provide code in fenced blocks; include only the relevant, minimal edits and necessary imports.

Quality gates:
- TDD first. Add tests before or with implementation. Maintain ≥ 90% coverage.
- All tests must pass (`make test`), including `-race`.
- Linters must be clean (`make lint`). Do not reformat unrelated code; preserve file indentation style.

Agentic prompting controls:
- Prefer minimal exploration unless the task requires deeper analysis.
- Use one of the following per task:

```
<context_gathering>
Goal: Get enough context fast. Parallelize discovery and stop as soon as you can act.
Method: start broad → focused subqueries; run one parallel batch; deduplicate.
Early stop: can name exact content to change and top hits converge (~70%).
Loop: batch search → minimal plan → complete task; re-search only if validation fails.
</context_gathering>
```

```
<persistence>
- Keep going until the task is fully resolved; proceed under reasonable assumptions and document them.
- Only hand back when the problem is solved.
</persistence>
```

Tool preambles:
- Rephrase the goal in one sentence, outline a short plan, give brief progress notes while executing, and end with a short summary of changes and validation status.

Stop conditions (success):
- Tests green, coverage ≥ 90%, linters clean, no unrelated formatting, minimal focused edits.

Safe vs risky actions:
- Safe: searches, reading files, small edits, table-driven tests, missing imports, non-breaking refactors.
- Risky: deleting/renaming files or packages, changing public APIs, adding heavy dependencies, altering DB schemas. Do only with strong rationale and tests.

Execution checklist:
1) Brief plan and assumptions; identify target files/functions.
2) Minimal edits; preserve indentation and existing formatting.
3) Add/adjust tests (table-driven) and implement.
4) Validate: tests, coverage, lint, formatting.
5) Summarize changes and impact.

 
