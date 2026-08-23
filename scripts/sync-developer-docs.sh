#!/usr/bin/env bash
# Sync canonical developer docs from docs/ to backend embeds and agent skill copies.
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"

embed_body() {
  local src="$1"
  local canonical="$2"
  head -1 "$src"
  echo ""
  printf '> **Embedded copy:** Do not edit here. Canonical source: `%s`. Run `./scripts/sync-developer-docs.sh` after editing.\n' "$canonical"
  echo ""
  awk 'NR==1 { next }
       NR==2 && /^$/ { next }
       /^> \*\*/ && !body { next }
       { body=1; print }' "$src"
}

GUIDE_SRC="$ROOT/docs/developer-integration-guide.md"
PLAYBOOK_SRC="$ROOT/docs/developer-agent-playbook.md"

embed_body "$GUIDE_SRC" "docs/developer-integration-guide.md" \
  > "$ROOT/backend/internal/developer/integration_guide.md"
echo "Synced docs/developer-integration-guide.md -> backend/internal/developer/integration_guide.md"

embed_body "$PLAYBOOK_SRC" "docs/developer-agent-playbook.md" \
  > "$ROOT/backend/internal/developer/agent_playbook.md"
echo "Synced docs/developer-agent-playbook.md -> backend/internal/developer/agent_playbook.md"

embed_body "$PLAYBOOK_SRC" "docs/developer-agent-playbook.md" \
  > "$ROOT/.agents/skills/joinquest-integration/playbook.md"
echo "Synced docs/developer-agent-playbook.md -> .agents/skills/joinquest-integration/playbook.md"

# Mirror every agent skill into the plugin's skills/ directory. Skills are
# discovered from the filesystem rather than listed here, so adding a skill
# folder ships it without editing this script.
PLUGIN_SKILLS="$ROOT/plugins/joinquest/skills"
mkdir -p "$PLUGIN_SKILLS"
for skill_dir in "$ROOT/.agents/skills"/*/; do
  skill="$(basename "$skill_dir")"
  target="$PLUGIN_SKILLS/$skill"
  # Replace rather than overlay, so files deleted upstream don't linger here.
  rm -rf "$target"
  mkdir -p "$target"
  cp -R "$skill_dir." "$target/"
  echo "Synced agent skill -> plugins/joinquest/skills/$skill/"
done

# Drop plugin copies whose source skill is gone.
for target in "$PLUGIN_SKILLS"/*/; do
  skill="$(basename "$target")"
  if [ ! -d "$ROOT/.agents/skills/$skill" ]; then
    rm -rf "$target"
    echo "Removed stale plugin skill copy: $skill"
  fi
done

echo "Done."
