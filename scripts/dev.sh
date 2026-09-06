#!/bin/bash

# JoinQuest Development Script
# This script starts both frontend and backend development servers

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Function to print colored output
print_status() {
    echo -e "${GREEN}✓${NC} $1"
}

print_info() {
    echo -e "${BLUE}ℹ${NC} $1"
}

print_warning() {
    echo -e "${YELLOW}⚠${NC} $1"
}

print_error() {
    echo -e "${RED}✗${NC} $1"
}

# Function to cleanup background processes
cleanup() {
    print_info "Shutting down development servers..."
    if [ ! -z "$BACKEND_PID" ]; then
        kill $BACKEND_PID 2>/dev/null || true
    fi
    if [ ! -z "$FRONTEND_PID" ]; then
        kill $FRONTEND_PID 2>/dev/null || true
    fi
    exit 0
}

# Set up signal handlers
trap cleanup SIGINT SIGTERM

echo "🚀 Starting JoinQuest development servers..."
echo ""

# Check if we're in the right directory
if [ ! -f "README.md" ] || [ ! -d "backend" ] || [ ! -d "frontend" ]; then
    print_error "Please run this script from the JoinQuest project root directory"
    exit 1
fi

ROOT="$(pwd)"
if [ -f "$ROOT/.env" ]; then
    set -a
    # shellcheck disable=SC1091
    . "$ROOT/.env"
    set +a
fi
# shellcheck source=lib/lobby-smtp.sh
. "$ROOT/scripts/lib/lobby-smtp.sh"
# shellcheck source=lib/lobby-auth-peppers.sh
. "$ROOT/scripts/lib/lobby-auth-peppers.sh"
# shellcheck source=lib/lobby-openai.sh
. "$ROOT/scripts/lib/lobby-openai.sh"
# shellcheck source=lib/lobby-oauth.sh
. "$ROOT/scripts/lib/lobby-oauth.sh"
load_lobby_smtp_env_from_file
load_lobby_auth_peppers_env_from_file
load_lobby_openai_env_from_file
load_lobby_oauth_env_from_file

# Start shared PostgreSQL for local development
if command -v docker &> /dev/null; then
    print_info "Starting PostgreSQL..."
    "$ROOT/scripts/db.sh" up
    "$ROOT/scripts/db.sh" migrate
    export DATABASE_URL="${DATABASE_URL:-$("$ROOT/scripts/db.sh" url)}"
    if [[ "$DATABASE_URL" == *"@localhost:"* ]]; then
        export DATABASE_URL="${DATABASE_URL/@localhost:/@127.0.0.1:}"
    fi
    export MAGIC_LINK_BASE_URL="${MAGIC_LINK_BASE_URL:-http://localhost:5173/auth/complete?token=}"
    export CORS_ALLOWED_ORIGINS="${CORS_ALLOWED_ORIGINS:-http://localhost:5173,http://127.0.0.1:5173}"
    # Ephemeral host port, so ask the running stack rather than assuming 6379.
    export REDIS_URL="${REDIS_URL:-$("$ROOT/scripts/db.sh" redis-url)}"
    export GAME_API_BASE_URL="${GAME_API_BASE_URL:-http://localhost:3001}"
    export LOBBY_STALE_MATCH_MINUTES="${LOBBY_STALE_MATCH_MINUTES:-5}"
    export LOBBY_ADMIN_EMAILS="${LOBBY_ADMIN_EMAILS:-ryan.c.kohler@gmail.com}"
    export LOBBY_ISSUER_URL="${LOBBY_ISSUER_URL:-http://localhost:8080}"
    export LOBBY_PUBLIC_URL="${LOBBY_PUBLIC_URL:-http://localhost:5173}"
    # Optional: k8s/secrets/lobby-auth-peppers.yaml or .env for MAGIC_LINK_PEPPER / LOBBY_GAME_TOKEN_PEPPER.
    # Legacy dev fallback: LOBBY_GAME_SERVICE_TOKEN in .env (global token; production uses per-game tokens).
else
    print_warning "Docker not found. Set DATABASE_URL manually or the backend will use mock data."
fi

# Start backend server
print_info "Starting backend server..."
cd backend

if lsof -ti :8080 >/dev/null 2>&1; then
    print_error "Port 8080 is already in use by another backend process."
    print_info "Stop it and retry, e.g.: kill \$(lsof -ti :8080)"
    exit 1
fi

# Check if backend dependencies are installed
if [ ! -d "vendor" ] && [ ! -f "go.sum" ]; then
    print_warning "Backend dependencies not found. Running setup..."
    go mod download
    go run github.com/99designs/gqlgen@v0.17.81 generate
fi

# Stable JWT signing key for game handoff (see backend/.dev-jwt-key.pem).
# Restart the game API after deleting that file so LOBBY_JWKS_URL cache refreshes.
print_info "JWT dev key: backend/.dev-jwt-key.pem (auto-created; gitignored)"

# Start backend in background
go run server.go &
BACKEND_PID=$!

# Wait a moment for backend to start
sleep 2

# Check if backend started successfully
if ! kill -0 $BACKEND_PID 2>/dev/null; then
    print_error "Failed to start backend server"
    exit 1
fi

print_status "Backend server started (PID: $BACKEND_PID) - http://localhost:8080"

# Start frontend server
print_info "Starting frontend server..."
cd ../frontend

# Check if frontend dependencies are installed
if [ ! -d "node_modules" ]; then
    print_warning "Frontend dependencies not found. Installing..."
    npm install
fi

# Start frontend in background
npm run dev &
FRONTEND_PID=$!

# Wait a moment for frontend to start
sleep 3

# Check if frontend started successfully
if ! kill -0 $FRONTEND_PID 2>/dev/null; then
    print_error "Failed to start frontend server"
    cleanup
    exit 1
fi

print_status "Frontend server started (PID: $FRONTEND_PID) - http://localhost:5173"

echo ""
echo "🎉 Development servers are running!"
echo ""
echo "📱 Frontend: http://localhost:5173"
echo "🔧 Backend:  http://localhost:8080"
echo "📊 GraphQL:  http://localhost:8080/graphql"
echo ""
echo "Press Ctrl+C to stop all servers"
echo ""

# Wait for user to stop servers
wait
