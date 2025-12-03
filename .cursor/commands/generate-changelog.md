---
description: Generate a changelog entry for a PR using OpenAI
---

# Generate Changelog Entry

Generate a changelog entry for the current branch's PR or a specific PR number.

## Usage

Run this command to generate a changelog entry. The script will:
1. Fetch PR metadata from GitHub
2. Use OpenAI to generate a properly formatted entry
3. Categorize it (Added/Changed/Fixed/Reverted)
4. Merge it into CHANGELOG.md under the correct date

## Options

- **Dry-run (preview)**: See what would be generated without modifying files
- **Apply**: Generate and apply the entry to CHANGELOG.md

## Instructions

```bash
# Preview changelog entry for current branch's PR
./scripts/generate-changelog-entry.sh --dry-run

# Apply changelog entry for current branch's PR  
./scripts/generate-changelog-entry.sh

# Preview changelog entry for a specific PR
./scripts/generate-changelog-entry.sh 184 --dry-run

# Apply changelog entry for a specific PR
./scripts/generate-changelog-entry.sh 184
```

## Requirements

- `OPENAI_API_KEY` environment variable must be set
- `gh` CLI must be authenticated
- PR must exist on GitHub
