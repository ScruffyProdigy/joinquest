package store

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/scruffyprodigy/joinquest/internal/gameclient"
)

func TestAppendRatingInputRoundTrips(t *testing.T) {
	st := openTestStore(t)
	cleaner := st.NewTestCleaner(t)
	ctx := context.Background()

	sessionID, userA, userB := seedMatchedSession(t, st, ctx, cleaner)
	gameID, modeKey := gameAndModeForSession(t, st, ctx, sessionID)
	at := time.Now().UTC().Truncate(time.Millisecond)

	in := RatingInput{
		SessionID: sessionID,
		GameID:    gameID,
		ModeKey:   modeKey,
		RatedAt:   at,
		Sides: []RatingSideRow{
			{Rank: 0, Entrants: []RatingEntrantRow{{Key: "player:" + userA.String()}}},
			{Rank: 1, Entrants: []RatingEntrantRow{{Key: "player:" + userB.String()}}},
		},
	}
	if err := st.AppendRatingInput(ctx, in); err != nil {
		t.Fatalf("AppendRatingInput: %v", err)
	}

	got, err := st.ListRatingInputs(ctx, gameID, modeKey)
	if err != nil {
		t.Fatalf("ListRatingInputs: %v", err)
	}
	row := findRatingInputBySession(t, got, sessionID)
	if len(row.Sides) != 2 || row.Sides[1].Rank != 1 {
		t.Fatalf("sides round-tripped wrong: %+v", row.Sides)
	}
}

// TestAppendRatingInputIsIdempotent asserts the load-bearing property behind
// the ON CONFLICT (session_id) DO NOTHING clause: retrying a result report
// for the same session must not duplicate its rating input.
func TestAppendRatingInputIsIdempotent(t *testing.T) {
	st := openTestStore(t)
	cleaner := st.NewTestCleaner(t)
	ctx := context.Background()

	sessionID, userA, userB := seedMatchedSession(t, st, ctx, cleaner)
	gameID, modeKey := gameAndModeForSession(t, st, ctx, sessionID)
	at := time.Now().UTC().Truncate(time.Millisecond)

	in := RatingInput{
		SessionID: sessionID,
		GameID:    gameID,
		ModeKey:   modeKey,
		RatedAt:   at,
		Sides: []RatingSideRow{
			{Rank: 0, Entrants: []RatingEntrantRow{{Key: "player:" + userA.String()}}},
			{Rank: 1, Entrants: []RatingEntrantRow{{Key: "player:" + userB.String()}}},
		},
	}
	for i := 0; i < 2; i++ {
		if err := st.AppendRatingInput(ctx, in); err != nil {
			t.Fatalf("AppendRatingInput (attempt %d): %v", i+1, err)
		}
	}

	got, err := st.ListRatingInputs(ctx, gameID, modeKey)
	if err != nil {
		t.Fatalf("ListRatingInputs: %v", err)
	}
	matches := 0
	for _, row := range got {
		if row.SessionID == sessionID {
			matches++
		}
	}
	if matches != 1 {
		t.Fatalf("rows for session %s = %d, want 1 (retry must not duplicate)", sessionID, matches)
	}
}

// TestAppendRatingInputCorrectionUpdatesSides covers the DO UPDATE half of
// the ON CONFLICT (session_id) clause: a game server correcting an earlier
// report (different winner) must have that correction propagate into the
// stored row, not be silently dropped the way DO NOTHING would drop it.
func TestAppendRatingInputCorrectionUpdatesSides(t *testing.T) {
	st := openTestStore(t)
	cleaner := st.NewTestCleaner(t)
	ctx := context.Background()

	sessionID, userA, userB := seedMatchedSession(t, st, ctx, cleaner)
	gameID, modeKey := gameAndModeForSession(t, st, ctx, sessionID)
	at := time.Now().UTC().Truncate(time.Millisecond)

	original := RatingInput{
		SessionID: sessionID,
		GameID:    gameID,
		ModeKey:   modeKey,
		RatedAt:   at,
		Sides: []RatingSideRow{
			{Rank: 0, Entrants: []RatingEntrantRow{{Key: "player:" + userA.String()}}},
			{Rank: 1, Entrants: []RatingEntrantRow{{Key: "player:" + userB.String()}}},
		},
	}
	if err := st.AppendRatingInput(ctx, original); err != nil {
		t.Fatalf("AppendRatingInput (original): %v", err)
	}

	// The game server corrects its report: userB actually won.
	corrected := original
	corrected.RatedAt = at.Add(time.Minute)
	corrected.Sides = []RatingSideRow{
		{Rank: 0, Entrants: []RatingEntrantRow{{Key: "player:" + userB.String()}}},
		{Rank: 1, Entrants: []RatingEntrantRow{{Key: "player:" + userA.String()}}},
	}
	if err := st.AppendRatingInput(ctx, corrected); err != nil {
		t.Fatalf("AppendRatingInput (corrected): %v", err)
	}

	got, err := st.ListRatingInputs(ctx, gameID, modeKey)
	if err != nil {
		t.Fatalf("ListRatingInputs: %v", err)
	}
	matches := 0
	for _, row := range got {
		if row.SessionID == sessionID {
			matches++
		}
	}
	if matches != 1 {
		t.Fatalf("rows for session %s = %d, want 1 (correction must update, not duplicate)", sessionID, matches)
	}

	row := findRatingInputBySession(t, got, sessionID)
	var winnerKey string
	for _, side := range row.Sides {
		if side.Rank == 0 {
			winnerKey = side.Entrants[0].Key
		}
	}
	if winnerKey != "player:"+userB.String() {
		t.Fatalf("winner after correction = %q, want player:%s (correction must propagate, not be silently dropped)", winnerKey, userB)
	}

	// rated_at must NOT move to the correction's later report timestamp: it
	// orders replay (ListRatingInputs' ORDER BY rated_at ASC, session_id
	// ASC), and a correction is a restatement of a match that already
	// happened, not a new event. Bumping it would shift this match's
	// position relative to everything reported in between the original and
	// corrected reports, changing replay order and potentially producing
	// different recomputed ratings.
	if !row.RatedAt.Equal(at) {
		t.Fatalf("rated_at after correction = %v, want unchanged original %v (rated_at must not move on correction)", row.RatedAt, at)
	}
}

func TestListRatingInputsIsTotallyOrdered(t *testing.T) {
	st := openTestStore(t)
	cleaner := st.NewTestCleaner(t)
	ctx := context.Background()

	// Two inputs sharing a rated_at: replay must still order them the same way
	// every time, or the recompute is not reproducible.
	at := time.Now().UTC()
	gameID, modeKey := seedTwoInputsAtSameInstant(t, st, ctx, cleaner, at)

	first, err := st.ListRatingInputs(ctx, gameID, modeKey)
	if err != nil {
		t.Fatalf("ListRatingInputs: %v", err)
	}
	for i := 0; i < 5; i++ {
		got, err := st.ListRatingInputs(ctx, gameID, modeKey)
		if err != nil {
			t.Fatalf("ListRatingInputs: %v", err)
		}
		for j := range got {
			if got[j].SessionID != first[j].SessionID {
				t.Fatalf("order changed between calls at index %d", j)
			}
		}
	}
}

func TestSaveAndLoadRatings(t *testing.T) {
	st := openTestStore(t)
	cleaner := st.NewTestCleaner(t)
	ctx := context.Background()

	sessionID, userA, _ := seedMatchedSession(t, st, ctx, cleaner)
	gameID, modeKey := gameAndModeForSession(t, st, ctx, sessionID)
	at := time.Now().UTC()

	if err := st.SaveRatings(ctx, gameID, modeKey, "weng-lin/plackett-luce@1", at,
		map[string]RatingValue{"player:" + userA.String(): {Mu: 26.5, Sigma: 7.9, MatchesPlayed: 1}},
		map[string]RatingValue{"scenario": {Mu: 23.5, Sigma: 7.9, MatchesPlayed: 1}},
	); err != nil {
		t.Fatalf("SaveRatings: %v", err)
	}

	players, err := st.LoadPlayerRatings(ctx, gameID, modeKey)
	if err != nil {
		t.Fatalf("LoadPlayerRatings: %v", err)
	}
	if got := players["player:"+userA.String()].Mu; got != 26.5 {
		t.Errorf("mu = %v, want 26.5", got)
	}

	entities, err := st.LoadNonPlayerRatings(ctx, gameID, modeKey)
	if err != nil {
		t.Fatalf("LoadNonPlayerRatings: %v", err)
	}
	if got := entities["scenario"].Mu; got != 23.5 {
		t.Errorf("scenario mu = %v, want 23.5", got)
	}
}

// SaveRatings must be idempotent: replay writes the same keys repeatedly.
func TestSaveRatingsOverwritesRatherThanDuplicating(t *testing.T) {
	st := openTestStore(t)
	cleaner := st.NewTestCleaner(t)
	ctx := context.Background()

	sessionID, userA, _ := seedMatchedSession(t, st, ctx, cleaner)
	gameID, modeKey := gameAndModeForSession(t, st, ctx, sessionID)
	key := "player:" + userA.String()
	at := time.Now().UTC()

	for _, mu := range []float64{25, 27} {
		if err := st.SaveRatings(ctx, gameID, modeKey, "weng-lin/plackett-luce@1", at,
			map[string]RatingValue{key: {Mu: mu, Sigma: 8, MatchesPlayed: 1}}, nil); err != nil {
			t.Fatalf("SaveRatings: %v", err)
		}
	}

	players, _ := st.LoadPlayerRatings(ctx, gameID, modeKey)
	if players[key].Mu != 27 {
		t.Errorf("mu = %v, want the second write, 27", players[key].Mu)
	}
}

// findRatingInputBySession scopes an assertion to the row a test created,
// rather than asserting an exact count for the game/mode: the demo
// (gameID, modeKey) pair is shared across every test in this package (and the
// demo game is never deleted by TestCleaner), so any other test appending an
// input against the same mode would make a bare len(got) assertion fragile.
func findRatingInputBySession(t *testing.T, got []RatingInput, sessionID uuid.UUID) RatingInput {
	t.Helper()
	for _, row := range got {
		if row.SessionID == sessionID {
			return row
		}
	}
	t.Fatalf("no rating input found for session %s among %d rows", sessionID, len(got))
	return RatingInput{}
}

// gameAndModeForSession resolves the catalog game and mode key a matched
// session was created against, the way rating replay keys its inputs.
func gameAndModeForSession(t *testing.T, st *Store, ctx context.Context, sessionID uuid.UUID) (uuid.UUID, string) {
	t.Helper()
	var gameID uuid.UUID
	var modeKey string
	err := st.db.QueryRowContext(ctx, `
		SELECT gs.game_id, gm.mode_key
		FROM game_sessions gs
		JOIN game_modes gm ON gm.id = gs.mode_id
		WHERE gs.id = $1
	`, sessionID).Scan(&gameID, &modeKey)
	if err != nil {
		t.Fatalf("gameAndModeForSession: %v", err)
	}
	return gameID, modeKey
}

// seedTwoInputsAtSameInstant appends rating inputs for two distinct matched
// sessions, both stamped with the same rated_at, and returns the game/mode
// they share so ListRatingInputs can be exercised against the tie.
func seedTwoInputsAtSameInstant(t *testing.T, st *Store, ctx context.Context, cleaner *TestCleaner, at time.Time) (uuid.UUID, string) {
	t.Helper()

	session1, userA, userB := seedMatchedSession(t, st, ctx, cleaner)
	session2, userC, userD := seedMatchedSession(t, st, ctx, cleaner)
	gameID, modeKey := gameAndModeForSession(t, st, ctx, session1)

	inputs := []RatingInput{
		{
			SessionID: session1,
			GameID:    gameID,
			ModeKey:   modeKey,
			RatedAt:   at,
			Sides: []RatingSideRow{
				{Rank: 0, Entrants: []RatingEntrantRow{{Key: "player:" + userA.String()}}},
				{Rank: 1, Entrants: []RatingEntrantRow{{Key: "player:" + userB.String()}}},
			},
		},
		{
			SessionID: session2,
			GameID:    gameID,
			ModeKey:   modeKey,
			RatedAt:   at,
			Sides: []RatingSideRow{
				{Rank: 0, Entrants: []RatingEntrantRow{{Key: "player:" + userC.String()}}},
				{Rank: 1, Entrants: []RatingEntrantRow{{Key: "player:" + userD.String()}}},
			},
		},
	}
	for _, in := range inputs {
		if err := st.AppendRatingInput(ctx, in); err != nil {
			t.Fatalf("AppendRatingInput: %v", err)
		}
	}
	return gameID, modeKey
}

// Assertions here are scoped to the session each test created rather than
// asserting a bare len(inputs): the demo (game, mode) pair is shared and
// TestCleaner never deletes rating_match_inputs rows or the demo game (see
// findRatingInputBySession above), so a raw count breaks as soon as other
// tests in this package have appended their own inputs against the same
// mode. Confirmed against the running suite: TestRecordMatchResultAppendsRatingInput
// saw 16 pre-existing rows for the demo mode before scoping was applied.
func TestRecordMatchResultAppendsRatingInput(t *testing.T) {
	st := openTestStore(t)
	cleaner := st.NewTestCleaner(t)
	ctx := context.Background()

	sessionID, userA, userB := seedMatchedSession(t, st, ctx, cleaner)
	gameID, modeKey := gameAndModeForSession(t, st, ctx, sessionID)

	if _, err := st.RecordMatchResult(ctx, sessionID, "COMPLETED",
		[]uuid.UUID{userA}, nil, time.Now()); err != nil {
		t.Fatalf("RecordMatchResult: %v", err)
	}

	inputs, err := st.ListRatingInputs(ctx, gameID, modeKey)
	if err != nil {
		t.Fatalf("ListRatingInputs: %v", err)
	}
	row := findRatingInputBySession(t, inputs, sessionID)

	var winnerRank, loserRank int
	for _, side := range row.Sides {
		for _, e := range side.Entrants {
			switch e.Key {
			case "player:" + userA.String():
				winnerRank = side.Rank
			case "player:" + userB.String():
				loserRank = side.Rank
			}
		}
	}
	if winnerRank >= loserRank {
		t.Errorf("winner rank %d must beat loser rank %d", winnerRank, loserRank)
	}
}

// A game server that retries its report must not rate the match twice.
func TestRecordMatchResultTwiceAppendsOneRatingInput(t *testing.T) {
	st := openTestStore(t)
	cleaner := st.NewTestCleaner(t)
	ctx := context.Background()

	sessionID, userA, _ := seedMatchedSession(t, st, ctx, cleaner)
	gameID, modeKey := gameAndModeForSession(t, st, ctx, sessionID)

	for i := 0; i < 2; i++ {
		if _, err := st.RecordMatchResult(ctx, sessionID, "COMPLETED",
			[]uuid.UUID{userA}, nil, time.Now()); err != nil {
			t.Fatalf("RecordMatchResult %d: %v", i, err)
		}
	}

	inputs, err := st.ListRatingInputs(ctx, gameID, modeKey)
	if err != nil {
		t.Fatalf("ListRatingInputs: %v", err)
	}
	matches := 0
	for _, row := range inputs {
		if row.SessionID == sessionID {
			matches++
		}
	}
	if matches != 1 {
		t.Fatalf("inputs for session %s = %d, want 1 after a retried report", sessionID, matches)
	}
}

// An unrateable outcome must not fail the result write — the game already
// committed its report, and failing here would make it retry an applied write.
func TestRecordMatchResultSurvivesUnrateableOutcome(t *testing.T) {
	st := openTestStore(t)
	cleaner := st.NewTestCleaner(t)
	ctx := context.Background()

	sessionID, _, _ := seedMatchedSession(t, st, ctx, cleaner)
	gameID, modeKey := gameAndModeForSession(t, st, ctx, sessionID)

	// ABANDONED with no winners and no placements: nothing to rate.
	if _, err := st.RecordMatchResult(ctx, sessionID, "ABANDONED", nil, nil, time.Now()); err != nil {
		t.Fatalf("RecordMatchResult must not fail on an unrateable outcome: %v", err)
	}

	result, err := st.GetMatchResult(ctx, sessionID)
	if err != nil {
		t.Fatalf("GetMatchResult: %v", err)
	}
	if result.Status == nil || *result.Status != "ABANDONED" {
		t.Fatalf("status = %v, want ABANDONED", result.Status)
	}

	inputs, err := st.ListRatingInputs(ctx, gameID, modeKey)
	if err != nil {
		t.Fatalf("ListRatingInputs: %v", err)
	}
	for _, row := range inputs {
		if row.SessionID == sessionID {
			t.Errorf("found a rating input for session %s, want none for an unrateable match", sessionID)
		}
	}
}

// newSessionFixture registers a fresh game and mode good for n free-agent
// players, matches all n through it, and returns the matched session
// alongside the game id and the matched user ids. newCompetitiveSessionFixture
// and newCooperativeSessionFixture are thin wrappers over this shared setup —
// per the plan, the cooperative fixture differs from the competitive one only
// in the mode's declared social mode.
func newSessionFixture(t *testing.T, n int, socialMode string) (st *Store, sessionID uuid.UUID, gameID uuid.UUID, users []uuid.UUID) {
	t.Helper()
	st = openTestStore(t)
	cleaner := st.NewTestCleaner(t)
	ctx := context.Background()

	slug := "rating-fixture-" + uuid.NewString()
	manifest := &gameclient.Manifest{
		Modes: []gameclient.ModeManifest{{
			Key:          "arena",
			DisplayName:  "Arena",
			SocialMode:   socialMode,
			SeatTemplate: json.RawMessage(fmt.Sprintf(`{"count":%d}`, n)),
		}},
		Status:     gameclient.StatusResponse{Game: "Rating Fixture", Version: "1.0.0"},
		ETag:       `"rating-fixture"`,
		RawJSON:    []byte(`{"modes":[{"key":"arena"}]}`),
		SHA256Hash: uuid.NewString(),
	}
	result, err := st.RegisterGame(ctx, RegisterGameParams{
		Slug:       slug,
		IconURL:    "/games/default.svg",
		HeroURL:    "/games/default-hero.svg",
		APIBaseURL: "https://api.example.com/" + slug,
	}, manifest)
	if err != nil {
		t.Fatalf("RegisterGame: %v", err)
	}
	cleaner.TrackGame(result.Game.ID)

	modes, err := st.ListGameModesByGameID(ctx, result.Game.ID)
	if err != nil {
		t.Fatalf("ListGameModesByGameID: %v", err)
	}
	queues, err := st.ListModeQueuesByModeID(ctx, modes[0].ID)
	if err != nil {
		t.Fatalf("ListModeQueuesByModeID: %v", err)
	}
	queueID := queues[0].ID

	users = make([]uuid.UUID, n)
	for i := range users {
		user, err := st.CreateUser(ctx, CreateUserParams{Email: fmt.Sprintf("fixture-%d-%s@example.com", i, uuid.NewString())})
		if err != nil {
			t.Fatalf("CreateUser: %v", err)
		}
		cleaner.TrackUser(user.ID)
		users[i] = user.ID
	}

	var rec *FormingReconcileResult
	for _, userID := range users {
		if _, err := st.JoinModeQueue(ctx, queueID, userID, "", nil); err != nil {
			t.Fatalf("JoinModeQueue: %v", err)
		}
		rec = mustReconcileForming(t, st, ctx, queueID)
	}
	if rec == nil || !rec.Fired || rec.SessionID == nil {
		t.Fatalf("expected a match to fire after %d joins, got %+v", n, rec)
	}

	return st, *rec.SessionID, result.Game.ID, users
}

// newCompetitiveSessionFixture registers and matches an n-player session with
// no declared social mode.
func newCompetitiveSessionFixture(t *testing.T, n int) (*Store, uuid.UUID, uuid.UUID, []uuid.UUID) {
	t.Helper()
	return newSessionFixture(t, n, "")
}

// newCooperativeSessionFixture registers and matches a two-player session
// whose mode declares game_modes.social_mode = 'co-op'.
func newCooperativeSessionFixture(t *testing.T) (*Store, uuid.UUID, uuid.UUID, []uuid.UUID) {
	t.Helper()
	return newSessionFixture(t, 2, "co-op")
}

func TestRecordMatchResultCooperativeUsesReportedScenarioKeys(t *testing.T) {
	// Build a co-op session (social_mode 'co-op') with two participants,
	// mirroring the fixture setup in TestRecordMatchResultAppendsRatingInput.
	st, sessionID, gameID, users := newCooperativeSessionFixture(t)

	rated, err := st.RecordMatchResult(context.Background(), sessionID, "COMPLETED",
		users, map[string]any{"scenarios": []any{"hard", "night"}}, time.Now())
	if err != nil {
		t.Fatalf("RecordMatchResult: %v", err)
	}
	if rated == nil {
		t.Fatal("RecordMatchResult reported no rated mode; want the co-op match rated")
	}
	if rated.GameID != gameID {
		t.Fatalf("rated game = %s, want %s", rated.GameID, gameID)
	}

	inputs, err := st.ListRatingInputs(context.Background(), gameID, rated.ModeKey)
	if err != nil {
		t.Fatalf("ListRatingInputs: %v", err)
	}
	if len(inputs) != 1 {
		t.Fatalf("got %d inputs, want 1", len(inputs))
	}

	scenario := inputs[0].Sides[len(inputs[0].Sides)-1]
	var keys []string
	for _, e := range scenario.Entrants {
		keys = append(keys, e.Key)
	}
	want := []string{"scenario:hard", "scenario:night"}
	if !reflect.DeepEqual(keys, want) {
		t.Fatalf("scenario entrants = %v, want %v", keys, want)
	}
}

func TestRecordMatchResultCooperativeWithoutScenariosIsNotRated(t *testing.T) {
	// gameID is unused here: this test only needs to prove the match stayed
	// unrated, not which (game, mode) it would have landed under.
	st, sessionID, _, users := newCooperativeSessionFixture(t)

	rated, err := st.RecordMatchResult(context.Background(), sessionID, "COMPLETED", users, nil, time.Now())
	if err != nil {
		t.Fatalf("RecordMatchResult: %v", err)
	}
	if rated != nil {
		t.Fatal("co-op match with no reported scenarios was rated; want it skipped")
	}

	var count int
	if err := st.db.QueryRow(`SELECT count(*) FROM rating_match_inputs WHERE session_id = $1`, sessionID).Scan(&count); err != nil {
		t.Fatalf("count inputs: %v", err)
	}
	if count != 0 {
		t.Fatalf("got %d rating inputs, want 0", count)
	}
}

func TestRecordMatchResultExcludesDisconnectedPlayers(t *testing.T) {
	// A 1v1-shaped competitive session with three participants so that
	// dropping one still leaves two rateable sides.
	st, sessionID, gameID, users := newCompetitiveSessionFixture(t, 3)

	if err := st.RecordPlayerFinish(context.Background(), sessionID, users[2], "DISCONNECT", nil, nil); err != nil {
		t.Fatalf("RecordPlayerFinish: %v", err)
	}

	rated, err := st.RecordMatchResult(context.Background(), sessionID, "COMPLETED", users[:1], nil, time.Now())
	if err != nil {
		t.Fatalf("RecordMatchResult: %v", err)
	}
	if rated == nil {
		t.Fatal("match was not rated")
	}

	inputs, err := st.ListRatingInputs(context.Background(), gameID, rated.ModeKey)
	if err != nil {
		t.Fatalf("ListRatingInputs: %v", err)
	}
	disconnected := PlayerRatingKey(users[2])
	for _, side := range inputs[0].Sides {
		for _, e := range side.Entrants {
			if e.Key == disconnected {
				t.Fatal("disconnected player appears in the rated sides")
			}
		}
	}
}

func TestRecordMatchResultRatesForfeitAsALoss(t *testing.T) {
	st, sessionID, gameID, users := newCompetitiveSessionFixture(t, 2)

	if err := st.RecordPlayerFinish(context.Background(), sessionID, users[1], "FORFEIT", nil, nil); err != nil {
		t.Fatalf("RecordPlayerFinish: %v", err)
	}

	rated, err := st.RecordMatchResult(context.Background(), sessionID, "COMPLETED", users[:1], nil, time.Now())
	if err != nil {
		t.Fatalf("RecordMatchResult: %v", err)
	}
	if rated == nil {
		t.Fatal("forfeited match was not rated; a forfeit is a loss, not an exclusion")
	}

	inputs, err := st.ListRatingInputs(context.Background(), gameID, rated.ModeKey)
	if err != nil {
		t.Fatalf("ListRatingInputs: %v", err)
	}
	forfeiter := PlayerRatingKey(users[1])
	found := false
	for _, side := range inputs[0].Sides {
		for _, e := range side.Entrants {
			if e.Key == forfeiter {
				found = true
				if side.Rank == 0 {
					t.Fatal("forfeiter is on the winning side")
				}
			}
		}
	}
	if !found {
		t.Fatal("forfeiter was dropped from the sides; want them rated as a loss")
	}
}
