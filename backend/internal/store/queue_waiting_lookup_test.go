package store

import (
	"context"
	"testing"
)

// The seat-hold reconciles on a returning player, and it needs to know which queue
// to reconcile. Without this the hold only clears on the 30s sweep, so a player who
// came straight back would sit and wait for a tick.
func TestWaitingModeQueueIDForUserFindsTheQueue(t *testing.T) {
	st := openTestStore(t)
	cleaner := st.NewTestCleaner(t)
	ctx := context.Background()
	resetDemoQueue(t, st, ctx)

	user := newPresenceUser(t, st, cleaner, ctx)
	if _, err := st.JoinModeQueue(ctx, DemoDefaultQueueID, user, "", nil); err != nil {
		t.Fatalf("join: %v", err)
	}

	queueID, ok, err := st.WaitingModeQueueIDForUser(ctx, user)
	if err != nil {
		t.Fatalf("WaitingModeQueueIDForUser: %v", err)
	}
	if !ok {
		t.Fatal("queued player reported as not waiting anywhere")
	}
	if queueID != DemoDefaultQueueID {
		t.Fatalf("got queue %s, want %s", queueID, DemoDefaultQueueID)
	}
}

// A player who is not queued has nothing to reconcile, and the caller must be able
// to tell that apart from an error.
func TestWaitingModeQueueIDForUserReportsNotWaiting(t *testing.T) {
	st := openTestStore(t)
	cleaner := st.NewTestCleaner(t)
	ctx := context.Background()

	user := newPresenceUser(t, st, cleaner, ctx)

	if _, ok, err := st.WaitingModeQueueIDForUser(ctx, user); err != nil {
		t.Fatalf("WaitingModeQueueIDForUser: %v", err)
	} else if ok {
		t.Fatal("unqueued player reported as waiting")
	}
}
