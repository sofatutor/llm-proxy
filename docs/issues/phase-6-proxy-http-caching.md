# Phase 6: Proxy HTTP Caching (Opt-in via Standard Headers)

## Summary
Introduce a shared HTTP response cache in the proxy that honors standard HTTP caching semantics. The cache supports a Redis backend (production) with in-memory fallback. The cache engages when upstream responses explicitly indicate cacheability or when the client explicitly opts in via request `Cache-Control` (for benchmarking or controlled cases). Default behavior: cache enabled, backend selectable via env; policy remains conservative and standards-aligned.

Tracking: to be created (GitHub Issue)

## Rationale
- Reduce latency and upstream/provider load for repeat requests
- Lower cost where providers bill per request/transfer
- Respect web standards (no custom headers required), making the feature predictable and audit-friendly

## Scope
In-scope:
- Shared (surrogate) cache behavior based on standard HTTP semantics
- Redis as the primary cache store (production-ready), with in-memory as fallback
- GET/HEAD caching; optional POST caching when client explicitly opts in via request `Cache-Control`
- Conditional responses for GET/HEAD (304 to the client based on validators)
- Streaming responses: capture while streaming, store after completion, and serve from cache on subsequent requests

Out of scope (for this phase):
- Advanced cache invalidation APIs (beyond basic purge potential)
- Stale-while-revalidate and stale-if-error
- Surrogate keys/tags and cache groups

## Requirements
- Feature can be toggled via env; default enabled. Disable with `HTTP_CACHE_ENABLED=false`
- Only cache when allowed by policy:
  - Honors `Cache-Control` directives: `no-store` (never cache), `private` (do not store in shared cache), `public`, `max-age`, `s-maxage`
  - Honors presence of `Authorization` header: serve cached responses to authenticated requests only if the cached response is explicitly cacheable for shared caches (e.g., `Cache-Control: public` or `s-maxage>0`). Authorization is not part of the cache key
  - Honors validators: `ETag` and `Last-Modified` for client conditionals (304 handling)
- Cache only cacheable status codes (subset of RFC 9111 defaults): `200, 203, 301, 308, 404, 410`. `304` is a conditional response to the client, not stored
- Enforce maximum object size (bytes) to protect memory/Redis; skip caching if exceeded
- TTL derivation:
  - Prefer `s-maxage` (shared caches) over `max-age` from response
  - Fallback: when upstream permits caching but omits explicit TTL, use configured default TTL
  - Client-forced (request) caching: if upstream has no explicit cache directives, and the request includes `Cache-Control: public, max-age=...` (or `s-maxage`), allow storage with that TTL (used notably for benchmarks)
- Set response headers for observability: `Cache-Status`, `X-PROXY-CACHE`, and `X-PROXY-CACHE-KEY`
- Observability: cache hits are not published to the event bus; misses/stores are published

## Design / Approach
1) Configuration (ENV/flags)
- `HTTP_CACHE_ENABLED` (default: `true`; set to `false` to disable)
- `HTTP_CACHE_BACKEND` (values: `redis`, `in-memory`; default: `in-memory` if not set to `redis`)
- `REDIS_CACHE_URL` (e.g., `redis://localhost:6379/0`; default applied when `HTTP_CACHE_BACKEND=redis` and unset)
- `REDIS_CACHE_KEY_PREFIX` (key prefix; default: `llmproxy:cache:`)
- `HTTP_CACHE_MAX_OBJECT_BYTES` (e.g., `1048576` for 1 MiB)
- `HTTP_CACHE_DEFAULT_TTL` (duration/seconds; only used if upstream permits caching but no explicit TTL provided)

2) Cache integration (internal/proxy)
- Lookup runs before reverse proxy upstream call
- Keying includes method, path, sorted query, and a conservative `Vary` subset from request headers (`Accept`, `Accept-Encoding`, `Accept-Language`); Authorization and `X-*` headers are excluded from the key. For POST/PUT/PATCH, a body hash is included
- On hit:
  - Serve cached status/headers/body; set `Cache-Status: hit` and `X-PROXY-CACHE: hit`
  - If client sent `If-None-Match` or `If-Modified-Since` (GET/HEAD) that match cached validators, respond `304 Not Modified` (`Cache-Status: conditional-hit`)
  - Event bus: hits bypass publishing
- On miss/bypass:
  - Proxy upstream, evaluate cacheability, and store if compliant; set `Cache-Status: miss`, `bypass`, or `stored`
  - Client-forced (request) caching for POST/GET: if upstream lacked directives and request has `Cache-Control: public, max-age=..`, store with that TTL and add synthetic `Cache-Control` to cached headers to allow shared reuse, including for authenticated requests
- Streaming:
  - For streaming responses, wrap the body with a capture reader; store after completion if policy/size allow; subsequent requests can hit the cached full payload

3) Storage (Redis + in-memory fallback)
- Interface abstraction with two implementations:
  - Redis-backed JSON blob with Redis TTL (primary)
  - In-memory map with expiry (dev/test fallback)
- Guards: size limit, TTL, skip on non-2xx responses

4) Policy details and caveats
- Authorization never part of the cache key. Authenticated requests only use cache entries marked for shared caching (`public`/`s-maxage>0`)
- POST caching is allowed only when client explicitly opts in via request `Cache-Control` and uses body hash in key; POST hits return 200 (not 304)
- Conditional GET/HEAD: we currently respond 304 to client if validators match cached entry; we do not (yet) revalidate upstream with conditional requests
- Vary: current implementation uses a conservative request-header subset; full per-response `Vary` handling can be a follow-up

5) Observability
- Headers: `Cache-Status: hit|miss|bypass|stored|conditional-hit`, `X-PROXY-CACHE`, `X-PROXY-CACHE-KEY`
- Event bus: cache hits are not published; misses and stored responses are
- Metrics (follow-up): `proxy_cache_hits_total`, `proxy_cache_misses_total`, `proxy_cache_bypass_total`, `proxy_cache_store_total`

6) Admin / CLI
- Benchmark CLI can force cacheability for testing via request `Cache-Control: public, max-age=<ttl>` and supports `--method GET|POST` and `--debug` to inspect headers/body
- Optional (future): management endpoint/CLI to purge by URL or key prefix

## Tasks
- [x] Add configuration flags/envs for cache feature (env toggles, backend selection, defaults)
- [x] Introduce Redis cache adapter and cache interfaces
- [x] Implement proxy integration for lookup/store; include POST (client-forced) and streaming capture
- [x] Honor `ETag`/`Last-Modified` for client conditionals (respond 304 when applicable)
- [x] Enforce size limits and conservative `Vary` keying
- [x] Add `Cache-Status`, `X-PROXY-CACHE`, and `X-PROXY-CACHE-KEY` response headers
- [x] Bypass event publishing for cache hits
- [ ] Metrics for hits/misses/bypass/store
- [ ] Full `Vary` header handling (per-response driven)
- [ ] Upstream conditional revalidation path (If-None-Match/If-Modified-Since)
- [ ] (Optional) Purge endpoint + CLI

## Acceptance Criteria
- Cache can be enabled/disabled via env; backend selectable (Redis/in-memory)
- GET/HEAD cached per HTTP semantics; POST caching only when client explicitly opts in via request `Cache-Control`
- Authenticated requests only use cached entries explicitly marked shared-cacheable
- Streaming responses are stored after completion and served on subsequent requests
- Size limits and TTL rules enforced; headers present for observability
- Cache hits are not published to the event bus; misses/stores are
- No regression to existing proxy behavior when cache is disabled

## Notes & Follow-ups
- Future enhancements: `stale-while-revalidate`, `stale-if-error`, surrogate keys/tags, full `Vary` support, upstream revalidation, bounded L1 in-memory cache (opt-in)

