# Lobby Google and Discord OAuth credentials.
# Source from deploy scripts after NAMESPACE is set.

apply_lobby_oauth_secret() {
  local secrets_file="${1:-k8s/secrets/lobby-oauth.yaml}"
  # -s, not -f: an empty file is unusable too, and kubectl apply would fail on it.
  if [ ! -s "$secrets_file" ]; then
    echo "OAuth: $secrets_file missing or empty — Google/Discord sign-in disabled until configured"
    return 0
  fi

  echo "Applying $secrets_file (namespace ${NAMESPACE})..."
  kubectl apply -f "$secrets_file"

  if kubectl get secret lobby-oauth -n "$NAMESPACE" >/dev/null 2>&1; then
    kubectl set env deployment/lobby-backend -n "$NAMESPACE" \
      --from=secret/lobby-oauth \
      --containers=backend
  fi
}

load_lobby_oauth_env_from_file() {
  local secrets_file="${1:-k8s/secrets/lobby-oauth.yaml}"
  [ -s "$secrets_file" ] || return 0

  local google_id google_secret discord_id discord_secret
  google_id="$(grep -E '^[[:space:]]*GOOGLE_OAUTH_CLIENT_ID:' "$secrets_file" | head -1 | sed -E 's/^[[:space:]]*GOOGLE_OAUTH_CLIENT_ID:[[:space:]]*//; s/^["'\''"]//; s/["'\''"]$//')"
  google_secret="$(grep -E '^[[:space:]]*GOOGLE_OAUTH_CLIENT_SECRET:' "$secrets_file" | head -1 | sed -E 's/^[[:space:]]*GOOGLE_OAUTH_CLIENT_SECRET:[[:space:]]*//; s/^["'\''"]//; s/["'\''"]$//')"
  discord_id="$(grep -E '^[[:space:]]*DISCORD_OAUTH_CLIENT_ID:' "$secrets_file" | head -1 | sed -E 's/^[[:space:]]*DISCORD_OAUTH_CLIENT_ID:[[:space:]]*//; s/^["'\''"]//; s/["'\''"]$//')"
  discord_secret="$(grep -E '^[[:space:]]*DISCORD_OAUTH_CLIENT_SECRET:' "$secrets_file" | head -1 | sed -E 's/^[[:space:]]*DISCORD_OAUTH_CLIENT_SECRET:[[:space:]]*//; s/^["'\''"]//; s/["'\''"]$//')"

  case "$google_id" in
    '' | REPLACE_WITH_GOOGLE_CLIENT_ID) ;;
    *) export GOOGLE_OAUTH_CLIENT_ID="${GOOGLE_OAUTH_CLIENT_ID:-$google_id}" ;;
  esac
  case "$google_secret" in
    '' | REPLACE_WITH_GOOGLE_CLIENT_SECRET) ;;
    *) export GOOGLE_OAUTH_CLIENT_SECRET="${GOOGLE_OAUTH_CLIENT_SECRET:-$google_secret}" ;;
  esac
  case "$discord_id" in
    '' | REPLACE_WITH_DISCORD_CLIENT_ID) ;;
    *) export DISCORD_OAUTH_CLIENT_ID="${DISCORD_OAUTH_CLIENT_ID:-$discord_id}" ;;
  esac
  case "$discord_secret" in
    '' | REPLACE_WITH_DISCORD_CLIENT_SECRET) ;;
    *) export DISCORD_OAUTH_CLIENT_SECRET="${DISCORD_OAUTH_CLIENT_SECRET:-$discord_secret}" ;;
  esac
}
