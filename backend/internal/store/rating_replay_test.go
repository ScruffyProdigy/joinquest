package store

import (
	"context"
	"reflect"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/scruffyprodigy/joinquest/internal/rating"
)

func TestReplayOverRealHistoryIsReproducible(t *testing.T) {
	st := openTestStore(t)
	cleaner := st.NewTestCleaner(t)
	ctx := context.Background()

	gameID, modeKey, userA, _ := seedThreeFinishedMatches(t, st, ctx, cleaner)
	e, _ := rating.NewWengLin("plackett-luce")
	r := rating.NewReplayer(e, st.RatingSource())

	if _, err := r.ReplayMode(ctx, gameID.String(), modeKey); err != nil {
		t.Fatalf("first replay: %v", err)
	}
	first, err := st.LoadPlayerRatings(ctx, gameID, modeKey)
	if err != nil {
		t.Fatalf("LoadPlayerRatings: %v", err)
	}

	// userA played all three seeded matches: MatchesPlayed must accumulate
	// across them rather than resetting to 1, which a seed using a fresh
	// pair of users per match could never exercise.
	if got := first[PlayerRatingKey(userA)].MatchesPlayed; got != 3 {
		t.Errorf("userA MatchesPlayed = %d, want 3", got)
	}

	// Each match also carries a "seat:P1"/"seat:P2" modifier entrant, so the
	// nonplayer_ratings write/read path is exercised, not just player_ratings.
	firstEntities, err := st.LoadNonPlayerRatings(ctx, gameID, modeKey)
	if err != nil {
		t.Fatalf("LoadNonPlayerRatings: %v", err)
	}
	if got, ok := firstEntities["seat:P1"]; !ok || got.MatchesPlayed != 3 {
		t.Errorf("seat:P1 = %+v (present=%v), want MatchesPlayed 3", got, ok)
	}

	// last_rated_at must be stamped from the newest input's rated_at (see
	// ratingStoreAdapter.ListInputs/SaveAll), not from wall-clock time at the
	// moment the replay ran. seedThreeFinishedMatches spaces its three inputs
	// a second apart, so a wall-clock stamp and the newest-input stamp would
	// only coincide by the test racing the clock — they're asserted equal
	// here, not just close, which a time.Now() stamp could not reliably
	// satisfy. Without this assertion, a future refactor could silently
	// revert to time.Now() and the only symptom would be Task 6's sweep
	// (which compares max-input against last-rated) looping forever.
	lastRatedAt := loadPlayerLastRatedAt(t, st, gameID, modeKey, userA)
	newestInputAt := maxRatingInputRatedAt(t, st, gameID, modeKey)
	if !lastRatedAt.Equal(newestInputAt) {
		t.Errorf("player_ratings.last_rated_at = %v, want newest input's rated_at %v", lastRatedAt, newestInputAt)
	}

	// No explicit ClearRatings here: SaveAll (via SaveRatings) clears the
	// mode's cached ratings in the same transaction as it writes, so a
	// second replay is atomically a full recompute without the caller having
	// to remember to reset the cache first.
	if _, err := r.ReplayMode(ctx, gameID.String(), modeKey); err != nil {
		t.Fatalf("second replay: %v", err)
	}
	second, _ := st.LoadPlayerRatings(ctx, gameID, modeKey)

	if !reflect.DeepEqual(first, second) {
		t.Errorf("replay from scratch differs:\nfirst  = %+v\nsecond = %+v", first, second)
	}

	secondEntities, err := st.LoadNonPlayerRatings(ctx, gameID, modeKey)
	if err != nil {
		t.Fatalf("LoadNonPlayerRatings (second): %v", err)
	}
	if !reflect.DeepEqual(firstEntities, secondEntities) {
		t.Errorf("nonplayer replay from scratch differs:\nfirst  = %+v\nsecond = %+v", firstEntities, secondEntities)
	}
}

// TestReplaySaveAllClearsStaleEntrants proves SaveAll performs a full
// recompute rather than an upsert: an entrant present in an earlier replay's
// input log but absent from a later one must not survive in the cache.
// Before SaveRatings cleared in the same transaction as it writes, removing
// or correcting an input row left the previously-cached rating for an
// entrant no longer in the log stranded forever, and the cache silently
// stopped equalling the replay — the one invariant this whole path exists to
// uphold.
func TestReplaySaveAllClearsStaleEntrants(t *testing.T) {
	st := openTestStore(t)
	cleaner := st.NewTestCleaner(t)
	ctx := context.Background()

	game, mode := setupDuelMode(t, st, cleaner)
	queues, err := st.ListModeQueuesByModeID(ctx, mode.ID)
	if err != nil {
		t.Fatalf("ListModeQueuesByModeID: %v", err)
	}
	if len(queues) == 0 {
		t.Fatal("expected at least one mode queue for duel mode")
	}
	queueID := queues[0].ID

	userA, err := st.CreateUser(ctx, CreateUserParams{Email: "stale-a-" + uuid.NewString() + "@example.com"})
	if err != nil {
		t.Fatalf("CreateUser A: %v", err)
	}
	cleaner.TrackUser(userA.ID)
	userB, err := st.CreateUser(ctx, CreateUserParams{Email: "stale-b-" + uuid.NewString() + "@example.com"})
	if err != nil {
		t.Fatalf("CreateUser B: %v", err)
	}
	cleaner.TrackUser(userB.ID)
	userC, err := st.CreateUser(ctx, CreateUserParams{Email: "stale-c-" + uuid.NewString() + "@example.com"})
	if err != nil {
		t.Fatalf("CreateUser C: %v", err)
	}
	cleaner.TrackUser(userC.ID)

	base := time.Now().UTC()

	// Match 1: A vs B. This is the session whose input row gets removed
	// below, so B must not survive the second replay.
	staleSessionID := matchAndAppendInput(t, st, ctx, queueID, game.ID, mode.ModeKey, userA.ID, userB.ID, base)

	// Release A and B so they can requeue for a different pairing, following
	// the pattern in TestRequeueAfterReturnUsesNewSessionNotStaleActive.
	if err := st.ReleaseUserMatchedQueue(ctx, queueID, userA.ID); err != nil {
		t.Fatalf("ReleaseUserMatchedQueue A: %v", err)
	}
	if err := st.ReleaseUserMatchedQueue(ctx, queueID, userB.ID); err != nil {
		t.Fatalf("ReleaseUserMatchedQueue B: %v", err)
	}

	// Match 2: A vs C. This session's input row stays, so the mode still has
	// input rows (and A still has a rating) after match 1 is removed.
	matchAndAppendInput(t, st, ctx, queueID, game.ID, mode.ModeKey, userA.ID, userC.ID, base.Add(time.Second))

	e, _ := rating.NewWengLin("plackett-luce")
	r := rating.NewReplayer(e, st.RatingSource())

	if _, err := r.ReplayMode(ctx, game.ID.String(), mode.ModeKey); err != nil {
		t.Fatalf("first replay: %v", err)
	}
	before, err := st.LoadPlayerRatings(ctx, game.ID, mode.ModeKey)
	if err != nil {
		t.Fatalf("LoadPlayerRatings (before): %v", err)
	}
	if _, ok := before[PlayerRatingKey(userB.ID)]; !ok {
		t.Fatal("expected userB rated after first replay")
	}

	// Simulate a corrected/removed input row: match 1's session no longer
	// appears in the log at all.
	if _, err := st.db.ExecContext(ctx, `DELETE FROM rating_match_inputs WHERE session_id = $1`, staleSessionID); err != nil {
		t.Fatalf("delete stale input: %v", err)
	}

	if _, err := r.ReplayMode(ctx, game.ID.String(), mode.ModeKey); err != nil {
		t.Fatalf("second replay: %v", err)
	}
	after, err := st.LoadPlayerRatings(ctx, game.ID, mode.ModeKey)
	if err != nil {
		t.Fatalf("LoadPlayerRatings (after): %v", err)
	}
	if _, ok := after[PlayerRatingKey(userB.ID)]; ok {
		t.Errorf("userB rating survived a replay whose input log no longer contains it: %+v", after[PlayerRatingKey(userB.ID)])
	}
	if _, ok := after[PlayerRatingKey(userA.ID)]; !ok {
		t.Error("expected userA to still be rated from the surviving match 2 input")
	}
}

// TestListInputsDeduplicatesEntrantKeysWithinASide proves ListInputs collapses a
// duplicated entrant key on one side into a single entrant, rather than passing the
// duplicate through to the engine. A side can never legitimately hold the same entrant
// key twice; this shape can arise today from carrySourceRatingHistoryTx (both accounts
// having sat in the same match before an account merge), but ListInputs's dedup is
// unconditionally correct regardless of source, and it is the backstop that keeps that
// merge collision bounded rather than nondeterministic (see the comment on
// carrySourceRatingHistoryTx in user_merge_carry.go).
func TestListInputsDeduplicatesEntrantKeysWithinASide(t *testing.T) {
	st := openTestStore(t)
	cleaner := st.NewTestCleaner(t)
	ctx := context.Background()

	game, mode := setupDuelMode(t, st, cleaner)
	queues, err := st.ListModeQueuesByModeID(ctx, mode.ID)
	if err != nil {
		t.Fatalf("ListModeQueuesByModeID: %v", err)
	}
	if len(queues) == 0 {
		t.Fatal("expected at least one mode queue for duel mode")
	}
	queueID := queues[0].ID

	userA, err := st.CreateUser(ctx, CreateUserParams{Email: "dedup-a-" + uuid.NewString() + "@example.com"})
	if err != nil {
		t.Fatalf("CreateUser A: %v", err)
	}
	cleaner.TrackUser(userA.ID)
	userB, err := st.CreateUser(ctx, CreateUserParams{Email: "dedup-b-" + uuid.NewString() + "@example.com"})
	if err != nil {
		t.Fatalf("CreateUser B: %v", err)
	}
	cleaner.TrackUser(userB.ID)

	if _, err := st.JoinModeQueue(ctx, queueID, userA.ID, "", nil); err != nil {
		t.Fatalf("join A: %v", err)
	}
	if _, err := st.JoinModeQueue(ctx, queueID, userB.ID, "", nil); err != nil {
		t.Fatalf("join B: %v", err)
	}
	mustReconcileForming(t, st, ctx, queueID)

	view, err := st.GetUserActiveIntent(ctx, userA.ID)
	if err != nil {
		t.Fatalf("GetUserActiveIntent: %v", err)
	}
	if view == nil || view.SessionID == nil {
		t.Fatal("expected matched session")
	}
	sessionID := *view.SessionID

	dupKey := "player:" + userA.ID.String()

	// Side 0 stores the same entrant key twice, as if a merge had collapsed both
	// accounts' keys onto one side of a match they both played in.
	in := RatingInput{
		SessionID: sessionID,
		GameID:    game.ID,
		ModeKey:   mode.ModeKey,
		RatedAt:   time.Now().UTC(),
		Sides: []RatingSideRow{
			{Rank: 0, Entrants: []RatingEntrantRow{{Key: dupKey}, {Key: dupKey}}},
			{Rank: 1, Entrants: []RatingEntrantRow{{Key: "player:" + userB.ID.String()}}},
		},
	}
	if err := st.AppendRatingInput(ctx, in); err != nil {
		t.Fatalf("AppendRatingInput: %v", err)
	}

	adapter := st.RatingSource()
	inputs, err := adapter.ListInputs(ctx, game.ID.String(), mode.ModeKey)
	if err != nil {
		t.Fatalf("ListInputs: %v", err)
	}
	if len(inputs) != 1 {
		t.Fatalf("expected 1 input, got %d", len(inputs))
	}

	side0 := inputs[0].Sides[0]
	if len(side0.Entrants) != 1 {
		t.Fatalf("side 0 entrants = %+v, want exactly 1 (duplicate collapsed)", side0.Entrants)
	}
	if side0.Entrants[0].Key != dupKey {
		t.Errorf("side 0 entrant key = %q, want %q", side0.Entrants[0].Key, dupKey)
	}
}

// TestSaveAllWithoutListInputsErrors proves the JQ-241 guard: a bare
// ratingStoreAdapter — matchesPlayed still nil because ListInputs was never
// called on it — must refuse SaveAll rather than silently write zero
// matches-played for every entrant. This needs no database at all: the guard
// fires before SaveAll does anything else.
func TestSaveAllWithoutListInputsErrors(t *testing.T) {
	a := &ratingStoreAdapter{}
	ctx := context.Background()

	err := a.SaveAll(ctx, uuid.New().String(), "arena", "engine", nil, nil)
	if err == nil {
		t.Fatal("SaveAll without a preceding ListInputs must error, got nil")
	}
}

// TestSaveAllAfterListInputsOverEmptyLogSucceeds proves the guard is keyed on
// matchesPlayed being nil, not on it being empty: ListInputs over a mode with
// no rating inputs at all sets matchesPlayed to a non-nil empty map (see the
// `counts := make(map[string]int)` in ListInputs, unconditionally assigned to
// a.matchesPlayed even when the input log is empty), and SaveAll must accept
// that — an empty log is a legitimate mode with nothing rated yet, not a
// misuse of the adapter.
func TestSaveAllAfterListInputsOverEmptyLogSucceeds(t *testing.T) {
	st := openTestStore(t)
	cleaner := st.NewTestCleaner(t)
	ctx := context.Background()

	game, mode := setupDuelMode(t, st, cleaner)

	adapter := st.RatingSource()
	inputs, err := adapter.ListInputs(ctx, game.ID.String(), mode.ModeKey)
	if err != nil {
		t.Fatalf("ListInputs: %v", err)
	}
	if len(inputs) != 0 {
		t.Fatalf("expected no rating inputs for a freshly created mode, got %d", len(inputs))
	}

	if err := adapter.SaveAll(ctx, game.ID.String(), mode.ModeKey, "engine", nil, nil); err != nil {
		t.Fatalf("SaveAll after ListInputs over an empty log must succeed, got: %v", err)
	}
}

// loadPlayerLastRatedAt reads player_ratings.last_rated_at directly, since
// LoadPlayerRatings' RatingValue doesn't carry it (nothing outside this test
// needs it yet).
func loadPlayerLastRatedAt(t *testing.T, st *Store, gameID uuid.UUID, modeKey string, userID uuid.UUID) time.Time {
	t.Helper()
	var at time.Time
	err := st.db.QueryRowContext(context.Background(), `
		SELECT last_rated_at FROM player_ratings WHERE user_id = $1 AND game_id = $2 AND mode_key = $3
	`, userID, gameID, modeKey).Scan(&at)
	if err != nil {
		t.Fatalf("query player_ratings.last_rated_at: %v", err)
	}
	return at
}

// maxRatingInputRatedAt reads the newest rated_at across a mode's rating
// input log directly from rating_match_inputs.
func maxRatingInputRatedAt(t *testing.T, st *Store, gameID uuid.UUID, modeKey string) time.Time {
	t.Helper()
	var at time.Time
	err := st.db.QueryRowContext(context.Background(), `
		SELECT MAX(rated_at) FROM rating_match_inputs WHERE game_id = $1 AND mode_key = $2
	`, gameID, modeKey).Scan(&at)
	if err != nil {
		t.Fatalf("query MAX(rating_match_inputs.rated_at): %v", err)
	}
	return at
}

// matchAndAppendInput matches userA against userB through queueID and
// appends a rating input for the resulting session, carrying a "seat:P1" /
// "seat:P2" modifier entrant on each side so the nonplayer_ratings path is
// exercised alongside player_ratings. It returns the matched session id.
func matchAndAppendInput(t *testing.T, st *Store, ctx context.Context, queueID, gameID uuid.UUID, modeKey string, userA, userB uuid.UUID, ratedAt time.Time) uuid.UUID {
	t.Helper()

	if _, err := st.JoinModeQueue(ctx, queueID, userA, "", nil); err != nil {
		t.Fatalf("join A: %v", err)
	}
	if _, err := st.JoinModeQueue(ctx, queueID, userB, "", nil); err != nil {
		t.Fatalf("join B: %v", err)
	}
	mustReconcileForming(t, st, ctx, queueID)

	view, err := st.GetUserActiveIntent(ctx, userA)
	if err != nil {
		t.Fatalf("GetUserActiveIntent: %v", err)
	}
	if view == nil || view.SessionID == nil {
		t.Fatal("expected matched session")
	}
	sessionID := *view.SessionID

	in := RatingInput{
		SessionID: sessionID,
		GameID:    gameID,
		ModeKey:   modeKey,
		RatedAt:   ratedAt,
		Sides: []RatingSideRow{
			{Rank: 0, Entrants: []RatingEntrantRow{{Key: "player:" + userA.String()}, {Key: "seat:P1"}}},
			{Rank: 1, Entrants: []RatingEntrantRow{{Key: "player:" + userB.String()}, {Key: "seat:P2"}}},
		},
	}
	if err := st.AppendRatingInput(ctx, in); err != nil {
		t.Fatalf("AppendRatingInput: %v", err)
	}
	return sessionID
}

// seedThreeFinishedMatches creates a game and duel mode owned entirely by
// this test (never the shared demo game), records three finished matches
// between the SAME two users against it (so MatchesPlayed accumulates rather
// than sitting at 1 for every key), and returns the game id, mode key, and
// both user ids to replay. Each match also carries a "seat:P1"/"seat:P2"
// modifier entrant so nonplayer_ratings is actually written and read back,
// not just player_ratings. Using a test-owned game rather than the demo game
// means TrackGame's cascading delete also clears the rating_match_inputs
// rows this test appends — TestCleaner never deletes those rows itself (see
// package-level test hygiene notes), so a test that reused the demo game
// would leak input rows into every later replay of that mode.
func seedThreeFinishedMatches(t *testing.T, st *Store, ctx context.Context, cleaner *TestCleaner) (gameID uuid.UUID, modeKey string, userA, userB uuid.UUID) {
	t.Helper()

	game, mode := setupDuelMode(t, st, cleaner)
	queues, err := st.ListModeQueuesByModeID(ctx, mode.ID)
	if err != nil {
		t.Fatalf("ListModeQueuesByModeID: %v", err)
	}
	if len(queues) == 0 {
		t.Fatal("expected at least one mode queue for duel mode")
	}
	queueID := queues[0].ID

	a, err := st.CreateUser(ctx, CreateUserParams{Email: "replay-a-" + uuid.NewString() + "@example.com"})
	if err != nil {
		t.Fatalf("CreateUser A: %v", err)
	}
	cleaner.TrackUser(a.ID)

	b, err := st.CreateUser(ctx, CreateUserParams{Email: "replay-b-" + uuid.NewString() + "@example.com"})
	if err != nil {
		t.Fatalf("CreateUser B: %v", err)
	}
	cleaner.TrackUser(b.ID)

	base := time.Now().UTC()
	for i := 0; i < 3; i++ {
		if i > 0 {
			// Free both users from their previous matched-queue row so they
			// can requeue for another match, following the pattern in
			// TestRequeueAfterReturnUsesNewSessionNotStaleActive.
			if err := st.ReleaseUserMatchedQueue(ctx, queueID, a.ID); err != nil {
				t.Fatalf("ReleaseUserMatchedQueue A (match %d): %v", i, err)
			}
			if err := st.ReleaseUserMatchedQueue(ctx, queueID, b.ID); err != nil {
				t.Fatalf("ReleaseUserMatchedQueue B (match %d): %v", i, err)
			}
		}

		matchAndAppendInput(t, st, ctx, queueID, game.ID, mode.ModeKey, a.ID, b.ID, base.Add(time.Duration(i)*time.Second))
	}

	return game.ID, mode.ModeKey, a.ID, b.ID
}
