#!/usr/bin/env bash
# Deprecated: use install-joinquest-dev.sh
exec "$(CDPATH= cd -- "$(dirname "$0")" && pwd)/install-joinquest-dev.sh" --skill-only "$@"
