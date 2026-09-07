package database

import (
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
