## Relevant Files

- `internal/server/server.go` - Management API handlers for tokens/projects; add GET/PATCH/DELETE token routes, project `is_active`, bulk revoke.
- `internal/server/management_api_test.go` - Unit tests for new/changed handlers and edge cases.
- `internal/admin/server.go` - Admin UI routes to edit/revoke tokens and deactivate projects; bulk revoke action.
- `internal/admin/server_test.go` - Tests for Admin server handlers/rendering and error cases.
- `internal/admin/client.go` - HTTP client methods for new Management API endpoints (token GET/PATCH/DELETE, bulk revoke, project is_active).
- `internal/admin/client_test.go` - Tests for client request/response handling and failures.
- `internal/database/models.go` - Add `projects.is_active`, `projects.deactivated_at`, `tokens.deactivated_at`.
- `internal/database/<migrations>` - SQL migrations for SQLite (and groundwork for PostgreSQL).
- `internal/database/token.go` - Implement `RevocationStore` methods via SQL UPDATEs; add single-token GET if missing.
- `internal/database/project.go` - Read/write `is_active`; optionally guard API key retrieval for inactive projects.
- `internal/token/revoke.go` - Wiring to DB-backed revocation; keep interface stable.
- `internal/token/*_test.go` - Tests for revocation flows, idempotency, and expired/project bulk revoke.
- `web/templates/tokens/list.html` - Add Edit/Revoke actions.
- `web/templates/tokens/edit.html` - New edit form for token properties.
- `web/templates/tokens/show.html` - Adjust revoke button to new Admin route.
- `web/templates/projects/*` - Activate/Deactivate toggle and “Revoke all tokens” action.
- `api/openapi.yaml` - Document new/changed Management API endpoints and schemas.
- `docs/tasks/prd-token-and-project-deactivation.md` - PRD driving this work.
- `docs/architecture.md` - Update with deactivation model and optional proxy guard.
- `docs/instrumentation.md` - Add lifecycle audit events.
- `PLAN.md` - Reflect roadmap/scope changes for deactivation.

### Notes

- Go unit tests live alongside code as `*_test.go` in the same package.
- Prefer targeted test runs during iteration.

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

- [ ] 1.0 Database and Store Layer for Soft Deactivation & Revocation
  - [ ] 1.1 Add migrations: `projects.is_active BOOLEAN DEFAULT 1 NOT NULL`, `projects.deactivated_at TIMESTAMP NULL`.
  - [ ] 1.2 Add migrations: `tokens.deactivated_at TIMESTAMP NULL` (keep existing `is_active`).
  - [ ] 1.3 Extend `internal/database/models.go` `Project` to include `IsActive`, `DeactivatedAt`; extend `Token` with `DeactivatedAt`.
  - [ ] 1.4 Implement DB-backed `RevocationStore` on the DB adapter:
    - [ ] 1.4.1 `RevokeToken(ctx, tokenID)` → `UPDATE tokens SET is_active=0, deactivated_at=NOW() WHERE token=? AND is_active=1`.
    - [ ] 1.4.2 `RevokeProjectTokens(ctx, projectID)` (active-only).
    - [ ] 1.4.3 `RevokeBatchTokens(ctx, tokenIDs)` (active-only).
    - [ ] 1.4.4 `RevokeExpiredTokens(ctx)` (active-only).
  - [ ] 1.5 Wire `DBTokenStoreAdapter` to implement `token.RevocationStore` and expose `GetTokenByID` for Admin UI.
  - [ ] 1.6 Update project store to read/write `IsActive` and `DeactivatedAt`.
  - [ ] 1.7 Tests: DB migrations apply cleanly; unit tests for each revocation method; verify idempotency and no hard deletes used by API paths.

- [ ] 2.0 Management API Extensions for Token Edit/Revocation and Project Deactivation
  - [ ] 2.1 Add routing for `/manage/tokens/{id}`: GET, PATCH (update `is_active`, `expires_at`, `max_requests`), DELETE (soft-revoke).
  - [ ] 2.2 Implement handler(s) in `internal/server/server.go` with validation and sanitized responses (never return secret token value).
  - [ ] 2.3 Add POST `/manage/projects/{id}/tokens/revoke` for bulk project token revocation; returns `{ revoked: <count> }`.
  - [ ] 2.4 Extend PATCH `/manage/projects/{id}` to accept `is_active`; if `false` and `?revoke_tokens=true`, call bulk revoke.
  - [ ] 2.5 Make DELETE `/manage/projects/{id}` return 405 with guidance to use PATCH deactivation.
  - [ ] 2.6 Emit audit events for all new actions (`token.get`, `token.update`, `token.revoke`, `token.revoke_project`, `project.update`, `project.tokens.revoke`).
  - [ ] 2.7 Tests in `internal/server/management_api_test.go`: happy paths, validation errors, idempotent revoke, 405 on DELETE project.

- [ ] 3.0 Admin UI: Token Edit & Revoke, Project Activate/Deactivate, Bulk Revoke
  - [ ] 3.1 Add client methods in `internal/admin/client.go`:
    - [ ] 3.1.1 `GetToken(id)`, `UpdateToken(id, payload)`, `RevokeToken(id)`.
    - [ ] 3.1.2 `RevokeProjectTokens(projectID)`; extend `UpdateProject` to pass `is_active`.
  - [ ] 3.2 Add routes in `internal/admin/server.go`:
    - [ ] 3.2.1 GET `/tokens/:id/edit` (fetch + render form).
    - [ ] 3.2.2 POST `/tokens/:id` with `_method=PATCH` → call Management API PATCH.
    - [ ] 3.2.3 POST `/tokens/:id/revoke` → call Management API DELETE.
    - [ ] 3.2.4 POST `/projects/:id/tokens/revoke` → bulk revoke.
    - [ ] 3.2.5 PATCH `/projects/:id` for `is_active` changes.
  - [ ] 3.3 Templates:
    - [ ] 3.3.1 Update `web/templates/tokens/list.html` to include Edit and Revoke buttons.
    - [ ] 3.3.2 Create `web/templates/tokens/edit.html` with fields: `is_active`, `expires_at`, `max_requests`.
    - [ ] 3.3.3 Update `web/templates/tokens/show.html` revoke form to new Admin route.
    - [ ] 3.3.4 Update `web/templates/projects/*` to toggle Activate/Deactivate and add “Revoke all tokens”.
  - [ ] 3.4 Tests in `internal/admin/server_test.go` and `internal/admin/client_test.go`: route handlers, form binding errors, API error propagation, template rendering.

- [ ] 4.0 Proxy/Project Active Guard Decision and Implementation (if chosen)
  - [ ] 4.1 Decide approach:
    - [ ] 4.1.1 A) Only revoke tokens on project deactivation (minimal change).
    - [ ] 4.1.2 B) Additionally guard API key retrieval for inactive projects (defense-in-depth). Default: B.
  - [ ] 4.2 If B: modify DB adapter `GetAPIKeyForProject` to reject inactive projects; update proxy to translate into 403/401.
  - [ ] 4.3 Tests: proxy request with inactive project denied; ensure audit entry `project.inactive.block` if implemented.

- [ ] 5.0 Audit & Observability: Emit Lifecycle Events and Update OpenAPI Spec
  - [ ] 5.1 Add/confirm audit action constants; ensure all new handlers log audit events with `request_id`, `actor`, and details.
  - [ ] 5.2 Update `api/openapi.yaml` with new endpoints, request/response schemas, and 405 for project DELETE.
  - [ ] 5.3 Verify dispatcher compatibility (no change expected); add tests to assert event emission paths where practical.

- [ ] 6.0 Testing, Linting, and Coverage (≥ 90% across `./internal/...`)
  - [ ] 6.1 Add table-driven unit tests for DB revocation methods and project `is_active` behavior.
  - [ ] 6.2 Add unit tests for all new server handlers and admin routes; include idempotent revoke cases.
  - [ ] 6.3 Add integration tests that simulate project deactivation and bulk revoke → proxy rejects requests.
  - [ ] 6.4 Run `make lint`, `make test` (with `-race`) and CI-style coverage; ensure ≥ 90%.

- [ ] 7.0 Documentation Updates (Architecture, Instrumentation, README/CLI, PLAN)
  - [ ] 7.1 Update `docs/instrumentation.md` with new audit events and fields.
  - [ ] 7.2 Update `docs/architecture.md` to document deactivation model and optional proxy guard.
  - [ ] 7.3 Update `README.md` and/or `docs/cli-reference.md` with Management API additions and Admin UI actions.
  - [ ] 7.4 Update `PLAN.md` with scope decisions and Option A/B outcome.
  - [ ] 7.5 Ensure `docs/tasks/prd-token-and-project-deactivation.md` cross-links are present.


