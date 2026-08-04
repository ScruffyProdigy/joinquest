#!/usr/bin/env bash
# Install JoinQuest agent skill + MCP — delegates to the joinquest npm CLI.
#
# Preferred:
#   JOINQUEST_API_KEY=lq_dev_... npx joinquest install cursor
#   npm create joinquest@latest -- --cursor
set -euo pipefail

PLATFORM=""
PLUGIN=false
DRY_RUN=false

usage() {
  cat <<EOF
Deprecated wrapper — use: npx joinquest install <platform>

  JOINQUEST_API_KEY=lq_dev_... npx joinquest install cursor
  npm create joinquest@latest -- --cursor

This script forwards to npx joinquest for backward compatibility.
EOF
}

while [ $# -gt 0 ]; do
  case "$1" in
    --cursor) PLATFORM=cursor ;;
    --claude) PLATFORM=claude ;;
    --claude-desktop) PLATFORM=claude-desktop ;;
    --copilot) PLATFORM=copilot ;;
    --roo) PLATFORM=roo ;;
    --windsurf) PLATFORM=windsurf ;;
    --cline) PLATFORM=cline ;;
    --gemini) PLATFORM=gemini ;;
    --skill-only) PLATFORM=skill ;;
    --all)
      echo "→ npx joinquest install cursor && npx joinquest install claude"
      exec bash -c 'npx -y joinquest install cursor && npx -y joinquest install claude'
      ;;
    --dry-run) DRY_RUN=true ;;
    --plugin) PLUGIN=true ;;
    -h | --help) usage; exit 0 ;;
    *) echo "Unknown option: $1" >&2; usage >&2; exit 1 ;;
  esac
  shift
done

if [ -z "$PLATFORM" ]; then
  echo "No platform flag. Example: npx joinquest install cursor" >&2
  usage >&2
  exit 1
fi

CMD=(npx -y joinquest install "$PLATFORM")
if $DRY_RUN; then CMD+=(--dry-run); fi
if $PLUGIN; then CMD+=(--plugin); fi

ROOT="$(CDPATH= cd -- "$(dirname "$0")/.." && pwd)"
LOCAL_CLI="$ROOT/packages/joinquest/bin/joinquest.js"
if [ -f "$LOCAL_CLI" ]; then
  echo "→ node $LOCAL_CLI install $PLATFORM$( $DRY_RUN && echo ' --dry-run' )$( $PLUGIN && echo ' --plugin' )"
  LOCAL_ARGS=(install "$PLATFORM")
  if $DRY_RUN; then LOCAL_ARGS+=(--dry-run); fi
  if $PLUGIN; then LOCAL_ARGS+=(--plugin); fi
  exec node "$LOCAL_CLI" "${LOCAL_ARGS[@]}"
fi

echo "→ ${CMD[*]}"
exec "${CMD[@]}"
