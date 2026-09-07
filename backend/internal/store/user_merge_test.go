package store

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
)

// mergeTestUser creates a tracked user with a unique email.
func mergeTestUser(t *testing.T, st *Store, cleaner *TestCleaner, ctx context.Context, label string) *User {
	t.Helper()
	email := "merge-" + label + "-" + uuid.NewString() + "@example.com"
	user, err := st.CreateUser(ctx, CreateUserParams{Email: email, DisplayName: label})
	if err != nil {
		t.Fatalf("create %s user: %v", label, err)
	}
	cleaner.TrackUser(user.ID)
	cleaner.TrackEmail(email)
	return user
}

// countRows is a small assertion helper for the tables the merge has to carry.
func countRows(t *testing.T, st *Store, ctx context.Context, query string, args ...any) int {
	t.Helper()
	var n int
	if err := st.db.QueryRowContext(ctx, query, args...).Scan(&n); err != nil {
		t.Fatalf("count query %q: %v", query, err)
	}
	return n
}

// playCompletedMatch puts userID through a full match and returns the session id.
func playCompletedMatch(t *testing.T, st *Store, cleaner *TestCleaner, ctx context.Context, userID uuid.UUID) uuid.UUID {
	t.Helper()
	game, err := st.InsertTestGame(ctx, "Merge History Game "+uuid.NewString()[:8])
	if err != nil {
		t.Fatalf("insert game: %v", err)
	}
	cleaner.TrackGame(game.ID)

	session, err := st.CreateSession(ctx, game.ID)
	if err != nil {
		t.Fatalf("create session: %v", err)
	}
	if err := st.AddSessionParticipant(ctx, session.ID, userID, "player"); err != nil {
		t.Fatalf("add participant: %v", err)
	}
	if err := st.MarkParticipantFinished(ctx, session.ID, userID, time.Now()); err != nil {
		t.Fatalf("mark finished: %v", err)
	}
	if err := st.CompleteSession(ctx, session.ID, time.Now()); err != nil {
		t.Fatalf("complete session: %v", err)
	}
	return session.ID
}

// The acceptance-criteria case: a guest plays a match, merges into an account, and the
// match is still associated with the surviving user.
func TestMergeUserIntoCarriesCompletedMatchHistory(t *testing.T) {
	st := openTestStore(t)
	cleaner := st.NewTestCleaner(t)
	ctx := context.Background()

	source := mergeTestUser(t, st, cleaner, ctx, "source")
	target := mergeTestUser(t, st, cleaner, ctx, "target")
	sessionID := playCompletedMatch(t, st, cleaner, ctx, source.ID)

	if err := st.MergeUserInto(ctx, source.ID, target.ID); err != nil {
		t.Fatalf("MergeUserInto: %v", err)
	}

	onTarget := countRows(t, st, ctx,
		`SELECT COUNT(*) FROM game_session_participants WHERE session_id = $1 AND user_id = $2`,
		sessionID, target.ID)
	if onTarget != 1 {
		t.Fatalf("participation rows on target = %d, want 1", onTarget)
	}
	onSource := countRows(t, st, ctx,
		`SELECT COUNT(*) FROM game_session_participants WHERE user_id = $1`, source.ID)
	if onSource != 0 {
		t.Fatalf("participation rows left on source = %d, want 0", onSource)
	}
}

// Both accounts hold a waiting queue row. idx_game_queues_one_waiting_per_user is global,
// so a naive UPDATE ... SET user_id = target violates it. The target's live row survives
// and the source's is carried across as cancelled history.
func TestMergeUserIntoResolvesCompetingWaitingQueues(t *testing.T) {
	st := openTestStore(t)
	cleaner := st.NewTestCleaner(t)
	ctx := context.Background()

	source := mergeTestUser(t, st, cleaner, ctx, "qsource")
	target := mergeTestUser(t, st, cleaner, ctx, "qtarget")

	// A dedicated game, not the shared demo queue: two waiting players there would trip
	// matchmaking and strand state other tests read. The subject here is the global
	// one-waiting-row-per-user index, which direct inserts exercise exactly.
	game, err := st.InsertTestGame(ctx, "Merge Queue Game "+uuid.NewString()[:8])
	if err != nil {
		t.Fatalf("insert game: %v", err)
	}
	cleaner.TrackGame(game.ID)
	for _, userID := range []uuid.UUID{source.ID, target.ID} {
		if _, err := st.db.ExecContext(ctx, `
			INSERT INTO game_queues (game_id, user_id, status) VALUES ($1, $2, 'waiting')
		`, game.ID, userID); err != nil {
			t.Fatalf("insert waiting queue row: %v", err)
		}
	}

	if err := st.MergeUserInto(ctx, source.ID, target.ID); err != nil {
		t.Fatalf("MergeUserInto: %v", err)
	}

	waiting := countRows(t, st, ctx,
		`SELECT COUNT(*) FROM game_queues WHERE user_id = $1 AND status = 'waiting'`, target.ID)
	if waiting != 1 {
		t.Fatalf("waiting queue rows on target = %d, want 1", waiting)
	}
	total := countRows(t, st, ctx, `SELECT COUNT(*) FROM game_queues WHERE user_id = $1`, target.ID)
	if total != 2 {
		t.Fatalf("total queue rows on target = %d, want 2 (one live, one carried history)", total)
	}
	left := countRows(t, st, ctx, `SELECT COUNT(*) FROM game_queues WHERE user_id = $1`, source.ID)
	if left != 0 {
		t.Fatalf("queue rows left on source = %d, want 0", left)
	}
}

// A source mid-live-session resolves to finished history rather than handing the target a
// live seat the game server minted against the source user id.
func TestMergeUserIntoFinishesSourceLiveSession(t *testing.T) {
	st := openTestStore(t)
	cleaner := st.NewTestCleaner(t)
	ctx := context.Background()

	source := mergeTestUser(t, st, cleaner, ctx, "livesource")
	target := mergeTestUser(t, st, cleaner, ctx, "livetarget")

	game, err := st.InsertTestGame(ctx, "Merge Live Game "+uuid.NewString()[:8])
	if err != nil {
		t.Fatalf("insert game: %v", err)
	}
	cleaner.TrackGame(game.ID)
	session, err := st.CreateSession(ctx, game.ID)
	if err != nil {
		t.Fatalf("create session: %v", err)
	}
	if err := st.AddSessionParticipant(ctx, session.ID, source.ID, "player"); err != nil {
		t.Fatalf("add participant: %v", err)
	}

	if err := st.MergeUserInto(ctx, source.ID, target.ID); err != nil {
		t.Fatalf("MergeUserInto: %v", err)
	}

	finished := countRows(t, st, ctx,
		`SELECT COUNT(*) FROM game_session_participants
		 WHERE session_id = $1 AND user_id = $2 AND finished_at IS NOT NULL`,
		session.ID, target.ID)
	if finished != 1 {
		t.Fatalf("finished participation rows on target = %d, want 1", finished)
	}

	live, err := st.GetUserActiveSessionParticipation(ctx, target.ID)
	if err != nil {
		t.Fatalf("GetUserActiveSessionParticipation: %v", err)
	}
	if live != nil {
		t.Fatalf("target inherited a live session %s, want none", live.SessionID)
	}
}

// Chat history follows the player; the source's room seat is released rather than moved.
func TestMergeUserIntoCarriesRoomMessagesAndReleasesSeat(t *testing.T) {
	st := openTestStore(t)
	cleaner := st.NewTestCleaner(t)
	ctx := context.Background()

	source := mergeTestUser(t, st, cleaner, ctx, "roomsource")
	target := mergeTestUser(t, st, cleaner, ctx, "roomtarget")

	room, err := st.CreateRoom(ctx, source.ID)
	if err != nil {
		t.Fatalf("create room: %v", err)
	}
	if _, err := st.SendRoomMessage(ctx, room.ID, source.ID, "hello from the guest"); err != nil {
		t.Fatalf("send room message: %v", err)
	}

	if err := st.MergeUserInto(ctx, source.ID, target.ID); err != nil {
		t.Fatalf("MergeUserInto: %v", err)
	}

	messages := countRows(t, st, ctx,
		`SELECT COUNT(*) FROM room_messages WHERE user_id = $1`, target.ID)
	if messages != 1 {
		t.Fatalf("room messages on target = %d, want 1", messages)
	}
	members := countRows(t, st, ctx,
		`SELECT COUNT(*) FROM room_members WHERE user_id = $1 OR user_id = $2`, source.ID, target.ID)
	if members != 0 {
		t.Fatalf("room memberships left = %d, want 0 (source seat released, target untouched)", members)
	}
}

// A merged developer must not lose their published games or their API keys.
func TestMergeUserIntoCarriesGameOwnershipAndAPIKeys(t *testing.T) {
	st := openTestStore(t)
	cleaner := st.NewTestCleaner(t)
	ctx := context.Background()

	source := mergeTestUser(t, st, cleaner, ctx, "devsource")
	target := mergeTestUser(t, st, cleaner, ctx, "devtarget")

	game, err := st.InsertTestGame(ctx, "Merge Owned Game "+uuid.NewString()[:8])
	if err != nil {
		t.Fatalf("insert game: %v", err)
	}
	cleaner.TrackGame(game.ID)
	if _, err := st.db.ExecContext(ctx,
		`UPDATE games SET owner_user_id = $2 WHERE id = $1`, game.ID, source.ID); err != nil {
		t.Fatalf("set game owner: %v", err)
	}
	if _, _, err := st.CreateDeveloperAPIKey(ctx, source.ID, "merge test key"); err != nil {
		t.Fatalf("create api key: %v", err)
	}

	if err := st.MergeUserInto(ctx, source.ID, target.ID); err != nil {
		t.Fatalf("MergeUserInto: %v", err)
	}

	games, err := st.ListMyGames(ctx, target.ID)
	if err != nil {
		t.Fatalf("ListMyGames: %v", err)
	}
	found := false
	for _, g := range games {
		if g.ID == game.ID {
			found = true
		}
	}
	if !found {
		t.Fatalf("target does not own the carried game %s", game.ID)
	}

	keys, err := st.ListDeveloperAPIKeys(ctx, target.ID)
	if err != nil {
		t.Fatalf("ListDeveloperAPIKeys: %v", err)
	}
	if len(keys) != 1 {
		t.Fatalf("api keys on target = %d, want 1", len(keys))
	}
}

// Magic links for a deactivated account are live login tokens, so they are destroyed
// rather than handed to the survivor.
func TestMergeUserIntoDeletesSourceMagicLinks(t *testing.T) {
	st := openTestStore(t)
	cleaner := st.NewTestCleaner(t)
	ctx := context.Background()

	source := mergeTestUser(t, st, cleaner, ctx, "linksource")
	target := mergeTestUser(t, st, cleaner, ctx, "linktarget")

	email := "merge-link-" + uuid.NewString() + "@example.com"
	cleaner.TrackEmail(email)
	if _, err := st.CreateMagicLink(ctx, CreateMagicLinkParams{
		Email:     email,
		UserID:    &source.ID,
		TokenHash: "merge-hash-" + uuid.NewString(),
		ExpiresAt: time.Now().Add(15 * time.Minute),
	}); err != nil {
		t.Fatalf("create magic link: %v", err)
	}

	if err := st.MergeUserInto(ctx, source.ID, target.ID); err != nil {
		t.Fatalf("MergeUserInto: %v", err)
	}

	left := countRows(t, st, ctx,
		`SELECT COUNT(*) FROM magic_links WHERE user_id = $1 OR user_id = $2`, source.ID, target.ID)
	if left != 0 {
		t.Fatalf("magic links surviving the merge = %d, want 0", left)
	}
}
