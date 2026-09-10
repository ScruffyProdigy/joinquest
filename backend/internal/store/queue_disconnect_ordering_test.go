package store

import (
	"context"
	"testing"

	"github.com/google/uuid"
)

// A disconnected player keeps their place in the queue but goes to the back of the
// pool: matching consumes it in order up to a fixed PlayersToStart, so sorting them
// last is exactly "matchable only if the pool is otherwise too thin to form".
//
// The early player joined FIRST, so a passing test cannot be explained by joined_at.
func TestWaitingEntriesSortDisconnectedPlayersLast(t *testing.T) {
	st := openTestStore(t)
	cleaner := st.NewTestCleaner(t)
	ctx := context.Background()

	early := newPresenceUser(t, st, cleaner, ctx)
	if _, err := st.JoinModeQueue(ctx, DemoDefaultQueueID, early, "", nil); err != nil {
		t.Fatalf("join early: %v", err)
	}
	if _, err := st.PresenceConnected(ctx, early); err != nil {
		t.Fatalf("connect early: %v", err)
	}
	if _, err := st.PresenceDisconnected(ctx, early); err != nil {
		t.Fatalf("disconnect early: %v", err)
	}

	late := newPresenceUser(t, st, cleaner, ctx)
	if _, err := st.JoinModeQueue(ctx, DemoDefaultQueueID, late, "", nil); err != nil {
		t.Fatalf("join late: %v", err)
	}
	if _, err := st.PresenceConnected(ctx, late); err != nil {
		t.Fatalf("connect late: %v", err)
	}

	tx, err := st.db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	defer tx.Rollback()

	entries, err := listWaitingModeQueueEntriesTx(ctx, tx, DemoDefaultQueueID)
	if err != nil {
		t.Fatalf("listWaitingModeQueueEntriesTx: %v", err)
	}

	var order []uuid.UUID
	for _, e := range entries {
		if e.UserID == early || e.UserID == late {
			order = append(order, e.UserID)
		}
	}
	if len(order) != 2 {
		t.Fatalf("expected both fixtures in the queue, got %d", len(order))
	}
	if order[0] != late {
		t.Fatal("the connected player should come first even though they joined later")
	}
	if order[1] != early {
		t.Fatal("the disconnected player should sort last")
	}
}

// A player with no presence row at all — never held a socket this deployment, or
// their row was cleaned up — must not be treated as disconnected. The LEFT JOIN makes
// their disconnected_at NULL, which has to read as "present", or every player without
// a presence row would silently sort last.
func TestWaitingEntriesTreatAMissingPresenceRowAsPresent(t *testing.T) {
	st := openTestStore(t)
	cleaner := st.NewTestCleaner(t)
	ctx := context.Background()

	noPresence := newPresenceUser(t, st, cleaner, ctx)
	if _, err := st.JoinModeQueue(ctx, DemoDefaultQueueID, noPresence, "", nil); err != nil {
		t.Fatalf("join no-presence: %v", err)
	}

	disconnected := newPresenceUser(t, st, cleaner, ctx)
	if _, err := st.JoinModeQueue(ctx, DemoDefaultQueueID, disconnected, "", nil); err != nil {
		t.Fatalf("join disconnected: %v", err)
	}
	if _, err := st.PresenceConnected(ctx, disconnected); err != nil {
		t.Fatalf("connect: %v", err)
	}
	if _, err := st.PresenceDisconnected(ctx, disconnected); err != nil {
		t.Fatalf("disconnect: %v", err)
	}

	tx, err := st.db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	defer tx.Rollback()

	entries, err := listWaitingModeQueueEntriesTx(ctx, tx, DemoDefaultQueueID)
	if err != nil {
		t.Fatalf("listWaitingModeQueueEntriesTx: %v", err)
	}

	var order []uuid.UUID
	for _, e := range entries {
		if e.UserID == noPresence || e.UserID == disconnected {
			order = append(order, e.UserID)
		}
	}
	if len(order) != 2 {
		t.Fatalf("expected both fixtures in the queue, got %d", len(order))
	}
	if order[0] != noPresence {
		t.Fatal("a player with no presence row was sorted behind a disconnected one")
	}
}

// The join must not drop or duplicate rows — it exists only to order them.
func TestWaitingEntriesReturnEveryWaitingRowExactlyOnce(t *testing.T) {
	st := openTestStore(t)
	cleaner := st.NewTestCleaner(t)
	ctx := context.Background()

	userID := newPresenceUser(t, st, cleaner, ctx)
	if _, err := st.JoinModeQueue(ctx, DemoDefaultQueueID, userID, "", nil); err != nil {
		t.Fatalf("join: %v", err)
	}
	if _, err := st.PresenceConnected(ctx, userID); err != nil {
		t.Fatalf("connect: %v", err)
	}

	tx, err := st.db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	defer tx.Rollback()

	entries, err := listWaitingModeQueueEntriesTx(ctx, tx, DemoDefaultQueueID)
	if err != nil {
		t.Fatalf("listWaitingModeQueueEntriesTx: %v", err)
	}

	seen := 0
	for _, e := range entries {
		if e.UserID == userID {
			seen++
		}
		// Every scanned row must be populated: a mis-ordered qualified column list
		// would still scan, just into the wrong fields.
		if e.ID == uuid.Nil || e.GameID == uuid.Nil || e.Status != "waiting" {
			t.Fatalf("row scanned into the wrong fields: %+v", e)
		}
	}
	if seen != 1 {
		t.Fatalf("expected the waiting row exactly once, saw it %d times", seen)
	}
}
