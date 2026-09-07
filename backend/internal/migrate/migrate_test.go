package migrate

import (
	"database/sql"
	"path/filepath"
	"strings"
	"testing"

	"github.com/scruffyprodigy/joinquest/internal/testdb"

	_ "github.com/lib/pq"
)

func TestFindMigrationsPathFromTestPackage(t *testing.T) {
	path := findMigrationsPath()
	if !hasMigrationFiles(path) {
		t.Fatalf("expected migrations directory with SQL files, got %q", path)
	}

	if !strings.Contains(path, filepath.Join("backend", "migrations")) {
		t.Fatalf("expected backend migrations path, got %q", path)
	}
}

func TestMigrator(t *testing.T) {
	// Skip if no database URL is provided
	databaseURL := testdb.RequireURL(t)

	// Connect to test database
	db, err := sql.Open("postgres", databaseURL)
	if err != nil {
		t.Fatalf("Failed to connect to database: %v", err)
	}
	defer db.Close()

	// Test connection
	if err := db.Ping(); err != nil {
		t.Fatalf("Failed to ping database: %v", err)
	}

	// Create migrator
	migrator, err := NewMigrator(db)
	if err != nil {
		t.Fatalf("Failed to create migrator: %v", err)
	}
	defer migrator.Close()

	// Test version check
	version, dirty, err := migrator.Version()
	if err != nil {
		t.Fatalf("Failed to get version: %v", err)
	}
	t.Logf("Current version: %d, dirty: %v", version, dirty)

	// Test running migrations up (should be idempotent)
	if err := migrator.Up(); err != nil {
		t.Fatalf("Failed to run migrations up: %v", err)
	}

	// Check version again
	version, dirty, err = migrator.Version()
	if err != nil {
		t.Fatalf("Failed to get version after up: %v", err)
	}
	t.Logf("Version after up: %d, dirty: %v", version, dirty)

	// Verify that the users table exists
	var exists bool
	err = db.QueryRow("SELECT EXISTS (SELECT FROM information_schema.tables WHERE table_name = 'users')").Scan(&exists)
	if err != nil {
		t.Fatalf("Failed to check if users table exists: %v", err)
	}
	if !exists {
		t.Error("Users table should exist after migration")
	}

	// Verify that the magic_links table exists
	err = db.QueryRow("SELECT EXISTS (SELECT FROM information_schema.tables WHERE table_name = 'magic_links')").Scan(&exists)
	if err != nil {
		t.Fatalf("Failed to check if magic_links table exists: %v", err)
	}
	if !exists {
		t.Error("Magic_links table should exist after migration")
	}
}
