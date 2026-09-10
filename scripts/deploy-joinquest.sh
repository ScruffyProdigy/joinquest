#!/bin/bash
# Deploy Lobby to namespace joinquest (GKE demo).
# Secrets in k8s/secrets/*.yaml must already use metadata.namespace: joinquest.
set -e

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT"
# shellcheck source=lib/lobby-smtp.sh
. "$ROOT/scripts/lib/lobby-smtp.sh"
# shellcheck source=lib/lobby-game-service.sh
. "$ROOT/scripts/lib/lobby-game-service.sh"
# shellcheck source=lib/lobby-auth-peppers.sh
. "$ROOT/scripts/lib/lobby-auth-peppers.sh"
# shellcheck source=lib/lobby-openai.sh
. "$ROOT/scripts/lib/lobby-openai.sh"
# shellcheck source=lib/lobby-oauth.sh
. "$ROOT/scripts/lib/lobby-oauth.sh"
# shellcheck source=lib/lobby-migrations.sh
. "$ROOT/scripts/lib/lobby-migrations.sh"

NAMESPACE="joinquest"
CONTEXT="${KUBE_CONTEXT:-}"
LOBBY_PUBLIC_URL="${LOBBY_PUBLIC_URL:-https://joinquest.cc}"
SMTP_FROM="${SMTP_FROM:-noreply@joinquest.cc}"
SMTP_FROM_NAME="${SMTP_FROM_NAME:-JoinQuest}"
INSTALL_CERT_MANAGER="${INSTALL_CERT_MANAGER:-false}"

echo "Deploying Lobby to namespace: $NAMESPACE"

if ! command -v kubectl &> /dev/null; then
  echo "kubectl not found" >&2
  exit 1
fi

if [ -n "$CONTEXT" ]; then
  echo "Using context: $CONTEXT"
  kubectl config use-context "$CONTEXT"
fi

if ! kubectl get namespace "$NAMESPACE" >/dev/null 2>&1; then
  kubectl create namespace "$NAMESPACE"
fi

if [ "$INSTALL_CERT_MANAGER" = "true" ]; then
  echo "Installing cert-manager..."
  kubectl apply -f https://github.com/cert-manager/cert-manager/releases/download/v1.14.5/cert-manager.yaml
  kubectl wait --for=condition=available --timeout=300s deployment/cert-manager -n cert-manager
  kubectl wait --for=condition=available --timeout=300s deployment/cert-manager-webhook -n cert-manager
fi

# GKE Autopilot: controller cannot use kube-system for leader election (Warden denies leases).
"$ROOT/scripts/patch-cert-manager-gke.sh"

echo "Applying Let's Encrypt ClusterIssuer (cluster-wide)..."
kubectl apply -f k8s/env/cluster-issuer-letsencrypt-prod.yaml

echo "Applying secrets..."
kubectl apply -f k8s/secrets/pg-auth.yaml
kubectl apply -f k8s/secrets/pg-dsn.yaml
kubectl apply -f k8s/secrets/jwks-secret.yaml

# Everything except the backend. The backend is applied further down, after the
# migration job, so its pod never races that job for schema_migrations.
echo "Applying base manifests (namespace joinquest)..."
kubectl apply -f k8s/base/postgres.yaml
kubectl apply -f k8s/base/redis.yaml
kubectl apply -f k8s/base/frontend.yaml

echo "Applying joinquest TLS certificate + ingress..."
kubectl apply -f k8s/env/joinquest-certificate.yaml
kubectl apply -f k8s/env/joinquest-ingress.yaml

echo "Applying joinquest frontend config..."
kubectl apply -f k8s/env/joinquest.yaml

echo "Waiting for Postgres..."
kubectl wait --for=condition=ready --timeout=300s pod -l app=pg -n "$NAMESPACE"

echo "Waiting for Redis..."
kubectl wait --for=condition=ready --timeout=120s pod -l app=lobby-redis -n "$NAMESPACE"

# GKE nodes may cache :latest; pin to the digest we just pushed when available locally.
BACKEND_IMAGE="docker.io/scruffyprodigy/joinquest-backend:latest"
FRONTEND_IMAGE="docker.io/scruffyprodigy/joinquest-frontend:latest"

resolve_local_image() {
  local ref=$1
  if ! command -v docker >/dev/null 2>&1; then
    echo "$ref"
    return
  fi
  local digest
  digest=$(docker image inspect "$ref" --format '{{index .RepoDigests 0}}' 2>/dev/null | sed 's/.*@//')
  if [ -n "$digest" ]; then
    echo "${ref%%@*}@${digest}"
  else
    echo "$ref"
  fi
}

pin_deployment_image() {
  local deploy=$1 container=$2 ref=$3
  echo "Pinning ${deploy}/${container} to ${ref}"
  kubectl set image "deployment/${deploy}" -n "$NAMESPACE" "${container}=${ref}"
}

BACKEND_IMAGE="$(resolve_local_image "$BACKEND_IMAGE")"
FRONTEND_IMAGE="$(resolve_local_image "$FRONTEND_IMAGE")"

preflight_migration_state "$NAMESPACE" "$BACKEND_IMAGE"
apply_database_migrations "$NAMESPACE" "$BACKEND_IMAGE"

echo "Applying stale session cleanup CronJob..."
kubectl apply -f k8s/jobs/stale-session-cleanup.yaml

# Talks to Postgres directly, so it does not need the backend up yet.
echo "Patching game catalog handoff URLs for production..."
run_patch_game_handoff_urls_job "$NAMESPACE"

# ---------------------------------------------------------------------------
# Backend rollout. Everything below restarts lobby-backend, so it must stay
# after the migration job -- see scripts/check-deploy-migration-order.sh.
# ---------------------------------------------------------------------------
echo "Applying backend manifest (namespace joinquest)..."
kubectl apply -f k8s/base/backend.yaml

echo "Configuring backend for browser auth at $LOBBY_PUBLIC_URL ..."
kubectl set env deployment/lobby-backend -n "$NAMESPACE" \
  APP_ENV=production \
  CORS_ALLOWED_ORIGINS="$LOBBY_PUBLIC_URL" \
  MAGIC_LINK_BASE_URL="${LOBBY_PUBLIC_URL}/auth/complete?token=" \
  LOBBY_ISSUER_URL="${LOBBY_ISSUER_URL:-$LOBBY_PUBLIC_URL}" \
  SESSION_COOKIE_SECURE=true \
  LOBBY_ADMIN_EMAILS="${LOBBY_ADMIN_EMAILS:-ryan.c.kohler@gmail.com}" \
  REDIS_URL="redis://lobby-redis:6379/0" \
  --containers=backend
apply_lobby_smtp_secret
apply_lobby_auth_peppers_secret
apply_lobby_game_service_secret
apply_lobby_openai_secret
apply_lobby_oauth_secret

pin_deployment_image lobby-backend backend "$BACKEND_IMAGE"
pin_deployment_image lobby-frontend frontend "$FRONTEND_IMAGE"

echo "Waiting for app deployments..."
kubectl wait --for=condition=available --timeout=300s deployment/lobby-backend -n "$NAMESPACE"
kubectl wait --for=condition=available --timeout=300s deployment/lobby-frontend -n "$NAMESPACE"

kubectl get pods,svc,ingress -n "$NAMESPACE"
echo "TLS: kubectl get certificate -n $NAMESPACE"
kubectl get certificate -n "$NAMESPACE" 2>/dev/null || true
echo "Done. Public URL: $LOBBY_PUBLIC_URL"
echo "DNS: point joinquest.cc (A or proxied CNAME) at the ingress ADDRESS below."
echo "Frontend API URL: $(kubectl get configmap lobby-frontend-config -n "$NAMESPACE" -o jsonpath='{.data.REACT_APP_API_BASE_URL}')"
