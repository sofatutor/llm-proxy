#!/usr/bin/env bash
set -euo pipefail

# Usage:
#   scripts/run-coverage.sh ci [--coverpkg PATTERN] [--outfile FILE]
#   scripts/run-coverage.sh watch [--coverpkg PATTERN] [--outfile FILE]
#   scripts/run-coverage.sh dev   [--coverpkg PATTERN] [--outfile FILE]
#   scripts/run-coverage.sh html  [--in FILE] [--out HTML_FILE]
#
# Defaults:
#   --coverpkg default: ./internal/...
#   --outfile default (ci): coverage_ci.txt
#   --outfile default (watch/dev): coverage_dev.out

MODE=${1:-}
shift || true

COVERPKG="./internal/..."
OUTFILE_CI="coverage_ci.txt"
OUTFILE_DEV="coverage_dev.out"
INFILE_HTML="coverage.out"
OUTHTML="coverage.html"
VIEW_HTML=0

while [[ $# -gt 0 ]]; do
  case "$1" in
    --coverpkg)
      COVERPKG="$2"; shift 2 ;;
    --outfile)
      OUTFILE_CI="$2"; OUTFILE_DEV="$2"; shift 2 ;;
    --in)
      INFILE_HTML="$2"; shift 2 ;;
    --out)
      OUTHTML="$2"; shift 2 ;;
    --view)
      VIEW_HTML=1; shift 1 ;;
    *)
      echo "Unknown argument: $1" >&2; exit 2 ;;
  esac
done

gotestsum_bin=$(command -v gotestsum 2>/dev/null || echo "$HOME/go/bin/gotestsum")
if [[ ! -x "$gotestsum_bin" ]]; then
  echo "gotestsum not found. Install with: go install gotest.tools/gotestsum@latest" >&2
  exit 1
fi

case "$MODE" in
  ci)
    "$gotestsum_bin" --format standard-quiet -- \
      -v -race -parallel=4 -coverprofile="$OUTFILE_CI" -covermode=atomic -coverpkg="$COVERPKG" ./...
    go tool cover -func="$OUTFILE_CI" | tail -n 1
    ;;

  watch|dev)
    # Uppercasing via tr for portability (avoid ${VAR^^} which may not be available)
    MODE_UPPER=$(printf "%s" "$MODE" | tr '[:lower:]' '[:upper:]')
    echo "🧪 ${MODE_UPPER} Mode: Running tests + coverage, then watching..."
    # Initial run (strip per-package coverage noise)
    "$gotestsum_bin" --format pkgname --hide-summary=skipped,output -- \
      -coverprofile="$OUTFILE_DEV" -covermode=atomic -coverpkg="$COVERPKG" ./... \
      | sed -E 's/ \(coverage: [^\)]+\)//g'
    echo "📊 Coverage: $(go tool cover -func="$OUTFILE_DEV" | tail -n 1 | awk '{print $3}')"
    echo "👀 Watching for changes (Ctrl+C to exit)..."
    # Subsequent runs with coverage summary after each run
    "$gotestsum_bin" --watch --rerun-fails --format pkgname --hide-summary=skipped,output \
      --post-run-command='./scripts/show-coverage.sh' -- \
      -coverprofile="$OUTFILE_DEV" -covermode=atomic -coverpkg="$COVERPKG" ./... \
      | sed -E 's/ \(coverage: [^\)]+\)//g'
    ;;

  html)
    go tool cover -html="$INFILE_HTML" -o "$OUTHTML"
    echo "Wrote $OUTHTML"
    if [[ "$VIEW_HTML" -eq 1 ]]; then
      if command -v open >/dev/null 2>&1; then
        nohup open "$OUTHTML" >/dev/null 2>&1 &
      elif command -v xdg-open >/dev/null 2>&1; then
        nohup xdg-open "$OUTHTML" >/dev/null 2>&1 &
      else
        echo "No system opener found. Please open $OUTHTML manually."
      fi
    fi
    ;;

  *)
    echo "Usage: scripts/run-coverage.sh {ci|watch|dev|html} [options]" >&2
    exit 2
    ;;
esac
