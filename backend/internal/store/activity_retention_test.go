package store

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
)

// seedActivityGame returns a tracked game for retention tests.
func seedActivityGame(t *testing.T, st *Store, cleaner *TestCleaner, ctx context.Context) uuid.UUID {
	t.Helper()
	game, err := st.InsertTestGame(ctx, "Activity Retention "+uuid.NewString()[:8])
	if err != nil {
		t.Fatalf("insert game: %v", err)
	}
	cleaner.TrackGame(game.ID)
	t.Cleanup(func() {
		_, _ = st.db.Exec(`DELETE FROM player_activity_events WHERE game_id = $1`, game.ID)
		_, _ = st.db.Exec(`DELETE FROM player_activity_daily WHERE game_id = $1`, game.ID)
		_, _ = st.db.Exec(`DELETE FROM player_first_match_summary WHERE game_id = $1`, game.ID)
	})
	return game.ID
}

// insertRawEvent writes straight to the table, bypassing the asynchronous writer, so
// a retention test can place an event at an arbitrary point in the past.
func insertRawEvent(t *testing.T, st *Store, ctx context.Context, eventType string, userID, gameID uuid.UUID, sessionID *uuid.UUID, at time.Time) {
	t.Helper()
	_, err := st.db.ExecContext(ctx, `
		INSERT INTO player_activity_events (event_type, source, user_id, game_id, session_id, occurred_at, payload)
		VALUES ($1, 'lobby', $2, $3, $4, $5, '{}'::jsonb)
	`, eventType, userID, gameID, sessionID, at)
	if err != nil {
		t.Fatalf("insert raw event: %v", err)
	}
}

// The property that matters most about the rollup: it is the only surviving record
// once the raw rows are swept, and nothing can rebuild it if a retry doubles it.
func TestRollUpActivityDailyIsIdempotent(t *testing.T) {
	st := openTestStore(t)
	cleaner := st.NewTestCleaner(t)
	ctx := context.Background()

	user := mergeTestUser(t, st, cleaner, ctx, "rollup")
	gameID := seedActivityGame(t, st, cleaner, ctx)
	day := time.Now().UTC().AddDate(0, 0, -3)

	for i := 0; i < 5; i++ {
		insertRawEvent(t, st, ctx, "queue_joined", user.ID, gameID, nil, day)
	}

	through := time.Now().UTC().AddDate(0, 0, -1)
	if _, err := st.RollUpActivityDaily(ctx, through); err != nil {
		t.Fatalf("first rollup: %v", err)
	}
	if _, err := st.RollUpActivityDaily(ctx, through); err != nil {
		t.Fatalf("second rollup: %v", err)
	}

	var count, distinct int64
	err := st.db.QueryRowContext(ctx, `
		SELECT event_count, distinct_users
		FROM player_activity_daily
		WHERE game_id = $1 AND event_type = 'queue_joined'
	`, gameID).Scan(&count, &distinct)
	if err != nil {
		t.Fatalf("reading the rollup: %v", err)
	}

	if count != 5 {
		t.Errorf("event_count = %d, want 5 -- a second rollup must not double it", count)
	}
	if distinct != 1 {
		t.Errorf("distinct_users = %d, want 1", distinct)
	}
}

// Today is still accumulating, so folding it up would store a number that a later run
// has to correct, and anyone reading in between sees a partial day presented as a
// whole one.
func TestRollUpActivityDailyStopsAtThrough(t *testing.T) {
	st := openTestStore(t)
	cleaner := st.NewTestCleaner(t)
	ctx := context.Background()

	user := mergeTestUser(t, st, cleaner, ctx, "rolluptoday")
	gameID := seedActivityGame(t, st, cleaner, ctx)

	insertRawEvent(t, st, ctx, "queue_joined", user.ID, gameID, nil, time.Now().UTC())

	if _, err := st.RollUpActivityDaily(ctx, time.Now().UTC().AddDate(0, 0, -1)); err != nil {
		t.Fatalf("rollup: %v", err)
	}

	var rows int
	if err := st.db.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM player_activity_daily WHERE game_id = $1
	`, gameID).Scan(&rows); err != nil {
		t.Fatalf("counting rollup rows: %v", err)
	}
	if rows != 0 {
		t.Errorf("rolled up %d rows for today, want 0", rows)
	}
}

// A player's first match is a fact about the past. Once the raw event behind it has
// been swept, the view stops reporting it -- and the summary must keep what it knew
// rather than promoting their second match to first, which would silently understate
// how long they had been bouncing off the game.
func TestRefreshFirstMatchSummaryNeverMovesFirstMatchForward(t *testing.T) {
	st := openTestStore(t)
	cleaner := st.NewTestCleaner(t)
	ctx := context.Background()

	user := mergeTestUser(t, st, cleaner, ctx, "firstmatchkeep")
	gameID := seedActivityGame(t, st, cleaner, ctx)

	firstSession := uuid.New()
	first := time.Now().UTC().AddDate(0, 0, -200)
	second := time.Now().UTC().AddDate(0, 0, -10)

	insertRawEvent(t, st, ctx, "match_started", user.ID, gameID, &firstSession, first)
	if _, err := st.RefreshFirstMatchSummary(ctx); err != nil {
		t.Fatalf("first refresh: %v", err)
	}

	// The 200-day-old event now falls outside the window and is swept.
	if _, _, err := st.DeleteActivityEventsOlderThan(ctx, time.Now().UTC().AddDate(0, 0, -180), 100); err != nil {
		t.Fatalf("sweep: %v", err)
	}

	laterSession := uuid.New()
	insertRawEvent(t, st, ctx, "match_started", user.ID, gameID, &laterSession, second)
	if _, err := st.RefreshFirstMatchSummary(ctx); err != nil {
		t.Fatalf("second refresh: %v", err)
	}

	var storedFirst time.Time
	err := st.db.QueryRowContext(ctx, `
		SELECT first_started_at FROM player_first_match_summary
		WHERE user_id = $1 AND game_id = $2
	`, user.ID, gameID).Scan(&storedFirst)
	if err != nil {
		t.Fatalf("reading the summary: %v", err)
	}

	if !storedFirst.UTC().Truncate(time.Second).Equal(first.Truncate(time.Second)) {
		t.Errorf("first_started_at = %v, want the original %v -- the first match moved forward", storedFirst.UTC(), first)
	}
}

func TestDeleteActivityEventsOlderThanKeepsEventsInsideTheWindow(t *testing.T) {
	st := openTestStore(t)
	cleaner := st.NewTestCleaner(t)
	ctx := context.Background()

	user := mergeTestUser(t, st, cleaner, ctx, "retention")
	gameID := seedActivityGame(t, st, cleaner, ctx)

	insertRawEvent(t, st, ctx, "queue_joined", user.ID, gameID, nil, time.Now().UTC().AddDate(0, 0, -200))
	insertRawEvent(t, st, ctx, "queue_joined", user.ID, gameID, nil, time.Now().UTC().AddDate(0, 0, -5))

	cutoff := time.Now().UTC().Add(-DefaultActivityRetention)

	expired, err := st.CountActivityEventsOlderThan(ctx, cutoff)
	if err != nil {
		t.Fatalf("count: %v", err)
	}
	if expired != 1 {
		t.Fatalf("expired = %d, want 1", expired)
	}

	deleted, more, err := st.DeleteActivityEventsOlderThan(ctx, cutoff, 100)
	if err != nil {
		t.Fatalf("delete: %v", err)
	}
	if deleted != 1 {
		t.Errorf("deleted = %d, want 1", deleted)
	}
	if more {
		t.Error("more = true, but the batch was not full")
	}

	var remaining int
	if err := st.db.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM player_activity_events WHERE game_id = $1
	`, gameID).Scan(&remaining); err != nil {
		t.Fatalf("counting remaining: %v", err)
	}
	if remaining != 1 {
		t.Errorf("%d events remain, want 1 -- the recent one must survive", remaining)
	}
}

// A full batch reports that more remain, so the sweep's loop keeps going rather than
// stopping halfway through a backlog.
func TestDeleteActivityEventsOlderThanReportsMoreOnAFullBatch(t *testing.T) {
	st := openTestStore(t)
	cleaner := st.NewTestCleaner(t)
	ctx := context.Background()

	user := mergeTestUser(t, st, cleaner, ctx, "retentionbatch")
	gameID := seedActivityGame(t, st, cleaner, ctx)

	old := time.Now().UTC().AddDate(0, 0, -200)
	for i := 0; i < 5; i++ {
		insertRawEvent(t, st, ctx, "queue_joined", user.ID, gameID, nil, old)
	}

	cutoff := time.Now().UTC().Add(-DefaultActivityRetention)
	deleted, more, err := st.DeleteActivityEventsOlderThan(ctx, cutoff, 2)
	if err != nil {
		t.Fatalf("delete: %v", err)
	}
	if deleted != 2 {
		t.Errorf("deleted = %d, want 2", deleted)
	}
	if !more {
		t.Error("more = false, but a full batch was deleted and three rows remain")
	}
}

// The guest is by definition the earlier encounter with the game, so a merge must
// keep the guest's first match, not the account's later one.
func TestMergeUserIntoKeepsTheEarlierFirstMatch(t *testing.T) {
	st := openTestStore(t)
	cleaner := st.NewTestCleaner(t)
	ctx := context.Background()

	guest := mergeTestUser(t, st, cleaner, ctx, "firstguest")
	account := mergeTestUser(t, st, cleaner, ctx, "firstaccount")
	gameID := seedActivityGame(t, st, cleaner, ctx)

	guestFirst := time.Now().UTC().AddDate(0, 0, -30)
	accountFirst := time.Now().UTC().AddDate(0, 0, -2)

	insertRawEvent(t, st, ctx, "match_started", guest.ID, gameID, nil, guestFirst)
	insertRawEvent(t, st, ctx, "match_started", account.ID, gameID, nil, accountFirst)
	if _, err := st.RefreshFirstMatchSummary(ctx); err != nil {
		t.Fatalf("refresh: %v", err)
	}

	if err := st.MergeUserInto(ctx, guest.ID, account.ID); err != nil {
		t.Fatalf("MergeUserInto: %v", err)
	}

	var storedFirst time.Time
	err := st.db.QueryRowContext(ctx, `
		SELECT first_started_at FROM player_first_match_summary
		WHERE user_id = $1 AND game_id = $2
	`, account.ID, gameID).Scan(&storedFirst)
	if err != nil {
		t.Fatalf("reading the surviving summary: %v", err)
	}
	if !storedFirst.UTC().Truncate(time.Second).Equal(guestFirst.Truncate(time.Second)) {
		t.Errorf("first_started_at = %v, want the guest's %v", storedFirst.UTC(), guestFirst)
	}

	var guestRows int
	if err := st.db.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM player_first_match_summary WHERE user_id = $1
	`, guest.ID).Scan(&guestRows); err != nil {
		t.Fatalf("counting the guest's rows: %v", err)
	}
	if guestRows != 0 {
		t.Errorf("%d summary rows stayed on the merged-away guest", guestRows)
	}
}

// The equal case: the first match is already summarised, and the game reports its
// outcome afterwards. The refresh has to pick that up, or a finish reason arriving
// after the first sweep would never reach the row that outlives the raw events.
func TestRefreshFirstMatchSummaryPicksUpALaterReportedOutcome(t *testing.T) {
	st := openTestStore(t)
	cleaner := st.NewTestCleaner(t)
	ctx := context.Background()

	user := mergeTestUser(t, st, cleaner, ctx, "firstmatchoutcome")
	gameID := seedActivityGame(t, st, cleaner, ctx)

	sessionID := uuid.New()
	started := time.Now().UTC().AddDate(0, 0, -3)

	insertRawEvent(t, st, ctx, "match_started", user.ID, gameID, &sessionID, started)
	if _, err := st.RefreshFirstMatchSummary(ctx); err != nil {
		t.Fatalf("first refresh: %v", err)
	}

	var observed bool
	if err := st.db.QueryRowContext(ctx, `
		SELECT first_outcome_observed FROM player_first_match_summary
		WHERE user_id = $1 AND game_id = $2
	`, user.ID, gameID).Scan(&observed); err != nil {
		t.Fatalf("reading the summary: %v", err)
	}
	if observed {
		t.Fatal("first_outcome_observed = true before the game reported anything")
	}

	insertRawEvent(t, st, ctx, "match_finished", user.ID, gameID, &sessionID, started.Add(time.Minute))
	if _, err := st.db.ExecContext(ctx, `
		UPDATE player_activity_events SET payload = '{"reason":"FORFEIT"}'::jsonb
		WHERE event_type = 'match_finished' AND session_id = $1
	`, sessionID); err != nil {
		t.Fatalf("setting the finish reason: %v", err)
	}
	if _, err := st.RefreshFirstMatchSummary(ctx); err != nil {
		t.Fatalf("second refresh: %v", err)
	}

	var reason *string
	if err := st.db.QueryRowContext(ctx, `
		SELECT first_outcome_observed, first_finish_reason FROM player_first_match_summary
		WHERE user_id = $1 AND game_id = $2
	`, user.ID, gameID).Scan(&observed, &reason); err != nil {
		t.Fatalf("re-reading the summary: %v", err)
	}
	if !observed {
		t.Error("first_outcome_observed = false after the game reported a finish")
	}
	if reason == nil || *reason != "FORFEIT" {
		t.Errorf("first_finish_reason = %v, want FORFEIT", reason)
	}
}

// The later case, which is the one that is easy to get wrong: after the raw event
// behind a stored first match is swept, the view reports a DIFFERENT, later match as
// that player's first. Its finish reason must not be attached to the original.
func TestRefreshFirstMatchSummaryDoesNotAdoptALaterMatchsOutcome(t *testing.T) {
	st := openTestStore(t)
	cleaner := st.NewTestCleaner(t)
	ctx := context.Background()

	user := mergeTestUser(t, st, cleaner, ctx, "firstmatchnoadopt")
	gameID := seedActivityGame(t, st, cleaner, ctx)

	oldSession := uuid.New()
	oldStart := time.Now().UTC().AddDate(0, 0, -200)
	insertRawEvent(t, st, ctx, "match_started", user.ID, gameID, &oldSession, oldStart)
	if _, err := st.RefreshFirstMatchSummary(ctx); err != nil {
		t.Fatalf("first refresh: %v", err)
	}

	if _, _, err := st.DeleteActivityEventsOlderThan(ctx, time.Now().UTC().AddDate(0, 0, -180), 100); err != nil {
		t.Fatalf("sweep: %v", err)
	}

	// A later match, which the game DID report on.
	newSession := uuid.New()
	newStart := time.Now().UTC().AddDate(0, 0, -2)
	insertRawEvent(t, st, ctx, "match_started", user.ID, gameID, &newSession, newStart)
	insertRawEvent(t, st, ctx, "match_finished", user.ID, gameID, &newSession, newStart.Add(time.Minute))
	if _, err := st.db.ExecContext(ctx, `
		UPDATE player_activity_events SET payload = '{"reason":"DISCONNECT"}'::jsonb
		WHERE event_type = 'match_finished' AND session_id = $1
	`, newSession); err != nil {
		t.Fatalf("setting the finish reason: %v", err)
	}
	if _, err := st.RefreshFirstMatchSummary(ctx); err != nil {
		t.Fatalf("second refresh: %v", err)
	}

	var (
		storedFirst time.Time
		observed    bool
		reason      *string
	)
	err := st.db.QueryRowContext(ctx, `
		SELECT first_started_at, first_outcome_observed, first_finish_reason
		FROM player_first_match_summary
		WHERE user_id = $1 AND game_id = $2
	`, user.ID, gameID).Scan(&storedFirst, &observed, &reason)
	if err != nil {
		t.Fatalf("reading the summary: %v", err)
	}

	if !storedFirst.UTC().Truncate(time.Second).Equal(oldStart.Truncate(time.Second)) {
		t.Errorf("first_started_at = %v, want the original %v", storedFirst.UTC(), oldStart)
	}
	if observed {
		t.Error("first_outcome_observed = true -- the later match's outcome was adopted by the first")
	}
	if reason != nil {
		t.Errorf("first_finish_reason = %v, want NULL -- DISCONNECT belongs to a different match", *reason)
	}
}
