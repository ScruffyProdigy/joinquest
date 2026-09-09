package store

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
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

	if err := st.RecordMatchResult(ctx, sessionID, "COMPLETED",
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
		if err := st.RecordMatchResult(ctx, sessionID, "COMPLETED",
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
	if err := st.RecordMatchResult(ctx, sessionID, "ABANDONED", nil, nil, time.Now()); err != nil {
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
