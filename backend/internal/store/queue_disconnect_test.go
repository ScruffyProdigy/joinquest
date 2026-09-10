package store

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
)

// queueAndDisconnect puts the user in the demo queue and drops their last socket,
// returning the stamp the grace window is running from.
//
// It deliberately does NOT touch GetUserActiveIntent or myActiveIntent. Those run an
// unguarded heal (queue_active.go) that cancels any matched row with no session
// behind it — which is the shape of a held seat, and would destroy the row under test
// in TestEvictNeverTouchesAMatchedRow for reasons nothing to do with this code.
func queueAndDisconnect(t *testing.T, st *Store, ctx context.Context, userID uuid.UUID) time.Time {
	t.Helper()
	if _, err := st.JoinModeQueue(ctx, DemoDefaultQueueID, userID, "", nil); err != nil {
		t.Fatalf("join: %v", err)
	}
	if _, err := st.PresenceConnected(ctx, userID); err != nil {
		t.Fatalf("connect: %v", err)
	}
	dropped, err := st.PresenceDisconnected(ctx, userID)
	if err != nil {
		t.Fatalf("disconnect: %v", err)
	}
	if dropped.DisconnectedAt == nil {
		t.Fatal("disconnect did not stamp")
	}
	return *dropped.DisconnectedAt
}

func waitingRowCount(t *testing.T, st *Store, ctx context.Context, userID uuid.UUID) int {
	t.Helper()
	var n int
	if err := st.db.QueryRowContext(ctx,
		`SELECT count(*) FROM game_queues WHERE user_id = $1 AND status = 'waiting'`, userID,
	).Scan(&n); err != nil {
		t.Fatalf("count waiting: %v", err)
	}
	return n
}

func TestDefaultQueueDisconnectGraceIs90Seconds(t *testing.T) {
	if DefaultQueueDisconnectGrace != 90*time.Second {
		t.Fatalf("grace window: got %s, want 90s", DefaultQueueDisconnectGrace)
	}
}

func TestEvictDisconnectedWaitingEntryRemovesTheRow(t *testing.T) {
	st := openTestStore(t)
	cleaner := st.NewTestCleaner(t)
	ctx := context.Background()
	userID := newPresenceUser(t, st, cleaner, ctx)

	stamp := queueAndDisconnect(t, st, ctx, userID)

	got, err := st.EvictDisconnectedWaitingEntry(ctx, userID, stamp)
	if err != nil {
		t.Fatalf("EvictDisconnectedWaitingEntry: %v", err)
	}
	if !got.Acted {
		t.Fatal("expected the eviction to act")
	}
	if got.ModeQueueID != DemoDefaultQueueID {
		t.Fatalf("mode queue: got %s, want %s", got.ModeQueueID, DemoDefaultQueueID)
	}
	if got.GameID == uuid.Nil {
		t.Fatal("eviction returned no game id, so the caller cannot publish")
	}
	if n := waitingRowCount(t, st, ctx, userID); n != 0 {
		t.Fatalf("row survived eviction: %d waiting rows", n)
	}
}

// Reconnecting inside the window must save the player, even though the expiry timer
// still fires with the stamp it was armed on.
func TestEvictDeclinesAfterReconnect(t *testing.T) {
	st := openTestStore(t)
	cleaner := st.NewTestCleaner(t)
	ctx := context.Background()
	userID := newPresenceUser(t, st, cleaner, ctx)

	stamp := queueAndDisconnect(t, st, ctx, userID)
	if _, err := st.PresenceConnected(ctx, userID); err != nil {
		t.Fatalf("reconnect: %v", err)
	}

	got, err := st.EvictDisconnectedWaitingEntry(ctx, userID, stamp)
	if err != nil {
		t.Fatalf("EvictDisconnectedWaitingEntry: %v", err)
	}
	if got.Acted {
		t.Fatal("evicted a player who had reconnected")
	}
	if n := waitingRowCount(t, st, ctx, userID); n != 1 {
		t.Fatalf("reconnected player lost their row: %d waiting rows", n)
	}
}

// disconnect, reconnect, disconnect again — the FIRST timer must not evict, because
// the second disconnect owns the window now.
func TestEvictDeclinesOnStaleStamp(t *testing.T) {
	st := openTestStore(t)
	cleaner := st.NewTestCleaner(t)
	ctx := context.Background()
	userID := newPresenceUser(t, st, cleaner, ctx)

	firstStamp := queueAndDisconnect(t, st, ctx, userID)
	if _, err := st.PresenceConnected(ctx, userID); err != nil {
		t.Fatalf("reconnect: %v", err)
	}
	second, err := st.PresenceDisconnected(ctx, userID)
	if err != nil {
		t.Fatalf("second disconnect: %v", err)
	}
	if second.DisconnectedAt.Equal(firstStamp) {
		t.Fatal("the second disconnect reused the first stamp, so this test proves nothing")
	}

	got, err := st.EvictDisconnectedWaitingEntry(ctx, userID, firstStamp)
	if err != nil {
		t.Fatalf("EvictDisconnectedWaitingEntry: %v", err)
	}
	if got.Acted {
		t.Fatal("the first timer evicted using a stamp the second disconnect replaced")
	}
	if n := waitingRowCount(t, st, ctx, userID); n != 1 {
		t.Fatalf("stale timer removed the row: %d waiting rows", n)
	}
}

// A deliberate leave during the window must not be counted twice: the row is already
// cancelled, so the expiry has nothing to do and must report that it did nothing.
func TestEvictDeclinesAfterDeliberateLeave(t *testing.T) {
	st := openTestStore(t)
	cleaner := st.NewTestCleaner(t)
	ctx := context.Background()
	userID := newPresenceUser(t, st, cleaner, ctx)

	stamp := queueAndDisconnect(t, st, ctx, userID)
	if _, err := st.LeaveModeQueue(ctx, DemoDefaultQueueID, userID); err != nil {
		t.Fatalf("LeaveModeQueue: %v", err)
	}

	got, err := st.EvictDisconnectedWaitingEntry(ctx, userID, stamp)
	if err != nil {
		t.Fatalf("EvictDisconnectedWaitingEntry: %v", err)
	}
	if got.Acted {
		t.Fatal("expiry acted on a row the player had already left")
	}
}

// THE JQ-199 INTERLEAVING. Disconnect at t=0, a match forms at t=30s, JQ-199 holds
// the seat to t=2min, and this ticket's expiry fires at t=90s. It must decline — and
// crucially the matched row must still be MATCHED afterwards, not merely "not
// waiting": the failure mode this guards against produces a CANCELLED matched row,
// which would satisfy a weaker assertion perfectly well.
func TestEvictNeverTouchesAMatchedRow(t *testing.T) {
	st := openTestStore(t)
	cleaner := st.NewTestCleaner(t)
	ctx := context.Background()
	userID := newPresenceUser(t, st, cleaner, ctx)

	stamp := queueAndDisconnect(t, st, ctx, userID)

	// The match forms while the player is still inside their grace window.
	if _, err := st.db.ExecContext(ctx, `
		UPDATE game_queues
		SET status = 'matched', matched_at = NOW()
		WHERE user_id = $1 AND status = 'waiting'
	`, userID); err != nil {
		t.Fatalf("simulate match forming: %v", err)
	}

	got, err := st.EvictDisconnectedWaitingEntry(ctx, userID, stamp)
	if err != nil {
		t.Fatalf("EvictDisconnectedWaitingEntry: %v", err)
	}
	if got.Acted {
		t.Fatal("expiry acted on a row that had become matched")
	}

	var status string
	if err := st.db.QueryRowContext(ctx,
		`SELECT status FROM game_queues WHERE user_id = $1`, userID,
	).Scan(&status); err != nil {
		t.Fatalf("read status: %v", err)
	}
	if status != "matched" {
		t.Fatalf("held seat was destroyed: status is %q, want \"matched\"", status)
	}
}

// The caller publishes the queued count to everyone still waiting, so it has to be
// the count AFTER the eviction, not before.
func TestEvictReportsQueuedCountAfterRemoval(t *testing.T) {
	st := openTestStore(t)
	cleaner := st.NewTestCleaner(t)
	ctx := context.Background()

	staying := newPresenceUser(t, st, cleaner, ctx)
	if _, err := st.JoinModeQueue(ctx, DemoDefaultQueueID, staying, "", nil); err != nil {
		t.Fatalf("join staying: %v", err)
	}

	leaving := newPresenceUser(t, st, cleaner, ctx)
	stamp := queueAndDisconnect(t, st, ctx, leaving)

	before, err := st.CountWaitingInModeQueue(ctx, DemoDefaultQueueID)
	if err != nil {
		t.Fatalf("count before: %v", err)
	}

	got, err := st.EvictDisconnectedWaitingEntry(ctx, leaving, stamp)
	if err != nil {
		t.Fatalf("EvictDisconnectedWaitingEntry: %v", err)
	}
	if !got.Acted {
		t.Fatal("expected the eviction to act")
	}
	if got.QueuedCount != before-1 {
		t.Fatalf("queued count: got %d, want %d (the count after removal)", got.QueuedCount, before-1)
	}
}
