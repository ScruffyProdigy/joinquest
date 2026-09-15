package store

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
)

// A hold is stamped by the database and was aged by this process. Those are two
// clocks on two machines, and whatever they disagreed by was added to every hold
// window: a database running ahead made chairs expire late, and one running ahead
// by more than the window meant they never expired at all -- a chair held for
// good, which wedges the mode queue.
//
// A frozen NOW() is how the two clocks are pulled apart without two machines.
// Inside one transaction the database's clock stands still while this process's
// keeps moving, so a window aged by the wrong clock runs out here and one aged by
// the database's does not.
//
// This is the half TestHoldExpiryIsMeasuredOnTheClockThatStampedIt cannot cover, and
// the reason both exist. That one stamps a window a full window ago and asks whether
// it has expired, which the wrong clock only gets wrong while the database is running
// ahead -- real time passing between the stamp and the question covers for it
// otherwise. CI's database is a service container on the runner, so its clocks never
// disagree and that test passes on the broken code there. This one needs no drift.
func TestHoldAgeIsMeasuredByTheDatabaseClockNotThisProcess(t *testing.T) {
	st := openTestStore(t)
	cleaner := st.NewTestCleaner(t)
	ctx := context.Background()
	resetDemoQueue(t, st, ctx)

	away := newPresenceUser(t, st, cleaner, ctx)
	joinAndPlace(t, st, ctx, away)

	var matchID uuid.UUID
	if err := st.db.QueryRowContext(ctx, `
		SELECT id FROM forming_matches WHERE mode_queue_id = $1 AND status = $2
	`, DemoDefaultQueueID, FormingMatchStatusFilling).Scan(&matchID); err != nil {
		t.Fatalf("find the filling match: %v — no match to hold, so this test proves nothing", err)
	}

	const window = 100 * time.Millisecond

	tx, err := st.db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	defer tx.Rollback()

	// Start the window inside the transaction, so it is stamped with the NOW() that
	// is frozen for the rest of it and its database-side age stays exactly zero.
	if err := clearHoldTx(ctx, tx, matchID); err != nil {
		t.Fatalf("clear hold: %v", err)
	}
	expired, err := advanceHoldTx(ctx, tx, matchID, away, window)
	if err != nil {
		t.Fatalf("start hold: %v", err)
	}
	if expired {
		t.Fatal("a hold expired the instant it started")
	}

	// Only this process's clock moves past the window.
	time.Sleep(4 * window)

	expired, err = advanceHoldTx(ctx, tx, matchID, away, window)
	if err != nil {
		t.Fatalf("advance hold: %v", err)
	}
	if expired {
		t.Fatal("the window ran out on this process's clock while the database's had not moved; " +
			"a database whose clock differs from the app's would hold chairs past their window, or forever")
	}
}
