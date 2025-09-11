#!/bin/bash

# Start E2E test servers for LLM Proxy
# This script starts both the management API and Admin UI servers

set -e

MGMT_PORT=${MGMT_PORT:-8098}
ADMIN_PORT=${ADMIN_PORT:-8099}
MANAGEMENT_TOKEN=${MANAGEMENT_TOKEN:-e2e-management-token}

echo "Starting E2E test servers on ports $MGMT_PORT and $ADMIN_PORT..."
echo "Management token: $MANAGEMENT_TOKEN"

# Kill any process currently listening on the given TCP port (best-effort)
kill_port() {
    local port="$1"
    if lsof -nti tcp:"$port" >/dev/null 2>&1; then
        echo "Killing processes on port $port..."
        # Try graceful TERM first
        lsof -nti tcp:"$port" | xargs -r kill 2>/dev/null || true
        sleep 1
        # Force kill if still present
        if lsof -nti tcp:"$port" >/dev/null 2>&1; then
            lsof -nti tcp:"$port" | xargs -r kill -9 2>/dev/null || true
        fi
    fi
}

# Ensure tmp directory exists for ephemeral DBs
mkdir -p ./tmp

# Proactively free required ports before starting servers
kill_port "$MGMT_PORT"
kill_port "$ADMIN_PORT"

export MGMT_BASE_URL="http://localhost:$MGMT_PORT"
export ADMIN_BASE_URL="http://localhost:$ADMIN_PORT"

echo "Starting LLM Proxy management API server on port $MGMT_PORT..."
DB_PATH="./tmp/e2e-db-$$.sqlite"
export DATABASE_PATH="$DB_PATH"
LOG_LEVEL="info" MANAGEMENT_TOKEN="$MANAGEMENT_TOKEN" LLM_PROXY_EVENT_BUS="in-memory" ./bin/llm-proxy server --addr ":$MGMT_PORT" &
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
    # Best-effort: ensure ports are free after shutting down
    kill_port "$MGMT_PORT"
    kill_port "$ADMIN_PORT"
    # Remove this run's DB and any leftover E2E DBs from prior runs (including WAL/SHM)
    rm -f ./tmp/e2e-db-*.sqlite* 2>/dev/null || true
}

# Set trap to cleanup on script exit
trap cleanup EXIT INT TERM

echo "Servers started. Management API PID: $MGMT_PID, Admin UI PID: $ADMIN_PID"
echo "Management API: $MGMT_BASE_URL"
echo "Admin UI: $ADMIN_BASE_URL"

# Wait for both processes
wait