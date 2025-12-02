#!/bin/bash
# PostgreSQL Integration Test Script
# This script starts PostgreSQL via Docker Compose and runs integration tests
# against a real PostgreSQL instance.
#
# Usage:
#   ./scripts/run-postgres-integration.sh          # Run integration tests
#   ./scripts/run-postgres-integration.sh teardown # Stop and remove containers
#   ./scripts/run-postgres-integration.sh logs     # Show PostgreSQL logs
#
# Prerequisites:
#   - Docker and Docker Compose installed
#   - Go 1.23+ installed
#
# Environment Variables:
#   POSTGRES_PASSWORD - PostgreSQL password (default: secret)
#   SKIP_BUILD        - Skip building Docker image if set
#   KEEP_RUNNING      - Keep containers running after tests

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
cd "$PROJECT_ROOT"

# Configuration
COMPOSE_PROFILE="postgres"
POSTGRES_PASSWORD="${POSTGRES_PASSWORD:-secret}"
POSTGRES_HOST="${POSTGRES_HOST:-localhost}"
POSTGRES_PORT="${POSTGRES_PORT:-5432}"
POSTGRES_USER="llmproxy"
POSTGRES_DB="llmproxy"
MAX_WAIT_SECONDS=60

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

log_info() {
    echo -e "${GREEN}[INFO]${NC} $1"
}

log_warn() {
    echo -e "${YELLOW}[WARN]${NC} $1"
}

log_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

# Cleanup function
cleanup() {
    local exit_code=$?
    if [ -z "$KEEP_RUNNING" ]; then
        log_info "Stopping PostgreSQL container..."
        docker compose --profile "$COMPOSE_PROFILE" down -v 2>/dev/null || true
    else
        log_warn "KEEP_RUNNING is set - containers left running"
        log_info "To stop: docker compose --profile postgres down -v"
    fi
    exit $exit_code
}

# Start PostgreSQL with Docker Compose
start_postgres() {
    log_info "Starting PostgreSQL container..."
    
    export POSTGRES_PASSWORD
    
    # Start only the postgres container (not the full app)
    docker compose --profile "$COMPOSE_PROFILE" up -d postgres 2>&1 | grep -v "^time=" || true
    
    log_info "Waiting for PostgreSQL to be ready..."
    wait_for_postgres
}

# Wait for PostgreSQL to be healthy
wait_for_postgres() {
    local elapsed=0
    
    while [ $elapsed -lt $MAX_WAIT_SECONDS ]; do
        if docker compose --profile "$COMPOSE_PROFILE" exec -T postgres \
           pg_isready -U "$POSTGRES_USER" -d "$POSTGRES_DB" &>/dev/null; then
            log_info "PostgreSQL is ready!"
            return 0
        fi
        
        sleep 1
        elapsed=$((elapsed + 1))
        
        if [ $((elapsed % 10)) -eq 0 ]; then
            log_info "Still waiting for PostgreSQL... ($elapsed seconds)"
        fi
    done
    
    log_error "PostgreSQL did not become ready within $MAX_WAIT_SECONDS seconds"
    docker compose --profile "$COMPOSE_PROFILE" logs postgres
    return 1
}

# Run PostgreSQL integration tests
run_tests() {
    log_info "Running PostgreSQL integration tests..."
    
    # Build the database URL
    local database_url="postgres://${POSTGRES_USER}:${POSTGRES_PASSWORD}@${POSTGRES_HOST}:${POSTGRES_PORT}/${POSTGRES_DB}?sslmode=disable"
    
    # Export environment variables for the tests
    export DATABASE_URL="$database_url"
    export DB_DRIVER="postgres"
    export TEST_POSTGRES_URL="$database_url"
    
    # Run integration tests with postgres build tag
    # The -tags=postgres,integration enables both PostgreSQL code and integration tests
    go test -v -race -tags=postgres,integration \
        -timeout=5m \
        ./internal/database/... \
        ./test/... \
        2>&1 | tee /tmp/postgres-integration-test.log
    
    local test_exit_code=${PIPESTATUS[0]}
    
    if [ $test_exit_code -eq 0 ]; then
        log_info "All PostgreSQL integration tests passed!"
    else
        log_error "Some PostgreSQL integration tests failed!"
    fi
    
    return $test_exit_code
}

# Show PostgreSQL logs
show_logs() {
    docker compose --profile "$COMPOSE_PROFILE" logs postgres
}

# Teardown containers
teardown() {
    log_info "Stopping and removing PostgreSQL containers..."
    docker compose --profile "$COMPOSE_PROFILE" down -v
    log_info "Cleanup complete"
}

# Main execution
case "${1:-}" in
    teardown|down|stop)
        teardown
        ;;
    logs)
        show_logs
        ;;
    start)
        start_postgres
        log_info "PostgreSQL is running. Connection string:"
        echo "  postgres://${POSTGRES_USER}:${POSTGRES_PASSWORD}@${POSTGRES_HOST}:${POSTGRES_PORT}/${POSTGRES_DB}?sslmode=disable"
        log_info "To stop: ./scripts/run-postgres-integration.sh teardown"
        ;;
    test)
        # Just run tests (assumes PostgreSQL is already running)
        run_tests
        ;;
    *)
        # Default: full integration test run
        trap cleanup EXIT
        start_postgres
        run_tests
        ;;
esac
