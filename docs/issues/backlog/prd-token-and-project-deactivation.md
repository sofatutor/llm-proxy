## PRD: Token and Project Deactivation, Revocation, and Admin Edit Actions

### Introduction/Overview

This PRD defines the work to make token and project lifecycle operations compliant with auditing requirements while improving the admin UX. Specifically:
- Replace destructive deletes with soft deactivation for both tokens and projects.
- Add first-class token revocation (single, batch, and per-project) via the Management API.
- Add token edit capabilities in the Admin UI, and enable revoking tokens from the UI.
- Support deactivating a project and revoking all tokens of that project in one operation.

This aligns with the architecture in `docs/architecture/index.md` and the repo’s rules in `AGENTS.md` (audit-first, transparent behavior, 90%+ coverage, no TODOs).

### Goals

- Ensure no hard-deletes of tokens or projects through the Management API or Admin UI (auditing compliance).
- Provide explicit revocation/deactivation flows that are auditable and reversible.
- Enable admins to edit token properties (expiration, limits, active state) and to revoke a token.
- Allow project deactivation and bulk token revocation at the project level.
- Maintain the proxy’s minimal latency and transparent behavior.

### User Stories

- As an admin, I can revoke a token so it immediately stops working, and I can later re-activate it if required.
- As an admin, I can edit a token’s properties (expires_at, max_requests, active state) from the Admin UI.
- As an admin, I can deactivate a project and optionally revoke all its tokens in the same action.
- As an auditor/SRE, I can see all deactivation/revocation events with actor, reason, request ID, and timestamp in the audit log and, if configured, in the DB.
- As an operator, I can revoke all expired tokens via a management endpoint or automatic job without deleting them.

### Functional Requirements

1) Management API (in `internal/server`)
   - Add token endpoints:
     - GET `/manage/tokens/{id}`: return a single token (sanitized; never return the raw token string).
     - PATCH `/manage/tokens/{id}`: update fields `{ is_active, expires_at, max_requests }`.
       - Validations: `expires_at` must be RFC3339 or null; `max_requests` ≥ 0 or null; `is_active` boolean.
       - Response: 200 JSON of updated token (sanitized) and full audit trail event.
     - DELETE `/manage/tokens/{id}`: soft-revoke (set `is_active=false`).
       - Idempotent: if already inactive, return 200 with current state (no error).
       - Response: 200 JSON `{ token_id, is_active: false }` or 204; choose 200 for better UX feedback.
     - POST `/manage/projects/{projectId}/tokens/revoke`:
       - Body: `{ include_inactive?: bool }` (default false; skip already inactive).
       - Behavior: set `is_active=false` for all active tokens of the project; return `{ revoked: <count> }`.
   - Project endpoints:
     - PATCH `/manage/projects/{id}` already exists for name/API key; extend to support `{ is_active }`.
       - When `is_active` transitions to `false`, optionally revoke all tokens if `?revoke_tokens=true`.
       - If `is_active=false`, the proxy must refuse routing for that project’s API key (see 4).
     - DELETE `/manage/projects/{id}`: deprecate and return `405 Method Not Allowed` from the Management API.
       - Keep DB support for internal tests if needed, but do not expose destructive delete in the API.
   - All endpoints require `MANAGEMENT_TOKEN` and must emit audit events:
     - Token: `token.revoke`, `token.update`, `token.get`, `token.revoke_project`, `token.revoke_expired` (if implemented).
     - Project: `project.update` (with `is_active` change), `project.tokens.revoke`.

2) Admin UI (in `internal/admin` + templates under `web/templates/tokens` and `web/templates/projects`)
   - Tokens list (`web/templates/tokens/list.html`):
     - Add an Edit action (pencil icon) → `GET /tokens/:token/edit`.
     - Add a Revoke action (key-off or x-octagon icon) → POST form to `/tokens/:token/revoke`.
   - Token edit page (`web/templates/tokens/edit.html`, new):
     - Fields: `is_active` (toggle), `expires_at` (datetime-local), `max_requests` (int or blank=unlimited).
     - Submit PATCH to Management API `/manage/tokens/{id}` via Admin server.
   - Token show page: keep revoke button; update form endpoints to use Admin routes that call Management API PATCH/DELETE (not `_method=DELETE` hacks).
   - Project pages (`web/templates/projects/*`):
     - Add Activate/Deactivate toggle; saving issues PATCH `/manage/projects/{id}` with `is_active`.
     - Provide a “Revoke all tokens” action that calls POST `/manage/projects/{id}/tokens/revoke`.
   - Admin server routes (`internal/admin/server.go`):
     - GET `/tokens/:id/edit` → render edit form (fetch token via GET `/manage/tokens/{id}`).
     - POST `/tokens/:id/revoke` → call DELETE `/manage/tokens/{id}` then redirect back.
     - POST `/tokens/:id` with `_method=PATCH` (form) → PATCH `/manage/tokens/{id}`.
     - POST `/projects/:id/tokens/revoke` → Management API bulk revoke endpoint.
     - PATCH `/projects/:id` should pass `is_active` changes; if deactivated and `revoke_tokens=true`, call bulk revoke.

3) Database & Storage (in `internal/database`)
   - Projects: add soft-activation flags
     - Add columns: `projects.is_active BOOLEAN NOT NULL DEFAULT 1`, `projects.deactivated_at TIMESTAMP NULL`.
     - Migrations for SQLite (and PostgreSQL later per `docs/issues/done/phase-5-postgres-support.md`).
     - DB adapters must include `IsActive` on the `proxy.Project` model and read/write it.
   - Tokens (already have `is_active`):
     - Add optional `deactivated_at TIMESTAMP NULL` for audit clarity.
     - Implement revocation operations (SQL UPDATEs, no DELETEs):
       - `RevokeToken(ctx, tokenID)` → `UPDATE tokens SET is_active=0, deactivated_at=NOW() WHERE token=? AND is_active=1`.
       - `RevokeProjectTokens(ctx, projectID)` → same update with `WHERE project_id=? AND is_active=1`.
       - `RevokeExpiredTokens(ctx)` → same update with `WHERE expires_at < NOW() AND is_active=1`.
     - Deprecate/disable `CleanExpiredTokens` (hard-delete). Keep for internal tooling only; not used by API.
   - New store interfaces and adapters:
     - Extend DB adapters to implement `internal/token.RevocationStore` against SQL UPDATEs (not DELETEs).
     - Add `GetTokenByID` passthrough for Admin UI GET single-token.
   - Project deactivation enforcement:
     - Option A (minimal risk): project deactivation performs bulk token revocation; proxy continues to rely on token validity only.
     - Option B (defense-in-depth, optional): also add `projects.is_active` guard in `projectStore.GetAPIKeyForProject` so inactive projects cannot proxy even with stale tokens.

4) Proxy & Validation (`internal/proxy`, `internal/token`)
   - No behavior change for request routing beyond token inactivity checks.
   - If Option B above is chosen: update `GetAPIKeyForProject` (DB adapter) to reject inactive projects and propagate an appropriate 403/401 in the proxy layer. Emit audit entry `project.inactive.block`.

5) Observability & Audit (`internal/audit`, `internal/server`)
   - Emit structured audit events on every token and project lifecycle change with fields: `action`, `actor`, `project_id`, `token_id`, `request_id`, `reason` (optional), `origin` (`admin-ui` when applicable).
   - Extend `api/openapi.yaml` to document new endpoints and responses.

6) Backward Compatibility & Deprecations
   - Management API DELETE `/manage/projects/{id}` returns 405 with guidance to use PATCH `{ is_active: false }`.
   - `CleanExpiredTokens` is deprecated; new jobs should call `RevokeExpiredTokens` instead.
   - Token JSON in list/single responses remains sanitized (never return secret token string).

### Non‑Goals

- No changes to proxy request/response transformation beyond the optional project active guard.
- No token value re-issuance/rotation in this PRD (future work).
- No RBAC/role model; Management API continues to use a single `MANAGEMENT_TOKEN` in this iteration.

### Design Considerations

- Follows reverse-proxy transparency and minimal overhead from `docs/architecture/architecture.md`.
- Uses existing revocation types in `internal/token/revoke.go`; provides concrete DB-backed implementations.
- Keep DB schema compatible with both SQLite and PostgreSQL; write migrations accordingly.
- Ensure Admin UI never exposes raw token values except at creation time (already implemented).

### Technical Considerations

- Go 1.23+, tests with `-race` and coverage aggregation (`-coverpkg=./internal/...`).
- SQL migrations for `projects.is_active`, `projects.deactivated_at`, `tokens.deactivated_at`.
- Idempotent revocation endpoints: multiple calls are safe.
- Concurrency: bulk updates should be transactional where appropriate; ensure no long locks.
- Validation: strict parsing for `expires_at` and bounds on `max_requests`.
- Update `internal/admin/client.go` to support the new Management API endpoints.
- Update OpenAPI spec (`api/openapi.yaml`).

### Success Metrics

- Tests: `make test` and `-race` green; coverage ≥ 90% CI-style across `./internal/...`.
- Linters: `make lint` returns 0 issues; formatting unchanged.
- Manual admin flows verified:
  - Edit token → properties saved and reflected in list.
  - Revoke token (single) → status changes to inactive; proxy rejects token.
  - Deactivate project with `revoke_tokens=true` → all tokens inactive; proxy rejects.
  - Revoke project tokens endpoint works and returns count.
  - DELETE `/manage/projects/{id}` returns 405.

### Open Questions

1. Should project deactivation strictly block API key retrieval (Option B) in addition to revoking tokens? Default proposal: Yes, defense-in-depth.
2. For DELETE `/manage/tokens/{id}`, prefer 200 with JSON payload vs 204 with no body? Default proposal: 200.
3. Do we need a GET `/manage/projects/{id}/tokens` to simplify Admin UI edit forms? Default proposal: optional, nice-to-have.
4. Should Admin UI support re-activating a token (flip `is_active=true`)? Default proposal: yes, include toggle.
5. Do we need a `reason` field for revocation in the API/UI for better audit trails? Default proposal: yes, optional string.

### Impacted Files/Packages (high-level)

- `internal/server/server.go` (new handlers, audit events, method switch changes)
- `internal/admin/server.go` (new routes + handlers)
- `internal/admin/client.go` (new client methods)
- `internal/database/*` (migrations, new SQL updates, adapters, models)
- `web/templates/tokens/*`, `web/templates/projects/*` (edit/revoke UI)
- `api/openapi.yaml` (spec updates)
- Tests: `internal/server/*_test.go`, `internal/admin/*_test.go`, `internal/database/*_test.go`, `internal/token/*_test.go`

### Acceptance Criteria

- No destructive deletes in Management API for tokens/projects; DELETE project returns 405.
- Admin UI exposes Edit and Revoke for tokens; Deactivate and “Revoke all tokens” for projects.
- Revocation and deactivation events are fully auditable and persisted per configuration.
- All tests and linters green; coverage ≥ 90% across `./internal/...`.


