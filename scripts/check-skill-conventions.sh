#!/usr/bin/env bash
# Fail if an agent skill breaks the conventions the bundle relies on.
#
# Two rules, both load-bearing:
#
#   1. Frontmatter is `name` + `description` only, and `name` matches the folder.
#      Skills are routed by these fields; a mismatched name or stray key from an
#      upstream copy makes a skill trigger wrongly or not at all.
#
#   2. Every skill except joinquest-integration is generic and unbranded. These
#      ship to external developers as standalone skills, so a JoinQuest mention
#      in one is a leak, not a feature. joinquest-integration is the branded one
#      by design and is exempt.
#
# Dependency-free on purpose: this runs in CI before anything else is installed.
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT"

BRANDED_SKILL="joinquest-integration"
failures=0

fail() {
  echo "❌ $1" >&2
  failures=$((failures + 1))
}

for skill_dir in .agents/skills/*/; do
  skill="$(basename "$skill_dir")"
  file="$skill_dir/SKILL.md"

  if [ ! -f "$file" ]; then
    fail "$skill: no SKILL.md"
    continue
  fi

  if [ "$(head -1 "$file")" != "---" ]; then
    fail "$skill: SKILL.md does not open with YAML frontmatter"
    continue
  fi

  # Frontmatter is everything up to the second `---`. Continuation lines of a
  # folded scalar are indented, so only top-level keys match `^[a-z-]+:`.
  frontmatter="$(awk 'NR==1 { next } /^---$/ { exit } { print }' "$file")"
  keys="$(printf '%s\n' "$frontmatter" | grep -oE '^[a-z][a-z-]*:' | tr -d ':' | sort | tr '\n' ' ')"

  if [ "$keys" != "description name " ]; then
    fail "$skill: frontmatter keys are [${keys% }], expected exactly [description name]"
  fi

  declared="$(printf '%s\n' "$frontmatter" | sed -n 's/^name:[[:space:]]*//p' | tr -d '"'"'" | head -1)"
  if [ "$declared" != "$skill" ]; then
    fail "$skill: frontmatter name is '$declared', expected '$skill' to match the folder"
  fi

  if [ "$skill" != "$BRANDED_SKILL" ] && grep -qi joinquest "$file"; then
    fail "$skill: generic skill mentions JoinQuest — these ship unbranded (see the naming rule)"
    grep -in joinquest "$file" | sed 's/^/    /' >&2
  fi
done

if [ "$failures" -gt 0 ]; then
  echo "" >&2
  echo "$failures skill convention violation(s)." >&2
  exit 1
fi

echo "✅ Agent skill conventions OK."
