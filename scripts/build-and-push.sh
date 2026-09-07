#!/bin/bash

# Build (and optionally push) Lobby Docker images.
# Push is opt-in so local builds do not require docker login or overwrite :latest on Hub.
#
#   ./scripts/build-and-push.sh           # build only
#   ./scripts/build-and-push.sh --push    # build + push to Docker Hub

set -e

PUSH=false
for arg in "$@"; do
  case "$arg" in
    --push) PUSH=true ;;
    -h|--help)
      echo "Usage: $0 [--push]"
      echo "  --push  Push images to Docker Hub after building (requires docker login)"
      exit 0
      ;;
    *)
      echo "Unknown option: $arg" >&2
      exit 1
      ;;
  esac
done

echo "🐳 Building Lobby Docker images..."

DOCKER_REGISTRY="docker.io"
DOCKER_USERNAME="scruffyprodigy"
BACKEND_IMAGE="joinquest-backend"
FRONTEND_IMAGE="joinquest-frontend"
TAG="${TAG:-latest}"

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

print_status() { echo -e "${GREEN}[INFO]${NC} $1"; }
print_warning() { echo -e "${YELLOW}[WARN]${NC} $1"; }
print_error() { echo -e "${RED}[ERROR]${NC} $1"; }

docker_config_json() {
  echo "${DOCKER_CONFIG:-$HOME/.docker}/config.json"
}

# Hub creds live in ~/.docker/config.json (index.docker.io/v1). Modern Docker/Colima
# often omit Username from `docker info`, so do not use that for login detection.
docker_hub_auth_configured() {
  local config
  config="$(docker_config_json)"
  [ -f "$config" ] || return 1
  grep -q 'index.docker.io' "$config" 2>/dev/null
}

if ! docker info > /dev/null 2>&1; then
  print_error "Docker is not running. Please start Docker and try again."
  exit 1
fi

if [ "$PUSH" = true ] && ! docker_hub_auth_configured; then
  print_error "Docker Hub login not found in $(docker_config_json). Run: docker login"
  exit 1
fi

ROOT="$(cd "$(dirname "$0")/.." && pwd)"

print_status "Building backend image..."
docker build -t "${DOCKER_REGISTRY}/${DOCKER_USERNAME}/${BACKEND_IMAGE}:${TAG}" "$ROOT/backend"

print_status "Building frontend image..."
docker build -t "${DOCKER_REGISTRY}/${DOCKER_USERNAME}/${FRONTEND_IMAGE}:${TAG}" "$ROOT/frontend"

if [ "$PUSH" = true ]; then
  print_status "Pushing backend image..."
  docker push "${DOCKER_REGISTRY}/${DOCKER_USERNAME}/${BACKEND_IMAGE}:${TAG}"
  print_status "Pushing frontend image..."
  docker push "${DOCKER_REGISTRY}/${DOCKER_USERNAME}/${FRONTEND_IMAGE}:${TAG}"
else
  print_warning "Images built locally only. To push: $0 --push"
fi

print_status "Backend:  ${DOCKER_REGISTRY}/${DOCKER_USERNAME}/${BACKEND_IMAGE}:${TAG}"
print_status "Frontend: ${DOCKER_REGISTRY}/${DOCKER_USERNAME}/${FRONTEND_IMAGE}:${TAG}"
