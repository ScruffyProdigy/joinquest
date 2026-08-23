# Shared Resend/SMTP setup for deploy scripts. Source from repo root:
#   . "$(dirname "$0")/lib/lobby-smtp.sh"
#
# Expects: NAMESPACE, optional SMTP_FROM, SMTP_FROM_NAME

apply_lobby_smtp_secret() {
  local secrets_file="${1:-k8s/secrets/lobby-smtp.yaml}"
  # -s, not -f: an empty file is unusable too, and kubectl apply would fail on it.
  if [ ! -s "$secrets_file" ]; then
    echo "SMTP: $secrets_file missing or empty — magic links log to backend stdout only"
    return 0
  fi

  echo "Applying $secrets_file ..."
  kubectl apply -f "$secrets_file"

  local from="${SMTP_FROM:-noreply@joinquest.cc}"
  local from_name="${SMTP_FROM_NAME:-JoinQuest}"

  kubectl set env deployment/lobby-backend -n "$NAMESPACE" \
    SMTP_HOST=smtp.resend.com \
    SMTP_PORT=587 \
    SMTP_USERNAME=resend \
    SMTP_FROM="$from" \
    SMTP_FROM_NAME="$from_name" \
    --containers=backend

  if kubectl get secret lobby-smtp -n "$NAMESPACE" >/dev/null 2>&1; then
    kubectl set env deployment/lobby-backend -n "$NAMESPACE" \
      --from=secret/lobby-smtp --containers=backend
  fi
}

# Load SMTP_* from lobby-smtp.yaml for local ./scripts/dev.sh (no kubectl).
load_lobby_smtp_env_from_file() {
  local secrets_file="${1:-k8s/secrets/lobby-smtp.yaml}"
  [ -s "$secrets_file" ] || return 0
  if [ -n "${SMTP_PASSWORD:-}" ]; then
    return 0
  fi

  local pass
  pass="$(grep -E '^[[:space:]]*SMTP_PASSWORD:' "$secrets_file" | head -1 | sed -E 's/^[[:space:]]*SMTP_PASSWORD:[[:space:]]*//; s/^["'\''"]//; s/["'\''"]$//')"
  case "$pass" in
    '' | REPLACE_WITH_RESEND_API_KEY | PASTE_RESEND_API_KEY_HERE)
      return 0
      ;;
  esac

  export SMTP_HOST="${SMTP_HOST:-smtp.resend.com}"
  export SMTP_PORT="${SMTP_PORT:-587}"
  export SMTP_USERNAME="${SMTP_USERNAME:-resend}"
  export SMTP_PASSWORD="$pass"
  export SMTP_FROM="${SMTP_FROM:-noreply@joinquest.cc}"
  export SMTP_FROM_NAME="${SMTP_FROM_NAME:-JoinQuest}"
}
