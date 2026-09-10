# Database migration steps for the joinquest deploy scripts.
#
# The backend and the joinquest-db-migrate Job can both write schema_migrations.
# If they run at once one blocks on the other's locks, gets killed, and leaves
# the version dirty -- which then makes golang-migrate refuse to start at all.
# So: migrations finish here, before anything rolls the backend.
#
# Source from deploy scripts after NAMESPACE is set.

MIGRATION_JOB_MANIFEST="${MIGRATION_JOB_MANIFEST:-k8s/jobs/migration.yaml}"

# Print k8s/jobs/migration.yaml with a different job name, image and -action.
render_migration_job() {
  local name=$1 image=$2 action=$3
  sed -e "s|name: joinquest-db-migrate|name: ${name}|" \
      -e "s|docker.io/scruffyprodigy/joinquest-backend:latest|${image}|" \
      -e "s|\"-action=up\"|${action}|" \
      "$MIGRATION_JOB_MANIFEST"
}

# Run a migration job to completion and echo its logs. Returns non-zero if the
# job never completes; the caller decides how loud to be about it.
run_migration_job() {
  local ns=$1 name=$2 image=$3 action=$4 timeout=${5:-120s}

  kubectl delete job "$name" -n "$ns" --ignore-not-found >/dev/null
  render_migration_job "$name" "$image" "$action" | kubectl apply -f - >/dev/null
  kubectl wait --for=condition=complete --timeout="$timeout" "job/${name}" -n "$ns" >/dev/null 2>&1
}

# Fail early and clearly when schema_migrations is dirty.
#
# Without this the deploy just sits on `kubectl wait` for two minutes and then
# reports a timeout, saying nothing about why. This is what took the API down
# for 15 minutes on 2026-09-10.
preflight_migration_state() {
  local ns=$1 image=$2
  local job="joinquest-db-migrate-version"
  local out=""

  echo "Checking schema_migrations state..."
  if run_migration_job "$ns" "$job" "$image" '"-action=version"' 120s; then
    out="$(kubectl logs "job/${job}" -n "$ns" 2>/dev/null || true)"
  else
    echo "Could not read the migration version (job/${job} did not complete):" >&2
    kubectl logs "job/${job}" -n "$ns" >&2 2>/dev/null || true
    kubectl delete job "$job" -n "$ns" --ignore-not-found >/dev/null
    return 1
  fi
  kubectl delete job "$job" -n "$ns" --ignore-not-found >/dev/null

  echo "  ${out:-unknown}"

  case "$out" in
    *"(dirty)"*) ;;
    *) return 0 ;;
  esac

  local dirty previous
  dirty="$(printf '%s\n' "$out" | sed -n 's/.*Current version: \([0-9][0-9]*\).*/\1/p' | head -1)"
  previous=$((dirty - 1))

  cat >&2 <<EOF

ERROR: schema_migrations is dirty at version ${dirty} -- refusing to deploy.

golang-migrate will not run any migration while the version is dirty, so the
backend cannot start either. Migration ${dirty} applied only partway; check what
it left behind, then roll the recorded version back and deploy again:

  ./scripts/fix-dirty-migration.sh ${previous}
  ./scripts/deploy-joinquest.sh

See docs/database-migrations.md for the longer version.
EOF
  return 1
}

# Apply the pending migrations. On failure, say what the job actually printed.
apply_database_migrations() {
  local ns=$1 image=$2
  local job="joinquest-db-migrate"

  echo "Running database migrations with ${image}..."
  if ! run_migration_job "$ns" "$job" "$image" '"-action=up"' 300s; then
    echo "Migration job did not complete. Logs:" >&2
    kubectl logs "job/${job}" -n "$ns" >&2 2>/dev/null || true
    kubectl describe job "$job" -n "$ns" >&2 2>/dev/null || true
    echo "If this left the version dirty, ./scripts/fix-dirty-migration.sh <version> resets it." >&2
    return 1
  fi
  kubectl logs "job/${job}" -n "$ns"
}
