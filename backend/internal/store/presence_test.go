package store

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
)

func newPresenceUser(t *testing.T, st *Store, cleaner *TestCleaner, ctx context.Context) uuid.UUID {
	t.Helper()
	user, err := st.CreateUser(ctx, CreateUserParams{
		Email:       "presence-" + uuid.NewString() + "@example.com",
		DisplayName: "Presence Fixture",
	})
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}
	cleaner.TrackUser(user.ID)
	return user.ID
}

func TestPresenceFirstConnectIsAnEdge(t *testing.T) {
	st := openTestStore(t)
	cleaner := st.NewTestCleaner(t)
	ctx := context.Background()
	userID := newPresenceUser(t, st, cleaner, ctx)

	got, err := st.PresenceConnected(ctx, userID)
	if err != nil {
		t.Fatalf("PresenceConnected: %v", err)
	}
	if got.ConnectionCount != 1 || !got.Edge {
		t.Fatalf("first connect: got count=%d edge=%t, want 1/true", got.ConnectionCount, got.Edge)
	}
	if got.DisconnectedAt != nil {
		t.Fatalf("first connect left a disconnected_at: %v", got.DisconnectedAt)
	}
}

// One tab can hold three sockets, so a second connect must not be an edge and a
// single close must not report the player gone.
func TestPresenceSecondSocketIsNotAnEdgeAndClosingOneDoesNotStamp(t *testing.T) {
	st := openTestStore(t)
	cleaner := st.NewTestCleaner(t)
	ctx := context.Background()
	userID := newPresenceUser(t, st, cleaner, ctx)

	if _, err := st.PresenceConnected(ctx, userID); err != nil {
		t.Fatalf("connect 1: %v", err)
	}
	second, err := st.PresenceConnected(ctx, userID)
	if err != nil {
		t.Fatalf("connect 2: %v", err)
	}
	if second.ConnectionCount != 2 || second.Edge {
		t.Fatalf("second connect: got count=%d edge=%t, want 2/false", second.ConnectionCount, second.Edge)
	}

	dropped, err := st.PresenceDisconnected(ctx, userID)
	if err != nil {
		t.Fatalf("disconnect 1: %v", err)
	}
	if dropped.ConnectionCount != 1 || dropped.Edge {
		t.Fatalf("first close: got count=%d edge=%t, want 1/false", dropped.ConnectionCount, dropped.Edge)
	}
	if dropped.DisconnectedAt != nil {
		t.Fatalf("first close stamped disconnected_at while a socket was still open: %v", dropped.DisconnectedAt)
	}
}

func TestPresenceLastSocketClosingStampsAndIsAnEdge(t *testing.T) {
	st := openTestStore(t)
	cleaner := st.NewTestCleaner(t)
	ctx := context.Background()
	userID := newPresenceUser(t, st, cleaner, ctx)

	if _, err := st.PresenceConnected(ctx, userID); err != nil {
		t.Fatalf("connect: %v", err)
	}
	got, err := st.PresenceDisconnected(ctx, userID)
	if err != nil {
		t.Fatalf("disconnect: %v", err)
	}
	if got.ConnectionCount != 0 || !got.Edge {
		t.Fatalf("last close: got count=%d edge=%t, want 0/true", got.ConnectionCount, got.Edge)
	}
	if got.DisconnectedAt == nil {
		t.Fatal("last close did not stamp disconnected_at")
	}
}

// Reconnecting clears the stamp, which is what makes the grace window survivable.
func TestPresenceReconnectClearsStamp(t *testing.T) {
	st := openTestStore(t)
	cleaner := st.NewTestCleaner(t)
	ctx := context.Background()
	userID := newPresenceUser(t, st, cleaner, ctx)

	if _, err := st.PresenceConnected(ctx, userID); err != nil {
		t.Fatalf("connect: %v", err)
	}
	if _, err := st.PresenceDisconnected(ctx, userID); err != nil {
		t.Fatalf("disconnect: %v", err)
	}
	back, err := st.PresenceConnected(ctx, userID)
	if err != nil {
		t.Fatalf("reconnect: %v", err)
	}
	if back.DisconnectedAt != nil {
		t.Fatalf("reconnect left a stamp: %v", back.DisconnectedAt)
	}
	if !back.Edge {
		t.Fatal("reconnect from zero should be an edge")
	}
}

// An unbalanced release must not restart a window that is already running, or a
// player could be held in grace indefinitely by repeated stray closes.
func TestPresenceExtraDisconnectPreservesTheOriginalStamp(t *testing.T) {
	st := openTestStore(t)
	cleaner := st.NewTestCleaner(t)
	ctx := context.Background()
	userID := newPresenceUser(t, st, cleaner, ctx)

	if _, err := st.PresenceConnected(ctx, userID); err != nil {
		t.Fatalf("connect: %v", err)
	}
	first, err := st.PresenceDisconnected(ctx, userID)
	if err != nil {
		t.Fatalf("disconnect: %v", err)
	}
	original := *first.DisconnectedAt

	extra, err := st.PresenceDisconnected(ctx, userID)
	if err != nil {
		t.Fatalf("extra disconnect: %v", err)
	}
	if extra.DisconnectedAt == nil {
		t.Fatal("extra disconnect cleared the stamp")
	}
	if !extra.DisconnectedAt.Equal(original) {
		t.Fatalf("extra disconnect restarted the window: got %v, want %v", extra.DisconnectedAt, original)
	}
	if extra.Edge {
		t.Fatal("an extra disconnect is not an edge; the player was already gone")
	}
}

// A restart means no live sockets, so boot is pessimistic — but it must not restart
// the window of someone already inside one.
func TestResetPresenceOnBootZeroesCountsAndPreservesExistingStamp(t *testing.T) {
	st := openTestStore(t)
	cleaner := st.NewTestCleaner(t)
	ctx := context.Background()

	live := newPresenceUser(t, st, cleaner, ctx)
	alreadyGone := newPresenceUser(t, st, cleaner, ctx)

	if _, err := st.PresenceConnected(ctx, live); err != nil {
		t.Fatalf("connect live: %v", err)
	}
	if _, err := st.PresenceConnected(ctx, alreadyGone); err != nil {
		t.Fatalf("connect gone: %v", err)
	}
	dropped, err := st.PresenceDisconnected(ctx, alreadyGone)
	if err != nil {
		t.Fatalf("disconnect gone: %v", err)
	}
	original := *dropped.DisconnectedAt

	if _, err := st.ResetPresenceOnBoot(ctx); err != nil {
		t.Fatalf("ResetPresenceOnBoot: %v", err)
	}

	var liveCount int
	var liveStamp *time.Time
	if err := st.db.QueryRowContext(ctx,
		`SELECT connection_count, disconnected_at FROM user_presence WHERE user_id = $1`, live,
	).Scan(&liveCount, &liveStamp); err != nil {
		t.Fatalf("read live: %v", err)
	}
	if liveCount != 0 || liveStamp == nil {
		t.Fatalf("boot left a live user connected: count=%d stamp=%v", liveCount, liveStamp)
	}

	var goneStamp time.Time
	if err := st.db.QueryRowContext(ctx,
		`SELECT disconnected_at FROM user_presence WHERE user_id = $1`, alreadyGone,
	).Scan(&goneStamp); err != nil {
		t.Fatalf("read gone: %v", err)
	}
	if !goneStamp.Equal(original) {
		t.Fatalf("boot restarted an existing window: got %v, want %v", goneStamp, original)
	}
}
