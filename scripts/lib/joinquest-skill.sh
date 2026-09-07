#!/usr/bin/env bash
# Install JoinQuest agent skill into a project directory.
set -euo pipefail

joinquest_install_skill() {
  local dest="${1:-.agents/skills/joinquest-integration}"
  local branch="${JOINQUEST_SKILL_BRANCH:-main}"
  local repo="https://github.com/scruffyprodigy/joinquest"

  _joinquest_copy_skill() {
    local src=$1
    mkdir -p "$(dirname "$dest")"
    rm -rf "$dest"
    cp -R "$src" "$dest"
  }

  local script_dir
  script_dir="$(CDPATH= cd -- "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
  local local_skill="$script_dir/../../.agents/skills/joinquest-integration"

  if [ -f "$local_skill/SKILL.md" ]; then
    _joinquest_copy_skill "$local_skill"
    echo "Installed JoinQuest agent skill to $dest"
    return 0
  fi

  if ! command -v curl >/dev/null 2>&1; then
    echo "curl is required to download the skill from GitHub." >&2
    return 1
  fi

  local tmp
  tmp="$(mktemp -d)"

  echo "Downloading JoinQuest agent skill from $repo ($branch)..."
  curl -fsSL "https://codeload.github.com/scruffyprodigy/joinquest/tar.gz/refs/heads/$branch" \
    | tar -xz -C "$tmp"

  local extracted skill_src
  extracted="$(find "$tmp" -mindepth 1 -maxdepth 1 -type d | head -1)"
  skill_src="$extracted/.agents/skills/joinquest-integration"

  if [ ! -f "$skill_src/SKILL.md" ]; then
    rm -rf "$tmp"
    echo "Could not find skill in GitHub tarball (branch: $branch)." >&2
    return 1
  fi

  _joinquest_copy_skill "$skill_src"
  rm -rf "$tmp"
  echo "Installed JoinQuest agent skill to $dest"
}
