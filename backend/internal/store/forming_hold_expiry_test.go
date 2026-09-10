package store

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
)

// backdateHold ages the running hold so expiry can be tested without sleeping.
func backdateHold(t *testing.T, st *Store, ctx context.Context, queueID uuid.UUID, by time.Duration) {
	t.Helper()
	res, err := st.db.ExecContext(ctx, `
		UPDATE forming_matches
		SET hold_started_at = hold_started_at - $2::interval
		WHERE mode_queue_id = $1 AND status = $3 AND hold_started_at IS NOT NULL
	`, queueID, pgInterval(by), FormingMatchStatusFilling)
	if err != nil {
		t.Fatalf("backdate hold: %v", err)
	}
	if n, _ := res.RowsAffected(); n != 1 {
		t.Fatalf("backdate hold touched %d rows, want 1 — no hold was running, so this test proves nothing", n)
	}
}

// A hold that never ends is a wedged queue: filling matches do not expire, so a
// chair held for someone who never comes back would block that mode queue for
// good.
func TestHeldChairIsVacatedOnceTheWindowExpires(t *testing.T) {
	st := openTestStore(t)
	cleaner := st.NewTestCleaner(t)
	ctx := context.Background()
	resetDemoQueue(t, st, ctx)

	away := newPresenceUser(t, st, cleaner, ctx)
	joinAndPlace(t, st, ctx, away)
	goAway(t, st, ctx, away)

	present := newPresenceUser(t, st, cleaner, ctx)
	if _, err := st.JoinModeQueue(ctx, DemoDefaultQueueID, present, "", nil); err != nil {
		t.Fatalf("join second player: %v", err)
	}
	if rec := mustReconcileForming(t, st, ctx, DemoDefaultQueueID); rec.Fired {
		t.Fatal("fixture fired instead of holding")
	}

	backdateHold(t, st, ctx, DemoDefaultQueueID, HoldUnreachableFloor+time.Second)
	mustReconcileForming(t, st, ctx, DemoDefaultQueueID)

	if n := assignedSeatCount(t, st, ctx, away); n != 0 {
		t.Fatalf("expired hold left the chair assigned (%d seats), want it vacated for the waiting pool", n)
	}
	// Vacating the chair is not ejecting the player. They were never told a match
	// formed, so there is nothing to explain and nothing to requeue for — they are
	// simply still in line.
	if n := waitingRowCount(t, st, ctx, away); n != 1 {
		t.Fatalf("expired hold removed the player from the queue (%d waiting rows), want them still queued", n)
	}
}

// Holding two chairs at once is what makes a pre-announce notification
// falsifiable: push both, one returns, the other does not, and the returner
// arrives to find no match. One held chair cannot fail that way, because the
// returning player's arrival is itself the fire condition.
func TestTwoAwayPlayersVacateBothRatherThanHolding(t *testing.T) {
	st := openTestStore(t)
	cleaner := st.NewTestCleaner(t)
	ctx := context.Background()
	resetDemoQueue(t, st, ctx)

	first := newPresenceUser(t, st, cleaner, ctx)
	joinAndPlace(t, st, ctx, first)
	goAway(t, st, ctx, first)

	second := newPresenceUser(t, st, cleaner, ctx)
	if _, err := st.JoinModeQueue(ctx, DemoDefaultQueueID, second, "", nil); err != nil {
		t.Fatalf("join second player: %v", err)
	}
	goAway(t, st, ctx, second)

	if rec := mustReconcileForming(t, st, ctx, DemoDefaultQueueID); rec.Fired {
		t.Fatal("match fired with two away players")
	}
	for name, userID := range map[string]uuid.UUID{"first": first, "second": second} {
		if n := assignedSeatCount(t, st, ctx, userID); n != 0 {
			t.Fatalf("%s away player kept their chair (%d seats); two holds should vacate both", name, n)
		}
		if n := waitingRowCount(t, st, ctx, userID); n != 1 {
			t.Fatalf("%s away player was dropped from the queue (%d waiting rows)", name, n)
		}
	}
}

// The window belongs to the absence being waited out, not to the match. If one
// player returns and a different one wanders off, the second player gets their
// own full window rather than inheriting however much of the first was left.
func TestHoldWindowRestartsWhenADifferentPlayerGoesAway(t *testing.T) {
	st := openTestStore(t)
	cleaner := st.NewTestCleaner(t)
	ctx := context.Background()
	resetDemoQueue(t, st, ctx)

	first := newPresenceUser(t, st, cleaner, ctx)
	joinAndPlace(t, st, ctx, first)
	goAway(t, st, ctx, first)

	second := newPresenceUser(t, st, cleaner, ctx)
	if _, err := st.JoinModeQueue(ctx, DemoDefaultQueueID, second, "", nil); err != nil {
		t.Fatalf("join second player: %v", err)
	}
	mustReconcileForming(t, st, ctx, DemoDefaultQueueID)

	// Age the first player's hold to the brink, then swap who is absent.
	backdateHold(t, st, ctx, DemoDefaultQueueID, HoldUnreachableFloor-time.Second)
	comeBack(t, st, ctx, first)
	goAway(t, st, ctx, second)

	mustReconcileForming(t, st, ctx, DemoDefaultQueueID)
	if n := assignedSeatCount(t, st, ctx, second); n != 1 {
		t.Fatalf("second player's chair was vacated on the first player's stale window (%d seats)", n)
	}
}
