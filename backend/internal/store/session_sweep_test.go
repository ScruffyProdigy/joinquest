package store

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
)

// newQueuedUser creates a user and puts them in the demo mode queue.
func newQueuedUser(t *testing.T, st *Store, cleaner *TestCleaner, ctx context.Context, label string) *User {
	t.Helper()
	user, err := st.CreateUser(ctx, CreateUserParams{Email: label + "-" + uuid.NewString() + "@example.com"})
	if err != nil {
		t.Fatalf("CreateUser %s: %v", label, err)
	}
	cleaner.TrackUser(user.ID)
	if _, err := st.JoinModeQueue(ctx, DemoDefaultQueueID, user.ID, "", nil); err != nil {
		t.Fatalf("join %s: %v", label, err)
	}
	return user
}

// formDuel matches two fresh users on the demo queue and returns their session id.
func formDuel(t *testing.T, st *Store, cleaner *TestCleaner, ctx context.Context, label string) (uuid.UUID, *User, *User) {
	t.Helper()
	a := newQueuedUser(t, st, cleaner, ctx, label+"-a")
	b := newQueuedUser(t, st, cleaner, ctx, label+"-b")
	mustReconcileForming(t, st, ctx, DemoDefaultQueueID)

	view, err := st.GetUserActiveIntent(ctx, a.ID)
	if err != nil {
		t.Fatalf("GetUserActiveIntent(%s): %v", label, err)
	}
	if view == nil || !view.Matched || view.SessionID == nil {
		t.Fatalf("%s: expected a matched session", label)
	}
	return *view.SessionID, a, b
}

// backdateSession ages a session so the sweep's age guard no longer protects it.
func backdateSession(t *testing.T, st *Store, ctx context.Context, sessionID uuid.UUID, age time.Duration) {
	t.Helper()
	if _, err := st.db.ExecContext(ctx, `
		UPDATE game_sessions SET started_at = NOW() - $2::interval WHERE id = $1
	`, sessionID, pgInterval(age)); err != nil {
		t.Fatalf("backdate session: %v", err)
	}
}

func mustSessionStatus(t *testing.T, st *Store, ctx context.Context, sessionID uuid.UUID) string {
	t.Helper()
	session, err := st.GetSessionByID(ctx, sessionID)
	if err != nil {
		t.Fatalf("GetSessionByID(%s): %v", sessionID, err)
	}
	return session.Status
}

// isStaleCandidate reports whether the sweep currently considers sessionID stale.
//
// The sweep is fleet-wide and the test database is shared, so these tests assert on their
// own session rather than on the sweep's global counters, which other fixtures — several
// of which insert sessions with a backdated started_at — would otherwise perturb.
func isStaleCandidate(t *testing.T, st *Store, ctx context.Context, sessionID uuid.UUID, olderThan time.Duration) bool {
	t.Helper()
	ids, err := st.ListStaleSessions(ctx, olderThan)
	if err != nil {
		t.Fatalf("ListStaleSessions: %v", err)
	}
	for _, id := range ids {
		if id == sessionID {
			return true
		}
	}
	return false
}

func matchedQueueRowCount(t *testing.T, st *Store, ctx context.Context, userIDs ...uuid.UUID) int {
	t.Helper()
	total := 0
	for _, uid := range userIDs {
		var n int
		if err := st.db.QueryRowContext(ctx, `
			SELECT count(*) FROM game_queues WHERE user_id = $1 AND status = 'matched'
		`, uid).Scan(&n); err != nil {
			t.Fatalf("count matched rows for %s: %v", uid, err)
		}
		total += n
	}
	return total
}

// Two matches running at once on one mode queue is normal: mode_queues is catalog
// configuration, not a match. The sweep must not end one because the other is newer.
func TestSweepStaleSessionsKeepsConcurrentSessionsOnOneModeQueue(t *testing.T) {
	st := openTestStore(t)
	cleaner := st.NewTestCleaner(t)
	ctx := context.Background()

	older, _, _ := formDuel(t, st, cleaner, ctx, "concurrent-older")
	newer, _, _ := formDuel(t, st, cleaner, ctx, "concurrent-newer")

	if older == newer {
		t.Fatal("expected two distinct concurrent sessions on the same mode queue")
	}

	for name, id := range map[string]uuid.UUID{"older": older, "newer": newer} {
		if isStaleCandidate(t, st, ctx, id, DefaultStaleSessionAge) {
			t.Fatalf("%s session is a sweep candidate; neither is old enough", name)
		}
	}

	if _, err := st.SweepStaleSessions(ctx, DefaultStaleSessionAge); err != nil {
		t.Fatalf("SweepStaleSessions: %v", err)
	}

	for name, id := range map[string]uuid.UUID{"older": older, "newer": newer} {
		if status := mustSessionStatus(t, st, ctx, id); status != "active" {
			t.Fatalf("%s session status = %q, want active", name, status)
		}
	}
}

// The age guard is absolute: a young session is never a candidate, whatever else is
// active on its queue.
func TestSweepStaleSessionsSkipsSessionYoungerThanThreshold(t *testing.T) {
	st := openTestStore(t)
	cleaner := st.NewTestCleaner(t)
	ctx := context.Background()

	sessionID, _, _ := formDuel(t, st, cleaner, ctx, "young")
	backdateSession(t, st, ctx, sessionID, 30*time.Minute)

	if isStaleCandidate(t, st, ctx, sessionID, time.Hour) {
		t.Fatal("a 30m session is a candidate against a 1h threshold")
	}

	if _, err := st.SweepStaleSessions(ctx, time.Hour); err != nil {
		t.Fatalf("SweepStaleSessions: %v", err)
	}
	if status := mustSessionStatus(t, st, ctx, sessionID); status != "active" {
		t.Fatalf("session status = %q, want active", status)
	}
}

// A session past the threshold is genuinely stuck, and completing it must be the full
// CompleteSession behaviour — not a status-only UPDATE that orphans queue rows.
func TestSweepStaleSessionsCompletesAgedSessionAndReleasesQueueRows(t *testing.T) {
	st := openTestStore(t)
	cleaner := st.NewTestCleaner(t)
	ctx := context.Background()

	sessionID, userA, userB := formDuel(t, st, cleaner, ctx, "aged")
	backdateSession(t, st, ctx, sessionID, 7*time.Hour)

	if got := matchedQueueRowCount(t, st, ctx, userA.ID, userB.ID); got != 2 {
		t.Fatalf("matched queue rows before sweep = %d, want 2", got)
	}

	if !isStaleCandidate(t, st, ctx, sessionID, DefaultStaleSessionAge) {
		t.Fatal("a 7h session is not a candidate against the 6h threshold")
	}

	if _, err := st.SweepStaleSessions(ctx, DefaultStaleSessionAge); err != nil {
		t.Fatalf("SweepStaleSessions: %v", err)
	}

	if status := mustSessionStatus(t, st, ctx, sessionID); status != "completed" {
		t.Fatalf("session status = %q, want completed", status)
	}
	if isStaleCandidate(t, st, ctx, sessionID, DefaultStaleSessionAge) {
		t.Fatal("session is still a candidate after its own sweep")
	}
	if got := matchedQueueRowCount(t, st, ctx, userA.ID, userB.ID); got != 0 {
		t.Fatalf("matched queue rows after sweep = %d, want 0: the sweep orphaned them", got)
	}
}

// Sweeping an aged session must leave a concurrent young one on the same queue alone.
func TestSweepStaleSessionsCompletesOnlyTheAgedSessionOnAQueue(t *testing.T) {
	st := openTestStore(t)
	cleaner := st.NewTestCleaner(t)
	ctx := context.Background()

	aged, _, _ := formDuel(t, st, cleaner, ctx, "mixed-aged")
	live, liveA, liveB := formDuel(t, st, cleaner, ctx, "mixed-live")
	backdateSession(t, st, ctx, aged, 7*time.Hour)

	if _, err := st.SweepStaleSessions(ctx, DefaultStaleSessionAge); err != nil {
		t.Fatalf("SweepStaleSessions: %v", err)
	}

	if status := mustSessionStatus(t, st, ctx, aged); status != "completed" {
		t.Fatalf("aged session status = %q, want completed", status)
	}
	if status := mustSessionStatus(t, st, ctx, live); status != "active" {
		t.Fatalf("live session status = %q, want active", status)
	}
	if got := matchedQueueRowCount(t, st, ctx, liveA.ID, liveB.ID); got != 2 {
		t.Fatalf("live players' matched rows = %d, want 2", got)
	}
}

// The sweep is idempotent: a second pass finds nothing left to do.
func TestSweepStaleSessionsIsIdempotent(t *testing.T) {
	st := openTestStore(t)
	cleaner := st.NewTestCleaner(t)
	ctx := context.Background()

	sessionID, _, _ := formDuel(t, st, cleaner, ctx, "idempotent")
	backdateSession(t, st, ctx, sessionID, 7*time.Hour)

	if _, err := st.SweepStaleSessions(ctx, DefaultStaleSessionAge); err != nil {
		t.Fatalf("first SweepStaleSessions: %v", err)
	}
	if isStaleCandidate(t, st, ctx, sessionID, DefaultStaleSessionAge) {
		t.Fatal("session is still a candidate after the first sweep")
	}

	endedAfterFirst := mustSessionEndedAt(t, st, ctx, sessionID)
	if _, err := st.SweepStaleSessions(ctx, DefaultStaleSessionAge); err != nil {
		t.Fatalf("second SweepStaleSessions: %v", err)
	}
	if got := mustSessionEndedAt(t, st, ctx, sessionID); !got.Equal(endedAfterFirst) {
		t.Fatalf("ended_at moved from %v to %v: the second pass rewrote a completed session", endedAfterFirst, got)
	}
}

func mustSessionEndedAt(t *testing.T, st *Store, ctx context.Context, sessionID uuid.UUID) time.Time {
	t.Helper()
	session, err := st.GetSessionByID(ctx, sessionID)
	if err != nil {
		t.Fatalf("GetSessionByID(%s): %v", sessionID, err)
	}
	if session.EndedAt == nil {
		t.Fatalf("session %s has no ended_at", sessionID)
	}
	return *session.EndedAt
}

// Firing a new match must not touch other live sessions on the same queue. This was the
// in-process half of JQ-171: completePriorActiveSessionsForModeQueueTx ended them on every
// formation, status-only, orphaning the earlier players' matched queue rows.
func TestFireFormingMatchLeavesEarlierSessionOnSameQueueActive(t *testing.T) {
	st := openTestStore(t)
	cleaner := st.NewTestCleaner(t)
	ctx := context.Background()

	first, firstA, firstB := formDuel(t, st, cleaner, ctx, "formation-first")
	second, _, _ := formDuel(t, st, cleaner, ctx, "formation-second")

	if first == second {
		t.Fatal("expected the second formation to create its own session")
	}
	if status := mustSessionStatus(t, st, ctx, first); status != "active" {
		t.Fatalf("first session status = %q, want active: the second formation ended it", status)
	}
	if got := matchedQueueRowCount(t, st, ctx, firstA.ID, firstB.ID); got != 2 {
		t.Fatalf("first match players' matched rows = %d, want 2", got)
	}
}

// Re-queueing must finish the player's own stale session without ending it for a partner
// who is still in it. The old queue-wide sweep could not tell those apart (JQ-171).
func TestRequeueLeavesPartnersStillPlayingSessionActive(t *testing.T) {
	st := openTestStore(t)
	cleaner := st.NewTestCleaner(t)
	ctx := context.Background()

	sessionID, userA, userB := formDuel(t, st, cleaner, ctx, "partner")

	// A goes through the return hub and re-queues; B never returns and is still playing.
	if err := st.AcknowledgePlayerReturn(ctx, sessionID, userA.ID, time.Now()); err != nil {
		t.Fatalf("AcknowledgePlayerReturn A: %v", err)
	}
	if _, err := st.JoinModeQueue(ctx, DemoDefaultQueueID, userA.ID, "", nil); err != nil {
		t.Fatalf("re-queue A: %v", err)
	}

	if status := mustSessionStatus(t, st, ctx, sessionID); status != "active" {
		t.Fatalf("session status = %q, want active: B is still playing", status)
	}
	if got := matchedQueueRowCount(t, st, ctx, userB.ID); got != 1 {
		t.Fatalf("B's matched queue rows = %d, want 1: A's re-queue released them", got)
	}
}

// Once the last player leaves a session for a new queue, nothing is left in it, so it
// must be completed rather than lingering active until a sweep notices hours later.
func TestRequeueCompletesSessionOnceEveryPlayerHasLeft(t *testing.T) {
	st := openTestStore(t)
	cleaner := st.NewTestCleaner(t)
	ctx := context.Background()

	sessionID, userA, userB := formDuel(t, st, cleaner, ctx, "emptied")

	for _, u := range []*User{userA, userB} {
		if err := st.AcknowledgePlayerReturn(ctx, sessionID, u.ID, time.Now()); err != nil {
			t.Fatalf("AcknowledgePlayerReturn: %v", err)
		}
		if _, err := st.JoinModeQueue(ctx, DemoDefaultQueueID, u.ID, "", nil); err != nil {
			t.Fatalf("re-queue: %v", err)
		}
	}

	if status := mustSessionStatus(t, st, ctx, sessionID); status != "completed" {
		t.Fatalf("session status = %q, want completed: both players left it", status)
	}
}
