#!/bin/bash
# Guard the deploy ordering that took the GraphQL API down on 2026-09-10.
#
# The backend used to be restarted before the joinquest-db-migrate Job ran. Both
# write schema_migrations, so they blocked on each other's locks, the new pod was
# killed by its liveness probe mid-migration, and the version was left dirty --
# which makes golang-migrate refuse to start at all. Migrations must finish
# before anything rolls lobby-backend.
set -e

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT"

DEPLOY="scripts/deploy-joinquest.sh"
BACKEND_MANIFEST="k8s/base/backend.yaml"
failed=0

fail() {
  echo "❌ $1" >&2
  failed=1
}

# Strip comments and blank lines but keep original line numbers.
code_lines() {
  grep -n '' "$DEPLOY" | grep -v ':[[:space:]]*#' | grep -v ':[[:space:]]*$'
}

first_line_matching() {
  code_lines | grep -E "$1" | head -1 | cut -d: -f1
}

migrate_line="$(first_line_matching 'apply_database_migrations')"
if [ -z "$migrate_line" ]; then
  fail "$DEPLOY no longer calls apply_database_migrations — migrations must run from there."
  exit 1
fi

preflight_line="$(first_line_matching 'preflight_migration_state')"
if [ -z "$preflight_line" ]; then
  fail "$DEPLOY must call preflight_migration_state so a dirty schema_migrations is reported up front."
elif [ "$preflight_line" -gt "$migrate_line" ]; then
  fail "preflight_migration_state (line $preflight_line) must run before apply_database_migrations (line $migrate_line)."
fi

# Any lib helper that touches deployment/lobby-backend restarts it, so it counts
# as a backend mutator even though the call site does not say "lobby-backend".
lib_mutators="$(awk '
  /^[a-zA-Z_][a-zA-Z0-9_]*\(\)[[:space:]]*\{/ { fn = substr($0, 1, index($0, "(") - 1); next }
  /^\}/                                       { fn = ""; next }
  fn != "" && /deployment\/lobby-backend/     { print fn }
' scripts/lib/*.sh | sort -u)"

mutators="kubectl apply -f ${BACKEND_MANIFEST}
kubectl set env deployment/lobby-backend
kubectl rollout restart deployment/lobby-backend
kubectl scale deployment/lobby-backend
pin_deployment_image lobby-backend"

[ -n "$lib_mutators" ] && mutators="${mutators}
${lib_mutators}"

while IFS= read -r mutator; do
  [ -n "$mutator" ] || continue
  while IFS= read -r line; do
    [ -n "$line" ] || continue
    if [ "$line" -lt "$migrate_line" ]; then
      fail "$DEPLOY:$line restarts lobby-backend ('$mutator') before migrations run at line $migrate_line."
    fi
  done <<< "$(code_lines | grep -F "$mutator" | cut -d: -f1)"
done <<< "$mutators"

# Defense in depth: a slow startup must not be killed by the liveness probe.
if ! grep -q 'startupProbe' "$BACKEND_MANIFEST"; then
  fail "$BACKEND_MANIFEST needs a startupProbe; without it the liveness probe kills slow startup migrations."
fi
if ! grep -q 'RUN_STARTUP_MIGRATIONS' "$BACKEND_MANIFEST"; then
  fail "$BACKEND_MANIFEST must set RUN_STARTUP_MIGRATIONS=false so only the migration Job writes schema_migrations."
fi

if [ "$failed" -ne 0 ]; then
  exit 1
fi
echo "✅ deploy runs migrations before rolling lobby-backend"
