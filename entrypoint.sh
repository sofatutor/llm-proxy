#!/bin/sh
set -e

# Fix permissions on writable dirs, but only when running as root.
# In Kubernetes, we typically run as non-root and rely on fsGroup for volume permissions.
if [ "$(id -u)" = "0" ] && [ -d /app/logs ]; then
  chown -R appuser:appgroup /app/logs /app/data /app/config /app/certs || true
fi

CMD=${CMD:-"/app/bin/llm-proxy"}

# Exec llm-proxy with all arguments
exec "$CMD" "$@"
