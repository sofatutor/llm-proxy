#!/usr/bin/env bash
# Log Search Helper for LLM Proxy
# Usage: ./scripts/log-search.sh [OPTIONS] [FILE]
# 
# Filters JSON logs from LLM Proxy using jq.
# Reads from stdin if no file is specified.

set -euo pipefail

# Default values
REQUEST_ID=""
CORRELATION_ID=""
PROJECT_ID=""
SHOW_ERRORS=false
SLOW_MS=""
STATUS_CODE=""
PATH_FILTER=""
INPUT_FILE=""

# Help message
show_help() {
    cat << 'EOF'
Log Search Helper for LLM Proxy

USAGE:
    ./scripts/log-search.sh [OPTIONS] [FILE]
    cat logs.json | ./scripts/log-search.sh [OPTIONS]

OPTIONS:
    --request-id ID       Filter by request ID
    --correlation-id ID   Filter by correlation ID
    --project ID          Filter by project ID
    --errors              Show only error logs
    --slow MS             Show requests slower than MS milliseconds
    --status CODE         Filter by HTTP status code (exact match)
    --status-min CODE     Filter by minimum status code (>=)
    --path PATH           Filter by request path
    --file FILE           Read from file instead of stdin
    -h, --help            Show this help message

EXAMPLES:
    # Filter by request ID
    ./scripts/log-search.sh --request-id 01938a7c-4d5e-7f8a-9b0c-1d2e3f4a5b6c

    # Find all errors
    ./scripts/log-search.sh --errors < logs.json

    # Find slow requests (>500ms)
    ./scripts/log-search.sh --slow 500 --file logs.json

    # Find logs for a project
    ./scripts/log-search.sh --project proj-abc123

    # Combine filters
    ./scripts/log-search.sh --errors --project proj-abc123 < logs.json

    # Real-time log monitoring
    tail -f /var/log/llm-proxy.log | ./scripts/log-search.sh --errors

    # Docker logs
    docker logs llm-proxy 2>&1 | ./scripts/log-search.sh --slow 1000

NOTES:
    - Requires jq to be installed
    - Expects JSON-formatted log lines
    - Multiple filters are combined with AND logic
EOF
}

# Check for jq
if ! command -v jq &> /dev/null; then
    echo "Error: jq is required but not installed." >&2
    echo "Install with: brew install jq (macOS) or apt install jq (Debian/Ubuntu)" >&2
    exit 1
fi

# Parse arguments
while [[ $# -gt 0 ]]; do
    case $1 in
        --request-id)
            REQUEST_ID="$2"
            shift 2
            ;;
        --correlation-id)
            CORRELATION_ID="$2"
            shift 2
            ;;
        --project)
            PROJECT_ID="$2"
            shift 2
            ;;
        --errors)
            SHOW_ERRORS=true
            shift
            ;;
        --slow)
            SLOW_MS="$2"
            shift 2
            ;;
        --status)
            STATUS_CODE="$2"
            shift 2
            ;;
        --status-min)
            STATUS_MIN="$2"
            shift 2
            ;;
        --path)
            PATH_FILTER="$2"
            shift 2
            ;;
        --file)
            INPUT_FILE="$2"
            shift 2
            ;;
        -h|--help)
            show_help
            exit 0
            ;;
        -*)
            echo "Unknown option: $1" >&2
            show_help
            exit 1
            ;;
        *)
            # Treat positional argument as input file
            if [[ -z "$INPUT_FILE" ]]; then
                INPUT_FILE="$1"
            fi
            shift
            ;;
    esac
done

# Build jq filter
FILTERS=()

if [[ -n "$REQUEST_ID" ]]; then
    FILTERS+=(".request_id == \"$REQUEST_ID\"")
fi

if [[ -n "$CORRELATION_ID" ]]; then
    FILTERS+=(".correlation_id == \"$CORRELATION_ID\"")
fi

if [[ -n "$PROJECT_ID" ]]; then
    FILTERS+=(".project_id == \"$PROJECT_ID\"")
fi

if [[ "$SHOW_ERRORS" == "true" ]]; then
    FILTERS+=(".level == \"error\"")
fi

if [[ -n "$SLOW_MS" ]]; then
    FILTERS+=(".duration_ms > $SLOW_MS")
fi

if [[ -n "$STATUS_CODE" ]]; then
    FILTERS+=(".status_code == $STATUS_CODE")
fi

if [[ -n "${STATUS_MIN:-}" ]]; then
    FILTERS+=(".status_code >= $STATUS_MIN")
fi

if [[ -n "$PATH_FILTER" ]]; then
    FILTERS+=(".path == \"$PATH_FILTER\"")
fi

# Combine filters with AND
if [[ ${#FILTERS[@]} -eq 0 ]]; then
    JQ_FILTER="."
else
    # Join filters with " and "
    COMBINED=""
    for i in "${!FILTERS[@]}"; do
        if [[ $i -eq 0 ]]; then
            COMBINED="${FILTERS[$i]}"
        else
            COMBINED="$COMBINED and ${FILTERS[$i]}"
        fi
    done
    JQ_FILTER="select($COMBINED)"
fi

# Execute jq
if [[ -n "$INPUT_FILE" ]]; then
    if [[ ! -f "$INPUT_FILE" ]]; then
        echo "Error: File not found: $INPUT_FILE" >&2
        exit 1
    fi
    jq -c "$JQ_FILTER" "$INPUT_FILE"
else
    jq -c "$JQ_FILTER"
fi
