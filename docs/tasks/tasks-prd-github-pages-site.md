## Relevant Files

- `docs/index.md` - Homepage content with badges and CTA.
- `docs/features.md` - Feature highlights and links to deeper docs.
- `docs/screenshots.md` - Gallery using images from `docs/assets/screenshots/`.
- `docs/quickstart.md` - Minimal setup, CLI usage, example requests.
- `docs/contributing.md` - Contributor guide summary linking to `CONTRIBUTING.md` and issues.
- `docs/coverage/index.html` - HTML coverage report published by CI.
- `docs/_config.yml` - Optional Jekyll configuration (title, theme, plugins).
- `docs/assets/screenshots/*.png` - Admin UI screenshots used on the site.
- `.github/workflows/coverage-pages.yml` - CI to generate HTML coverage and publish into `docs/coverage/` on `main`.
- `README.md` - Add repo badges; link to the GitHub Pages site.
- `docs/tasks/prd-github-pages-site.md` - The PRD that drives this work.
- `docs/README.md` - Documentation index; add link to site pages.
- `PLAN.md` - Note site/coverage additions at high level.

### Notes

- Go unit tests live alongside code as `*_test.go` in the same package.
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

- [ ] 1.0 Scaffold GitHub Pages structure and navigation
  - [ ] 1.1 Create `docs/_config.yml` with site metadata (title, description) and simple theme (optional: `minima`)
  - [ ] 1.2 Create page stubs: `docs/index.md`, `docs/features.md`, `docs/screenshots.md`, `docs/quickstart.md`, `docs/contributing.md`
  - [ ] 1.3 Add a simple cross-page navbar section to each page header (Markdown links) matching nav: Home, Features, Screenshots, Quickstart, Architecture, Contributing, Coverage, Roadmap
  - [ ] 1.4 Link to existing architecture doc: add prominent link to `docs/architecture.md` from Home and Features

- [ ] 2.0 Author core pages content (Home, Features, Screenshots, Quickstart, Contributing)
  - [ ] 2.1 Homepage: concise value prop, feature bullets, contributor CTA, quick links to issues (`good first issue`) and CONTRIBUTING
  - [ ] 2.2 Features: bullets for Transparent Proxy, Withering Tokens, Project-based ACL, Async Events/Dispatcher, Admin UI, Observability; link into deeper docs
  - [ ] 2.3 Screenshots: embed images from `docs/assets/screenshots/` with alt text and captions (Login, Dashboard, Projects, Project Create, Project Show, Tokens, Token Created, Audit, Audit Detail)
  - [ ] 2.4 Quickstart: minimal steps to run `llm-proxy server` and `llm-proxy admin`, sample curl and `openai chat` through proxy, links to `docs/cli-reference.md`
  - [ ] 2.5 Contributing: summarize from `CONTRIBUTING.md` and `AGENTS.md` (tests, lint, ≥90% coverage), link to roadmap `PLAN.md` and issues

- [ ] 3.0 Integrate and optimize screenshots assets (`docs/assets/screenshots/`)
  - [ ] 3.1 Ensure all current images exist with canonical names: `login.png`, `dashboard.png`, `projects.png`, `project-new.png`, `project-new-filled.png`, `project-show.png`, `tokens.png`, `token-new.png`, `token-created.png`, `audit.png`, `audit-show.png`
  - [ ] 3.2 Add descriptive alt text and titles for accessibility
  - [ ] 3.3 Optimize image sizes (target ≤ 200 KB each) without visible quality loss

- [ ] 4.0 Implement CI job to generate and publish HTML coverage to `docs/coverage/` on `main`
  - [ ] 4.1 Create `.github/workflows/coverage-pages.yml` triggered on push to `main`
  - [ ] 4.2 Run CI-style coverage aggregation:
        `go test -v -race -parallel=4 -coverprofile=coverage_ci.txt -covermode=atomic -coverpkg=./internal/... ./...`
  - [ ] 4.3 Generate HTML report: `mkdir -p docs/coverage && go tool cover -html=coverage_ci.txt -o docs/coverage/index.html`
  - [ ] 4.4 Commit back to `main` with a distinct message tag (e.g., `chore(coverage): update [skip ci][coverage]`)
  - [ ] 4.5 Prevent loops: configure workflow conditionals and/or `paths-ignore` in other workflows to ignore `docs/coverage/**` or `[coverage]` commits
  - [ ] 4.6 Verify `https://sofatutor.github.io/llm-proxy/coverage/` serves the generated report

- [ ] 5.0 Add build/test/coverage badges and contributor CTAs (homepage and `README.md`)
  - [ ] 5.1 Add GitHub Actions build status badge for tests to `README.md` and `docs/index.md`
  - [ ] 5.2 Add a coverage badge (generated in workflow or static SVG updated from coverage value) and link it to `/coverage/`
  - [ ] 5.3 Add “Contributors welcome” banner/CTA on Home linking to `CONTRIBUTING.md` and `issues?q=is:issue+is:open+label:good%20first%20issue`

- [ ] 6.0 Configure and verify GitHub Pages deployment (source: `main`/`docs/`); validate links and responsiveness
  - [ ] 6.1 Enable GitHub Pages (Settings → Pages → Deploy from a branch → `main` / `/docs`)
  - [ ] 6.2 Verify all nav links resolve; fix relative paths for images and coverage sub‑page
  - [ ] 6.3 Smoke-check on mobile viewport widths; ensure basic responsiveness

- [ ] 7.0 Update documentation (docs index, PLAN) and ensure quality gates (tests, lint, coverage) stay green
  - [ ] 7.1 Update `docs/README.md` with links to the new pages and coverage
  - [ ] 7.2 Update `PLAN.md` with a short note about public site and coverage publishing
  - [ ] 7.3 Keep repo quality gates: run `make lint`, `make test`; confirm CI coverage ≥ 90%
  - [ ] 7.4 Cross-link `docs/tasks/prd-github-pages-site.md` from the new tasks file and vice versa


I have generated the high-level tasks based on the PRD. Ready to generate the sub-tasks? Respond with 'Go' to proceed.


