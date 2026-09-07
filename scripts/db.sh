#!/bin/bash
# Manage this working copy's JoinQuest PostgreSQL instance (docker compose).
#
# Each working copy gets its own compose project and an ephemeral host port, and
# each test run gets its own database, so several agents can run the suite at
# once without colliding (JQ-128). See scripts/lib/db-runtime.sh.
set -e

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT"

# shellcheck source=lib/db-runtime.sh
source "$ROOT/scripts/lib/db-runtime.sh"

# Keying the compose project to this directory means `docker compose down` here
# can never tear down another working copy's stack.
export COMPOSE_PROJECT_NAME="${COMPOSE_PROJECT_NAME:-$(lobby_compose_project "$ROOT")}"

# A run id makes the test database unique. test-backend.sh exports a fresh one
# per invocation; standalone use falls back to a stable per-working-copy id so
# `db.sh test-url` is repeatable.
LOBBY_TEST_RUN_ID="${LOBBY_TEST_RUN_ID:-$(lobby_workspace_id "$ROOT")}"
TEST_DB_NAME="$(lobby_test_database "$LOBBY_TEST_RUN_ID")"

pg_host_port() {
  if [ -n "${LOBBY_POSTGRES_HOST_PORT:-}" ]; then
    printf '%s' "$LOBBY_POSTGRES_HOST_PORT"
    return 0
  fi
  lobby_host_port postgres 5432
}

# Fails loudly rather than emitting a URL with an empty port, which produces a
# baffling "connection refused" several layers away.
require_pg_host_port() {
  local port
  port="$(pg_host_port)"
  if [ -z "$port" ]; then
    echo "Postgres is not running for this working copy (compose project $COMPOSE_PROJECT_NAME)." >&2
    echo "Start it first:  ./scripts/db.sh up" >&2
    exit 1
  fi
  printf '%s' "$port"
}

# macOS often resolves localhost to ::1 while Docker publishes on IPv4, so build
# URLs against 127.0.0.1 and rewrite any inherited localhost.
build_url() {
  local db="$1" port
  port="$(require_pg_host_port)"
  printf 'postgres://app:app-pass@127.0.0.1:%s/%s?sslmode=disable' "$port" "$db"
}

normalize_url() {
  printf '%s' "${1/@localhost:/@127.0.0.1:}"
}

redis_url() {
  if [ -n "${REDIS_URL:-}" ]; then
    normalize_url "$REDIS_URL"
    return 0
  fi
  local port
  if [ -n "${LOBBY_REDIS_HOST_PORT:-}" ]; then
    port="$LOBBY_REDIS_HOST_PORT"
  else
    port="$(lobby_host_port redis 6379)"
  fi
  if [ -z "$port" ]; then
    echo "Redis is not running for this working copy (compose project $COMPOSE_PROJECT_NAME)." >&2
    echo "Start it first:  ./scripts/db.sh up" >&2
    exit 1
  fi
  printf 'redis://127.0.0.1:%s/0' "$port"
}

database_url() {
  if [ -n "${DATABASE_URL:-}" ]; then
    normalize_url "$DATABASE_URL"
    return 0
  fi
  normalize_url "$(build_url joinquest)"
}

test_database_url() {
  if [ -n "${TEST_DATABASE_URL:-}" ]; then
    normalize_url "$TEST_DATABASE_URL"
    return 0
  fi
  normalize_url "$(build_url "$TEST_DB_NAME")"
}

psql_postgres() {
  docker compose exec -T postgres psql -U app -d postgres -v ON_ERROR_STOP=1 "$@"
}

ensure_test_database() {
  psql_postgres -v dbname="$TEST_DB_NAME" <<'SQL'
SELECT format('CREATE DATABASE %I OWNER app', :'dbname')
WHERE NOT EXISTS (SELECT FROM pg_database WHERE datname = :'dbname')\gexec
SQL
}

drop_test_database() {
  psql_postgres -v dbname="$TEST_DB_NAME" <<'SQL'
DROP DATABASE IF EXISTS :"dbname" WITH (FORCE);
SQL
}

require_docker() {
  if ! command -v docker &> /dev/null; then
    echo "Docker is required. Install Docker and try again."
    exit 1
  fi

  if ! docker info >/dev/null 2>&1; then
    echo "Cannot connect to the Docker daemon."
    if command -v colima &> /dev/null; then
      echo ""
      echo "Colima is installed but Docker is not reachable. Try:"
      echo "  colima start"
      echo ""
      echo "If that does not help, restart Colima:"
      echo "  colima stop && colima start"
    else
      echo "Start Docker Desktop or your Docker runtime, then try again."
    fi
    exit 1
  fi
}

# Only pinned ports can conflict — the default is ephemeral. Report the conflict
# by name so nobody has to decode a raw "Bind for 0.0.0.0:5432 failed".
check_pinned_port() {
  local var="$1" service="$2" container_port="$3" pinned="$4"
  [ -n "$pinned" ] || return 0
  # Our own already-running stack holding the port is not a conflict.
  [ "$(lobby_host_port "$service" "$container_port")" = "$pinned" ] && return 0
  lobby_port_in_use "$pinned" || return 0

  echo "Port $pinned is already in use, so the $service container cannot bind it." >&2
  echo "" >&2
  echo "You set $var=$pinned. Something else already holds that port —" >&2
  echo "often another working copy's stack, a sibling game repo, or a local $service." >&2
  echo "" >&2
  echo "Resolve it by one of:" >&2
  echo "  • unset $var        — take an ephemeral port instead (the default)" >&2
  echo "  • $var=<free port>  — pin a different port" >&2
  echo "  • docker ps         — find and stop whatever holds $pinned" >&2
  exit 1
}

preflight_ports() {
  check_pinned_port LOBBY_POSTGRES_HOST_PORT postgres 5432 "${LOBBY_POSTGRES_HOST_PORT:-}"
  check_pinned_port LOBBY_REDIS_HOST_PORT redis 6379 "${LOBBY_REDIS_HOST_PORT:-}"
}

wait_for_postgres() {
  echo "Waiting for PostgreSQL..."
  for i in {1..30}; do
    if docker compose exec -T postgres pg_isready -U app -d joinquest >/dev/null 2>&1; then
      break
    fi
    if [ "$i" -eq 30 ]; then
      echo "PostgreSQL failed to become ready"
      exit 1
    fi
    sleep 1
  done

  echo "Waiting for PostgreSQL port on host..."
  local port
  for i in {1..30}; do
    port="$(pg_host_port)"
    if [ -n "$port" ] && nc -z 127.0.0.1 "$port" >/dev/null 2>&1; then
      echo "PostgreSQL is ready on 127.0.0.1:$port (compose project $COMPOSE_PROJECT_NAME)"
      return 0
    fi
    sleep 1
  done

  echo "PostgreSQL is running in Docker but its published port is not reachable on 127.0.0.1"
  echo "Published ports for this stack:"
  docker compose ps --format '  {{.Service}}\t{{.Ports}}' 2>/dev/null
  exit 1
}

case "${1:-}" in
  up)
    require_docker
    preflight_ports
    docker compose up -d postgres redis
    wait_for_postgres
    ;;
  down)
    require_docker
    docker compose down
    ;;
  wait)
    require_docker
    wait_for_postgres
    ;;
  migrate)
    require_docker
    wait_for_postgres
    (cd backend && DATABASE_URL="$(database_url)" make migrate-up)
    ;;
  test-url)
    test_database_url
    echo ""
    ;;
  test-db)
    echo "$TEST_DB_NAME"
    ;;
  test-migrate)
    require_docker
    wait_for_postgres
    ensure_test_database
    (cd backend && DATABASE_URL="$(test_database_url)" make migrate-up)
    echo "Migrations applied to $TEST_DB_NAME (dev database joinquest unchanged)."
    ;;
  test-reset)
    require_docker
    wait_for_postgres
    drop_test_database
    psql_postgres -v dbname="$TEST_DB_NAME" <<'SQL'
CREATE DATABASE :"dbname" OWNER app;
SQL
    (cd backend && DATABASE_URL="$(test_database_url)" make migrate-up)
    echo "Reset and migrated $TEST_DB_NAME."
    ;;
  test-drop)
    require_docker
    drop_test_database
    echo "Dropped $TEST_DB_NAME."
    ;;
  compose-project)
    echo "$COMPOSE_PROJECT_NAME"
    ;;
  reset)
    require_docker
    preflight_ports
    docker compose down -v
    docker compose up -d postgres redis
    wait_for_postgres
    (cd backend && DATABASE_URL="$(database_url)" make migrate-up)
    ;;
  clean-test-data)
    require_docker
    wait_for_postgres
    docker compose exec -T postgres psql -U app -d joinquest <<'SQL'
BEGIN;
DELETE FROM magic_links WHERE email LIKE '%@example.com';
DELETE FROM users WHERE email LIKE '%@example.com';
DELETE FROM games WHERE category IS DISTINCT FROM 'demo';
COMMIT;
SQL
    echo "Removed test games, @example.com users, and their magic links."
    echo "Demo games and real user accounts were kept."
    ;;
  reset-demo-handoff)
    # Dev database only (joinquest). Integration tests use their own per-run database.
    require_docker
    wait_for_postgres
    docker compose exec -T postgres psql -U app -d joinquest <<'SQL'
UPDATE games
SET play_url = 'http://localhost:5174',
    api_base_url = 'http://localhost:3001'
WHERE id = 'a1000000-0000-4000-8000-000000000001';
SQL
    echo "Restored demo quick-match handoff URLs (play :5174, API :3001)."
    ;;
  url)
    database_url
    echo ""
    ;;
  redis-url)
    redis_url
    echo ""
    ;;
  *)
    echo "Usage: $0 {up|down|wait|migrate|reset|url|redis-url|test-url|test-db|test-migrate|test-reset|test-drop|compose-project|clean-test-data|reset-demo-handoff}"
    echo ""
    echo "  url / migrate     — development database (joinquest)"
    echo "  redis-url         — this working copy's Redis URL"
    echo "  test-url          — integration test database URL ($TEST_DB_NAME)"
    echo "  test-db           — integration test database name"
    echo "  test-migrate      — migrate the test database only"
    echo "  test-reset        — drop and recreate the test database, then migrate"
    echo "  test-drop         — drop the test database"
    echo "  compose-project   — this working copy's compose project name"
    echo ""
    echo "Host ports are ephemeral by default so working copies do not collide."
    echo "Pin one with LOBBY_POSTGRES_HOST_PORT / LOBBY_REDIS_HOST_PORT if an"
    echo "external client needs a fixed address."
    exit 1
    ;;
esac
