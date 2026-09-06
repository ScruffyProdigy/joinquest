package testdb

import "testing"

func TestDatabaseName(t *testing.T) {
	tests := []struct {
		raw  string
		want string
	}{
		{"postgres://app:app-pass@127.0.0.1:5432/playhub_test?sslmode=disable", "playhub_test"},
		{"postgres://app:app-pass@127.0.0.1:5432/playhub?sslmode=disable", "playhub"},
	}
	for _, tc := range tests {
		got, err := DatabaseName(tc.raw)
		if err != nil {
			t.Fatalf("%q: %v", tc.raw, err)
		}
		if got != tc.want {
			t.Fatalf("%q: got %q want %q", tc.raw, got, tc.want)
		}
	}
}

func TestIsTestDatabase(t *testing.T) {
	if !IsTestDatabase("postgres://app:app-pass@127.0.0.1:5432/playhub_test?sslmode=disable") {
		t.Fatal("expected playhub_test")
	}
	if IsTestDatabase("postgres://app:app-pass@127.0.0.1:5432/playhub?sslmode=disable") {
		t.Fatal("expected not playhub_test")
	}
}

func TestIsTestDatabaseAcceptsPerRunDatabases(t *testing.T) {
	// Concurrent runs get their own database (playhub_test_<run id>) so two
	// agents running the suite at once cannot truncate each other's rows.
	perRun := []string{
		"postgres://app:app-pass@127.0.0.1:54321/playhub_test_a1b2c3?sslmode=disable",
		"postgres://app:app-pass@127.0.0.1:5432/playhub_test_20260906t141248z_7f3a?sslmode=disable",
	}
	for _, raw := range perRun {
		if !IsTestDatabase(raw) {
			t.Fatalf("expected per-run test database to be accepted: %q", raw)
		}
	}
}

func TestIsTestDatabaseRejectsLookalikes(t *testing.T) {
	// The guard's whole job is keeping go test off the dev database, so the
	// prefix must not open the door to anything that merely starts like it.
	lookalikes := []string{
		"postgres://app:app-pass@127.0.0.1:5432/playhub?sslmode=disable",
		"postgres://app:app-pass@127.0.0.1:5432/playhub_testing?sslmode=disable",
		"postgres://app:app-pass@127.0.0.1:5432/playhub_prod?sslmode=disable",
	}
	for _, raw := range lookalikes {
		if IsTestDatabase(raw) {
			t.Fatalf("expected non-test database to be rejected: %q", raw)
		}
	}
}
