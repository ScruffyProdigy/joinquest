package store

import (
	"context"
	"testing"

	"github.com/google/uuid"
)

// A client that has never reported visibility must read as present, not away.
// This is the direction that matters: an older client, a JS error, or a browser
// that never fires visibilitychange must not cost the player their seat. Unknown
// means present, which is exactly today's behaviour.
func TestUserIsAwayFalseWhenNoDocumentEverReported(t *testing.T) {
	st := openTestStore(t)
	cleaner := st.NewTestCleaner(t)
	ctx := context.Background()
	userID := newPresenceUser(t, st, cleaner, ctx)

	if _, err := st.PresenceConnected(ctx, userID); err != nil {
		t.Fatalf("PresenceConnected: %v", err)
	}

	away, err := st.UserIsAway(ctx, userID)
	if err != nil {
		t.Fatalf("UserIsAway: %v", err)
	}
	if away {
		t.Fatal("connected user who never reported visibility read as away, want present")
	}
}

// One document, hidden, while the socket is still live: this is the case JQ-199
// exists for and the one connection_count alone cannot see.
func TestUserIsAwayWhenOnlyDocumentIsHidden(t *testing.T) {
	st := openTestStore(t)
	cleaner := st.NewTestCleaner(t)
	ctx := context.Background()
	userID := newPresenceUser(t, st, cleaner, ctx)

	if _, err := st.PresenceConnected(ctx, userID); err != nil {
		t.Fatalf("PresenceConnected: %v", err)
	}
	if err := st.SetDocumentVisibility(ctx, userID, uuid.New(), false); err != nil {
		t.Fatalf("SetDocumentVisibility: %v", err)
	}

	away, err := st.UserIsAway(ctx, userID)
	if err != nil {
		t.Fatalf("UserIsAway: %v", err)
	}
	if !away {
		t.Fatal("connected user whose only document is hidden read as present, want away")
	}
}

// Any visible document means present. Two tabs, one backgrounded, must not read
// as away just because one of them reported hidden.
func TestUserIsPresentWhenAnyDocumentVisible(t *testing.T) {
	st := openTestStore(t)
	cleaner := st.NewTestCleaner(t)
	ctx := context.Background()
	userID := newPresenceUser(t, st, cleaner, ctx)

	if _, err := st.PresenceConnected(ctx, userID); err != nil {
		t.Fatalf("PresenceConnected: %v", err)
	}
	if err := st.SetDocumentVisibility(ctx, userID, uuid.New(), false); err != nil {
		t.Fatalf("SetDocumentVisibility hidden: %v", err)
	}
	if err := st.SetDocumentVisibility(ctx, userID, uuid.New(), true); err != nil {
		t.Fatalf("SetDocumentVisibility visible: %v", err)
	}

	away, err := st.UserIsAway(ctx, userID)
	if err != nil {
		t.Fatalf("UserIsAway: %v", err)
	}
	if away {
		t.Fatal("user with one visible and one hidden document read as away, want present")
	}
}

// A document reporting again replaces its own previous answer rather than adding
// a second row, so a tab that flips hidden->visible->hidden cannot accumulate.
func TestSetDocumentVisibilityReplacesItsOwnReport(t *testing.T) {
	st := openTestStore(t)
	cleaner := st.NewTestCleaner(t)
	ctx := context.Background()
	userID := newPresenceUser(t, st, cleaner, ctx)
	documentID := uuid.New()

	if _, err := st.PresenceConnected(ctx, userID); err != nil {
		t.Fatalf("PresenceConnected: %v", err)
	}
	for _, visible := range []bool{false, true, false} {
		if err := st.SetDocumentVisibility(ctx, userID, documentID, visible); err != nil {
			t.Fatalf("SetDocumentVisibility(%t): %v", visible, err)
		}
	}

	away, err := st.UserIsAway(ctx, userID)
	if err != nil {
		t.Fatalf("UserIsAway: %v", err)
	}
	if !away {
		t.Fatal("document ending hidden read as present, want away")
	}
}

// A disconnected user is not "away" — that is JQ-216's signal, and the two must
// stay distinguishable. Away means socket alive, attention gone.
func TestUserIsAwayFalseWhenDisconnected(t *testing.T) {
	st := openTestStore(t)
	cleaner := st.NewTestCleaner(t)
	ctx := context.Background()
	userID := newPresenceUser(t, st, cleaner, ctx)

	if _, err := st.PresenceConnected(ctx, userID); err != nil {
		t.Fatalf("PresenceConnected: %v", err)
	}
	if err := st.SetDocumentVisibility(ctx, userID, uuid.New(), false); err != nil {
		t.Fatalf("SetDocumentVisibility: %v", err)
	}
	if _, err := st.PresenceDisconnected(ctx, userID); err != nil {
		t.Fatalf("PresenceDisconnected: %v", err)
	}

	away, err := st.UserIsAway(ctx, userID)
	if err != nil {
		t.Fatalf("UserIsAway: %v", err)
	}
	if away {
		t.Fatal("disconnected user read as away; away and disconnected must stay distinct")
	}
}

// Losing every socket clears the user's visibility reports, so a later session
// starts from "never reported" rather than inheriting a stale hidden row.
func TestDisconnectClearsVisibilityReports(t *testing.T) {
	st := openTestStore(t)
	cleaner := st.NewTestCleaner(t)
	ctx := context.Background()
	userID := newPresenceUser(t, st, cleaner, ctx)

	if _, err := st.PresenceConnected(ctx, userID); err != nil {
		t.Fatalf("PresenceConnected: %v", err)
	}
	if err := st.SetDocumentVisibility(ctx, userID, uuid.New(), false); err != nil {
		t.Fatalf("SetDocumentVisibility: %v", err)
	}
	if _, err := st.PresenceDisconnected(ctx, userID); err != nil {
		t.Fatalf("PresenceDisconnected: %v", err)
	}
	if _, err := st.PresenceConnected(ctx, userID); err != nil {
		t.Fatalf("PresenceConnected (reconnect): %v", err)
	}

	away, err := st.UserIsAway(ctx, userID)
	if err != nil {
		t.Fatalf("UserIsAway: %v", err)
	}
	if away {
		t.Fatal("reconnected user inherited a stale hidden report, want present")
	}
}
