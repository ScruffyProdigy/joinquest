#!/bin/bash

# JoinQuest Frontend Test Script
# This script runs frontend tests with detailed output

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

echo "🎨 Running JoinQuest Frontend Tests..."
echo ""

# Check if we're in the right directory
if [ ! -d "frontend" ]; then
    print_error "Please run this script from the JoinQuest project root directory"
    exit 1
fi

cd frontend

# Check if Node.js is available
if ! command -v node &> /dev/null; then
    print_error "Node.js not found. Please install Node.js 20+ from https://nodejs.org/"
    exit 1
fi

# Check if npm is available
if ! command -v npm &> /dev/null; then
    print_error "npm not found. Please install npm"
    exit 1
fi

# Check if dependencies are installed
if [ ! -d "node_modules" ]; then
    print_warning "Dependencies not found. Installing..."
    npm install
fi

# Run different types of tests
print_info "Running unit and integration tests..."
if npm run test:run; then
    print_status "Unit and integration tests passed"
else
    print_error "Unit and integration tests failed"
    exit 1
fi

echo ""

# Run E2E tests if requested
if [ "$1" = "--e2e" ] || [ "$1" = "-e" ]; then
    print_info "Running E2E tests..."
    if npm run test:e2e; then
        print_status "E2E tests passed"
    else
        print_error "E2E tests failed"
        exit 1
    fi
    echo ""
fi

# Run linting
print_info "Running linting..."
if npm run lint; then
    print_status "Linting passed"
else
    print_warning "Linting issues found (check output above)"
fi

echo ""

# Run coverage if available
if npm run test:coverage 2>/dev/null; then
    print_status "Coverage analysis completed"
else
    print_info "Coverage analysis not available (this is normal)"
fi

echo ""
print_status "All frontend tests completed successfully! 🎉"
