#!/bin/bash
# MySQL Integration Test Script
# This script starts MySQL via Docker Compose and runs integration tests
# against a real MySQL instance.
#
# Usage:
#   ./scripts/run-mysql-integration.sh          # Run integration tests
#   ./scripts/run-mysql-integration.sh teardown # Stop and remove containers
#   ./scripts/run-mysql-integration.sh logs     # Show MySQL logs
#
# Prerequisites:
#   - Docker and Docker Compose installed
#   - Go 1.23+ installed
#
# Environment Variables:
#   MYSQL_PASSWORD     - MySQL password (default: secret)
#   SKIP_BUILD         - Skip building Docker image if set
#   KEEP_RUNNING       - Keep containers running after tests

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
cd "$PROJECT_ROOT"

# Configuration
COMPOSE_PROFILE="mysql-test"
MYSQL_PASSWORD="${MYSQL_PASSWORD:-secret}"
MYSQL_ROOT_PASSWORD="${MYSQL_ROOT_PASSWORD:-secret}"
MYSQL_HOST="${MYSQL_HOST:-localhost}"
MYSQL_PORT="${MYSQL_PORT:-33306}"
MYSQL_USER="llmproxy"
MYSQL_DB="llmproxy"
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
        log_info "Stopping MySQL container..."
        docker compose --profile "$COMPOSE_PROFILE" down -v 2>/dev/null || true
    else
        log_warn "KEEP_RUNNING is set - containers left running"
        log_info "To stop: docker compose --profile mysql-test down -v"
    fi
    exit $exit_code
}

# Start MySQL with Docker Compose
start_mysql() {
    log_info "Starting MySQL container..."
    
    export MYSQL_PASSWORD
    export MYSQL_ROOT_PASSWORD
    
    # Start only the mysql-test container (not the full app)
    docker compose --profile "$COMPOSE_PROFILE" up -d mysql-test 2>&1 | grep -v "^time=" || true
    
    log_info "Waiting for MySQL to be ready..."
    wait_for_mysql
}

# Wait for MySQL to be healthy
wait_for_mysql() {
    local elapsed=0
    
    while [ $elapsed -lt $MAX_WAIT_SECONDS ]; do
        if docker compose --profile "$COMPOSE_PROFILE" exec -T mysql-test \
           mysqladmin ping -h localhost &>/dev/null; then
            log_info "MySQL is ready!"
            return 0
        fi
        
        sleep 1
        elapsed=$((elapsed + 1))
        
        if [ $((elapsed % 10)) -eq 0 ]; then
            log_info "Still waiting for MySQL... ($elapsed seconds)"
        fi
    done
    
    log_error "MySQL did not become ready within $MAX_WAIT_SECONDS seconds"
    docker compose --profile "$COMPOSE_PROFILE" logs mysql-test
    return 1
}

# Run MySQL integration tests
run_tests() {
    log_info "Running MySQL integration tests..."
    
    # Build the database URL
    local database_url="${MYSQL_USER}:${MYSQL_PASSWORD}@tcp(${MYSQL_HOST}:${MYSQL_PORT})/${MYSQL_DB}?parseTime=true"
    
    # Export environment variables for the tests
    export DATABASE_URL="$database_url"
    export DB_DRIVER="mysql"
    export TEST_MYSQL_URL="$database_url"
    
    # Run integration tests with mysql build tag
    # The -tags=mysql,integration enables both MySQL code and integration tests
    go test -v -race -tags=mysql,integration \
        -timeout=5m \
        ./internal/database/... \
        ./test/... \
        2>&1 | tee /tmp/mysql-integration-test.log
    
    local test_exit_code=${PIPESTATUS[0]}
    
    if [ $test_exit_code -eq 0 ]; then
        log_info "All MySQL integration tests passed!"
    else
        log_error "Some MySQL integration tests failed!"
    fi
    
    return $test_exit_code
}

# Show MySQL logs
show_logs() {
    docker compose --profile "$COMPOSE_PROFILE" logs mysql-test
}

# Teardown containers
teardown() {
    log_info "Stopping and removing MySQL containers..."
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
        start_mysql
        log_info "MySQL is running. Connection string:"
        echo "  ${MYSQL_USER}:******@tcp(${MYSQL_HOST}:${MYSQL_PORT})/${MYSQL_DB}?parseTime=true"
        log_info "To stop: ./scripts/run-mysql-integration.sh teardown"
        ;;
    test)
        # Just run tests (assumes MySQL is already running)
        run_tests
        ;;
    *)
        # Default: full integration test run
        trap cleanup EXIT
        start_mysql
        run_tests
        ;;
esac
