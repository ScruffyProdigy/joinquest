#!/bin/bash
# Per-working-copy compose stacks and per-run test databases (JQ-128).
#
# Ryan routinely runs several agents on different tickets at once. Without this,
# two of them running the backend suite collide twice over: on the published
# host port (only one process can bind 5432) and on the shared playhub_test
# database (each truncates the other's fixtures mid-test).
#
# Sourced by scripts/db.sh and scripts/test-backend.sh. Nothing here talks to
# Docker except lobby_host_port, so the naming helpers stay cheap and testable.

# Short, stable digest of a string. Used to key a stack to a working copy.
lobby_digest() {
  local input="$1"
  if command -v shasum >/dev/null 2>&1; then
    printf '%s' "$input" | shasum -a 256 | cut -c1-8
  elif command -v sha256sum >/dev/null 2>&1; then
    printf '%s' "$input" | sha256sum | cut -c1-8
  else
    printf '%s' "$input" | cksum | awk '{printf "%08x", $1}'
  fi
}

# Docker Compose project name for a working copy. Two checkouts of this repo —
# two worktrees, or a clone sitting beside it — must never share a project, or
# `docker compose down` in one agent's tree tears down another's stack.
lobby_compose_project() {
  local root="${1:-$PWD}"
  local slug
  slug="$(basename "$root" | tr '[:upper:]' '[:lower:]' | tr -c 'a-z0-9' '-' | sed -E 's/-+/-/g; s/^-|-$//g')"
  [ -n "$slug" ] || slug="lobby"
  # The digest keeps two directories with the same basename apart.
  printf 'lobby-%s-%s' "$slug" "$(lobby_digest "$root")"
}

# Stable per-working-copy id. Used when no explicit run id is set, so a
# developer running `./scripts/db.sh test-url` twice gets the same database.
lobby_workspace_id() {
  lobby_digest "${1:-$PWD}"
}

# Fresh id for one suite invocation, so two runs in the same working copy still
# get their own database. Timestamp keeps stray databases self-identifying.
lobby_new_run_id() {
  printf '%s_%s' "$(date -u +%Y%m%dt%H%M%SZ | tr '[:upper:]' '[:lower:]')" "$(lobby_digest "$$-$RANDOM-$(date +%s%N 2>/dev/null || date +%s)")"
}

# Postgres caps identifiers at 63 bytes; the prefix leaves ample room but the
# run id comes from the environment sometimes, so truncate rather than let
# Postgres silently fold a long name into a colliding one.
lobby_test_database() {
  local run_id="$1"
  local sanitized
  sanitized="$(printf '%s' "$run_id" | tr '[:upper:]' '[:lower:]' | tr -c 'a-z0-9_' '_' | sed -E 's/_+/_/g; s/^_|_$//g')"
  [ -n "$sanitized" ] || sanitized="default"
  printf 'playhub_test_%s' "${sanitized:0:50}"
}

# Host port docker actually published for a service, e.g. lobby_host_port postgres 5432.
# Empty when the stack is not running.
lobby_host_port() {
  local service="$1" container_port="$2"
  docker compose port "$service" "$container_port" 2>/dev/null | sed -E 's/.*:([0-9]+)$/\1/'
}

# True when something already holds a TCP port on the loopback interface.
lobby_port_in_use() {
  nc -z 127.0.0.1 "$1" >/dev/null 2>&1
}
