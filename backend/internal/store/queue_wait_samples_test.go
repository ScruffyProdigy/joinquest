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
	seedQueueRowOnPath(t, st, cleaner, queueID, "", status, joinedAt, matchedAt)
}

// seedQueueRowOnPath is seedQueueRow for a composition mode, where the player
// waits in one role's line rather than the queue at large.
func seedQueueRowOnPath(t *testing.T, st *Store, cleaner *TestCleaner, queueID uuid.UUID, queuePath, status string, joinedAt, matchedAt time.Time) {
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
	var path any
	if queuePath != "" {
		path = queuePath
	}
	if _, err := st.db.ExecContext(ctx, `
		INSERT INTO game_queues (game_id, user_id, mode_queue_id, queue_path, status, joined_at, matched_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
	`, DemoPrimaryGameID, user.ID, queueID, path, status, joinedAt, matched); err != nil {
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

	byQueue, err := st.RecentModeQueueFills(ctx, queuewait.FillQuery{
		ModeQueueIDs:  []uuid.UUID{queueID},
		Since:         now.Add(-24 * time.Hour),
		LimitPerQueue: 100,
	})
	fills := byQueue[queuewait.QueueKey{ModeQueueID: queueID}]
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

	byQueue, err := st.RecentModeQueueFills(ctx, queuewait.FillQuery{
		ModeQueueIDs:  []uuid.UUID{queueID},
		Since:         now.Add(-24 * time.Hour),
		LimitPerQueue: 100,
	})
	fills := byQueue[queuewait.QueueKey{ModeQueueID: queueID}]
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

	byQueue, err := st.RecentModeQueueFills(ctx, queuewait.FillQuery{
		ModeQueueIDs:  []uuid.UUID{mine},
		Since:         now.Add(-24 * time.Hour),
		LimitPerQueue: 100,
	})
	fills := byQueue[queuewait.QueueKey{ModeQueueID: mine}]
	if err != nil {
		t.Fatalf("RecentModeQueueFills: %v", err)
	}
	if len(fills) != 1 {
		t.Fatalf("got %d fills, want only this queue's", len(fills))
	}
	if got := fills[0].Wait(); got != 10*time.Second {
		t.Errorf("got wait %v, want this queue's 10s and not the neighbour's", got)
	}
	// Naming a queue must actually narrow the read. Asserting only on our own
	// key would pass just as happily against a query that ignored the filter
	// and hauled back every queue in the database.
	if _, ok := byQueue[queuewait.QueueKey{ModeQueueID: theirs}]; ok {
		t.Errorf("got the unrequested queue %v back too, want the id filter to exclude it", theirs)
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

	byQueue, err := st.RecentModeQueueFills(ctx, queuewait.FillQuery{
		ModeQueueIDs:  []uuid.UUID{queueID},
		Since:         now.Add(-24 * time.Hour),
		LimitPerQueue: 2,
	})
	fills := byQueue[queuewait.QueueKey{ModeQueueID: queueID}]
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

func TestRecentModeQueueFillsReadsEveryQueueInOneQuery(t *testing.T) {
	st := openTestStore(t)
	cleaner := st.NewTestCleaner(t)
	ctx := context.Background()
	now := time.Now()
	quick := seedModeQueue(t, st, "batch-quick")
	slow := seedModeQueue(t, st, "batch-slow")

	seedQueueRow(t, st, cleaner, quick, "matched", now.Add(-time.Hour), now.Add(-time.Hour).Add(10*time.Second))
	seedQueueRow(t, st, cleaner, slow, "matched", now.Add(-time.Hour), now.Add(-time.Hour).Add(90*time.Second))

	byQueue, err := st.RecentModeQueueFills(ctx, queuewait.FillQuery{
		ModeQueueIDs:  []uuid.UUID{quick, slow},
		Since:         now.Add(-24 * time.Hour),
		LimitPerQueue: 100,
	})
	if err != nil {
		t.Fatalf("RecentModeQueueFills: %v", err)
	}
	quickFills := byQueue[queuewait.QueueKey{ModeQueueID: quick}]
	slowFills := byQueue[queuewait.QueueKey{ModeQueueID: slow}]
	if len(quickFills) != 1 || quickFills[0].Wait() != 10*time.Second {
		t.Errorf("quick queue got %v, want a single 10s fill", quickFills)
	}
	if len(slowFills) != 1 || slowFills[0].Wait() != 90*time.Second {
		t.Errorf("slow queue got %v, want a single 90s fill", slowFills)
	}
}

func TestRecentModeQueueFillsCapsEachQueueSeparately(t *testing.T) {
	st := openTestStore(t)
	cleaner := st.NewTestCleaner(t)
	ctx := context.Background()
	now := time.Now()
	busy := seedModeQueue(t, st, "cap-busy")
	quiet := seedModeQueue(t, st, "cap-quiet")

	// A busy queue must not consume the quiet one's share of the sample.
	for i := 1; i <= 4; i++ {
		at := now.Add(-time.Duration(i) * time.Minute)
		seedQueueRow(t, st, cleaner, busy, "matched", at, at.Add(time.Duration(i)*time.Second))
	}
	seedQueueRow(t, st, cleaner, quiet, "matched", now.Add(-time.Hour), now.Add(-time.Hour).Add(70*time.Second))

	byQueue, err := st.RecentModeQueueFills(ctx, queuewait.FillQuery{
		ModeQueueIDs:  []uuid.UUID{busy, quiet},
		Since:         now.Add(-24 * time.Hour),
		LimitPerQueue: 2,
	})
	if err != nil {
		t.Fatalf("RecentModeQueueFills: %v", err)
	}
	if got := len(byQueue[queuewait.QueueKey{ModeQueueID: busy}]); got != 2 {
		t.Errorf("busy queue got %d fills, want the per-queue cap of 2", got)
	}
	if got := len(byQueue[queuewait.QueueKey{ModeQueueID: quiet}]); got != 1 {
		t.Errorf("quiet queue got %d fills, want its own 1 kept despite the busy queue", got)
	}
}

func TestRecentModeQueueFillsReadsEveryQueueWhenNoneAreNamed(t *testing.T) {
	st := openTestStore(t)
	cleaner := st.NewTestCleaner(t)
	ctx := context.Background()
	now := time.Now()
	queueID := seedModeQueue(t, st, "unnamed")

	seedQueueRow(t, st, cleaner, queueID, "matched", now.Add(-time.Hour), now.Add(-time.Hour).Add(25*time.Second))

	// An empty id list is the whole-catalog snapshot the cache is built on.
	byQueue, err := st.RecentModeQueueFills(ctx, queuewait.FillQuery{
		Since:         now.Add(-24 * time.Hour),
		LimitPerQueue: 100,
	})
	if err != nil {
		t.Fatalf("RecentModeQueueFills: %v", err)
	}
	fills := byQueue[queuewait.QueueKey{ModeQueueID: queueID}]
	if len(fills) != 1 || fills[0].Wait() != 25*time.Second {
		t.Errorf("got %v for the seeded queue, want a single 25s fill", fills)
	}
}

func TestRecentModeQueueFillsSeparatesEachQueuePath(t *testing.T) {
	st := openTestStore(t)
	cleaner := st.NewTestCleaner(t)
	ctx := context.Background()
	now := time.Now()
	queueID := seedModeQueue(t, st, "roles")

	// One queue, two roles, wildly different waits — the case a queue-wide
	// median would flatten into a number describing neither.
	for _, wait := range []int{4, 8, 12} {
		at := now.Add(-time.Duration(wait) * time.Minute)
		seedQueueRowOnPath(t, st, cleaner, queueID, "tank", "matched", at, at.Add(time.Duration(wait)*time.Second))
	}
	seedQueueRowOnPath(t, st, cleaner, queueID, "damage", "matched", now.Add(-time.Hour), now.Add(-time.Hour).Add(240*time.Second))

	byQueue, err := st.RecentModeQueueFills(ctx, queuewait.FillQuery{
		ModeQueueIDs:  []uuid.UUID{queueID},
		Since:         now.Add(-24 * time.Hour),
		LimitPerQueue: 100,
	})
	if err != nil {
		t.Fatalf("RecentModeQueueFills: %v", err)
	}

	tank := byQueue[queuewait.QueueKey{ModeQueueID: queueID, QueuePath: "tank"}]
	damage := byQueue[queuewait.QueueKey{ModeQueueID: queueID, QueuePath: "damage"}]
	if len(tank) != 3 {
		t.Errorf("tank path got %d fills, want 3", len(tank))
	}
	if len(damage) != 1 || damage[0].Wait() != 240*time.Second {
		t.Errorf("damage path got %v, want a single 240s fill", damage)
	}
	if _, ok := byQueue[queuewait.QueueKey{ModeQueueID: queueID}]; ok {
		t.Error("got an unsplit bucket for a queue whose rows all carry a path, want none")
	}
}

func TestRecentModeQueueFillsCapsEachQueuePathSeparately(t *testing.T) {
	st := openTestStore(t)
	cleaner := st.NewTestCleaner(t)
	ctx := context.Background()
	now := time.Now()
	queueID := seedModeQueue(t, st, "role-caps")

	// A crowded role must not eat a scarce role's share of the sample.
	for i := 1; i <= 4; i++ {
		at := now.Add(-time.Duration(i) * time.Minute)
		seedQueueRowOnPath(t, st, cleaner, queueID, "damage", "matched", at, at.Add(time.Duration(i)*time.Second))
	}
	seedQueueRowOnPath(t, st, cleaner, queueID, "tank", "matched", now.Add(-time.Hour), now.Add(-time.Hour).Add(5*time.Second))

	byQueue, err := st.RecentModeQueueFills(ctx, queuewait.FillQuery{
		ModeQueueIDs:  []uuid.UUID{queueID},
		Since:         now.Add(-24 * time.Hour),
		LimitPerQueue: 2,
	})
	if err != nil {
		t.Fatalf("RecentModeQueueFills: %v", err)
	}
	if got := len(byQueue[queuewait.QueueKey{ModeQueueID: queueID, QueuePath: "damage"}]); got != 2 {
		t.Errorf("damage path got %d fills, want the per-path cap of 2", got)
	}
	if got := len(byQueue[queuewait.QueueKey{ModeQueueID: queueID, QueuePath: "tank"}]); got != 1 {
		t.Errorf("tank path got %d fills, want its own 1 kept despite the crowded role", got)
	}
}
