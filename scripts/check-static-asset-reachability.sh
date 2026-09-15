#!/usr/bin/env bash
# Fail if a static asset committed under frontend/public/ is not actually
# reachable on a deployed site.
#
# The existing post-deploy smoke checks all exercise the *app*: the page loads,
# /graphql answers, /developers renders, env.js is not cached. None of them
# exercise the *assets* the app serves, so an asset-layer failure ships green.
# JQ-127 was exactly that -- six CalSans .woff files served 403 from a healthy
# deploy with every check passing, because the page referencing them still
# rendered. This walks the assets themselves (JQ-129).
#
# Two ways an asset can be broken, both checked:
#   * a non-200 status  -- unreadable (403, as in JQ-127) or missing (404)
#   * a 200 that is HTML -- nginx's SPA fallback (`try_files ... /index.html`)
#     answers for any path whose extension misses the asset location blocks in
#     frontend/nginx.conf, so a missing .txt returns 200 text/html. Status alone
#     would call that healthy.
#
# Usage:
#   scripts/check-static-asset-reachability.sh [BASE_URL]
#
#   BASE_URL defaults to $LOBBY_PUBLIC_URL, then to https://joinquest.cc.
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT"

PUBLIC_DIR="frontend/public"
BASE_URL="${1:-${LOBBY_PUBLIC_URL:-https://joinquest.cc}}"
BASE_URL="${BASE_URL%/}"
PARALLEL="${ASSET_CHECK_PARALLEL:-8}"
# A rollout can report available a moment before every pod is serving, so a
# single miss is retried rather than failing the deploy on a race.
ATTEMPTS="${ASSET_CHECK_ATTEMPTS:-3}"
RETRY_DELAY="${ASSET_CHECK_RETRY_DELAY:-5}"

if [ ! -d "$PUBLIC_DIR" ]; then
  echo "❌ $PUBLIC_DIR does not exist." >&2
  exit 1
fi

TMPDIR_ASSETS="$(mktemp -d)"
trap 'rm -rf "$TMPDIR_ASSETS"' EXIT

PENDING="$TMPDIR_ASSETS/pending"
FAILURES="$TMPDIR_ASSETS/failures"

find "$PUBLIC_DIR" -type f -print \
  | sed "s|^$PUBLIC_DIR||" \
  | sort > "$PENDING"

TOTAL="$(wc -l < "$PENDING" | tr -d ' ')"

if [ "$TOTAL" -eq 0 ]; then
  echo "❌ No static assets found under $PUBLIC_DIR." >&2
  exit 1
fi

echo "Checking $TOTAL static assets against $BASE_URL ..."

# Probe one asset. Prints a tab-separated failure line, or nothing when healthy.
check_asset() {
  local path="$1"
  local result http ctype
  # Query string keeps an edge cache from answering for the origin. nginx
  # matches on $uri, so the extra parameter changes nothing server-side.
  result="$(curl -sS -I -o /dev/null \
    -w '%{http_code} %{content_type}' \
    --max-time 20 \
    "${BASE_URL}${path}?smoke=${RANDOM}${RANDOM}" 2>/dev/null || true)"
  [ -n "$result" ] || result="000 (no response)"

  http="${result%% *}"
  ctype="${result#* }"
  ctype="${ctype%%;*}"
  [ -n "$ctype" ] || ctype="(none)"

  if [ "$http" != "200" ]; then
    printf '%s\t%s\t%s\n' "$path" "$http" "$ctype"
    return 0
  fi

  case "$path" in
    *.html|*.htm) ;;
    *)
      if [ "$ctype" = "text/html" ]; then
        printf '%s\t%s\t%s\n' "$path" "$http" "$ctype — SPA fallback, asset is missing"
      fi
      ;;
  esac
}
export -f check_asset
export BASE_URL

attempt=1
while [ "$attempt" -le "$ATTEMPTS" ]; do
  xargs -P "$PARALLEL" -I{} bash -c 'check_asset "$1"' _ {} < "$PENDING" | sort > "$FAILURES"

  if [ ! -s "$FAILURES" ]; then
    break
  fi

  if [ "$attempt" -lt "$ATTEMPTS" ]; then
    echo "  $(wc -l < "$FAILURES" | tr -d ' ') asset(s) unreachable, retrying in ${RETRY_DELAY}s (attempt $((attempt + 1))/$ATTEMPTS)..."
    cut -f1 "$FAILURES" > "$PENDING"
    sleep "$RETRY_DELAY"
  fi

  attempt=$((attempt + 1))
done

if [ ! -s "$FAILURES" ]; then
  echo "✅ All $TOTAL static assets are reachable on $BASE_URL."
  exit 0
fi

echo "" >&2
echo "❌ Static assets that $BASE_URL does not serve:" >&2
while IFS=$'\t' read -r path http ctype; do
  [ -n "$path" ] || continue
  printf '  %-4s %-48s %s\n' "$http" "$path" "$ctype" >&2
done < "$FAILURES"
echo "" >&2
echo "A 403 means the file shipped unreadable — check permissions in the image:" >&2
echo "  scripts/check-static-asset-permissions.sh" >&2
echo "  kubectl exec deployment/lobby-frontend -n joinquest -- ls -la /usr/share/nginx/html/fonts/" >&2
echo "A 404, or a 200 serving text/html, means the file is not in the image at all." >&2
exit 1
