package store

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
)

// makeStaleMatchedRow puts the user in the demo queue and forces their row to
// matched with no session behind it, aged by the given duration.
func makeStaleMatchedRow(t *testing.T, st *Store, ctx context.Context, userID uuid.UUID, age time.Duration) uuid.UUID {
	t.Helper()

	if _, err := st.JoinModeQueue(ctx, DemoDefaultQueueID, userID, "", nil); err != nil {
		t.Fatalf("join: %v", err)
	}

	var rowID uuid.UUID
	if err := st.db.QueryRowContext(ctx, `
		UPDATE game_queues
		SET status = 'matched', matched_at = NOW() - $3::interval
		WHERE user_id = $1 AND mode_queue_id = $2 AND status = 'waiting'
		RETURNING id
	`, userID, DemoDefaultQueueID, age.String()).Scan(&rowID); err != nil {
		t.Fatalf("simulate stale matched row: %v", err)
	}
	return rowID
}

func newSweepUser(t *testing.T, st *Store, cleaner *TestCleaner, ctx context.Context, prefix string) uuid.UUID {
	t.Helper()
	user, err := st.CreateUser(ctx, CreateUserParams{
		Email: prefix + "-" + uuid.NewString() + "@example.com",
		// A display name keeps these fixtures usable by any match that picks them
		// up, so a leaked row can never fail an unrelated handoff test.
		DisplayName: "Sweep Fixture",
	})
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}
	cleaner.TrackUser(user.ID)
	return user.ID
}

func TestSweepStaleMatchedQueuesClearsAgedOrphanRow(t *testing.T) {
	st := openTestStore(t)
	cleaner := st.NewTestCleaner(t)
	ctx := context.Background()

	userID := newSweepUser(t, st, cleaner, ctx, "sweep-stale")
	rowID := makeStaleMatchedRow(t, st, ctx, userID, time.Hour)

	result, err := st.SweepStaleMatchedQueues(ctx, 5*time.Minute)
	if err != nil {
		t.Fatalf("SweepStaleMatchedQueues: %v", err)
	}
	if result.Cancelled < 1 {
		t.Fatalf("expected at least 1 cancelled row, got %d", result.Cancelled)
	}
	if status := queueRowStatus(t, st, rowID); status != "cancelled" {
		t.Fatalf("stale matched row status = %q, want cancelled", status)
	}
}

func TestSweepStaleMatchedQueuesLeavesRecentlyMatchedRowAlone(t *testing.T) {
	st := openTestStore(t)
	cleaner := st.NewTestCleaner(t)
	ctx := context.Background()

	userID := newSweepUser(t, st, cleaner, ctx, "sweep-fresh")
	rowID := makeStaleMatchedRow(t, st, ctx, userID, 10*time.Second)

	if _, err := st.SweepStaleMatchedQueues(ctx, 5*time.Minute); err != nil {
		t.Fatalf("SweepStaleMatchedQueues: %v", err)
	}
	if status := queueRowStatus(t, st, rowID); status != "matched" {
		t.Fatalf("row still provisioning was swept: status = %q, want matched", status)
	}
}

func TestSweepStaleMatchedQueuesIsIdempotent(t *testing.T) {
	st := openTestStore(t)
	cleaner := st.NewTestCleaner(t)
	ctx := context.Background()

	userID := newSweepUser(t, st, cleaner, ctx, "sweep-idem")
	makeStaleMatchedRow(t, st, ctx, userID, time.Hour)

	if _, err := st.SweepStaleMatchedQueues(ctx, 5*time.Minute); err != nil {
		t.Fatalf("first sweep: %v", err)
	}
	second, err := st.SweepStaleMatchedQueues(ctx, 5*time.Minute)
	if err != nil {
		t.Fatalf("second sweep: %v", err)
	}
	if second.Cancelled != 0 {
		t.Fatalf("second sweep cancelled %d rows, want 0", second.Cancelled)
	}
}

func TestSweepStaleMatchedQueuesSpansMultipleUsers(t *testing.T) {
	st := openTestStore(t)
	cleaner := st.NewTestCleaner(t)
	ctx := context.Background()

	userA := newSweepUser(t, st, cleaner, ctx, "sweep-multi-a")
	userB := newSweepUser(t, st, cleaner, ctx, "sweep-multi-b")
	rowA := makeStaleMatchedRow(t, st, ctx, userA, time.Hour)
	rowB := makeStaleMatchedRow(t, st, ctx, userB, time.Hour)

	if _, err := st.SweepStaleMatchedQueues(ctx, 5*time.Minute); err != nil {
		t.Fatalf("SweepStaleMatchedQueues: %v", err)
	}
	if status := queueRowStatus(t, st, rowA); status != "cancelled" {
		t.Fatalf("user A row status = %q, want cancelled", status)
	}
	if status := queueRowStatus(t, st, rowB); status != "cancelled" {
		t.Fatalf("user B row status = %q, want cancelled", status)
	}
}

func TestSweepStaleMatchedQueuesSparesLiveMatch(t *testing.T) {
	st := openTestStore(t)
	cleaner := st.NewTestCleaner(t)
	ctx := context.Background()

	userA := newSweepUser(t, st, cleaner, ctx, "sweep-live-a")
	userB := newSweepUser(t, st, cleaner, ctx, "sweep-live-b")

	if _, err := st.JoinModeQueue(ctx, DemoDefaultQueueID, userA, "", nil); err != nil {
		t.Fatalf("join A: %v", err)
	}
	if _, err := st.JoinModeQueue(ctx, DemoDefaultQueueID, userB, "", nil); err != nil {
		t.Fatalf("join B: %v", err)
	}
	mustReconcileForming(t, st, ctx, DemoDefaultQueueID)

	// Age the live match past the threshold: a real session exists, so the sweep
	// must spare it no matter how old the matched row is.
	if _, err := st.db.ExecContext(ctx, `
		UPDATE game_queues SET matched_at = NOW() - INTERVAL '1 hour'
		WHERE user_id IN ($1, $2) AND status = 'matched'
	`, userA, userB); err != nil {
		t.Fatalf("age live matched rows: %v", err)
	}

	if _, err := st.SweepStaleMatchedQueues(ctx, 5*time.Minute); err != nil {
		t.Fatalf("SweepStaleMatchedQueues: %v", err)
	}

	intent, err := st.GetUserActiveIntent(ctx, userA)
	if err != nil {
		t.Fatalf("GetUserActiveIntent: %v", err)
	}
	if intent == nil || !intent.Matched || intent.SessionID == nil {
		t.Fatalf("sweep destroyed a live match: intent = %+v", intent)
	}
}

func TestCountStaleMatchedQueuesMatchesSweep(t *testing.T) {
	st := openTestStore(t)
	cleaner := st.NewTestCleaner(t)
	ctx := context.Background()

	before, err := st.CountStaleMatchedQueues(ctx, 5*time.Minute)
	if err != nil {
		t.Fatalf("CountStaleMatchedQueues: %v", err)
	}

	userID := newSweepUser(t, st, cleaner, ctx, "sweep-count")
	makeStaleMatchedRow(t, st, ctx, userID, time.Hour)

	after, err := st.CountStaleMatchedQueues(ctx, 5*time.Minute)
	if err != nil {
		t.Fatalf("CountStaleMatchedQueues after: %v", err)
	}
	if after != before+1 {
		t.Fatalf("stale count = %d, want %d", after, before+1)
	}

	if _, err := st.SweepStaleMatchedQueues(ctx, 5*time.Minute); err != nil {
		t.Fatalf("SweepStaleMatchedQueues: %v", err)
	}

	cleared, err := st.CountStaleMatchedQueues(ctx, 5*time.Minute)
	if err != nil {
		t.Fatalf("CountStaleMatchedQueues cleared: %v", err)
	}
	if cleared != 0 {
		t.Fatalf("post-sweep stale count = %d, want 0", cleared)
	}
}
