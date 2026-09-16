#!/usr/bin/env bash
# Fail if frontend code is not formatted to the repo's prettier config.
#
# The style (single quotes, no semicolons, 100 columns) used to live only in the
# existing code, so `npx prettier --write` fetched latest prettier and applied
# its defaults instead — rewriting whole files and manufacturing merge conflicts
# against main. .prettierrc.json at the repo root now records the style, and
# prettier is a pinned devDependency so `npx prettier` inside frontend/ runs that
# version rather than whatever is newest. This gate keeps both honest (JQ-257).
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT/frontend"

echo "Checking frontend formatting..."

if [ ! -x node_modules/.bin/prettier ]; then
  echo "❌ prettier is not installed in frontend/node_modules." >&2
  echo "Run 'cd frontend && npm ci' first." >&2
  exit 1
fi

if npm run --silent format:check; then
  echo "✅ Frontend code is formatted."
  exit 0
fi

echo "❌ Frontend code is not formatted." >&2
echo "Run 'cd frontend && npm run format' and commit the result." >&2
echo "" >&2
echo "Keep the reformat in its own commit — formatting churn riding along with a" >&2
echo "feature change is what JQ-257 exists to prevent." >&2
exit 1
