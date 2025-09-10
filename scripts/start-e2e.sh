#!/bin/bash

# Start E2E test servers for LLM Proxy
# This script starts both the management API and Admin UI servers

set -e

MGMT_PORT=${MGMT_PORT:-8080}
ADMIN_PORT=${ADMIN_PORT:-8099}
MANAGEMENT_TOKEN=${MANAGEMENT_TOKEN:-e2e-management-token}

echo "Starting LLM Proxy management API server on port $MGMT_PORT..."
MANAGEMENT_TOKEN="$MANAGEMENT_TOKEN" LLM_PROXY_EVENT_BUS="in-memory" ./bin/llm-proxy server --addr ":$MGMT_PORT" --debug &
MGMT_PID=$!

sleep 3

echo "Starting Admin UI server on port $ADMIN_PORT..."
./bin/llm-proxy admin --listen ":$ADMIN_PORT" --api-base-url "http://localhost:$MGMT_PORT" --management-token "$MANAGEMENT_TOKEN" &
ADMIN_PID=$!

# Function to cleanup on exit
cleanup() {
    echo "Shutting down servers..."
    if [ ! -z "$MGMT_PID" ]; then
        kill $MGMT_PID 2>/dev/null || true
    fi
    if [ ! -z "$ADMIN_PID" ]; then
        kill $ADMIN_PID 2>/dev/null || true
    fi
}

# Set trap to cleanup on script exit
trap cleanup EXIT INT TERM

echo "Servers started. Management API PID: $MGMT_PID, Admin UI PID: $ADMIN_PID"
echo "Management API: http://localhost:$MGMT_PORT"
echo "Admin UI: http://localhost:$ADMIN_PORT"

# Wait for both processes
wait