# Token Management System Design

## Token Format

The token format will follow these principles:

1. **Base Format**: UUID v7 (time-ordered UUID) from github.com/google/uuid
   - Benefits: 
     - Widely adopted standard
     - Built-in cryptographic randomness
     - Time-ordered for sortability
     - Globally unique with no collision concerns

2. **String Representation**: Formatted as "tkn_" + base64url(UUID) to:
   - Provide a namespace/prefix for our tokens (distinguishable)
   - Reduce token length compared to standard UUID string format
   - Make tokens URL-safe and easy to copy/paste
   - Example: `tkn_1a2b3c4d5e6f7g8h9i0j`

3. **Properties**:
   - Length: ~22 characters after encoding (shorter than standard 36-char UUID)
   - Contains only URL-safe characters: [a-zA-Z0-9_-]
   - Impossible to guess or predict
   - Validates easily with a simple regex pattern
   - Can be traced back to creation time (using UUID v7 timestamp)

## Validation Rules

1. **Format Validation**:
   - Must match the pattern `^tkn_[A-Za-z0-9_-]{22}$`
   - Must be decodable to a valid UUID
   - Must be the correct length

2. **Existence Check**:
   - Token must exist in the database
   - Associated project must exist

3. **Active Status**:
   - Token must have `is_active` flag set to true
   - Tokens can be deactivated without deletion

4. **Expiration**:
   - If token has an `expires_at` value, it must be in the future
   - Non-expiring tokens will have a null `expires_at` value

5. **Rate Limiting**:
   - If token has a `max_requests` value, the `request_count` must be below this limit
   - Non-limited tokens will have a null `max_requests` value

## Token Generation

1. **Generation Process**:
   - Create a UUID v7 for built-in time ordering
   - Convert to URL-safe base64 encoding
   - Add "tkn_" prefix
   - Validate uniqueness in the database

2. **Configuration Options**:
   - Project ID (required)
   - Expiration duration (optional, default to no expiration)
   - Max requests (optional, default to unlimited)
   - Initial active status (optional, default to active)

## Token Revocation

1. **Soft Revocation**:
   - Set `is_active` to false (preferred method)
   - Immediately invalidates the token while preserving history

2. **Hard Deletion**:
   - Physical deletion from the database (special cases only)
   - Removes all record of the token's existence

## Performance Considerations

1. **Caching**:
   - Implement in-memory LRU cache for frequently validated tokens
   - Cache invalidation on token updates/revocation
   - Configurable cache size and TTL

2. **Batch Operations**:
   - Support for bulk token generation
   - Support for bulk token validation
   - Support for batch token revocation

## Security Considerations

1. **Token Entropy**:
   - UUIDv7 provides 122 bits of entropy (74 random bits + 48 timestamp bits)
   - Impossible to brute force or predict

2. **Protection Mechanisms**:
   - Rate limiting on validation attempts (prevent brute force)
   - Automatic cleanup of expired tokens
   - No token pattern in URLs (use POST body instead of URL params)

## Implementation Notes

The token management package will be separate from the database layer, providing:

1. Functions for:
   - Token generation
   - Token validation
   - Token format verification
   - Expiration calculation
   - Rate limiting logic

2. Interfaces for:
   - Storage backend (default: database, can be extended)
   - Caching mechanisms (default: in-memory LRU)
   - Custom validation rules

3. The system will be designed for:
   - High performance under load
   - Thread safety for concurrent access
   - Testability with mocks
   - Clear error messages