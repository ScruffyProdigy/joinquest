package store

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/scruffyprodigy/joinquest/internal/queuewait"
)

// flowOf runs one measurement over the last hour and returns the line asked
// about, so each test states only the rows it seeded.
func flowOf(t *testing.T, st *Store, now time.Time, key queuewait.QueueKey) queuewait.Flow {
	t.Helper()
	byQueue, err := st.RecentQueueFlow(context.Background(), queuewait.FlowQuery{
		ModeQueueIDs: []uuid.UUID{key.ModeQueueID},
		Since:        now.Add(-time.Hour),
		Now:          now,
	})
	if err != nil {
		t.Fatalf("RecentQueueFlow: %v", err)
	}
	return byQueue[key]
}

func TestRecentQueueFlowCountsArrivalsFillsAndDepth(t *testing.T) {
	st := openTestStore(t)
	cleaner := st.NewTestCleaner(t)
	now := time.Now()
	queueID := seedModeQueue(t, st, "flow-basic")

	// Joined and matched inside the window: one arrival and one fill.
	seedQueueRow(t, st, cleaner, queueID, "matched", now.Add(-10*time.Minute), now.Add(-9*time.Minute))
	// Joined inside the window and still waiting: an arrival and depth.
	seedQueueRow(t, st, cleaner, queueID, "waiting", now.Add(-5*time.Minute), time.Time{})
	// Gave up: an arrival, but neither a fill nor depth.
	seedQueueRow(t, st, cleaner, queueID, "cancelled", now.Add(-3*time.Minute), time.Time{})

	flow := flowOf(t, st, now, queuewait.QueueKey{ModeQueueID: queueID})

	if flow.Arrivals != 3 {
		t.Errorf("got %d arrivals, want 3", flow.Arrivals)
	}
	if flow.Fills != 1 {
		t.Errorf("got %d fills, want 1", flow.Fills)
	}
	if flow.Depth != 1 {
		t.Errorf("got depth %d, want 1", flow.Depth)
	}
}

// The window is what makes the measurement live rather than historical.
func TestRecentQueueFlowExcludesArrivalsAndFillsOutsideTheWindow(t *testing.T) {
	st := openTestStore(t)
	cleaner := st.NewTestCleaner(t)
	now := time.Now()
	queueID := seedModeQueue(t, st, "flow-window")

	// Joined and matched before the window opened: neither counts.
	seedQueueRow(t, st, cleaner, queueID, "matched", now.Add(-5*time.Hour), now.Add(-4*time.Hour))

	flow := flowOf(t, st, now, queuewait.QueueKey{ModeQueueID: queueID})

	if flow.Arrivals != 0 {
		t.Errorf("got %d arrivals, want 0", flow.Arrivals)
	}
	if flow.Fills != 0 {
		t.Errorf("got %d fills, want 0", flow.Fills)
	}
}

// A player who joined before the window opened and is still there is still in
// front of everyone behind them. Depth is a level, not a rate.
func TestRecentQueueFlowCountsDepthRegardlessOfTheWindow(t *testing.T) {
	st := openTestStore(t)
	cleaner := st.NewTestCleaner(t)
	now := time.Now()
	queueID := seedModeQueue(t, st, "flow-depth")

	seedQueueRow(t, st, cleaner, queueID, "waiting", now.Add(-6*time.Hour), time.Time{})

	flow := flowOf(t, st, now, queuewait.QueueKey{ModeQueueID: queueID})

	if flow.Depth != 1 {
		t.Errorf("got depth %d, want 1", flow.Depth)
	}
	if flow.Arrivals != 0 {
		t.Errorf("got %d arrivals, want 0 — the join was outside the window", flow.Arrivals)
	}
}

// A player who joined inside the window and matched outside it is an arrival
// but not a fill: the two legs are counted independently, not as one row test.
func TestRecentQueueFlowCountsArrivalsAndFillsIndependently(t *testing.T) {
	st := openTestStore(t)
	cleaner := st.NewTestCleaner(t)
	now := time.Now()
	queueID := seedModeQueue(t, st, "flow-legs")

	// Matched inside the window, joined long before it: a fill, not an arrival.
	seedQueueRow(t, st, cleaner, queueID, "matched", now.Add(-5*time.Hour), now.Add(-10*time.Minute))

	flow := flowOf(t, st, now, queuewait.QueueKey{ModeQueueID: queueID})

	if flow.Fills != 1 {
		t.Errorf("got %d fills, want 1", flow.Fills)
	}
	if flow.Arrivals != 0 {
		t.Errorf("got %d arrivals, want 0", flow.Arrivals)
	}
}

// Composition modes split into lines that move at different speeds, so the
// measurement has to split with them — the whole point of QueueKey.
func TestRecentQueueFlowSeparatesEachQueuePath(t *testing.T) {
	st := openTestStore(t)
	cleaner := st.NewTestCleaner(t)
	now := time.Now()
	queueID := seedModeQueue(t, st, "flow-paths")

	seedQueueRowOnPath(t, st, cleaner, queueID, "tank", "matched", now.Add(-10*time.Minute), now.Add(-9*time.Minute))
	seedQueueRowOnPath(t, st, cleaner, queueID, "support", "waiting", now.Add(-5*time.Minute), time.Time{})
	seedQueueRowOnPath(t, st, cleaner, queueID, "support", "waiting", now.Add(-4*time.Minute), time.Time{})

	byQueue, err := st.RecentQueueFlow(context.Background(), queuewait.FlowQuery{
		ModeQueueIDs: []uuid.UUID{queueID},
		Since:        now.Add(-time.Hour),
		Now:          now,
	})
	if err != nil {
		t.Fatalf("RecentQueueFlow: %v", err)
	}

	tank := byQueue[queuewait.QueueKey{ModeQueueID: queueID, QueuePath: "tank"}]
	support := byQueue[queuewait.QueueKey{ModeQueueID: queueID, QueuePath: "support"}]

	if tank.Fills != 1 || tank.Depth != 0 {
		t.Errorf("got tank fills %d depth %d, want 1 and 0", tank.Fills, tank.Depth)
	}
	if support.Fills != 0 || support.Depth != 2 {
		t.Errorf("got support fills %d depth %d, want 0 and 2", support.Fills, support.Depth)
	}
}

func TestRecentQueueFlowIgnoresQueuesNotAskedAbout(t *testing.T) {
	st := openTestStore(t)
	cleaner := st.NewTestCleaner(t)
	now := time.Now()
	wanted := seedModeQueue(t, st, "flow-wanted")
	other := seedModeQueue(t, st, "flow-other")

	seedQueueRow(t, st, cleaner, wanted, "waiting", now.Add(-5*time.Minute), time.Time{})
	seedQueueRow(t, st, cleaner, other, "waiting", now.Add(-5*time.Minute), time.Time{})

	byQueue, err := st.RecentQueueFlow(context.Background(), queuewait.FlowQuery{
		ModeQueueIDs: []uuid.UUID{wanted},
		Since:        now.Add(-time.Hour),
		Now:          now,
	})
	if err != nil {
		t.Fatalf("RecentQueueFlow: %v", err)
	}
	if _, present := byQueue[queuewait.QueueKey{ModeQueueID: other}]; present {
		t.Error("got the unrequested queue in the result, want it absent")
	}
}

// The window a rate divides by is the one actually measured over, so it travels
// back with the counts rather than being re-derived by every caller.
func TestRecentQueueFlowReportsTheWindowItMeasured(t *testing.T) {
	st := openTestStore(t)
	cleaner := st.NewTestCleaner(t)
	now := time.Now()
	queueID := seedModeQueue(t, st, "flow-reports-window")

	seedQueueRow(t, st, cleaner, queueID, "waiting", now.Add(-5*time.Minute), time.Time{})

	flow := flowOf(t, st, now, queuewait.QueueKey{ModeQueueID: queueID})

	if flow.Window != time.Hour {
		t.Errorf("got window %v, want 1h", flow.Window)
	}
}

func TestPositionOnLineCountsOnlyEarlierJoinersOnTheSameLine(t *testing.T) {
	st := openTestStore(t)
	cleaner := st.NewTestCleaner(t)
	ctx := context.Background()
	now := time.Now()
	queueID := seedModeQueue(t, st, "position-line")

	seedQueueRowOnPath(t, st, cleaner, queueID, "support", "waiting", now.Add(-10*time.Minute), time.Time{})
	seedQueueRowOnPath(t, st, cleaner, queueID, "support", "waiting", now.Add(-8*time.Minute), time.Time{})
	// Behind the player: does not lengthen their wait.
	seedQueueRowOnPath(t, st, cleaner, queueID, "support", "waiting", now.Add(-2*time.Minute), time.Time{})
	// A different role's line entirely.
	seedQueueRowOnPath(t, st, cleaner, queueID, "tank", "waiting", now.Add(-9*time.Minute), time.Time{})

	rowID := queueRowIDAt(t, st, queueID, "support", now.Add(-8*time.Minute))

	position, waiting, err := st.PositionOnLine(ctx, rowID)
	if err != nil {
		t.Fatalf("PositionOnLine: %v", err)
	}
	if !waiting {
		t.Fatal("got not waiting, want waiting")
	}
	if position.Ahead != 1 {
		t.Errorf("got %d ahead, want 1", position.Ahead)
	}
	if position.Depth != 3 {
		t.Errorf("got depth %d, want 3", position.Depth)
	}
	if position.Key.QueuePath != "support" {
		t.Errorf("got path %q, want support", position.Key.QueuePath)
	}
	if position.Key.ModeQueueID != queueID {
		t.Errorf("got queue %v, want %v", position.Key.ModeQueueID, queueID)
	}
}

func TestPositionOnLineReportsNotWaitingForAMatchedRow(t *testing.T) {
	st := openTestStore(t)
	cleaner := st.NewTestCleaner(t)
	ctx := context.Background()
	now := time.Now()
	queueID := seedModeQueue(t, st, "position-matched")

	seedQueueRow(t, st, cleaner, queueID, "matched", now.Add(-10*time.Minute), now.Add(-9*time.Minute))
	rowID := queueRowIDAt(t, st, queueID, "", now.Add(-10*time.Minute))

	_, waiting, err := st.PositionOnLine(ctx, rowID)
	if err != nil {
		t.Fatalf("PositionOnLine: %v", err)
	}
	if waiting {
		t.Error("got waiting, want not waiting for a matched row")
	}
}

func TestPositionOnLineReportsNotWaitingForAnUnknownRow(t *testing.T) {
	st := openTestStore(t)

	_, waiting, err := st.PositionOnLine(context.Background(), uuid.New())
	if err != nil {
		t.Fatalf("PositionOnLine: %v", err)
	}
	if waiting {
		t.Error("got waiting, want not waiting for a row that does not exist")
	}
}

// queueRowIDAt finds the seeded row for a line by its join time, so a test can
// name the player it cares about without threading ids through the seeders.
func queueRowIDAt(t *testing.T, st *Store, queueID uuid.UUID, queuePath string, joinedAt time.Time) uuid.UUID {
	t.Helper()
	var id uuid.UUID
	err := st.db.QueryRowContext(context.Background(), `
		SELECT id FROM game_queues
		WHERE mode_queue_id = $1
		  AND COALESCE(queue_path, '') = $2
		ORDER BY ABS(EXTRACT(EPOCH FROM (joined_at - $3::timestamptz)))
		LIMIT 1
	`, queueID, queuePath, joinedAt).Scan(&id)
	if err != nil {
		t.Fatalf("find seeded queue row: %v", err)
	}
	return id
}
