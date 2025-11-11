## Relevant Files

- `docs/tasks/prd-full-inventory-review.md` - The PRD driving this work.
- `docs/reviews/` - Location for per-window review reports `review-PRs-<start>-to-<end>.md`.
- `docs/architecture.md` - Cross-check design vs implementation; update if discrepancies found.
- `docs/security.md` - Security practices reference for access controls and secrets.
- `docs/cli-reference.md` - May reference how maintainers run checks locally.
- `PLAN.md` - Ensure plan aligns with current state; update if needed.
- `WIP.md` - Capture process adoption and notes after the first run.

### Notes

- Go unit tests live alongside code as `*_test.go`.
- Prefer targeted test runs during iteration.

Use the following commands to run tests:

#### Unit tests (fast)
- `make test`
- Equivalent (CI-style coverage aggregation):
  - `go test -v -race -parallel=4 -coverprofile=coverage_ci.txt -covermode=atomic -coverpkg=./internal/... ./...`
  - View summary: `go tool cover -func=coverage_ci.txt | tail -n 1`

#### Targeted tests
- By package: `go test ./internal/token -v -race`
- Single test regex: `go test ./internal/token -v -race -run TestName`

#### Integration tests
- `go test -v -race -parallel=4 -tags=integration -timeout=5m -run Integration ./...`

## Tasks

- [ ] 1.0 Create inventory report template and folder structure under `docs/reviews/`
  - [ ] 1.1 Add `docs/reviews/` directory if missing
  - [ ] 1.2 Create a seed file `docs/reviews/review-PRs-<start>-to-<end>.md` using the PRD’s checklist template
  - [ ] 1.3 Include instructions at the top of the template on how to fill it in

- [ ] 2.0 Define and document the manual review checklist (packages, docs, security/compliance)
  - [ ] 2.1 Expand checklist sections to enumerate packages: `internal/proxy`, `internal/token`, `internal/server`, `internal/admin`, `internal/eventbus`, `internal/dispatcher`, `internal/eventtransformer`, `internal/database`, `internal/logging`, `internal/obfuscate`, `internal/audit`, `cmd/*`, docs
  - [ ] 2.2 Include prompts for dead code, architectural drift, and unused dependencies
  - [ ] 2.3 Add doc alignment checks for `docs/**`, `PLAN.md`, `WIP.md`, `docs/issues/*`

- [ ] 3.0 Define coverage and quality gate verification steps for the review
  - [ ] 3.1 Add explicit commands for CI-style coverage; record the coverage value in the report
  - [ ] 3.2 Record `make test` and `make lint` results (pass/fail) in the report
  - [ ] 3.3 Add guidance on addressing coverage dips (identify low-covered files and add tests)

- [ ] 4.0 Document security/compliance review procedures (secrets, access controls, deps/licenses)
  - [ ] 4.1 Add steps for secret scanning and config drift checks
  - [ ] 4.2 Add steps to review access control (MANAGEMENT_TOKEN paths, admin endpoints)
  - [ ] 4.3 Add steps to review Go module dependencies and licenses; document any findings

- [ ] 5.0 Define ownership and sign‑off workflow for maintainers
  - [ ] 5.1 Specify maintainer lead as approver in the report’s sign‑off section
  - [ ] 5.2 Add guidance on creating follow-up issues for failures/findings (manual, with links in report)
  - [ ] 5.3 Clarify that the inventory process is non‑blocking; merges aren’t halted by the report

- [ ] 6.0 Update repository docs to reference the inventory process and template
  - [ ] 6.1 Update `README.md` (short note) or `docs/README.md` to link the process and template
  - [ ] 6.2 Update `CONTRIBUTING.md` only if needed (remove cadence/labeling references)
  - [ ] 6.3 Cross-link from `docs/architecture.md` and `docs/security.md` where relevant
  - [ ] 6.4 Ensure `PLAN.md` references this initial inventory in project governance


