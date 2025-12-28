#!/usr/bin/env bash
# Helm chart publishing script for LLM Proxy
# Usage: helm-publish.sh <command> [args...]
#
# Commands:
#   extract-version <github-ref>     - Extract version from GitHub ref
#   update-chart <version> <app-version> - Update Chart.yaml with versions
#   package <chart-path> <output-dir> - Package chart with dependencies
#   push <chart-package> <registry>   - Push chart to OCI registry

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "${SCRIPT_DIR}/../.." && pwd)"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

log_info() {
    echo -e "${GREEN}[INFO]${NC} $*"
}

log_warn() {
    echo -e "${YELLOW}[WARN]${NC} $*"
}

log_error() {
    echo -e "${RED}[ERROR]${NC} $*" >&2
}

# Extract version from GitHub ref
cmd_extract_version() {
    local github_ref="${1:-}"
    
    if [[ -z "$github_ref" ]]; then
        log_error "GitHub ref is required"
        exit 1
    fi
    
    local version=""
    
    if [[ "$github_ref" == refs/tags/v* ]]; then
        # Extract version from tag (e.g., refs/tags/v1.2.3 -> 1.2.3)
        version="${github_ref#refs/tags/v}"
        log_info "Extracted version from tag: $version"
    else
        # For non-tag refs, use test version
        version="0.0.0-test"
        log_info "Using test version: $version"
    fi
    
    echo "$version"
}

# Update Chart.yaml with version and appVersion
cmd_update_chart() {
    local chart_version="${1:-}"
    local app_version="${2:-}"
    local chart_path="${3:-deploy/helm/llm-proxy}"
    
    if [[ -z "$chart_version" ]] || [[ -z "$app_version" ]]; then
        log_error "Chart version and app version are required"
        exit 1
    fi
    
    local chart_yaml="${REPO_ROOT}/${chart_path}/Chart.yaml"
    
    if [[ ! -f "$chart_yaml" ]]; then
        log_error "Chart.yaml not found at: $chart_yaml"
        exit 1
    fi
    
    log_info "Updating Chart.yaml: version=$chart_version, appVersion=$app_version"
    
    # Use pipe delimiter to handle special characters
    sed -i "s|^version:.*|version: $chart_version|" "$chart_yaml"
    sed -i "s|^appVersion:.*|appVersion: \"$app_version\"|" "$chart_yaml"
    
    log_info "Updated Chart.yaml:"
    cat "$chart_yaml"
}

# Package chart with dependencies
cmd_package() {
    local chart_path="${1:-deploy/helm/llm-proxy}"
    local output_dir="${2:-/tmp/charts}"
    
    local full_chart_path="${REPO_ROOT}/${chart_path}"
    
    if [[ ! -d "$full_chart_path" ]]; then
        log_error "Chart directory not found: $full_chart_path"
        exit 1
    fi
    
    mkdir -p "$output_dir"
    
    log_info "Building chart dependencies..."
    # Build dependencies if Chart.yaml has dependencies
    if grep -q "^dependencies:" "${full_chart_path}/Chart.yaml"; then
        helm dependency build "$full_chart_path" || {
            log_warn "Dependency build failed, continuing without dependencies"
        }
    fi
    
    log_info "Packaging chart from: $chart_path"
    helm package "$full_chart_path" -d "$output_dir"
    
    log_info "Packaged charts:"
    ls -lh "$output_dir"
}

# Push chart to OCI registry
cmd_push() {
    local chart_package="${1:-}"
    local registry="${2:-}"
    
    if [[ -z "$chart_package" ]] || [[ -z "$registry" ]]; then
        log_error "Chart package and registry are required"
        exit 1
    fi
    
    if [[ ! -f "$chart_package" ]]; then
        log_error "Chart package not found: $chart_package"
        exit 1
    fi
    
    log_info "Pushing chart to registry: $registry"
    helm push "$chart_package" "$registry"
    
    log_info "Successfully pushed chart to $registry"
}

# Main command dispatcher
main() {
    local command="${1:-}"
    
    if [[ -z "$command" ]]; then
        log_error "Command is required"
        echo "Usage: helm-publish.sh <command> [args...]"
        echo ""
        echo "Commands:"
        echo "  extract-version <github-ref>              - Extract version from GitHub ref"
        echo "  update-chart <version> <app-version>      - Update Chart.yaml with versions"
        echo "  package <chart-path> <output-dir>         - Package chart with dependencies"
        echo "  push <chart-package> <registry>           - Push chart to OCI registry"
        exit 1
    fi
    
    shift
    
    case "$command" in
        extract-version)
            cmd_extract_version "$@"
            ;;
        update-chart)
            cmd_update_chart "$@"
            ;;
        package)
            cmd_package "$@"
            ;;
        push)
            cmd_push "$@"
            ;;
        *)
            log_error "Unknown command: $command"
            exit 1
            ;;
    esac
}

main "$@"
