package database

import (
	"os"
	"testing"

	"github.com/scruffyprodigy/joinquest/internal/testdb"
)

func TestInitWithMigrationsKeepsConnectionOpen(t *testing.T) {
	_ = testdb.RequireURL(t)

	if err := InitWithMigrations(); err != nil {
		t.Fatalf("InitWithMigrations() error: %v", err)
	}
	t.Cleanup(func() { _ = Close() })

	if err := GetDB().Ping(); err != nil {
		t.Fatalf("GetDB().Ping() after migrations: %v", err)
	}
}

// Production sets RUN_STARTUP_MIGRATIONS=false and lets the joinquest-db-migrate
// Job own migrations, so the server must still come up with a usable connection
// when it skips them.
func TestInitWithMigrationsSkippedStillConnects(t *testing.T) {
	_ = testdb.RequireURL(t)
	t.Setenv("RUN_STARTUP_MIGRATIONS", "false")

	if err := InitWithMigrations(); err != nil {
		t.Fatalf("InitWithMigrations() with migrations disabled: %v", err)
	}
	t.Cleanup(func() { _ = Close() })

	if err := GetDB().Ping(); err != nil {
		t.Fatalf("GetDB().Ping() with migrations disabled: %v", err)
	}
}

func TestStartupMigrationsEnabled(t *testing.T) {
	tests := []struct {
		name  string
		value string
		set   bool
		want  bool
	}{
		{name: "unset defaults to on", want: true},
		{name: "empty defaults to on", set: true, value: "", want: true},
		{name: "false disables", set: true, value: "false", want: false},
		{name: "mixed case false disables", set: true, value: "False", want: false},
		{name: "padded false disables", set: true, value: "  false  ", want: false},
		{name: "true enables", set: true, value: "true", want: true},
		// Only "false" turns migrations off. A typo must not silently leave a
		// deploy with no migrations at all.
		{name: "typo leaves migrations on", set: true, value: "no", want: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// t.Setenv first either way, so the original value is restored.
			t.Setenv("RUN_STARTUP_MIGRATIONS", tt.value)
			if !tt.set {
				os.Unsetenv("RUN_STARTUP_MIGRATIONS")
			}

			if got := startupMigrationsEnabled(); got != tt.want {
				t.Errorf("startupMigrationsEnabled() with %q = %v, want %v", tt.value, got, tt.want)
			}
		})
	}
}
