package store

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/scruffyprodigy/joinquest/internal/queuewait"
)

// countsFor reads the whole-catalog aggregate and keeps only the queues a test
// seeded. The query is catalog-wide by design, so a test that asserted on the
// whole map would be asserting on every other test's rows too.
func countsFor(t *testing.T, st *Store, queueIDs ...uuid.UUID) map[queuewait.QueueKey]int {
	t.Helper()
	all, err := st.CountWaitingByQueue(context.Background())
	if err != nil {
		t.Fatalf("CountWaitingByQueue: %v", err)
	}
	mine := make(map[queuewait.QueueKey]int)
	for key, waiting := range all {
		for _, queueID := range queueIDs {
			if key.ModeQueueID == queueID {
				mine[key] = waiting
			}
		}
	}
	return mine
}

func TestCountWaitingByQueueCountsOnlyPlayersStillWaiting(t *testing.T) {
	st := openTestStore(t)
	cleaner := st.NewTestCleaner(t)
	now := time.Now()
	queueID := seedModeQueue(t, st, "waiting-counts-basic")

	seedQueueRow(t, st, cleaner, queueID, "waiting", now.Add(-5*time.Minute), time.Time{})
	seedQueueRow(t, st, cleaner, queueID, "waiting", now.Add(-2*time.Minute), time.Time{})
	// Already matched, and gave up: both left the line, so neither is waiting.
	seedQueueRow(t, st, cleaner, queueID, "matched", now.Add(-10*time.Minute), now.Add(-9*time.Minute))
	seedQueueRow(t, st, cleaner, queueID, "cancelled", now.Add(-3*time.Minute), time.Time{})

	got := countsFor(t, st, queueID)

	if len(got) != 1 {
		t.Fatalf("got %d lines for one unsplit queue, want 1: %v", len(got), got)
	}
	// An unsplit mode collapses to the single empty path.
	if waiting := got[queuewait.QueueKey{ModeQueueID: queueID}]; waiting != 2 {
		t.Errorf("got %d waiting, want 2 — matched and cancelled rows are not in the line", waiting)
	}
}

func TestCountWaitingByQueueIgnoresHowLongAPlayerHasWaited(t *testing.T) {
	st := openTestStore(t)
	cleaner := st.NewTestCleaner(t)
	queueID := seedModeQueue(t, st, "waiting-counts-unwindowed")

	// Joined days ago and still there. Depth is a level, not a rate: they are
	// still in front of everyone behind them, so no window may exclude them.
	seedQueueRow(t, st, cleaner, queueID, "waiting", time.Now().Add(-72*time.Hour), time.Time{})

	if waiting := countsFor(t, st, queueID)[queuewait.QueueKey{ModeQueueID: queueID}]; waiting != 1 {
		t.Errorf("got %d waiting, want 1 — a long wait is still a wait", waiting)
	}
}

func TestCountWaitingByQueueSplitsACompositionModeByRole(t *testing.T) {
	st := openTestStore(t)
	cleaner := st.NewTestCleaner(t)
	now := time.Now()
	queueID := seedModeQueue(t, st, "waiting-counts-paths")

	seedQueueRowOnPath(t, st, cleaner, queueID, "tank", "waiting", now.Add(-time.Minute), time.Time{})
	seedQueueRowOnPath(t, st, cleaner, queueID, "damage", "waiting", now.Add(-time.Minute), time.Time{})
	seedQueueRowOnPath(t, st, cleaner, queueID, "damage", "waiting", now.Add(-time.Minute), time.Time{})
	seedQueueRowOnPath(t, st, cleaner, queueID, "damage", "matched", now.Add(-time.Hour), now.Add(-time.Minute))

	got := countsFor(t, st, queueID)

	if len(got) != 2 {
		t.Fatalf("got %d lines, want tank and damage: %v", len(got), got)
	}
	if waiting := got[queuewait.QueueKey{ModeQueueID: queueID, QueuePath: "tank"}]; waiting != 1 {
		t.Errorf("got %d waiting for tank, want 1", waiting)
	}
	// The scarce role and the crowded one are the whole point of splitting: a
	// mode-wide "3 waiting" would tell a tank nothing about their own line.
	if waiting := got[queuewait.QueueKey{ModeQueueID: queueID, QueuePath: "damage"}]; waiting != 2 {
		t.Errorf("got %d waiting for damage, want 2", waiting)
	}
}

func TestCountWaitingByQueueLeavesAnEmptyQueueOut(t *testing.T) {
	st := openTestStore(t)
	queueID := seedModeQueue(t, st, "waiting-counts-empty")

	// Nobody has ever queued for this mode. The only rows an aggregate can
	// return are rows that exist, so the queue is simply absent — and callers
	// read that absence as the zero it is rather than as missing data.
	if got := countsFor(t, st, queueID); len(got) != 0 {
		t.Errorf("got %v for a queue nobody is waiting in, want it absent", got)
	}
}

func TestCountWaitingByQueueKeepsQueuesApart(t *testing.T) {
	st := openTestStore(t)
	cleaner := st.NewTestCleaner(t)
	now := time.Now()
	first := seedModeQueue(t, st, "waiting-counts-first")
	second := seedModeQueue(t, st, "waiting-counts-second")

	seedQueueRow(t, st, cleaner, first, "waiting", now.Add(-time.Minute), time.Time{})
	seedQueueRow(t, st, cleaner, second, "waiting", now.Add(-time.Minute), time.Time{})
	seedQueueRow(t, st, cleaner, second, "waiting", now.Add(-time.Minute), time.Time{})

	// One aggregate serves a whole page of cards, which is the point — but only
	// if each card reads its own number out of it.
	got := countsFor(t, st, first, second)

	if waiting := got[queuewait.QueueKey{ModeQueueID: first}]; waiting != 1 {
		t.Errorf("got %d waiting in the first queue, want 1", waiting)
	}
	if waiting := got[queuewait.QueueKey{ModeQueueID: second}]; waiting != 2 {
		t.Errorf("got %d waiting in the second queue, want 2", waiting)
	}
}

// The single-queue read stays in the tree for matchmaking and queue
// notifications, so the two must not be able to disagree.
func TestCountWaitingByQueueAgreesWithTheSingleQueueCount(t *testing.T) {
	st := openTestStore(t)
	cleaner := st.NewTestCleaner(t)
	now := time.Now()
	queueID := seedModeQueue(t, st, "waiting-counts-agree")

	seedQueueRowOnPath(t, st, cleaner, queueID, "tank", "waiting", now.Add(-time.Minute), time.Time{})
	seedQueueRowOnPath(t, st, cleaner, queueID, "damage", "waiting", now.Add(-time.Minute), time.Time{})
	seedQueueRow(t, st, cleaner, queueID, "waiting", now.Add(-time.Minute), time.Time{})

	one, err := st.CountWaitingInModeQueue(context.Background(), queueID)
	if err != nil {
		t.Fatalf("CountWaitingInModeQueue: %v", err)
	}

	total := 0
	for _, waiting := range countsFor(t, st, queueID) {
		total += waiting
	}

	if total != one {
		t.Errorf("the grouped aggregate totals %d and the single-queue count says %d — they describe the same players", total, one)
	}
}
