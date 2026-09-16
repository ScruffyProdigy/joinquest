package store

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
)

// Every age in this file is the gap between a timestamp the database wrote and the
// moment it is judged against. Measuring that gap here, in this process, reads one
// end off a different machine's clock -- so whatever the two disagree by is added
// to every window, and the answer flips as soon as the disagreement outgrows the
// margin. The fix is to subtract in SQL, where both ends come from one clock.
//
// A frozen NOW() is how these tests pull the two clocks apart without two machines.
// Inside one transaction the database's clock stands still while this process's
// keeps moving, so a row aged just inside its window by the database's reckoning is
// already outside it by this process's. Each test asserts the database's answer,
// and so fails on a correct clock rather than only on a skewed one.

// skewWindow is how far these tests push the two clocks apart. Comfortably longer
// than the scheduling jitter of a sleep, and short enough to keep the suite quick.
const skewWindow = 3 * time.Second

func TestStaleMatchedQueueAgeIsMeasuredByTheDatabaseClock(t *testing.T) {
	t.Setenv("LOBBY_STALE_MATCH_MINUTES", "1")
	window := staleMatchMaxAge()

	st := openTestStore(t)
	cleaner := st.NewTestCleaner(t)
	ctx := context.Background()
	resetDemoQueue(t, st, ctx)

	for _, label := range []string{"a", "b"} {
		user, err := st.CreateUser(ctx, CreateUserParams{Email: "match-clock-" + label + "-" + uuid.NewString() + "@example.com"})
		if err != nil {
			t.Fatalf("CreateUser %s: %v", label, err)
		}
		cleaner.TrackUser(user.ID)
		if _, err := st.JoinModeQueue(ctx, DemoDefaultQueueID, user.ID, "", nil); err != nil {
			t.Fatalf("join %s: %v", label, err)
		}
	}
	mustReconcileForming(t, st, ctx, DemoDefaultQueueID)

	var matchedUser uuid.UUID
	if err := st.db.QueryRowContext(ctx, `
		SELECT user_id FROM game_queues WHERE mode_queue_id = $1 AND status = 'matched' LIMIT 1
	`, DemoDefaultQueueID).Scan(&matchedUser); err != nil {
		t.Fatalf("find a matched row: %v — nothing to expire, so this test proves nothing", err)
	}

	tx, err := st.db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(ctx, `
		UPDATE game_queues SET matched_at = NOW() - make_interval(secs => $3)
		WHERE mode_queue_id = $1 AND user_id = $2 AND status = 'matched'
	`, DemoDefaultQueueID, matchedUser, (window - skewWindow).Seconds()); err != nil {
		t.Fatalf("age the match: %v", err)
	}

	time.Sleep(2 * skewWindow)

	if err := expireStaleMatchedModeQueue(ctx, tx, DemoDefaultQueueID, matchedUser); err != nil {
		t.Fatalf("expireStaleMatchedModeQueue: %v", err)
	}

	var status string
	if err := tx.QueryRowContext(ctx, `
		SELECT status FROM game_queues WHERE mode_queue_id = $1 AND user_id = $2
	`, DemoDefaultQueueID, matchedUser).Scan(&status); err != nil {
		t.Fatalf("read status back: %v", err)
	}
	if status != "matched" {
		t.Fatalf("a match still inside its window by the database's clock was expired (status %q); "+
			"a database whose clock differs from the app's would drop players out of a match they are still in", status)
	}
}

func TestStalePlayingSessionAgeIsMeasuredByTheDatabaseClock(t *testing.T) {
	t.Setenv("LOBBY_STALE_PLAYING_MINUTES", "1")
	window := stalePlayingMaxAge()

	st := openTestStore(t)
	cleaner := st.NewTestCleaner(t)
	ctx := context.Background()

	game, err := st.InsertTestGame(ctx, "Playing Clock Game")
	if err != nil {
		t.Fatalf("InsertTestGame: %v", err)
	}
	cleaner.TrackGame(game.ID)

	player, err := st.CreateUser(ctx, CreateUserParams{Email: "playing-clock-" + uuid.NewString() + "@example.com"})
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}
	cleaner.TrackUser(player.ID)

	tx, err := st.db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	defer tx.Rollback()

	// A session started just inside the window, by the clock that stamped it.
	var sessionID uuid.UUID
	if err := tx.QueryRowContext(ctx, `
		INSERT INTO game_sessions (game_id, status, started_at)
		VALUES ($1, 'active', NOW() - make_interval(secs => $2))
		RETURNING id
	`, game.ID, (window - skewWindow).Seconds()).Scan(&sessionID); err != nil {
		t.Fatalf("insert session: %v", err)
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO game_session_participants (session_id, user_id, role) VALUES ($1, $2, 'player')
	`, sessionID, player.ID); err != nil {
		t.Fatalf("insert participant: %v", err)
	}

	time.Sleep(2 * skewWindow)

	counts, err := countLivePlayersByGame(ctx, tx)
	if err != nil {
		t.Fatalf("countLivePlayersByGame: %v", err)
	}
	if got := counts[game.ID].Playing; got != 1 {
		t.Fatalf("playing count = %d, want 1; a session still inside its window by the database's clock was "+
			"written off as stale, so a database whose clock differs from the app's would empty out live catalog cards", got)
	}
}

func TestOldestQueueWaitIsMeasuredByTheDatabaseClock(t *testing.T) {
	st := openTestStore(t)
	cleaner := st.NewTestCleaner(t)
	ctx := context.Background()
	resetDemoQueue(t, st, ctx)

	waiter, err := st.CreateUser(ctx, CreateUserParams{Email: "wait-clock-" + uuid.NewString() + "@example.com"})
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}
	cleaner.TrackUser(waiter.ID)
	if _, err := st.JoinModeQueue(ctx, DemoDefaultQueueID, waiter.ID, "", nil); err != nil {
		t.Fatalf("join: %v", err)
	}

	tx, err := st.db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	defer tx.Rollback()

	const waited = 30 * time.Second
	if _, err := tx.ExecContext(ctx, `
		UPDATE game_queues SET joined_at = NOW() - make_interval(secs => $3)
		WHERE mode_queue_id = $1 AND user_id = $2 AND status = 'waiting'
	`, DemoDefaultQueueID, waiter.ID, waited.Seconds()); err != nil {
		t.Fatalf("age the wait: %v", err)
	}

	waiting, err := listWaitingModeQueueEntriesTx(ctx, tx, DemoDefaultQueueID)
	if err != nil {
		t.Fatalf("listWaitingModeQueueEntriesTx: %v", err)
	}
	if len(waiting) == 0 {
		t.Fatal("no waiting entries, so this test proves nothing")
	}

	time.Sleep(2 * skewWindow)

	oldest, err := oldestWaitTx(ctx, tx, waiting)
	if err != nil {
		t.Fatalf("oldestWaitTx: %v", err)
	}
	// The sleep is the whole margin: read off the database's clock the wait is still
	// `waited`, and off this process's it is `waited` plus the sleep.
	if oldest < waited-skewWindow || oldest > waited+skewWindow {
		t.Fatalf("oldest wait = %s, want about %s; the wait was measured against this process's clock, "+
			"so a database whose clock differs from the app's would misjudge every deferral budget", oldest, waited)
	}
}
