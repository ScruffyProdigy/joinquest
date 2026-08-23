#!/usr/bin/env bash
# Fail if canonical developer docs are not synced to embeds and skill copies.
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT"

echo "Checking developer doc sync..."

"$ROOT/scripts/sync-developer-docs.sh" >/dev/null

# The whole plugin skills tree is checked, not a fixed file list, so a skill
# added to .agents/skills/ is covered without editing this script.
PATHS=(
  backend/internal/developer/integration_guide.md
  backend/internal/developer/agent_playbook.md
  .agents/skills/joinquest-integration/playbook.md
  plugins/joinquest/skills
)

# git diff only reports tracked files; a brand-new skill copy shows up as
# untracked, so check for that separately or a missing skill passes silently.
UNTRACKED="$(git ls-files --others --exclude-standard -- plugins/joinquest/skills)"

if git diff --quiet -- "${PATHS[@]}" && [ -z "$UNTRACKED" ]; then
  echo "✅ Developer doc copies are in sync."
  exit 0
fi

echo "❌ Developer doc copies are out of sync." >&2
echo "Run ./scripts/sync-developer-docs.sh and commit the updated copies." >&2
echo "" >&2
if [ -n "$UNTRACKED" ]; then
  echo "Untracked plugin skill copies:" >&2
  echo "$UNTRACKED" >&2
  echo "" >&2
fi
git diff -- "${PATHS[@]}" >&2
exit 1
