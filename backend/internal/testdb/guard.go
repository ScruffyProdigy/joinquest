package testdb

import (
	"errors"
	"net/url"
	"os"
	"strings"
	"testing"
)

const testDBName = "joinquest_test"

// testDBPrefix matches the per-run databases the harness creates so concurrent
// runs cannot share fixtures: joinquest_test_<run id>. See scripts/lib/db-runtime.sh.
const testDBPrefix = testDBName + "_"

// RequireURL returns DATABASE_URL for integration tests. It skips unless the database
// is joinquest_test or a per-run joinquest_test_<run id>, so go test does not mutate the
// dev joinquest database by accident.
// Override with ALLOW_TESTS_ON_DEV_DB=1 when intentionally testing against joinquest.
func RequireURL(t *testing.T) string {
	t.Helper()

	raw := strings.TrimSpace(os.Getenv("DATABASE_URL"))
	if raw == "" {
		t.Skip("DATABASE_URL not set, skipping integration test")
	}
	if os.Getenv("ALLOW_TESTS_ON_DEV_DB") == "1" {
		return raw
	}
	dbName, err := DatabaseName(raw)
	if err != nil {
		t.Fatalf("parse DATABASE_URL: %v", err)
	}
	if !isTestDatabaseName(dbName) {
		t.Skip("integration tests require database " + testDBName + " or " + testDBPrefix + "<run id>" +
			" (run ./scripts/test-backend.sh or: export DATABASE_URL=$(./scripts/db.sh test-url))")
	}
	return raw
}

// DatabaseName returns the database segment from a postgres connection URL.
func DatabaseName(raw string) (string, error) {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return "", err
	}
	name := strings.TrimPrefix(u.Path, "/")
	if idx := strings.Index(name, "?"); idx >= 0 {
		name = name[:idx]
	}
	if name == "" {
		return "", errors.New("database name is empty")
	}
	return name, nil
}

// IsTestDatabase reports whether raw points at an integration-test database.
func IsTestDatabase(raw string) bool {
	name, err := DatabaseName(raw)
	return err == nil && isTestDatabaseName(name)
}

func isTestDatabaseName(name string) bool {
	return name == testDBName || strings.HasPrefix(name, testDBPrefix)
}
