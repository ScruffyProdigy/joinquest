#!/bin/bash

# PlayHub Backend Test Script
# This script runs backend tests with detailed output

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

echo "🔧 Running PlayHub Backend Tests..."
echo ""

# Check if we're in the right directory
if [ ! -d "backend" ]; then
    print_error "Please run this script from the PlayHub project root directory"
    exit 1
fi

ROOT="$(cd "$(dirname "$0")/.." && pwd)"

# Start this working copy's PostgreSQL for integration tests. A fresh run id per
# invocation gives the run its own database, so a second agent running the suite
# at the same time cannot truncate our fixtures mid-test (JQ-128).
if command -v docker &> /dev/null && [ -z "${DATABASE_URL:-}" ]; then
    # shellcheck source=lib/db-runtime.sh
    source "$ROOT/scripts/lib/db-runtime.sh"
    export LOBBY_TEST_RUN_ID="${LOBBY_TEST_RUN_ID:-$(lobby_new_run_id)}"
    trap '"$ROOT/scripts/db.sh" test-drop >/dev/null 2>&1 || true' EXIT

    print_info "Starting PostgreSQL for tests..."
    "$ROOT/scripts/db.sh" up
    "$ROOT/scripts/db.sh" test-migrate
    export DATABASE_URL="$("$ROOT/scripts/db.sh" test-url)"
    print_info "Integration tests use DATABASE_URL=$DATABASE_URL"
elif [ -n "${DATABASE_URL:-}" ]; then
    print_info "Using DATABASE_URL from the environment: $DATABASE_URL"
else
    print_warning "Docker not found. Migration integration tests will be skipped without DATABASE_URL."
fi

cd backend

# Check if Go is available
if ! command -v go &> /dev/null; then
    print_error "Go not found. Please install Go 1.25+ from https://golang.org/dl/"
    exit 1
fi

# Check if dependencies are installed
if [ ! -f "go.sum" ]; then
    print_warning "Dependencies not found. Installing..."
    go mod download
fi

# Run different types of tests
print_info "Running unit tests..."
if go test -p 1 -v ./...; then
    print_status "Unit tests passed"
else
    print_error "Unit tests failed"
    exit 1
fi

echo ""

print_info "Running drift detection tests..."
if go test -v -run=TestGqlgenDrift ./graph; then
    print_status "Drift detection tests passed"
else
    print_error "Drift detection tests failed"
    exit 1
fi

echo ""

print_info "Running benchmarks..."
if go test -bench=. -benchmem ./graph; then
    print_status "Benchmarks completed"
else
    print_warning "Some benchmarks failed (this is often normal)"
fi

echo ""

print_info "Running with coverage..."
if go test -p 1 -cover ./...; then
    print_status "Coverage analysis completed"
else
    print_error "Coverage analysis failed"
    exit 1
fi

echo ""
print_status "All backend tests completed successfully! 🎉"
