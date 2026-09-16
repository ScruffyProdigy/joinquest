package store

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
)

// An empty forming table goes stale on a created_at the database stamped, so the
// age has to be taken on the database's clock. Taking it on this process's clock
// subtracts two clocks that drift apart on a container, and every discard window
// is then wrong by the drift.
//
// A frozen NOW() pulls the two apart without needing two machines: inside one
// transaction the database's clock stands still while this process's keeps moving.
// So a window measured on the wrong clock runs out here and one measured on the
// database's does not, whatever the machine's ambient drift happens to be.
func TestEmptyTableStalenessIsMeasuredByTheDatabaseClockNotThisProcess(t *testing.T) {
	st := openTestStore(t)
	cleaner := st.NewTestCleaner(t)
	ctx := context.Background()

	host, err := st.CreateUser(ctx, CreateUserParams{Email: "discard-clock-" + uuid.NewString() + "@example.com"})
	if err != nil {
		t.Fatalf("create host: %v", err)
	}
	cleaner.TrackUser(host.ID)

	game, mode, _ := setupWordHuntMode(t, st, cleaner)

	room, err := st.CreateRoom(ctx, host.ID)
	if err != nil {
		t.Fatalf("create room: %v", err)
	}
	table, err := st.CreateTable(ctx, room.ID, game.ID, mode.ID, host.ID)
	if err != nil {
		t.Fatalf("create table: %v", err)
	}

	// How far short of stale the table is left, and how far past that this
	// process's clock is then carried. The difference is the ambient drift the
	// test can absorb before it can no longer tell the two clocks apart.
	const shortOfStale = 500 * time.Millisecond
	const advance = 1500 * time.Millisecond

	tx, err := st.db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	defer tx.Rollback()

	// Age the table to just short of stale against the database's own clock, which
	// this transaction has now frozen: its database-side age stays exactly
	// staleEmptyTableAge - shortOfStale however long the rest of the test takes.
	if err := tx.QueryRowContext(ctx, `
		UPDATE room_tables SET created_at = NOW() - ($2 * INTERVAL '1 microsecond')
		WHERE id = $1
		RETURNING created_at
	`, table.ID, (staleEmptyTableAge - shortOfStale).Microseconds()).Scan(&table.CreatedAt); err != nil {
		t.Fatalf("age the table to just short of stale: %v", err)
	}

	// Only this process's clock moves on past the threshold.
	time.Sleep(advance)

	if time.Since(table.CreatedAt) < staleEmptyTableAge {
		t.Skipf("the database clock is over %s ahead of this one, which is further apart "+
			"than this test can pull them; it can prove nothing here", advance-shortOfStale)
	}

	discardable, err := tableCanDiscard(ctx, tx, table, 0, host.ID, nil)
	if err != nil {
		t.Fatalf("table can discard: %v", err)
	}
	if discardable {
		t.Fatalf("a table the database still reads as %s short of stale was called stale, because "+
			"its age was taken on this process's clock; against a database whose clock differs, the "+
			"%s window is wrong by the difference", shortOfStale, staleEmptyTableAge)
	}
}
