## PRD: Full Inventory & Review of Codebase (every 100 PRs)

### Introduction/Overview

This PRD defines the establishment of a manual "Full Inventory & Review of the Codebase" process to create an initial baseline report and framework for future governance. The process aims to prevent architectural drift, identify and remove dead/unused code, and ensure documentation, tests, and coverage remain aligned with the codebase.

This aligns with the architecture in `docs/architecture.md` and the repo's rules in `AGENTS.md` (audit-first, transparent behavior, 90%+ coverage, no TODOs). The initial implementation provides a manual checklist and reporting template that can be automated or made recurring in the future if desired.

### Goals

- Establish a comprehensive baseline review of the entire codebase to prevent architectural drift
- Create standardized templates and processes for systematic code inventory and quality assessment
- Ensure documentation alignment across `docs/**`, `PLAN.md`, `WIP.md`, and `docs/issues/*`
- Provide explicit acceptance gates for coverage, testing, and code quality metrics
- Enable systematic identification of dead code, unused dependencies, and security vulnerabilities
- Create maintainer sign-off workflows for governance and accountability

### User Stories

- As a maintainer, I can run a comprehensive codebase review using a standardized checklist to identify technical debt and architectural issues
- As a team lead, I can ensure all packages (`internal/*`, `cmd/*`) are reviewed systematically for code quality and alignment
- As a security reviewer, I can follow documented procedures to scan for secrets, review access controls, and audit dependencies
- As a project owner, I can produce baseline reports with clear acceptance criteria and follow-up actions
- As a contributor, I can understand the project's quality standards and governance process through clear documentation

### Functional Requirements

1) **Review Template Structure**
   - Create `docs/reviews/review-PRs<start>-to<end>.md` template with comprehensive checklist sections
   - Include package-by-package review sections for all `internal/*` and `cmd/*` components
   - Provide documentation alignment verification for all major doc files
   - Include explicit quality gate checkboxes for `make test`, `make lint`, and coverage ≥ 90%

2) **Package Inventory Checklist**
   - Systematic review prompts for each package: `internal/proxy`, `internal/token`, `internal/server`, `internal/admin`, `internal/eventbus`, `internal/dispatcher`, `internal/eventtransformer`, `internal/database`, `internal/logging`, `internal/obfuscate`, `internal/audit`, `internal/utils`
   - Dead code detection prompts with clear criteria and removal guidance
   - Architectural drift assessment against `docs/architecture.md`
   - Unused dependency identification and cleanup recommendations

3) **Quality Gates and Coverage Verification**
   - CI-style coverage command documentation: `go test -v -race -parallel=4 -coverprofile=coverage_ci.txt -covermode=atomic -coverpkg=./internal/... ./...`
   - Coverage summary extraction: `go tool cover -func=coverage_ci.txt | tail -n 1`
   - Test execution verification: `make test` and `make lint` pass/fail recording
   - Low-coverage file identification and remediation guidance

4) **Security and Compliance Review**
   - Secret scanning procedures and config drift detection
   - Access control review for `MANAGEMENT_TOKEN` paths and admin endpoints
   - Go module dependency audit with license compliance checks
   - Security vulnerability assessment and remediation tracking

5) **Governance and Sign-off Workflow**
   - Maintainer lead approval requirement in review reports
   - Follow-up issue creation guidance with proper linking to review reports
   - Non-blocking process clarification (merges not halted by review status)
   - Clear escalation paths for critical findings

### Non-Functional Requirements

1) **Documentation Quality**
   - All templates must include clear instructions and examples
   - Cross-references to existing documentation (`docs/architecture.md`, `docs/security.md`)
   - Integration with repository documentation structure

2) **Process Efficiency**
   - Manual checklist approach for initial implementation
   - Clear time estimates and effort guidelines for reviewers
   - Standardized report format for consistency across reviews

3) **Maintainability**
   - Template structure designed for future automation
   - Clear versioning and update procedures for the review process
   - Integration points identified for future tooling

### Success Criteria

- Ready-to-use template exists at `docs/reviews/review-PRs<start>-to<end>.md` with comprehensive checklist
- All internal packages and cmd directories included in review checklist
- Quality gates clearly defined: `make test` + `make lint` green, CI-style coverage ≥ 90%
- Security and compliance review procedures documented and actionable
- Maintainer sign-off workflow established with clear responsibilities
- Repository documentation updated to reference the inventory process

### Out of Scope

- Automated large refactors or code rewriting during reviews
- Database schema migrations or runtime behavior changes
- External SaaS integrations or tooling implementations
- Recurring cadence or scheduling automation (initial manual process only)
- Changes to reverse proxy runtime behavior or performance characteristics

### Implementation Notes

- This is a governance and documentation task with no runtime code changes
- Templates should be comprehensive but not overly prescriptive
- Process should integrate with existing repository workflows and conventions
- Future automation opportunities should be identified but not implemented initially