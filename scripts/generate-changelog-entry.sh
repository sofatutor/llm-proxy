#!/usr/bin/env bash
#
# generate-changelog-entry.sh
#
# Generates a changelog entry for a PR using the OpenAI API and merges it into CHANGELOG.md.
#
# Usage:
#   ./generate-changelog-entry.sh [--dry-run]           # Uses current branch's PR
#   ./generate-changelog-entry.sh [PR_NUMBER] [--dry-run]
#
# Options:
#   --dry-run    Print the entry without modifying CHANGELOG.md
#
# Required environment variables:
#   OPENAI_API_KEY  - OpenAI API key
#
# Optional environment variables:
#   CHANGELOG_PATH  - Path to CHANGELOG.md (default: CHANGELOG.md)
#
# Requires: gh (GitHub CLI), jq, curl
#
set -euo pipefail

# Parse arguments
DRY_RUN=false
PR_NUMBER=""

while [[ $# -gt 0 ]]; do
  case $1 in
    --dry-run)
      DRY_RUN=true
      shift
      ;;
    *)
      if [[ -z "$PR_NUMBER" && "$1" =~ ^[0-9]+$ ]]; then
        PR_NUMBER="$1"
      fi
      shift
      ;;
  esac
done

# Validate required tools
for cmd in gh jq curl; do
  if ! command -v "$cmd" &> /dev/null; then
    echo "Error: $cmd is required but not installed" >&2
    exit 1
  fi
done

# Validate required environment variables
if [[ -z "${OPENAI_API_KEY:-}" ]]; then
  echo "Error: OPENAI_API_KEY is required" >&2
  exit 1
fi

# Defaults
CHANGELOG_PATH="${CHANGELOG_PATH:-CHANGELOG.md}"

# Fetch PR metadata using gh CLI
if [[ -n "$PR_NUMBER" ]]; then
  echo "Fetching PR #${PR_NUMBER} metadata..." >&2
  PR_JSON=$(gh pr view "$PR_NUMBER" --json number,title,body,additions,deletions,changedFiles,files,url)
else
  echo "Fetching current branch PR metadata..." >&2
  PR_JSON=$(gh pr view --json number,title,body,additions,deletions,changedFiles,files,url)
  PR_NUMBER=$(echo "$PR_JSON" | jq -r '.number')
fi

PR_TITLE=$(echo "$PR_JSON" | jq -r '.title')
PR_BODY=$(echo "$PR_JSON" | jq -r '.body // "No description provided."')
PR_ADDITIONS=$(echo "$PR_JSON" | jq -r '.additions')
PR_DELETIONS=$(echo "$PR_JSON" | jq -r '.deletions')
PR_URL=$(echo "$PR_JSON" | jq -r '.url')
PR_FILES=$(echo "$PR_JSON" | jq -r '.files[].path' | head -50 | tr '\n' ', ' | sed 's/,$//')

# Build the prompt
read -r -d '' SYSTEM_PROMPT << 'SYSPROMPT' || true
You are a technical writer generating changelog entries for a Go-based LLM proxy project.

Generate a changelog entry in JSON format with the category and entry text.

Rules:
1. Output valid JSON only, no markdown code fences
2. Format: {"category": "Added|Changed|Fixed|Reverted", "entry": "- **Title** ([#NUMBER](URL)): Description."}
3. Category selection:
   - "Added" for new features, endpoints, commands, capabilities
   - "Changed" for modifications, refactors, documentation updates, improvements
   - "Fixed" for bug fixes, error corrections, security patches
   - "Reverted" for rollbacks or reverted changes
4. Title should be concise (2-6 words), derived from PR title but cleaned up
5. Description should be 1-3 sentences summarizing the key changes
6. Focus on WHAT changed and WHY it matters, not implementation details
7. Use technical but accessible language
8. Mention key features, new endpoints, config options, or breaking changes

Examples:
{"category": "Added", "entry": "- **Redis-Backed Distributed Rate Limiting** ([#151](url)): Horizontal scaling support with Redis-backed rate limiter using atomic Lua scripts. Configurable via `DISTRIBUTED_RATE_LIMIT_*` environment variables with automatic fallback to in-memory limiter."}
{"category": "Added", "entry": "- **Encryption for Sensitive Data at Rest** ([#160](url)): AES-256-GCM encryption for API keys with `enc:v1:` prefix. Added `llm-proxy migrate encrypt` CLI command. Configurable via `ENCRYPTION_KEY` environment variable."}
{"category": "Changed", "entry": "- **Phase 6 Documentation Polish** ([#108](url)): Updated CLI reference with cache purge command examples. Documented cache metrics counters in instrumentation docs."}
{"category": "Fixed", "entry": "- **HTTP Cache Header Ordering** ([#92](url)): Fixed cache status header ordering so modifyResponse correctly sets stored status. Restored proper cache opt-in logic honoring client Cache-Control headers."}
SYSPROMPT

# Truncate PR body if too long (keep first 3000 chars)
PR_BODY_TRUNCATED="${PR_BODY:0:3000}"
if [[ ${#PR_BODY} -gt 3000 ]]; then
  PR_BODY_TRUNCATED="${PR_BODY_TRUNCATED}... [truncated]"
fi

# Truncate files list if too long
PR_FILES_TRUNCATED="${PR_FILES:0:1500}"
if [[ ${#PR_FILES} -gt 1500 ]]; then
  PR_FILES_TRUNCATED="${PR_FILES_TRUNCATED}... [truncated]"
fi

read -r -d '' USER_PROMPT << USERPROMPT || true
Generate a changelog entry for this PR:

**PR Title:** ${PR_TITLE}
**PR Number:** #${PR_NUMBER}
**PR URL:** ${PR_URL}
**Stats:** +${PR_ADDITIONS} -${PR_DELETIONS} lines

**PR Description:**
${PR_BODY_TRUNCATED}

**Changed Files:**
${PR_FILES_TRUNCATED}
USERPROMPT

# Build JSON payload using jq for proper escaping
JSON_PAYLOAD=$(jq -n \
  --arg system "$SYSTEM_PROMPT" \
  --arg user "$USER_PROMPT" \
  '{
    model: "gpt-4o-mini",
    messages: [
      {role: "system", content: $system},
      {role: "user", content: $user}
    ],
    temperature: 0.3,
    max_tokens: 500,
    response_format: {type: "json_object"}
  }')

# Call OpenAI API
RESPONSE=$(curl -s -X POST "https://api.openai.com/v1/chat/completions" \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer ${OPENAI_API_KEY}" \
  -d "$JSON_PAYLOAD")

# Check for errors
if echo "$RESPONSE" | jq -e '.error' > /dev/null 2>&1; then
  ERROR_MSG=$(echo "$RESPONSE" | jq -r '.error.message')
  echo "Error from OpenAI API: $ERROR_MSG" >&2
  exit 1
fi

# Extract the generated JSON response
CONTENT=$(echo "$RESPONSE" | jq -r '.choices[0].message.content')

if [[ -z "$CONTENT" || "$CONTENT" == "null" ]]; then
  echo "Error: Failed to generate changelog entry" >&2
  exit 1
fi

# Parse category and entry from JSON response
CATEGORY=$(echo "$CONTENT" | jq -r '.category')
ENTRY=$(echo "$CONTENT" | jq -r '.entry')

if [[ -z "$ENTRY" || "$ENTRY" == "null" || -z "$CATEGORY" || "$CATEGORY" == "null" ]]; then
  echo "Error: Invalid JSON response from API" >&2
  echo "Response: $CONTENT" >&2
  exit 1
fi

# Validate category
if [[ ! "$CATEGORY" =~ ^(Added|Changed|Fixed|Reverted)$ ]]; then
  echo "Warning: Invalid category '$CATEGORY', defaulting to 'Changed'" >&2
  CATEGORY="Changed"
fi

# If dry-run, just output the entry and exit
if [[ "$DRY_RUN" == "true" ]]; then
  echo "Category: $CATEGORY"
  echo "$ENTRY"
  exit 0
fi

# Get today's date in the changelog format
TODAY=$(date +"%B %d, %Y")

# Merge entry into CHANGELOG.md
if [[ ! -f "$CHANGELOG_PATH" ]]; then
  echo "Error: $CHANGELOG_PATH not found" >&2
  exit 1
fi

# Check if today's date section exists
if grep -q "## $TODAY" "$CHANGELOG_PATH"; then
  # Date section exists, check if category exists under it
  if awk "/## $TODAY/,/^## /" "$CHANGELOG_PATH" | grep -q "### $CATEGORY"; then
    # Category exists, append entry after the category header
    awk -v date="## $TODAY" -v cat="### $CATEGORY" -v entry="$ENTRY" '
      $0 ~ date { in_date=1 }
      in_date && $0 ~ cat { print; getline; print entry; in_date=0; next }
      { print }
    ' "$CHANGELOG_PATH" > "${CHANGELOG_PATH}.tmp" && mv "${CHANGELOG_PATH}.tmp" "$CHANGELOG_PATH"
  else
    # Category doesn't exist, add it after the date header
    awk -v date="## $TODAY" -v cat="### $CATEGORY" -v entry="$ENTRY" '
      $0 ~ date { print; print ""; print cat; print ""; print entry; next }
      { print }
    ' "$CHANGELOG_PATH" > "${CHANGELOG_PATH}.tmp" && mv "${CHANGELOG_PATH}.tmp" "$CHANGELOG_PATH"
  fi
else
  # Date section doesn't exist, add new section after the --- separator
  awk -v date="## $TODAY" -v cat="### $CATEGORY" -v entry="$ENTRY" '
    /^---$/ && !done { print; print ""; print date; print ""; print cat; print ""; print entry; done=1; next }
    { print }
  ' "$CHANGELOG_PATH" > "${CHANGELOG_PATH}.tmp" && mv "${CHANGELOG_PATH}.tmp" "$CHANGELOG_PATH"
fi

echo "Added to $CHANGELOG_PATH under $TODAY / $CATEGORY:"
echo "$ENTRY"
