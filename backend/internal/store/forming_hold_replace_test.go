package store

import (
	"context"
	"testing"
	"time"
)

// After an away player's chair is vacated, somebody else from the queue should end
// up in it. The away player is still queued, so the question is whether the refill
// picks them straight back up.
func TestVacatedChairIsRefilledBySomebodyElse(t *testing.T) {
	st := openTestStore(t)
	cleaner := st.NewTestCleaner(t)
	ctx := context.Background()
	resetDemoQueue(t, st, ctx)

	away := newPresenceUser(t, st, cleaner, ctx)
	joinAndPlace(t, st, ctx, away)
	goAway(t, st, ctx, away)

	present := newPresenceUser(t, st, cleaner, ctx)
	if _, err := st.JoinModeQueue(ctx, DemoDefaultQueueID, present, "", nil); err != nil {
		t.Fatalf("join present: %v", err)
	}
	mustReconcileForming(t, st, ctx, DemoDefaultQueueID)

	// A third player is waiting to take the chair when it frees.
	replacement := newPresenceUser(t, st, cleaner, ctx)
	if _, err := st.JoinModeQueue(ctx, DemoDefaultQueueID, replacement, "", nil); err != nil {
		t.Fatalf("join replacement: %v", err)
	}
	if _, err := st.PresenceConnected(ctx, replacement); err != nil {
		t.Fatalf("PresenceConnected: %v", err)
	}

	backdateHold(t, st, ctx, DemoDefaultQueueID, HoldUnreachableFloor+time.Second)
	mustReconcileForming(t, st, ctx, DemoDefaultQueueID)
	t.Logf("after expiry: away=%d present=%d replacement=%d",
		assignedSeatCount(t, st, ctx, away),
		assignedSeatCount(t, st, ctx, present),
		assignedSeatCount(t, st, ctx, replacement))

	rec := mustReconcileForming(t, st, ctx, DemoDefaultQueueID)
	t.Logf("next reconcile fired=%t: away=%d present=%d replacement=%d",
		rec.Fired,
		assignedSeatCount(t, st, ctx, away),
		assignedSeatCount(t, st, ctx, present),
		assignedSeatCount(t, st, ctx, replacement))

	if !rec.Fired {
		t.Fatal("match never fired even with a present replacement waiting")
	}
	if n := assignedSeatCount(t, st, ctx, away); n != 0 {
		t.Fatalf("away player was re-seated into the chair they just lost (%d seats)", n)
	}
}
