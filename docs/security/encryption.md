# Encryption at Rest

This document describes the encryption-at-rest security features for protecting sensitive data in the LLM Proxy database.

## Overview

The LLM Proxy encrypts sensitive data when the `ENCRYPTION_KEY` environment variable is set:

- **Bearer tokens**: Hashed with SHA-256 for fast lookups (irreversible)
- **API keys**: Encrypted with AES-256-GCM (reversible for upstream calls)

This protects credentials even if the database is compromised.

## Configuration

### Generating an Encryption Key

```bash
# Generate a secure 32-byte key
openssl rand -base64 32
```

### Setting the Key

Add to your `.env` file or environment:

```bash
ENCRYPTION_KEY=<your-base64-encoded-key>
```

⚠️ **Important**: Store this key securely! Without it, you cannot decrypt API keys or verify tokens.

### Fail-Fast Enforcement

By default, if `ENCRYPTION_KEY` is not set, the proxy still starts but stores sensitive data in plaintext (and logs a warning). For production deployments, you can force a hard failure instead:

```bash
REQUIRE_ENCRYPTION_KEY=true
```

When `REQUIRE_ENCRYPTION_KEY=true` and `ENCRYPTION_KEY` is missing, the server exits on startup.

## Migration

### Encrypting Existing Data

If you have plaintext data in the database:

```bash
# Set the encryption key first
export ENCRYPTION_KEY=$(openssl rand -base64 32)

# Run the migration (idempotent - safe to run multiple times)
llm-proxy migrate encrypt
```

### Checking Encryption Status

```bash
llm-proxy migrate encrypt-status
```
