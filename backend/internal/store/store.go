package store

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/scruffyprodigy/joinquest/internal/activity"
)

// Store provides typed access to PostgreSQL persistence.
type Store struct {
	db *sql.DB

	// Where player activity events go (JQ-143). Never nil: New installs a
	// discarding recorder so that every tool and test which builds a Store without
	// wiring instrumentation keeps working, and so that adding an emit call to a
	// code path can never be what breaks one.
	activity activity.Recorder
}

// New creates a Store backed by the given database connection.
func New(db *sql.DB) *Store {
	return &Store{db: db, activity: activity.Nop{}}
}

// WithActivity directs player activity events to r, and returns the Store for
// chaining at the entrypoint.
//
// Opt-in rather than a second argument to New: recording is instrumentation, and a
// sweep or a one-off command has no business emitting queue-join events just
// because it happens to construct a Store.
func (s *Store) WithActivity(r activity.Recorder) *Store {
	if s == nil {
		return nil
	}
	if r == nil {
		r = activity.Nop{}
	}
	s.activity = r
	return s
}

// Ping verifies the database connection is alive.
func (s *Store) Ping(ctx context.Context) error {
	if s == nil || s.db == nil {
		return fmt.Errorf("store: database not configured")
	}
	return s.db.PingContext(ctx)
}

// DBStats returns connection pool statistics.
func (s *Store) DBStats() sql.DBStats {
	if s == nil || s.db == nil {
		return sql.DBStats{}
	}
	return s.db.Stats()
}
