# Phase 7: Header Whitelist Per Token (Token Metadata Enforcement)

Tracking: [Issue #50](https://github.com/sofatutor/llm-proxy/issues/50)

## Motivation
To enable fine-grained access control and security, we want to restrict token usage based on custom metadata. This allows us to require and validate specific headers (e.g., user ID, client IP) for each token, ensuring tokens are only usable in the intended context.

## Requirements
- Each token can have a `metadata` JSON field (e.g., `{ "user_id": "1", "client_ip": "xx.xx.xx.xx" }`).
- When a request is proxied, the proxy must require and validate headers corresponding to each metadata key:
  - For `user_id`, require header `X-PROXY-USER-ID` with value `1`.
  - For `client_ip`, require header `X-PROXY-CLIENT-IP` with value `xx.xx.xx.xx`.
- If any required header is missing or does not match, the request is rejected (401 or 403).
- Metadata is settable at token creation time (API, CLI, or admin UI).

## Implementation Plan
1. **Extend Token Model**
   - Add a `Metadata` field to the token struct and database (type: `map[string]interface{}` or `json.RawMessage`).
2. **Token Creation**
   - Allow specifying `metadata` as a JSON object when creating a token.
   - Store this in the database.
3. **Proxy Enforcement**
   - After token validation, retrieve the token's `Metadata`.
   - For each key in `Metadata`, require a header `X-PROXY-<KEY>` (case-insensitive, underscores to dashes).
   - The header value must match the value in `Metadata` (as a string).
   - If any required header is missing or does not match, return 401 Unauthorized or 403 Forbidden with a clear error message.
4. **Testing**
   - Add tests for token creation with metadata and for proxy enforcement of required headers.

## Example
- **Token Metadata:** `{ "user_id": "1", "client_ip": "192.168.1.1" }`
- **Required Headers:**
  - `X-PROXY-USER-ID: 1`
  - `X-PROXY-CLIENT-IP: 192.168.1.1`

---
This feature will enable per-token header-based restrictions for enhanced security and compliance. 

---

# Phase 8: Project-Level Param Whitelist Restriction and Provider Opt-Out

Tracking: [Issue #51](https://github.com/sofatutor/llm-proxy/issues/51)

## Motivation
Some projects may require stricter or more flexible parameter validation than what is defined globally per API provider. To support this, we need to allow projects to:
- Define their own param whitelist (overriding or restricting the provider-level list).
- Opt out of the provider-level param whitelist entirely, allowing all values for certain parameters.

## Requirements
- Projects can specify a param whitelist for API parameters, which takes precedence over the provider-level whitelist.
- Projects can opt out of the provider-level param whitelist for specific parameters (e.g., allow any value for `model`).
- If a project does not specify a whitelist for a parameter, the provider-level whitelist is enforced by default.
- If a project opts out for a parameter, no whitelist is enforced for that parameter.
- Configuration should be possible via YAML and/or management API.

## Implementation Plan
1. **Extend Project Model and Config**
   - Add a `param_whitelist` field to the project model/config (similar to provider config).
   - Add a way to specify opt-out (e.g., `param_whitelist: { model: null }` or a special value).
2. **Validation Logic**
   - When validating parameters, check for a project-level whitelist first.
   - If present, enforce it. If explicitly set to opt-out, skip validation for that parameter.
   - Otherwise, fall back to the provider-level whitelist.
3. **Configuration and API**
   - Allow setting project-level param whitelists and opt-outs via YAML and management API.
4. **Testing**
   - Add tests for project-level overrides, opt-outs, and fallback to provider-level validation.

## Example
- **Provider Config:**
  ```yaml
  param_whitelist:
    model:
      - gpt-4o
      - gpt-4.1-mini
  ```
- **Project Config (override):**
  ```yaml
  param_whitelist:
    model:
      - gpt-4o
  ```
- **Project Config (opt-out):**
  ```yaml
  param_whitelist:
    model: null
  ```

---
This feature will enable per-project parameter validation policies and greater flexibility for different use cases. 