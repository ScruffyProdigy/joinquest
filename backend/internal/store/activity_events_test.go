package store

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/scruffyprodigy/joinquest/internal/activity"
)

// openTestStoreRecording returns a Store wired to a real writer, plus a flush function
// that drains it. Flushing is explicit because recording is asynchronous by design --
// a test that asserted without draining would be racing the writer, and would fail
// only occasionally, which is the worst way for a test to fail.
func openTestStoreRecording(t *testing.T) (*Store, func()) {
	t.Helper()

	st := openTestStore(t)
	writer := activity.NewWriter(st.db)
	st.WithActivity(writer)

	flushed := false
	flush := func() {
		if flushed {
			return
		}
		flushed = true
		if err := writer.Close(); err != nil {
			t.Fatalf("flushing activity writer: %v", err)
		}
	}
	t.Cleanup(flush)
	return st, flush
}

// trackEventsFor removes the events a test produced. player_activity_events carries no
// foreign keys -- deliberately, so instrumentation can never block or fail a write
// elsewhere -- so TestCleaner does not reach it and tests clean up after themselves.
func trackEventsFor(t *testing.T, st *Store, userIDs ...uuid.UUID) {
	t.Helper()
	t.Cleanup(func() {
		for _, id := range userIDs {
			_, _ = st.db.Exec(`DELETE FROM player_activity_events WHERE user_id = $1`, id)
		}
	})
}

func countEvents(t *testing.T, st *Store, ctx context.Context, eventType string, userID uuid.UUID) int {
	t.Helper()
	var n int
	err := st.db.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM player_activity_events WHERE event_type = $1 AND user_id = $2
	`, eventType, userID).Scan(&n)
	if err != nil {
		t.Fatalf("counting %s events: %v", eventType, err)
	}
	return n
}

// The happy path end to end: two players queue, a match forms, and the funnel's first
// two steps are on disk against the right game.
func TestMatchmakingRecordsJoinAndStart(t *testing.T) {
	st, flush := openTestStoreRecording(t)
	cleaner := st.NewTestCleaner(t)
	ctx := context.Background()

	one := mergeTestUser(t, st, cleaner, ctx, "activityone")
	two := mergeTestUser(t, st, cleaner, ctx, "activitytwo")
	trackEventsFor(t, st, one.ID, two.ID)

	queueID := DemoDefaultQueueID
	if _, err := st.JoinModeQueue(ctx, queueID, one.ID, "", nil); err != nil {
		t.Fatalf("join one: %v", err)
	}
	if _, err := st.JoinModeQueue(ctx, queueID, two.ID, "", nil); err != nil {
		t.Fatalf("join two: %v", err)
	}
	mustReconcileForming(t, st, ctx, queueID)
	flush()

	if got := countEvents(t, st, ctx, activity.EventQueueJoined, one.ID); got != 1 {
		t.Errorf("queue_joined for player one = %d, want 1", got)
	}
	if got := countEvents(t, st, ctx, activity.EventMatchStarted, one.ID); got != 1 {
		t.Errorf("match_started for player one = %d, want 1", got)
	}
	if got := countEvents(t, st, ctx, activity.EventMatchStarted, two.ID); got != 1 {
		t.Errorf("match_started for player two = %d, want 1", got)
	}

	// Every event has to land on a game, or it never appears in that game's playtest
	// summary and the funnel silently under-reports.
	var missingGame int
	err := st.db.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM player_activity_events
		WHERE user_id IN ($1, $2) AND game_id IS NULL
	`, one.ID, two.ID).Scan(&missingGame)
	if err != nil {
		t.Fatalf("counting events without a game: %v", err)
	}
	if missingGame != 0 {
		t.Errorf("%d events landed with no game_id", missingGame)
	}
}

// A player who leaves before any match forms is a distinct and interesting signal:
// they looked at the wait and decided against it.
func TestLeaveModeQueueRecordsAbandon(t *testing.T) {
	st, flush := openTestStoreRecording(t)
	cleaner := st.NewTestCleaner(t)
	ctx := context.Background()

	user := mergeTestUser(t, st, cleaner, ctx, "activityleaver")
	trackEventsFor(t, st, user.ID)

	queueID := DemoDefaultQueueID
	if _, err := st.JoinModeQueue(ctx, queueID, user.ID, "", nil); err != nil {
		t.Fatalf("join: %v", err)
	}
	if _, err := st.LeaveModeQueue(ctx, queueID, user.ID); err != nil {
		t.Fatalf("leave: %v", err)
	}
	flush()

	if got := countEvents(t, st, ctx, activity.EventQueueAbandoned, user.ID); got != 1 {
		t.Fatalf("queue_abandoned = %d, want 1", got)
	}

	var reason string
	var gameID *uuid.UUID
	err := st.db.QueryRowContext(ctx, `
		SELECT payload->>'reason', game_id
		FROM player_activity_events
		WHERE event_type = $1 AND user_id = $2
	`, activity.EventQueueAbandoned, user.ID).Scan(&reason, &gameID)
	if err != nil {
		t.Fatalf("reading abandon event: %v", err)
	}
	// "left" and "evicted" must stay distinguishable: one is a decision about the
	// game, the other may be nothing but a bad connection.
	if reason != "left" {
		t.Errorf("reason = %q, want left", reason)
	}
	if gameID == nil {
		t.Error("abandon event has no game_id; it will not appear in the game's summary")
	}
}

// Leaving a queue twice records one abandonment. The second call removes nothing, and
// counting it would inflate the signal against a player who clicked twice.
func TestLeaveModeQueueTwiceRecordsOneAbandon(t *testing.T) {
	st, flush := openTestStoreRecording(t)
	cleaner := st.NewTestCleaner(t)
	ctx := context.Background()

	user := mergeTestUser(t, st, cleaner, ctx, "activitydoubleleaver")
	trackEventsFor(t, st, user.ID)

	queueID := DemoDefaultQueueID
	if _, err := st.JoinModeQueue(ctx, queueID, user.ID, "", nil); err != nil {
		t.Fatalf("join: %v", err)
	}
	if _, err := st.LeaveModeQueue(ctx, queueID, user.ID); err != nil {
		t.Fatalf("first leave: %v", err)
	}
	if _, err := st.LeaveModeQueue(ctx, queueID, user.ID); err != nil {
		t.Fatalf("second leave: %v", err)
	}
	flush()

	if got := countEvents(t, st, ctx, activity.EventQueueAbandoned, user.ID); got != 1 {
		t.Errorf("queue_abandoned = %d, want 1", got)
	}
}

// The acceptance criterion: a guest's activity survives account creation rather than
// fragmenting across throwaway identities. Nearly every player arrives as a guest, so
// without this a game's floor would be measured against accounts that each appear to
// have tried one game once.
func TestMergeUserIntoCarriesActivityEvents(t *testing.T) {
	st, flush := openTestStoreRecording(t)
	cleaner := st.NewTestCleaner(t)
	ctx := context.Background()

	guest := mergeTestUser(t, st, cleaner, ctx, "activityguest")
	account := mergeTestUser(t, st, cleaner, ctx, "activityaccount")
	trackEventsFor(t, st, guest.ID, account.ID)

	queueID := DemoDefaultQueueID
	if _, err := st.JoinModeQueue(ctx, queueID, guest.ID, "", nil); err != nil {
		t.Fatalf("guest join: %v", err)
	}
	if _, err := st.LeaveModeQueue(ctx, queueID, guest.ID); err != nil {
		t.Fatalf("guest leave: %v", err)
	}
	flush()

	before := countEvents(t, st, ctx, activity.EventQueueJoined, guest.ID) +
		countEvents(t, st, ctx, activity.EventQueueAbandoned, guest.ID)
	if before == 0 {
		t.Fatal("the guest recorded no events; the rest of this test proves nothing")
	}

	if err := st.MergeUserInto(ctx, guest.ID, account.ID); err != nil {
		t.Fatalf("MergeUserInto: %v", err)
	}

	var left int
	if err := st.db.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM player_activity_events WHERE user_id = $1
	`, guest.ID).Scan(&left); err != nil {
		t.Fatalf("counting the guest's remaining events: %v", err)
	}
	if left != 0 {
		t.Errorf("%d events stayed on the merged-away guest", left)
	}

	after := countEvents(t, st, ctx, activity.EventQueueJoined, account.ID) +
		countEvents(t, st, ctx, activity.EventQueueAbandoned, account.ID)
	if after != before {
		t.Errorf("the surviving account holds %d of the guest's %d events", after, before)
	}
}

// The honesty requirement, which is the part of this ticket easiest to get wrong. A
// match the game never reported on has an UNKNOWN outcome -- not a failed one and not
// a completed one. If this ever starts inferring, the platform's blind spots become
// invisible exactly when someone is deciding whether a game is hard to get into.
func TestGamePlaytestSummaryCountsUnreportedMatchesAsUnknown(t *testing.T) {
	st, flush := openTestStoreRecording(t)
	cleaner := st.NewTestCleaner(t)
	ctx := context.Background()

	user := mergeTestUser(t, st, cleaner, ctx, "activityunknown")
	trackEventsFor(t, st, user.ID)

	game, err := st.InsertTestGame(ctx, "Activity Unknown "+uuid.NewString()[:8])
	if err != nil {
		t.Fatalf("insert game: %v", err)
	}
	cleaner.TrackGame(game.ID)

	sessionID := uuid.New()
	st.recordMatchStarted(game.ID, sessionID, time.Now(), []uuid.UUID{user.ID}, "test")
	flush()

	var (
		starts  int
		unknown int
	)
	err = st.db.QueryRowContext(ctx, `
		SELECT match_starts, match_starts_outcome_unknown
		FROM game_playtest_summary
		WHERE game_id = $1
	`, game.ID).Scan(&starts, &unknown)
	if err != nil {
		t.Fatalf("reading the playtest summary: %v", err)
	}

	if starts != 1 {
		t.Errorf("match_starts = %d, want 1", starts)
	}
	if unknown != 1 {
		t.Errorf("match_starts_outcome_unknown = %d, want 1 -- an unreported match must stay unknown", unknown)
	}
}

// The floor hypothesis the ticket is built on: a player's first match of a game, how
// it ended, and whether they ever came back.
func TestPlayerActivityFirstMatchTracksReturn(t *testing.T) {
	st, flush := openTestStoreRecording(t)
	cleaner := st.NewTestCleaner(t)
	ctx := context.Background()

	user := mergeTestUser(t, st, cleaner, ctx, "activityfirst")
	trackEventsFor(t, st, user.ID)

	game, err := st.InsertTestGame(ctx, "Activity First "+uuid.NewString()[:8])
	if err != nil {
		t.Fatalf("insert game: %v", err)
	}
	cleaner.TrackGame(game.ID)

	firstSession := uuid.New()
	first := time.Now().Add(-2 * time.Hour)
	st.recordMatchStarted(game.ID, firstSession, first, []uuid.UUID{user.ID}, "test")
	st.recordMatchFinished(firstSession, user.ID, "FORFEIT", nil)
	st.recordMatchStarted(game.ID, uuid.New(), first.Add(90*time.Minute), []uuid.UUID{user.ID}, "test")
	flush()

	var (
		outcomeObserved bool
		reason          *string
		secondStarted   *time.Time
		gapSeconds      *float64
	)
	err = st.db.QueryRowContext(ctx, `
		SELECT first_outcome_observed, first_finish_reason, second_started_at,
		       EXTRACT(EPOCH FROM gap_to_second_match)
		FROM player_activity_first_match
		WHERE user_id = $1 AND game_id = $2
	`, user.ID, game.ID).Scan(&outcomeObserved, &reason, &secondStarted, &gapSeconds)
	if err != nil {
		t.Fatalf("reading the first-match view: %v", err)
	}
	if !outcomeObserved {
		t.Error("first_outcome_observed = false, want true -- the game reported a finish")
	}
	if reason == nil || *reason != "FORFEIT" {
		t.Errorf("first_finish_reason = %v, want FORFEIT", reason)
	}
	if secondStarted == nil {
		t.Fatal("second_started_at is NULL, but the player started a second match")
	}
	if gapSeconds == nil || *gapSeconds < 5000 || *gapSeconds > 5800 {
		t.Errorf("gap_to_second_match = %v seconds, want about 5400", gapSeconds)
	}
}
