#!/usr/bin/env bash

set -euo pipefail

RED='\033[0;31m'
GREEN='\033[0;32m'
NC='\033[0m'

SEMVER_TAG_PATTERN='^v([0-9]+)\.([0-9]+)\.([0-9]+)$'
STABLE_TAG_PATTERN='^v([0-9]+)\.([0-9]+)\.([0-9]+)-stable$'

log_info() {
    echo -e "${GREEN}[INFO]${NC} $*" >&2
}

log_error() {
    echo -e "${RED}[ERROR]${NC} $*" >&2
}

normalize_tag() {
    local raw_tag="${1:-}"

    if [[ -z "$raw_tag" ]]; then
        log_error "Tag is required"
        exit 1
    fi

    if [[ "$raw_tag" == refs/tags/* ]]; then
        raw_tag="${raw_tag#refs/tags/}"
    fi

    echo "$raw_tag"
}

validate_tag() {
    local tag
    tag="$(normalize_tag "${1:-}")"

    if [[ "$tag" =~ $SEMVER_TAG_PATTERN ]] || [[ "$tag" =~ $STABLE_TAG_PATTERN ]]; then
        echo "$tag"
        return 0
    fi

    log_error "Invalid release tag '$tag'. Supported patterns are 'vMAJOR.MINOR.PATCH' and 'vMAJOR.MINOR.PATCH-stable'."
    exit 1
}

cmd_classify() {
    local tag
    tag="$(validate_tag "${1:-}")"

    if [[ "$tag" =~ -stable$ ]]; then
        log_info "Validated stable release tag: $tag"
        echo "stable"
        return 0
    fi

    log_info "Validated version tag: $tag"
    echo "version"
}

cmd_version() {
    local tag
    tag="$(validate_tag "${1:-}")"
    tag="${tag#v}"
    tag="${tag%-stable}"
    echo "$tag"
}

main() {
    local command="${1:-}"

    if [[ -z "$command" ]]; then
        log_error "Command is required"
        echo "Usage: release-tag.sh <classify|version> <tag-or-ref>"
        exit 1
    fi

    shift

    case "$command" in
        classify)
            cmd_classify "$@"
            ;;
        version)
            cmd_version "$@"
            ;;
        *)
            log_error "Unknown command: $command"
            echo "Usage: release-tag.sh <classify|version> <tag-or-ref>"
            exit 1
            ;;
    esac
}

main "$@"
