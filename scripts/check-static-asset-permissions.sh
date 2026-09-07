#!/usr/bin/env bash
# Fail if a static asset in the frontend web root is not world-readable.
#
# frontend/Dockerfile builds from the working tree (COPY . .), not from git, so
# a file left at mode 600 on the build host ships that way into the nginx image
# and serves 403 in production. The Dockerfile normalises permissions on the
# copied web root, so this check is the fast, loud signal rather than the
# guarantee: it catches the file here instead of at runtime (JQ-127).
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT"

PUBLIC_DIR="frontend/public"

echo "Checking static asset permissions under $PUBLIC_DIR..."

if [ ! -d "$PUBLIC_DIR" ]; then
  echo "❌ $PUBLIC_DIR does not exist." >&2
  exit 1
fi

# Files must be world-readable; directories must also be world-executable or
# nginx cannot traverse into them.
OFFENDERS="$(find "$PUBLIC_DIR" \
  \( -type f ! -perm -o+r \) -o \
  \( -type d ! -perm -o+rx \) \
  | sort)"

if [ -z "$OFFENDERS" ]; then
  echo "✅ All static assets under $PUBLIC_DIR are world-readable."
  exit 0
fi

echo "❌ Static assets that nginx could not read in the image:" >&2
while IFS= read -r path; do
  [ -n "$path" ] || continue
  printf '  %s  %s\n' "$(ls -ld "$path" | awk '{print $1}')" "$path" >&2
done <<< "$OFFENDERS"
echo "" >&2
echo "Fix with:" >&2
echo "  chmod -R a+rX $PUBLIC_DIR" >&2
exit 1
