#!/bin/bash
# Verify that two agents can run the backend suite at the same time without
# colliding on the host Postgres port or sharing the test database (JQ-128).
#
#   ./scripts/check-concurrent-test-isolation.sh            hermetic checks only
#   ./scripts/check-concurrent-test-isolation.sh --docker   also start two real stacks
set -uo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT"

RUN_DOCKER=0
if [ "${1:-}" = "--docker" ]; then
  RUN_DOCKER=1
fi

FAILURES=0

pass() { echo "  ✓ $1"; }
fail() {
  echo "  ✗ $1"
  FAILURES=$((FAILURES + 1))
}

assert_eq() {
  if [ "$2" = "$3" ]; then pass "$1"; else fail "$1 (got '$2', want '$3')"; fi
}

assert_ne() {
  if [ "$2" != "$3" ]; then pass "$1"; else fail "$1 (both were '$2')"; fi
}

assert_match() {
  if [[ "$2" =~ $3 ]]; then pass "$1"; else fail "$1 ('$2' does not match /$3/)"; fi
}

assert_contains() {
  if [[ "$2" == *"$3"* ]]; then pass "$1"; else fail "$1 (missing '$3' in: $2)"; fi
}

echo "Compose project names are unique per working copy"
# shellcheck source=lib/db-runtime.sh
source "$ROOT/scripts/lib/db-runtime.sh"

project_a="$(lobby_compose_project /tmp/agent-a/lobby)"
project_b="$(lobby_compose_project /tmp/agent-b/lobby)"
project_a_again="$(lobby_compose_project /tmp/agent-a/lobby)"

assert_ne "two working copies get different compose projects" "$project_a" "$project_b"
assert_eq "the same working copy is stable across calls" "$project_a" "$project_a_again"
assert_match "compose project name is valid for docker" "$project_a" '^[a-z0-9][a-z0-9_-]*$'

echo ""
echo "Each run gets its own test database"
db_one="$(lobby_test_database "$(lobby_new_run_id)")"
db_two="$(lobby_test_database "$(lobby_new_run_id)")"

assert_ne "two runs get different databases" "$db_one" "$db_two"
assert_match "per-run database keeps the playhub_test_ prefix" "$db_one" '^playhub_test_[a-z0-9_]+$'
if [ "${#db_one}" -le 63 ]; then
  pass "per-run database fits Postgres' 63-byte identifier limit (${#db_one})"
else
  fail "per-run database is ${#db_one} bytes, over Postgres' 63-byte limit"
fi

echo ""
echo "The compose stack does not hardcode a published host port"
if grep -qE '^\s*-\s*"5432:5432"' docker-compose.yml; then
  fail "docker-compose.yml still publishes a fixed 5432:5432"
else
  pass "postgres host port is not pinned to 5432"
fi
if grep -qE '^\s*-\s*"6379:6379"' docker-compose.yml; then
  fail "docker-compose.yml still publishes a fixed 6379:6379"
else
  pass "redis host port is not pinned to 6379"
fi

echo ""
echo "A port conflict is reported by name, not as a raw Docker error"
# Occupy a port, then ask db.sh to use exactly that one.
python3 - <<'PY' > /tmp/jq128-port.txt 2>/dev/null || true
import socket
s = socket.socket()
s.bind(("127.0.0.1", 0))
print(s.getsockname()[1])
s.close()
PY
busy_port="$(cat /tmp/jq128-port.txt 2>/dev/null)"
rm -f /tmp/jq128-port.txt
if [ -n "$busy_port" ]; then
  python3 -c "
import socket, time
s = socket.socket()
s.setsockopt(socket.SOL_SOCKET, socket.SO_REUSEADDR, 1)
s.bind(('127.0.0.1', $busy_port))
s.listen(1)
time.sleep(20)
" &
  holder=$!
  sleep 1
  conflict_output="$(LOBBY_POSTGRES_HOST_PORT="$busy_port" "$ROOT/scripts/db.sh" up 2>&1)"
  kill "$holder" 2>/dev/null
  wait "$holder" 2>/dev/null
  assert_contains "the message names the conflicting port" "$conflict_output" "$busy_port"
  assert_contains "the message names LOBBY_POSTGRES_HOST_PORT as the resolution" "$conflict_output" "LOBBY_POSTGRES_HOST_PORT"
else
  fail "could not reserve a port to test the conflict message"
fi

if [ "$RUN_DOCKER" -eq 1 ]; then
  echo ""
  echo "Two suites really do run side by side"
  if ! docker info >/dev/null 2>&1; then
    fail "--docker was requested but the Docker daemon is not reachable"
  else
    copy_a="$(mktemp -d)/lobby-a"
    copy_b="$(mktemp -d)/lobby-b"
    git worktree list >/dev/null 2>&1
    mkdir -p "$copy_a" "$copy_b"
    for copy in "$copy_a" "$copy_b"; do
      cp docker-compose.yml "$copy/"
      mkdir -p "$copy/scripts/lib" "$copy/docker"
      cp scripts/db.sh "$copy/scripts/"
      cp scripts/lib/db-runtime.sh "$copy/scripts/lib/"
      cp -R docker/postgres "$copy/docker/" 2>/dev/null || true
    done

    ( cd "$copy_a" && ./scripts/db.sh up ) > /tmp/jq128-a.log 2>&1 &
    pid_a=$!
    ( cd "$copy_b" && ./scripts/db.sh up ) > /tmp/jq128-b.log 2>&1 &
    pid_b=$!
    wait "$pid_a"; rc_a=$?
    wait "$pid_b"; rc_b=$?

    assert_eq "first stack came up" "$rc_a" "0"
    assert_eq "second stack came up" "$rc_b" "0"
    if [ "$rc_a" -ne 0 ]; then echo "--- copy A ---"; cat /tmp/jq128-a.log; fi
    if [ "$rc_b" -ne 0 ]; then echo "--- copy B ---"; cat /tmp/jq128-b.log; fi

    if [ "$rc_a" -eq 0 ] && [ "$rc_b" -eq 0 ]; then
      url_a="$(cd "$copy_a" && ./scripts/db.sh test-url)"
      url_b="$(cd "$copy_b" && ./scripts/db.sh test-url)"
      assert_ne "the two stacks publish different host ports" \
        "$(sed -E 's|.*@[^:]+:([0-9]+)/.*|\1|' <<<"$url_a")" \
        "$(sed -E 's|.*@[^:]+:([0-9]+)/.*|\1|' <<<"$url_b")"
      assert_ne "the two runs target different databases" \
        "$(sed -E 's|.*/([^?]+).*|\1|' <<<"$url_a")" \
        "$(sed -E 's|.*/([^?]+).*|\1|' <<<"$url_b")"
    fi

    ( cd "$copy_a" && ./scripts/db.sh down ) >/dev/null 2>&1
    ( cd "$copy_b" && ./scripts/db.sh down ) >/dev/null 2>&1
    rm -rf "$copy_a" "$copy_b" /tmp/jq128-a.log /tmp/jq128-b.log
  fi
fi

echo ""
if [ "$FAILURES" -eq 0 ]; then
  echo "✓ Concurrent test isolation checks passed"
  exit 0
fi
echo "✗ $FAILURES concurrent test isolation check(s) failed"
exit 1
