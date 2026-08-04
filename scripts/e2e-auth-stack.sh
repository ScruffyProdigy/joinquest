#!/usr/bin/env bash
# Start PostgreSQL, backend, and frontend for auth E2E tests.
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT"

cleanup() {
  if [[ -n "${BACKEND_PID:-}" ]]; then
    kill "$BACKEND_PID" 2>/dev/null || true
  fi
  if [[ -n "${FRONTEND_PID:-}" ]]; then
    kill "$FRONTEND_PID" 2>/dev/null || true
  fi
}
trap cleanup EXIT INT TERM

export DATABASE_URL="${DATABASE_URL:-postgres://app:app-pass@127.0.0.1:5432/playhub?sslmode=disable}"
export MAGIC_LINK_BASE_URL="${MAGIC_LINK_BASE_URL:-http://127.0.0.1:5173/auth/complete?token=}"
export CORS_ALLOWED_ORIGINS="${CORS_ALLOWED_ORIGINS:-http://127.0.0.1:5173,http://localhost:5173}"
export E2E_SIGNIN_LOG_PATH="${E2E_SIGNIN_LOG_PATH:-$ROOT/tmp/e2e-sign-in.log}"

postgres_ready() {
  if command -v psql >/dev/null 2>&1; then
    psql "$DATABASE_URL" -c 'SELECT 1' >/dev/null 2>&1
    return $?
  fi

  if command -v docker >/dev/null 2>&1 && docker info >/dev/null 2>&1; then
    docker compose exec -T postgres pg_isready -U app -d playhub >/dev/null 2>&1
    return $?
  fi

  return 1
}

if ! postgres_ready; then
  if command -v docker >/dev/null 2>&1; then
    "$ROOT/scripts/db.sh" up
  else
    echo "PostgreSQL is not reachable and Docker is unavailable." >&2
    exit 1
  fi
fi

if command -v psql >/dev/null 2>&1; then
  (cd backend && make migrate-up)
elif command -v docker >/dev/null 2>&1 && docker info >/dev/null 2>&1; then
  "$ROOT/scripts/db.sh" migrate
fi

if lsof -ti :8080 >/dev/null 2>&1; then
  if curl -sf http://127.0.0.1:8080/healthz >/dev/null; then
    echo "Reusing existing backend on port 8080"
    BACKEND_PID=""
  else
    echo "Port 8080 is already in use by a non-backend process. Stop it before auth E2E." >&2
    exit 1
  fi
fi

frontend_ready() {
  curl -sf http://127.0.0.1:5173 >/dev/null || curl -sf http://localhost:5173 >/dev/null
}

if lsof -ti :5173 >/dev/null 2>&1; then
  if frontend_ready; then
    echo "Reusing existing frontend on port 5173"
    FRONTEND_PID=""
  else
    echo "Port 5173 is already in use by a non-frontend process. Stop it before auth E2E." >&2
    exit 1
  fi
fi

if [[ -z "${BACKEND_PID:-}" ]]; then
  echo "Starting backend for auth E2E..."
  mkdir -p "$ROOT/tmp"
  : > "$ROOT/tmp/e2e-backend.log"
  : > "$ROOT/tmp/e2e-sign-in.log"
  (
    cd backend
    go run server.go
  ) >> "$ROOT/tmp/e2e-backend.log" 2>&1 &
  BACKEND_PID=$!
fi

if [[ -z "${FRONTEND_PID:-}" ]]; then
  echo "Starting frontend for auth E2E..."
  (
    cd frontend
    npm run dev -- --host 127.0.0.1 --port 5173
  ) &
  FRONTEND_PID=$!
fi

for _ in {1..120}; do
  if curl -sf http://127.0.0.1:8080/healthz >/dev/null && frontend_ready; then
    echo "Auth E2E stack is ready"
    wait_pids=()
    if [[ -n "${BACKEND_PID:-}" ]]; then
      wait_pids+=("$BACKEND_PID")
    fi
    if [[ -n "${FRONTEND_PID:-}" ]]; then
      wait_pids+=("$FRONTEND_PID")
    fi
    if ((${#wait_pids[@]} > 0)); then
      wait "${wait_pids[@]}"
      exit $?
    fi
    while true; do sleep 3600; done
  fi
  sleep 1
done

echo "Auth E2E stack failed to become ready" >&2
exit 1
