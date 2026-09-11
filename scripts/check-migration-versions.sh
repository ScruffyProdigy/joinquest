#!/bin/bash
#
# Guards against two migrations claiming the same version number.
#
# golang-migrate refuses to build a migrator at all when it finds a duplicate
# version ("duplicate migration file: ..."), so this is not a style question:
# every migration fails, which means every deploy and every fresh test
# database fails with it.
#
# The failure is specific to how this repo is worked. Several agents branch
# from origin/main at once, each picks the next free number by looking at the
# tree, and both are right when they look. Nothing collides until both merge,
# and neither PR's CI sees the other's number — so the break lands on main
# having been green on both branches. That is exactly what happened between
# PRs #98 and #101, and a check on the merged tree is the only place it can
# be caught.
#
# Renumber the migration that merged second; the one already applied
# somewhere keeps its number.

set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
MIGRATIONS="$ROOT/backend/migrations"

if [ ! -d "$MIGRATIONS" ]; then
    echo "✗ no migrations directory at $MIGRATIONS"
    exit 1
fi

# Reduce each file to its "<version>_<name>" stem, take the distinct stems, and
# then look for a version claimed by more than one of them. Collapsing the
# up/down pair first is what makes the count meaningful: every migration
# legitimately contributes two files under one version, so counting files
# would flag every migration in the tree.
duplicates="$(
    find "$MIGRATIONS" -name '*.sql' -type f -print0 \
        | xargs -0 -n1 basename \
        | sed -E 's/\.(up|down)\.sql$//' \
        | sort -u \
        | sed -E 's/^([0-9]+)_.*$/\1/' \
        | uniq -c \
        | awk '$1 > 1 { print $2 }'
)"

if [ -n "$duplicates" ]; then
    echo "✗ more than one migration claims the same version:"
    for version in $duplicates; do
        echo
        find "$MIGRATIONS" -name "${version}_*.sql" -type f -exec basename {} \; | sort | sed 's/^/    /'
    done
    echo
    echo "  golang-migrate refuses every migration while this is true."
    echo "  Renumber the one that merged second to the next free version."
    exit 1
fi

count="$(find "$MIGRATIONS" -name '*.up.sql' -type f | wc -l | tr -d ' ')"
echo "✓ $count migration versions, no duplicates"
