# Lobby MAGIC_LINK_PEPPER and LOBBY_GAME_TOKEN_PEPPER.
# Source from deploy scripts after NAMESPACE is set.

apply_lobby_auth_peppers_secret() {
  local secrets_file="${1:-k8s/secrets/lobby-auth-peppers.yaml}"
  # -s, not -f: an empty file is unusable too, and kubectl apply would fail on it.
  if [ ! -s "$secrets_file" ]; then
    echo "Auth peppers: $secrets_file missing or empty — using dev defaults (not recommended in production)"
    return 0
  fi

  echo "Applying $secrets_file (namespace ${NAMESPACE})..."
  kubectl apply -f "$secrets_file"

  if kubectl get secret lobby-auth-peppers -n "$NAMESPACE" >/dev/null 2>&1; then
    kubectl set env deployment/lobby-backend -n "$NAMESPACE" \
      --from=secret/lobby-auth-peppers \
      --containers=backend
  fi
}

# Load pepper env vars from lobby-auth-peppers.yaml for local ./scripts/dev.sh (no kubectl).
load_lobby_auth_peppers_env_from_file() {
  local secrets_file="${1:-k8s/secrets/lobby-auth-peppers.yaml}"
  [ -s "$secrets_file" ] || return 0

  if [ -n "${MAGIC_LINK_PEPPER:-}" ] && [ -n "${LOBBY_GAME_TOKEN_PEPPER:-}" ]; then
    return 0
  fi

  local magic game
  magic="$(grep -E '^[[:space:]]*MAGIC_LINK_PEPPER:' "$secrets_file" | head -1 | sed -E 's/^[[:space:]]*MAGIC_LINK_PEPPER:[[:space:]]*//; s/^["'\''"]//; s/["'\''"]$//')"
  game="$(grep -E '^[[:space:]]*LOBBY_GAME_TOKEN_PEPPER:' "$secrets_file" | head -1 | sed -E 's/^[[:space:]]*LOBBY_GAME_TOKEN_PEPPER:[[:space:]]*//; s/^["'\''"]//; s/["'\''"]$//')"

  case "$magic" in
    '' | CHANGE_ME_MAGIC_LINK | PASTE_MAGIC_LINK_PEPPER_HERE) ;;
    *) export MAGIC_LINK_PEPPER="${MAGIC_LINK_PEPPER:-$magic}" ;;
  esac
  case "$game" in
    '' | CHANGE_ME_GAME_TOKEN | PASTE_LOBBY_GAME_TOKEN_PEPPER_HERE) ;;
    *) export LOBBY_GAME_TOKEN_PEPPER="${LOBBY_GAME_TOKEN_PEPPER:-$game}" ;;
  esac
}
