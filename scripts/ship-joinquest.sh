#!/usr/bin/env bash
# Pre-ship checks and optional deploy/publish helpers for JoinQuest lobby.
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT"

usage() {
  cat <<'EOF'
Usage: ./scripts/ship-joinquest.sh [--check] [--deploy] [--publish-mcp]

  --check        Run tests, gqlgen drift, and developer doc sync checks (default if no flags)
  --deploy       Build/push Docker images and deploy to joinquest GKE namespace
  --publish-mcp  Publish @joinquest/mcp-integration to npm (requires login)

Flags can be combined. Destructive steps require explicit user intent — this script
does not commit or push git changes.

Examples:
  ./scripts/ship-joinquest.sh --check
  ./scripts/ship-joinquest.sh --deploy
  ./scripts/ship-joinquest.sh --check --deploy --publish-mcp
EOF
}

CHECK=false
DEPLOY=false
PUBLISH_MCP=false

if [[ $# -eq 0 ]]; then
  CHECK=true
fi

for arg in "$@"; do
  case "$arg" in
    --check) CHECK=true ;;
    --deploy) DEPLOY=true ;;
    --publish-mcp) PUBLISH_MCP=true ;;
    -h|--help) usage; exit 0 ;;
    *)
      echo "Unknown option: $arg" >&2
      usage >&2
      exit 1
      ;;
  esac
done

run_check() {
  echo "==> gqlgen drift"
  (cd backend && make check-drift)

  echo "==> developer doc sync"
  "$ROOT/scripts/check-developer-docs-sync.sh"

  echo "==> static asset permissions"
  "$ROOT/scripts/check-static-asset-permissions.sh"

  echo "==> backend tests"
  "$ROOT/scripts/test-backend.sh"

  echo "==> frontend tests"
  "$ROOT/scripts/test-frontend.sh"

  echo "==> MCP tests"
  (cd mcp/joinquest-integration && npm test)

  echo "✅ All pre-ship checks passed."
}

if [[ "$CHECK" == true ]]; then
  run_check
fi

if [[ "$DEPLOY" == true ]]; then
  echo "==> build and push images"
  "$ROOT/scripts/build-and-push.sh" --push
  echo "==> deploy to joinquest namespace"
  "$ROOT/scripts/deploy-joinquest.sh"
fi

if [[ "$PUBLISH_MCP" == true ]]; then
  echo "==> publish MCP package"
  "$ROOT/scripts/publish-joinquest-mcp.sh"
fi
