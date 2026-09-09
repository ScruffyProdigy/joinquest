package store

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/scruffyprodigy/joinquest/internal/queuewait"
)

// seedModeQueue creates a throwaway mode and queue under the demo game and
// returns the queue id.
func seedModeQueue(t *testing.T, st *Store, name string) uuid.UUID {
	t.Helper()
	ctx := context.Background()
	modeID, queueID := uuid.New(), uuid.New()

	if _, err := st.db.ExecContext(ctx, `
		INSERT INTO game_modes (id, game_id, mode_key, display_name, min_players, max_players, seat_template, status)
		VALUES ($1, $2, $3, 'Fill samples mode', 2, 2, '{"count":2}'::jsonb, 'active')
	`, modeID, DemoPrimaryGameID, "fillsamples-"+name+"-"+uuid.NewString()[:8]); err != nil {
		t.Fatalf("insert mode: %v", err)
	}
	if _, err := st.db.ExecContext(ctx, `
		INSERT INTO mode_queues (id, mode_id, name, players_to_start, status, is_default)
		VALUES ($1, $2, $3, 2, 'active', false)
	`, queueID, modeID, "Fill samples "+name); err != nil {
		t.Fatalf("insert queue: %v", err)
	}
	t.Cleanup(func() {
		bg := context.Background()
		_, _ = st.db.ExecContext(bg, `DELETE FROM game_queues WHERE mode_queue_id = $1`, queueID)
		_, _ = st.db.ExecContext(bg, `DELETE FROM mode_queues WHERE id = $1`, queueID)
		_, _ = st.db.ExecContext(bg, `DELETE FROM game_modes WHERE id = $1`, modeID)
	})
	return queueID
}

// seedQueueRow inserts one game_queues row with explicit timings. A zero
// matchedAt is stored as NULL — the player never got matched.
func seedQueueRow(t *testing.T, st *Store, cleaner *TestCleaner, queueID uuid.UUID, status string, joinedAt, matchedAt time.Time) {
	t.Helper()
	ctx := context.Background()
	user, err := st.CreateUser(ctx, CreateUserParams{Email: "fill-" + uuid.NewString() + "@example.com"})
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}
	cleaner.TrackUser(user.ID)

	var matched any
	if !matchedAt.IsZero() {
		matched = matchedAt
	}
	if _, err := st.db.ExecContext(ctx, `
		INSERT INTO game_queues (game_id, user_id, mode_queue_id, status, joined_at, matched_at)
		VALUES ($1, $2, $3, $4, $5, $6)
	`, DemoPrimaryGameID, user.ID, queueID, status, joinedAt, matched); err != nil {
		t.Fatalf("insert queue row (%s): %v", status, err)
	}
}

func TestRecentModeQueueFillsReturnsCompletedWaitsInsideTheWindow(t *testing.T) {
	st := openTestStore(t)
	cleaner := st.NewTestCleaner(t)
	ctx := context.Background()
	now := time.Now()
	queueID := seedModeQueue(t, st, "window")

	// A fill inside the window: the one sample that should come back.
	seedQueueRow(t, st, cleaner, queueID, "matched", now.Add(-2*time.Hour), now.Add(-2*time.Hour).Add(30*time.Second))
	// Matched too long ago to describe the queue as it is now.
	seedQueueRow(t, st, cleaner, queueID, "matched", now.Add(-48*time.Hour), now.Add(-48*time.Hour).Add(90*time.Second))
	// Still waiting: no wait to observe yet.
	seedQueueRow(t, st, cleaner, queueID, "waiting", now.Add(-time.Minute), time.Time{})
	// Gave up before matching: says nothing about how long a fill takes.
	seedQueueRow(t, st, cleaner, queueID, "cancelled", now.Add(-time.Hour), time.Time{})

	fills, err := st.RecentModeQueueFills(ctx, queuewait.FillQuery{
		ModeQueueID: queueID,
		Since:       now.Add(-24 * time.Hour),
		Limit:       100,
	})
	if err != nil {
		t.Fatalf("RecentModeQueueFills: %v", err)
	}
	if len(fills) != 1 {
		t.Fatalf("got %d fills, want 1", len(fills))
	}
	if got := fills[0].Wait(); got != 30*time.Second {
		t.Errorf("got wait %v, want 30s", got)
	}
}

func TestRecentModeQueueFillsCountsSweptOrphansAsFills(t *testing.T) {
	st := openTestStore(t)
	cleaner := st.NewTestCleaner(t)
	ctx := context.Background()
	now := time.Now()
	queueID := seedModeQueue(t, st, "orphan")

	// The stale-matched sweep cancels rows whose session never came up. The
	// queue still filled in 45s, which is what a wait estimate is measuring.
	seedQueueRow(t, st, cleaner, queueID, "cancelled", now.Add(-time.Hour), now.Add(-time.Hour).Add(45*time.Second))

	fills, err := st.RecentModeQueueFills(ctx, queuewait.FillQuery{
		ModeQueueID: queueID,
		Since:       now.Add(-24 * time.Hour),
		Limit:       100,
	})
	if err != nil {
		t.Fatalf("RecentModeQueueFills: %v", err)
	}
	if len(fills) != 1 {
		t.Fatalf("got %d fills, want the swept orphan counted", len(fills))
	}
	if got := fills[0].Wait(); got != 45*time.Second {
		t.Errorf("got wait %v, want 45s", got)
	}
}

func TestRecentModeQueueFillsIgnoresOtherQueues(t *testing.T) {
	st := openTestStore(t)
	cleaner := st.NewTestCleaner(t)
	ctx := context.Background()
	now := time.Now()
	mine := seedModeQueue(t, st, "mine")
	theirs := seedModeQueue(t, st, "theirs")

	seedQueueRow(t, st, cleaner, mine, "matched", now.Add(-time.Hour), now.Add(-time.Hour).Add(10*time.Second))
	seedQueueRow(t, st, cleaner, theirs, "matched", now.Add(-time.Hour), now.Add(-time.Hour).Add(600*time.Second))

	fills, err := st.RecentModeQueueFills(ctx, queuewait.FillQuery{
		ModeQueueID: mine,
		Since:       now.Add(-24 * time.Hour),
		Limit:       100,
	})
	if err != nil {
		t.Fatalf("RecentModeQueueFills: %v", err)
	}
	if len(fills) != 1 {
		t.Fatalf("got %d fills, want only this queue's", len(fills))
	}
	if got := fills[0].Wait(); got != 10*time.Second {
		t.Errorf("got wait %v, want this queue's 10s and not the neighbour's", got)
	}
}

func TestRecentModeQueueFillsTakesTheMostRecentFillsUpToTheLimit(t *testing.T) {
	st := openTestStore(t)
	cleaner := st.NewTestCleaner(t)
	ctx := context.Background()
	now := time.Now()
	queueID := seedModeQueue(t, st, "limit")

	// Three fills, oldest carrying a wait no other row has.
	seedQueueRow(t, st, cleaner, queueID, "matched", now.Add(-3*time.Hour), now.Add(-3*time.Hour).Add(300*time.Second))
	seedQueueRow(t, st, cleaner, queueID, "matched", now.Add(-2*time.Hour), now.Add(-2*time.Hour).Add(20*time.Second))
	seedQueueRow(t, st, cleaner, queueID, "matched", now.Add(-1*time.Hour), now.Add(-1*time.Hour).Add(10*time.Second))

	fills, err := st.RecentModeQueueFills(ctx, queuewait.FillQuery{
		ModeQueueID: queueID,
		Since:       now.Add(-24 * time.Hour),
		Limit:       2,
	})
	if err != nil {
		t.Fatalf("RecentModeQueueFills: %v", err)
	}
	if len(fills) != 2 {
		t.Fatalf("got %d fills, want the limit of 2", len(fills))
	}
	for _, f := range fills {
		if f.Wait() == 300*time.Second {
			t.Errorf("got the oldest fill, want the limit to keep the two most recent")
		}
	}
}
