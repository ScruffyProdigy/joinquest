#!/bin/bash
# Reset a dirty schema_migrations version so deploys can run again.
#
# golang-migrate marks the version dirty when a migration dies partway through
# and then refuses to run anything, so the backend CrashLoopBackOffs. This forces
# the recorded version back to the last one known good and applies the rest.
#
# Usage: ./scripts/fix-dirty-migration.sh <last-good-version>
#
# Check what the half-applied migration left behind BEFORE running this -- force
# only rewrites the version number, it does not undo any SQL.
set -e

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT"
# shellcheck source=lib/lobby-migrations.sh
. "$ROOT/scripts/lib/lobby-migrations.sh"

NAMESPACE="${NAMESPACE:-joinquest}"
VERSION="${1:-}"
IMAGE="${MIGRATE_IMAGE:-docker.io/scruffyprodigy/joinquest-backend:latest}"

case "$VERSION" in
  '' | *[!0-9]*)
    echo "Usage: $0 <last-good-version>" >&2
    echo "Find the dirty version with: kubectl logs job/joinquest-db-migrate -n $NAMESPACE" >&2
    exit 1
    ;;
esac

echo "Namespace: $NAMESPACE"
echo "Forcing schema_migrations to version $VERSION using $IMAGE"

# Nothing else may hold migration locks while we do this.
echo "Scaling lobby-backend to 0..."
kubectl scale deployment/lobby-backend -n "$NAMESPACE" --replicas=0
kubectl wait --for=delete --timeout=120s pod -l app=lobby-backend -n "$NAMESPACE" 2>/dev/null || true

if ! run_migration_job "$NAMESPACE" joinquest-db-migrate-force "$IMAGE" "\"-action=force\", \"-version=${VERSION}\"" 120s; then
  echo "Force job did not complete. Logs:" >&2
  kubectl logs job/joinquest-db-migrate-force -n "$NAMESPACE" >&2 2>/dev/null || true
  kubectl scale deployment/lobby-backend -n "$NAMESPACE" --replicas=1
  exit 1
fi
kubectl logs job/joinquest-db-migrate-force -n "$NAMESPACE"
kubectl delete job joinquest-db-migrate-force -n "$NAMESPACE" --ignore-not-found

apply_database_migrations "$NAMESPACE" "$IMAGE"

echo "Scaling lobby-backend back to 1..."
kubectl scale deployment/lobby-backend -n "$NAMESPACE" --replicas=1
kubectl rollout status deployment/lobby-backend -n "$NAMESPACE" --timeout=300s

echo "Done. Re-run ./scripts/deploy-joinquest.sh to finish the deploy."
