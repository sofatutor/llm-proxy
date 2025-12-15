# Phase 6: DB-Backed Runtime Config + Per-Project Overrides

Tracking: [Issue #200](https://github.com/sofatutor/llm-proxy/issues/200)

## Summary
Introduce a structured configuration system that moves “runtime tunables” out of environment variables into the management API + database, with safe, validated **per-project overrides**. The system keeps a small “bootstrap” config in env (secrets + connectivity), and adds a new **runtime config layer** stored in DB, hot-reloadable in-process, and queryable/editable via the management API and Admin UI.

Primary motivation: reduce env var sprawl, enable operational tuning without redeploys, and support per-project policy overrides (e.g., CORS, model allowlists, rate limits).

## Goals
- Reduce the number of required/commonly-used environment variables.
- Support changing safe config at runtime via management API (no restart).
- Support **per-project overrides** for selected keys.
- Validate and audit all runtime changes.
- Keep request-path overhead low (constant-time reads from a cached snapshot).

## Non-Goals
- No “dynamic infrastructure” config (DB endpoints, encryption key, management token) in DB.
- No attempt to support arbitrarily complex user-defined logic; only typed keys with strict validation.
- No breaking changes to proxy routing compatibility unless explicitly opted-in.

## Current State (Problem)
- Core config is largely environment-variable driven (`internal/config`).
- API provider allowlists live in YAML today (`config/api_providers.yaml`).
- The YAML-based provider config is a second “config system” with different ergonomics than env/management API (harder to audit, harder to change without redeploy/restart, not per-project).
- Project-specific state already lives in DB (projects + tokens), but policy overrides don’t.
- CORS is effectively configured in multiple places:
  - Proxy OPTIONS short-circuit sets permissive headers.
  - Provider-level origin validation uses `allowed_origins`/`required_headers` from YAML.
  - There is no coherent per-project CORS policy.

## Proposed Architecture

### 1) Split Configuration into Two Layers

**A) Bootstrap Config (env/file only, restart required)**
- Needed before DB is available and/or operational secrets.
- Examples:
  - `MANAGEMENT_TOKEN`
  - DB driver/path/URL
  - `ENCRYPTION_KEY`
  - listen address
  - Redis connection info (if used)

**B) Runtime Config (DB-backed, hot-reloadable)**
- Tunables and policies that can change without redeploy.
- Examples:
  - global rate limit thresholds
  - cache defaults (TTL, max object size)
  - request validation policies (model allowlists)
  - CORS policy
  - project-active enforcement toggles

### 1b) Move API Provider Allowlists into DB (Provider Profiles)

Move the contents of `api_providers.yaml` into a DB-backed, management-controlled model.

Rationale:
- Provider allowlists are security policy. They should be:
  - editable via management API
  - auditable
  - optionally overridable per project (where safe)
  - hot-reloadable

Proposed model:
- Introduce **Provider Profiles** (DB rows) that capture what the YAML defines today:
  - base URL
  - allowed endpoint prefixes
  - allowed methods
  - param whitelist
  - required headers
  - provider-level allowed origins (optional; may be superseded by per-project policy)
  - timeouts + connection tuning

Project association:
- Each project references a provider profile (defaulting to the instance-wide default profile).
- Selected provider-policy fields may be overridden per-project via the runtime config override mechanism (e.g., model allowlist, endpoint opt-outs, origin policy), but *not* base URL unless explicitly allowed.

Back-compat / bootstrap:
- Keep `api_providers.yaml` support as a fallback (read-only) for a transition period.
- Precedence for provider config:
  1) DB provider profiles (preferred)
  2) YAML file (fallback)
  3) hard-coded default OpenAI profile (last resort)

### 1c) Per-Project Overrides: Scope and Semantics (OpenAI-focused)

Given the current product focus (OpenAI-compatible proxy), per-project overrides should primarily support:

1) **Different models per project** (common multi-tenant requirement)
- Each project can restrict which `model` values it may request.
- Semantics should be *restrictive only* (no expanding beyond the provider profile unless explicitly allowed).

2) **Opting out of certain API endpoints per project**
- Each project can further restrict which endpoint prefixes are allowed.
- This is useful for turning off higher-risk endpoints (e.g., file APIs) for some tenants.

Recommended override strategy:
- Provider profile defines the maximum allowed surface area.
- Project overrides can only narrow access (denylist/subset). This prevents accidental privilege escalation.

### 2) Config Registry + Values

Introduce a small “registry” of supported runtime keys that ships with the code:

- Registry defines:
  - key name (string)
  - type: `bool|int|duration|string|string_list|json`
  - scope: `global|project|both`
  - default value
  - validation rules (min/max, enum, regex, max length)
  - sensitivity flags (do not return raw values)
  - restart_required (false for runtime keys)

- Runtime values are stored in DB for:
  - global (applies to all projects)
  - per-project overrides

### 3) Precedence Rules

For an effective value lookup of key `K` for request with `project_id`:

1. Project override (DB) if present
2. Global runtime value (DB) if present
3. Code default (registry default)

Optional emergency escape hatch (recommended but not required):
- `LLM_PROXY_FORCE_<KEY>` env var overrides runtime values for incident response.

### 4) Request Path Integration

The token validation already yields `project_id`. After auth, resolve an immutable **effective policy snapshot**:

- `EffectiveProjectPolicy = merge(GlobalRuntime, ProjectOverrides(project_id))`

Implementation detail:
- Keep a process-local cache of the latest global snapshot and per-project snapshots.
- Reads in the request path should be O(1) and lock-free (atomic pointer swap is preferred).

### 5) Refresh / Distribution

Start with a simple approach:
- Poll DB every N seconds (e.g., 5–10s) for updated versions.
- On update, rebuild snapshots and atomically swap.

Optional enhancement:
- Publish a `config_changed` event on the existing event bus; instances invalidate immediately.

## Data Model (DB)

### Tables

**`runtime_config_keys`** (registry mirror; optional persistence)
- If persisted, it enables discovery via API/UI.
- Columns:
  - `key TEXT PRIMARY KEY`
  - `type TEXT NOT NULL`
  - `scope TEXT NOT NULL` (`global|project|both`)
  - `default_json TEXT NOT NULL`
  - `validation_json TEXT NOT NULL`
  - `is_sensitive BOOLEAN NOT NULL DEFAULT false`
  - `restart_required BOOLEAN NOT NULL DEFAULT false`
  - `created_at`, `updated_at`

**`runtime_config_values`**
- Stores actual values.
- Columns:
  - `id TEXT PRIMARY KEY` (UUID)
  - `scope TEXT NOT NULL` (`global|project`)
  - `project_id TEXT NULL`
  - `key TEXT NOT NULL`
  - `value_json TEXT NOT NULL`
  - `version INTEGER NOT NULL` (monotonic per row)
  - `updated_at TIMESTAMP NOT NULL`
  - `updated_by TEXT NOT NULL` (actor)
- Constraints:
  - unique `(scope, project_id, key)`
  - foreign key `project_id` → `projects.id` (when scope=project)

**`runtime_config_audit`** (or reuse existing audit pipeline)
- Captures old/new values, actor, request id.

### Migration Notes
- PostgreSQL: add goose migrations in `internal/database/migrations/sql/postgres/`.
- SQLite: update `scripts/schema.sql` (current-schema only).

## Management API Changes

### Endpoints

Global runtime config:
- `GET /manage/config` — list keys + current global values + defaults
- `PATCH /manage/config` — set global values (validated, audited)

Project overrides:
- `GET /manage/projects/{id}/config` — show effective + override view
- `PATCH /manage/projects/{id}/config` — set overrides for that project

Recommended request/response shape:
- accept patch body as `{ "set": { "key": <typed-json> }, "unset": ["key"] }`
- return `{ "effective": {...}, "overrides": {...}, "defaults": {...} }` (UI-friendly)

### Authorization
- All endpoints require `MANAGEMENT_TOKEN`.

### Validation & Safety
- Reject unknown keys.
- Enforce key type and validation rules.
- Enforce scope rules (e.g., project-only keys cannot be set globally).
- For sensitive keys: store encrypted (if needed), and never return raw values in list endpoints.

## Admin UI Changes
- Add a “Runtime Config” section:
  - Global settings
  - Per-project overrides (tab on project page)
- Show:
  - effective value
  - source (default/global/project)
  - last updated time/actor

## Per-Project CORS: Design Notes (Important)

### Constraint: OPTIONS Preflight Has No Auth
Browser preflight requests typically do not include the Authorization token. That means the server cannot reliably determine `project_id` for OPTIONS requests in the OpenAI-compatible `/v1/*` path.

### Proposed CORS Handling

**A) Preflight (OPTIONS)**
- Serve a safe, global preflight response using a **global allowed-origins list** (or `*` only if explicitly configured).
- This list can be:
  - a separate global runtime key (e.g., `cors.preflight_allowed_origins`), or
  - the union of all project allowed origins (less strict, easier operationally).

**B) Actual Requests (non-OPTIONS)**
- Enforce project-specific CORS origin validation after token validation.

Security note:
- CORS is a browser read policy, not the auth boundary. Auth remains the token.
- The important enforcement is rejecting disallowed origins (if enabled) on authenticated requests.

### Alternative (Strict Isolation)
If strict per-project preflight is required, introduce an explicit project identifier in the request path or host (e.g., `/p/{project_id}/v1/...` or per-project subdomain). This is likely a breaking change for OpenAI compatibility and should be opt-in.

## Suggested Runtime Keys (Initial Set)

### Global
- `cors.preflight_allowed_origins` (string_list)
- `cors.allowed_methods` (string_list)
- `cors.allowed_headers` (string_list)
- `cors.max_age_seconds` (int)

- `ratelimit.global_rpm` (int)
- `ratelimit.ip_rpm` (int)

- `cache.enabled` (bool)
- `cache.default_ttl_seconds` (int)
- `cache.max_object_bytes` (int)

- `project.enforce_active` (bool)

### Per-Project
- `project.cors.allowed_origins` (string_list)
- `project.request.model_allowlist` (string_list)
- `project.request.endpoint_denylist` (string_list)
- `project.ratelimit.rpm` (int)

Semantics:
- `project.request.model_allowlist`:
  - Applies to request parameter `model`.
  - If provider profile already defines a model allowlist, the effective allowlist is the intersection.
  - If provider profile does not define a model allowlist, the project allowlist becomes the effective allowlist.
- `project.request.endpoint_denylist`:
  - A list of endpoint prefixes to deny for that project.
  - Effective allowed endpoints are computed as: `provider_profile.allowed_endpoints` minus any denied prefixes.
  - This is intentionally “opt-out only”; projects cannot add endpoints beyond what the provider profile allows.

Notes:
- Keep the key count small; expand only as needs emerge.
- Prefer provider-agnostic naming and semantics.

## Rollout Plan (Incremental)

### Phase 1: Foundations
- Add runtime config tables + store interface.
- Add registry of keys + validator.
- Add management API endpoints (GET/PATCH) for global runtime config.
- Add polling cache (global snapshot only).

### Phase 2: Per-Project Overrides
- Add project override storage + endpoints.
- Add project snapshot cache + merge logic.
- Wire request-path resolution based on `project_id`.

### Phase 3: Move One Real Feature
- Implement per-project CORS enforcement on non-OPTIONS requests.
- Add configurable preflight policy.
- Ensure existing behavior remains default-compatible.

### Phase 4: Expand Coverage
- Move selected env vars to runtime config (with backward compatible env fallback).
- Update docs and examples.

## Testing Strategy
- Unit tests:
  - registry validation (types, min/max, enums)
  - precedence resolution (default/global/project)
  - cache refresh logic (version bump)
- Integration tests:
  - management API set/get
  - per-project override flow
  - CORS behavior (OPTIONS vs authenticated request enforcement)
- Race tests:
  - snapshot swap under concurrent reads

## Acceptance Criteria
- Runtime config can be set and retrieved via management API.
- Per-project overrides work and take precedence over global defaults.
- All writes validated; unknown keys rejected.
- Changes are audited (via DB table and/or existing audit system).
- Request-path reads use cached snapshots (no DB queries per request).
- Per-project CORS enforcement works for authenticated requests; preflight behavior is documented and configurable.
- `make test` and `make lint` are green; repo coverage remains ≥ 90%.

## Open Questions
1. Should we persist `runtime_config_keys` in DB, or keep the registry code-only and only store values?
  - Proposed answer: keep the key registry **code-only** (source of truth), and store only values in `runtime_config_values`.
  - Rationale: allowing keys to be created/changed in DB implies “schema-less config” and pushes validation/compat risk into runtime. A code-owned registry keeps typing/validation safe and reviewable.
  - UX implication: `GET /manage/config` returns the registry metadata (type/default/validation/scope) so the Admin UI can discover supported keys without a DB table.

2. Do we need encrypted-at-rest for certain runtime values, or are all runtime keys non-secret by design?
  - Proposed answer: treat runtime config as **non-secret by design**, and avoid storing secrets in runtime keys.
  - Rationale: secrets already belong in bootstrap env (e.g., `MANAGEMENT_TOKEN`, DB credentials, `ENCRYPTION_KEY`). Runtime config should be policy/tuning.
  - Future-proofing: keep a registry flag like `is_sensitive` for rare exceptions; for those, encrypt `value_json` at rest and never return raw values in list endpoints.

3. Should we support a “force env override” mechanism for incident response, and if so, which keys?
  - Proposed answer: yes, but keep it intentionally small and “break-glass”.
  - Recommended design:
    - `LLM_PROXY_RUNTIME_CONFIG_DISABLED=true` to ignore DB values (use registry defaults) if DB config causes instability.
    - Optional `LLM_PROXY_FORCE_<KEY>` overrides for a curated allowlist of keys that are operational toggles or can only tighten security.
  - Suggested allowlist (initial): `cache.enabled`, `project.enforce_active`, selected `ratelimit.*` ceilings, and CORS preflight strictness.
  - Safety rules: surface the forced source in `GET /manage/config` (e.g., `source=env-force`) and log activation (avoid logging raw values for anything sensitive).

4. Should the Admin UI expose editing for all keys or only a curated subset?
  - Proposed answer: expose **editing for a curated subset**, and show all keys read-only with metadata.
  - Rationale: keeps the UI usable and reduces sharp-edge operations; the management API can remain capable, but UI should steer toward safe/common knobs.
  - UX suggestion: show effective value + source (default/global/project/env-force) + last updated actor/time; hide or mark “advanced” keys as read-only unless explicitly enabled.
