#!/bin/sh
set -e

# Fix permissions on /app/logs if needed
if [ -d /app/logs ]; then
  chown -R appuser:appgroup /app/logs /app/data /app/config /app/certs || true
fi

CMD=${CMD:-"llm-proxy"}

# Exec llm-proxy with all arguments
exec "$CMD" "$@"
