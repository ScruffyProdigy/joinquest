package store

import (
	"context"
	"testing"

	"github.com/google/uuid"
)

// goAway puts the user in the state JQ-199 exists for: a live socket with no
// visible document behind it. Not a disconnect — their sockets stay up.
func goAway(t *testing.T, st *Store, ctx context.Context, userID uuid.UUID) {
	t.Helper()
	if _, err := st.PresenceConnected(ctx, userID); err != nil {
		t.Fatalf("PresenceConnected: %v", err)
	}
	if err := st.SetDocumentVisibility(ctx, userID, uuid.New(), false); err != nil {
		t.Fatalf("SetDocumentVisibility(false): %v", err)
	}
	away, err := st.UserIsAway(ctx, userID)
	if err != nil {
		t.Fatalf("UserIsAway: %v", err)
	}
	if !away {
		t.Fatal("fixture failed to make the player away, so this test proves nothing")
	}
}

func comeBack(t *testing.T, st *Store, ctx context.Context, userID uuid.UUID) {
	t.Helper()
	if err := st.SetDocumentVisibility(ctx, userID, uuid.New(), true); err != nil {
		t.Fatalf("SetDocumentVisibility(true): %v", err)
	}
}

// The whole point of the ticket: a table that is otherwise ready must not fire
// into a game one of its players is not watching. Before this, ReadyToFire was
// the only question asked and the match went ahead regardless.
func TestFormingHoldsRatherThanFiringWithAnAwayPlayer(t *testing.T) {
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

	rec := mustReconcileForming(t, st, ctx, DemoDefaultQueueID)
	if rec.Fired {
		t.Fatal("match fired with an away player in a seat; the hold did nothing")
	}
	if n := assignedSeatCount(t, st, ctx, away); n != 1 {
		t.Fatalf("held player lost their seat immediately (%d seats), want it kept for the window", n)
	}
}

// Returning inside the window is the good ending: the player never learns
// anything happened, and the match fires as though they had been there all along.
func TestFormingFiresOnceTheAwayPlayerComesBack(t *testing.T) {
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
		t.Fatal("fixture fired before the player returned, so the rest proves nothing")
	}

	comeBack(t, st, ctx, away)

	rec := mustReconcileForming(t, st, ctx, DemoDefaultQueueID)
	if !rec.Fired {
		t.Fatal("match did not fire after the away player returned")
	}
}

// A player who is merely present must not be held for. Guards against the away
// check reading connection state and holding every match that forms.
func TestFormingFiresNormallyWhenEveryoneIsPresent(t *testing.T) {
	st := openTestStore(t)
	cleaner := st.NewTestCleaner(t)
	ctx := context.Background()
	resetDemoQueue(t, st, ctx)

	first := newPresenceUser(t, st, cleaner, ctx)
	joinAndPlace(t, st, ctx, first)
	if _, err := st.PresenceConnected(ctx, first); err != nil {
		t.Fatalf("PresenceConnected: %v", err)
	}

	second := newPresenceUser(t, st, cleaner, ctx)
	if _, err := st.JoinModeQueue(ctx, DemoDefaultQueueID, second, "", nil); err != nil {
		t.Fatalf("join second player: %v", err)
	}

	rec := mustReconcileForming(t, st, ctx, DemoDefaultQueueID)
	if !rec.Fired {
		t.Fatal("match with two present players did not fire")
	}
}
