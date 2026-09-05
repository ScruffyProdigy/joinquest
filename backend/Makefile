# GraphQL Code Generation, Testing, and Database Migrations

.PHONY: generate test test-drift test-all migrate-up migrate-down migrate-version migrate-force

# Generate GraphQL code
generate:
	@echo "Generating GraphQL code..."
	go run github.com/99designs/gqlgen@v0.17.81 generate

# Run all tests
# -p 1: the graph and store packages share one Postgres and drive the same
# seeded demo queue, so running them at the same time makes them fight.
test:
	@echo "Running all tests..."
	go test -p 1 ./... -v

# Run only drift detection tests
test-drift:
	@echo "Checking for gqlgen drift..."
	go test ./graph -v -run="TestGqlgen"

# Run all tests including drift detection
test-all: test-drift test

# Check if generation is needed (useful for CI)
check-drift:
	@echo "Checking if gqlgen generate is needed..."
	@go test ./graph -run="TestGqlgenDrift" -v || (echo "❌ Drift detected! Run 'make generate' to fix." && exit 1)
	@echo "✅ No drift detected - generated code is up to date."

# Clean generated files (use with caution)
clean-generated:
	@echo "Cleaning generated files..."
	rm -f graph/generated/generated.go
	rm -f graph/generated.go
	rm -f graph/model/models_gen.go
	rm -f graph/core.resolvers.go

# Regenerate everything from scratch
regenerate: clean-generated generate
	@echo "✅ Regenerated all GraphQL code from scratch."

# Database Migration Commands
migrate-up:
	@echo "Running database migrations up..."
	@go run ./cmd/migrate -action=up

migrate-down:
	@echo "Running database migrations down..."
	@go run ./cmd/migrate -action=down

migrate-version:
	@echo "Checking migration version..."
	@go run ./cmd/migrate -action=version

migrate-force:
	@echo "Forcing migration version..."
	@go run ./cmd/migrate -action=force -version=$(VERSION)

# Help
help:
	@echo "Available targets:"
	@echo "  generate        - Generate GraphQL code using gqlgen"
	@echo "  test           - Run all tests"
	@echo "  test-drift     - Run only drift detection tests"
	@echo "  test-all       - Run drift detection and all tests"
	@echo "  check-drift    - Check if generation is needed (for CI)"
	@echo "  clean-generated - Remove all generated files"
	@echo "  regenerate     - Clean and regenerate all GraphQL code"
	@echo "  migrate-up     - Run all pending database migrations"
	@echo "  migrate-down   - Rollback the last database migration"
	@echo "  migrate-version - Show current migration version"
	@echo "  migrate-force  - Force migration version (use VERSION=n)"
	@echo "  help           - Show this help message"
