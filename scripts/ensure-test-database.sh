#!/bin/bash
# Create the integration test database if missing. Used by CI service containers,
# where the database name comes from DATABASE_URL. Local runs go through
# scripts/db.sh, which manages a per-run database instead (JQ-128).
set -euo pipefail

PGHOST="${PGHOST:-127.0.0.1}"
PGPORT="${PGPORT:-5432}"
PGUSER="${PGUSER:-app}"
PGPASSWORD="${PGPASSWORD:-app-pass}"

# Prefer the database DATABASE_URL actually points at, so this cannot drift from
# what the tests connect to.
TEST_DB="playhub_test"
if [ -n "${DATABASE_URL:-}" ]; then
  parsed="${DATABASE_URL##*/}"
  parsed="${parsed%%\?*}"
  [ -n "$parsed" ] && TEST_DB="$parsed"
fi

case "$TEST_DB" in
  playhub_test|playhub_test_*) ;;
  *)
    echo "Refusing to create '$TEST_DB': integration databases must be playhub_test or playhub_test_<run id>." >&2
    exit 1
    ;;
esac

export PGPASSWORD
if psql -h "$PGHOST" -p "$PGPORT" -U "$PGUSER" -d postgres -tc "SELECT 1 FROM pg_database WHERE datname = '$TEST_DB'" | grep -q 1; then
  exit 0
fi
psql -h "$PGHOST" -p "$PGPORT" -U "$PGUSER" -d postgres -c "CREATE DATABASE \"$TEST_DB\" OWNER app;"
