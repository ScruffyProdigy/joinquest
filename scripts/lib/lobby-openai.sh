# OpenAI credentials for spirit-animal avatar generation.
# Source from deploy scripts after NAMESPACE is set.

apply_lobby_openai_secret() {
  local secrets_file="${1:-k8s/secrets/lobby-openai.yaml}"
  # -s, not -f: an empty file is unusable too, and kubectl apply would fail on it.
  if [ ! -s "$secrets_file" ]; then
    echo "OpenAI: $secrets_file missing or empty — spirit animal flow uses mock LLM/images"
    return 0
  fi

  echo "Applying $secrets_file (namespace ${NAMESPACE})..."
  kubectl apply -f "$secrets_file"

  if kubectl get secret lobby-openai -n "$NAMESPACE" >/dev/null 2>&1; then
    kubectl set env deployment/lobby-backend -n "$NAMESPACE" \
      --from=secret/lobby-openai \
      --containers=backend
    kubectl rollout restart deployment/lobby-backend -n "$NAMESPACE"
  fi
}

# Load OPENAI_* from lobby-openai.yaml for local ./scripts/dev.sh (no kubectl).
load_lobby_openai_env_from_file() {
  local secrets_file="${1:-k8s/secrets/lobby-openai.yaml}"
  [ -s "$secrets_file" ] || return 0

  if [ -n "${OPENAI_API_KEY:-}" ]; then
    return 0
  fi

  local key chat image
  key="$(grep -E '^[[:space:]]*OPENAI_API_KEY:' "$secrets_file" | head -1 | sed -E 's/^[[:space:]]*OPENAI_API_KEY:[[:space:]]*//; s/^["'\''"]//; s/["'\''"]$//')"
  chat="$(grep -E '^[[:space:]]*OPENAI_CHAT_MODEL:' "$secrets_file" | head -1 | sed -E 's/^[[:space:]]*OPENAI_CHAT_MODEL:[[:space:]]*//; s/^["'\''"]//; s/["'\''"]$//')"
  image="$(grep -E '^[[:space:]]*OPENAI_IMAGE_MODEL:' "$secrets_file" | head -1 | sed -E 's/^[[:space:]]*OPENAI_IMAGE_MODEL:[[:space:]]*//; s/^["'\''"]//; s/["'\''"]$//')"

  case "$key" in
    '' | REPLACE_WITH_OPENAI_API_KEY | PASTE_OPENAI_API_KEY_HERE) return 0 ;;
    *) export OPENAI_API_KEY="$key" ;;
  esac
  case "$chat" in
    '' | '#'* ) ;;
    *) export OPENAI_CHAT_MODEL="${OPENAI_CHAT_MODEL:-$chat}" ;;
  esac
  case "$image" in
    '' | '#'* ) ;;
    *) export OPENAI_IMAGE_MODEL="${OPENAI_IMAGE_MODEL:-$image}" ;;
  esac
}
