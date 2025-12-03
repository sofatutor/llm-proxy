#!/usr/bin/env bash
#
# generate-changelog-entry.sh
#
# Generates a changelog entry for a PR using the OpenAI API and merges it into CHANGELOG.md.
# If an entry for the same PR already exists, it will be replaced. Other entries are preserved.
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

# Get today's date in the changelog format (force English locale for consistency)
TODAY=$(LC_ALL=C date +"%B %d, %Y")

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
# Handle null/empty PR body explicitly
if [[ "$PR_BODY" == "null" || -z "$PR_BODY" ]]; then
  PR_BODY="No description provided."
fi
PR_ADDITIONS=$(echo "$PR_JSON" | jq -r '.additions')
PR_DELETIONS=$(echo "$PR_JSON" | jq -r '.deletions')
PR_URL=$(echo "$PR_JSON" | jq -r '.url')
PR_FILES=$(echo "$PR_JSON" | jq -r '.files[].path' | head -50 | tr '\n' ', ' | sed 's/,$//')

# Extract existing entries for today's date (if any)
EXISTING_SECTION=""
if [[ -f "$CHANGELOG_PATH" ]] && grep -q "## $TODAY" "$CHANGELOG_PATH"; then
  # Extract the section from today's date until the next date section or end of meaningful content
  EXISTING_SECTION=$(awk -v date="## $TODAY" '
    $0 == date { found=1; next }
    found && /^## / { exit }
    found { print }
  ' "$CHANGELOG_PATH" | sed '/^$/N;/^\n$/d')
fi

# Build the prompt
read -r -d '' SYSTEM_PROMPT << 'SYSPROMPT' || true
You are a technical writer generating changelog entries for a Go-based LLM proxy project.

You will receive:
1. PR metadata for a new/updated entry
2. Existing changelog entries for today's date (if any)

Your task:
1. Generate entry/entries for the given PR
2. If entries for this PR number already exist in the existing entries, they should be REPLACED with new ones
3. Preserve all other existing entries exactly as they are
4. Return a complete JSON with all entries for the date section

Output format (always use this):
{
  "entries": [
    {"category": "Added|Changed|Fixed|Reverted", "entry": "- **Title** ([#NUMBER](URL)): Description."},
    ...
  ]
}

Rules for generating entries:
1. Category selection:
   - "Added" for new features, endpoints, commands, capabilities, new files/scripts
   - "Changed" for modifications, refactors, documentation updates, improvements
   - "Fixed" for bug fixes, error corrections, security patches
   - "Reverted" for rollbacks or reverted changes
2. Title should be concise (2-6 words), derived from PR content
3. Description should be 1-3 sentences summarizing the key changes
4. Focus on WHAT changed and WHY it matters, not implementation details
5. For multi-feature PRs: create separate entries for distinct capabilities (e.g., new automation + enhanced documentation)
6. IMPORTANT: Include ALL existing entries in your response (unchanged, unless same PR number)
7. Order entries by category: Added, Changed, Fixed, Reverted

Example output for a PR with automation + docs, plus one existing entry:
{
  "entries": [
    {"category": "Added", "entry": "- **Automated Changelog Generation** ([#184](url)): GitHub Actions workflow that generates changelog entries on PR approval using OpenAI API."},
    {"category": "Changed", "entry": "- **Enhanced CHANGELOG.md** ([#184](url)): Transformed 79 PR entries from basic titles to detailed entries with descriptions."},
    {"category": "Changed", "entry": "- **Existing Entry Title** ([#183](url)): This was already there and is preserved exactly."}
  ]
}
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

# Build user prompt with existing entries context
EXISTING_CONTEXT=""
if [[ -n "$EXISTING_SECTION" ]]; then
  EXISTING_CONTEXT="
**Existing entries for ${TODAY} (preserve these, unless same PR #${PR_NUMBER}):**
${EXISTING_SECTION}
"
fi

read -r -d '' USER_PROMPT << USERPROMPT || true
Generate changelog entries for this PR and merge with existing entries:

**PR Title:** ${PR_TITLE}
**PR Number:** #${PR_NUMBER}
**PR URL:** ${PR_URL}
**Stats:** +${PR_ADDITIONS} -${PR_DELETIONS} lines

**PR Description:**
${PR_BODY_TRUNCATED}

**Changed Files:**
${PR_FILES_TRUNCATED}
${EXISTING_CONTEXT}
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
    temperature: 0.1,
    max_tokens: 2000,
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

# Parse entries from JSON response
if ! echo "$CONTENT" | jq -e '.entries' > /dev/null 2>&1; then
  echo "Error: Invalid JSON response - missing entries array" >&2
  echo "Response (truncated): ${CONTENT:0:200}..." >&2
  exit 1
fi

ENTRIES_COUNT=$(echo "$CONTENT" | jq '.entries | length')

if [[ "$ENTRIES_COUNT" -eq 0 ]]; then
  echo "Error: No entries generated" >&2
  exit 1
fi

# Build the new date section from entries
# Group entries by category in order: Added, Changed, Fixed, Reverted
build_section() {
  local section=""
  
  for cat in Added Changed Fixed Reverted; do
    local cat_entries=$(echo "$CONTENT" | jq -r --arg cat "$cat" '.entries[] | select(.category == $cat) | .entry')
    if [[ -n "$cat_entries" ]]; then
      if [[ -n "$section" ]]; then
        section="${section}

"
      fi
      section="${section}### ${cat}

${cat_entries}"
    fi
  done
  
  echo "$section"
}

NEW_SECTION=$(build_section)

# If dry-run, just output the section and exit
if [[ "$DRY_RUN" == "true" ]]; then
  echo "## $TODAY"
  echo ""
  echo "$NEW_SECTION"
  exit 0
fi

# Merge into CHANGELOG.md
if [[ ! -f "$CHANGELOG_PATH" ]]; then
  echo "Error: $CHANGELOG_PATH not found" >&2
  exit 1
fi

# Write new section to a temp file for reliable insertion
SECTION_FILE=$(mktemp)
echo "$NEW_SECTION" > "$SECTION_FILE"

# Check if today's date section exists
if grep -q "## $TODAY" "$CHANGELOG_PATH"; then
  # Replace existing date section using line numbers
  # Find start line (the date header)
  START_LINE=$(grep -n "## $TODAY" "$CHANGELOG_PATH" | head -1 | cut -d: -f1)
  
  # Find end line (next date section or end of file)
  END_LINE=$(tail -n +$((START_LINE + 1)) "$CHANGELOG_PATH" | grep -n "^## " | head -1 | cut -d: -f1)
  
  if [[ -n "$END_LINE" ]]; then
    # There's another date section after - END_LINE is relative to START_LINE+1
    END_LINE=$((START_LINE + END_LINE - 1))
  else
    # No more date sections, but we need to find where meaningful content ends
    # Look for the next section or use a reasonable endpoint
    END_LINE=$(wc -l < "$CHANGELOG_PATH")
  fi
  
  # Build new file: before section + new section + after section
  {
    head -n "$START_LINE" "$CHANGELOG_PATH"
    echo ""
    cat "$SECTION_FILE"
    echo ""
    tail -n +$((END_LINE + 1)) "$CHANGELOG_PATH"
  } > "${CHANGELOG_PATH}.tmp"
  mv "${CHANGELOG_PATH}.tmp" "$CHANGELOG_PATH"
else
  # Add new date section after the --- separator
  SEPARATOR_LINE=$(grep -n "^---$" "$CHANGELOG_PATH" | head -1 | cut -d: -f1)
  
  if [[ -n "$SEPARATOR_LINE" ]]; then
    {
      head -n "$SEPARATOR_LINE" "$CHANGELOG_PATH"
      echo ""
      echo "## $TODAY"
      echo ""
      cat "$SECTION_FILE"
      echo ""
      tail -n +$((SEPARATOR_LINE + 1)) "$CHANGELOG_PATH"
    } > "${CHANGELOG_PATH}.tmp"
    mv "${CHANGELOG_PATH}.tmp" "$CHANGELOG_PATH"
  else
    echo "Error: Could not find --- separator in $CHANGELOG_PATH" >&2
    rm -f "$SECTION_FILE"
    exit 1
  fi
fi

rm -f "$SECTION_FILE"

echo "Updated $CHANGELOG_PATH for $TODAY:"
echo ""
echo "$NEW_SECTION"
