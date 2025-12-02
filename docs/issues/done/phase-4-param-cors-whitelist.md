# Phase 7: Param Whitelist, CORS, and Header Validation

## Summary
This issue documents the introduction of advanced request validation features in the LLM Proxy:
- **Parameter/model whitelist**: Restrict allowed values for specific request parameters (e.g., model).
- **CORS origin validation**: Restrict allowed origins for cross-origin requests.
- **Required header enforcement**: Require specific headers (e.g., Origin) for requests to be accepted.

## Rationale
- **Security**: Prevent abuse by restricting which models, endpoints, and origins can be accessed via the proxy.
- **Compliance**: Enforce organizational or customer-specific access policies.
- **Operational Safety**: Reduce risk of misconfiguration or accidental exposure.

## Configuration Example

```yaml
apis:
  openai:
    base_url: https://api.openai.com
    allowed_endpoints:
      - /v1/chat/completions
      - /v1/completions
    allowed_methods:
      - POST
    param_whitelist:
      model:
        - gpt-4o
        - gpt-4.1-*
        - text-embedding-3-small
    allowed_origins:
      - https://www.sofatutor.com
      - http://localhost:4000
    required_headers:
      - origin
```

## Implementation Notes
- The proxy validates POST request bodies for whitelisted parameters (e.g., `model`).
- CORS headers are set only for allowed origins.
- Requests missing required headers (e.g., `Origin`) are rejected.
- Wildcard/glob patterns are supported in param whitelists.

## Usage
- Update your `api_providers.yaml` to include `param_whitelist`, `allowed_origins`, and `required_headers` as needed.
- Test with both allowed and disallowed values/origins to verify enforcement.

## Migration
- Existing configs will continue to work; new features are opt-in.
- If you add a param whitelist, ensure all clients use allowed values.

## Related Files
- `config/api_providers.yaml`
- `internal/proxy/config_schema.go`
- `internal/proxy/interfaces.go`
- `internal/proxy/proxy.go`

## Status
- [x] Implemented in proxy and config
- [x] Documented here