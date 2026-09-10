package store

// End-to-end coverage for JQ-140: one test per match format, plus the
// pipeline's cross-cutting invariants. Every test here drives the real store
// path — build a session, report finishes and a result through the store,
// replay the mode's whole rating_match_inputs log, then assert on
// LoadPlayerRatings / LoadNonPlayerRatings.
//
// Assertions are on direction and identity, never an exact mu: the engine's
// constants are allowed to change, and a test pinned to a specific number
// would fail for the wrong reason. "Direction" is judged against the
// engine's own prior (see ratingPrior), not a hardcoded 25, for the same
// reason.
//
// Every test captures the (game, mode) it replays from the value
// RecordMatchResult itself returns (or, when no result was ever reported,
// from gameAndModeForSession, which reads it back out of the session row) —
// never a hardcoded "arena" — so a change to the fixture's mode key breaks
// loudly on a missing rating instead of silently replaying an empty log.

import (
	"context"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/scruffyprodigy/joinquest/internal/rating"
)

// replayAndLoad replays a game/mode's full rating_match_inputs log through a
// fresh Weng-Lin engine and returns the resulting player and non-player
// rating maps, exactly as a caller of LoadPlayerRatings/LoadNonPlayerRatings
// would see them afterward.
func replayAndLoad(t *testing.T, st *Store, gameID uuid.UUID, modeKey string) (players, entities map[string]RatingValue) {
	t.Helper()
	ctx := context.Background()

	engine, err := rating.NewWengLin("plackett-luce")
	if err != nil {
		t.Fatalf("NewWengLin: %v", err)
	}
	replayer := rating.NewReplayer(engine, st.RatingSource())
	if _, err := replayer.ReplayMode(ctx, gameID.String(), modeKey); err != nil {
		t.Fatalf("ReplayMode: %v", err)
	}

	players, err = st.LoadPlayerRatings(ctx, gameID, modeKey)
	if err != nil {
		t.Fatalf("LoadPlayerRatings: %v", err)
	}
	entities, err = st.LoadNonPlayerRatings(ctx, gameID, modeKey)
	if err != nil {
		t.Fatalf("LoadNonPlayerRatings: %v", err)
	}
	return players, entities
}

// ratingPrior returns the engine's starting mu for an entrant with no
// history, so a test can assert "rose"/"fell" against the actual prior
// rather than a hardcoded constant that would drift out of sync with the
// engine.
func ratingPrior(t *testing.T) float64 {
	t.Helper()
	engine, err := rating.NewWengLin("plackett-luce")
	if err != nil {
		t.Fatalf("NewWengLin: %v", err)
	}
	return engine.Prior().Mu
}

// 1v1 — winner vs loser. Written out in full as the pattern the rest follow:
// drive the real store path, replay, then assert on direction and identity.
func TestPipelineOneVersusOneMovesWinnerAboveLoser(t *testing.T) {
	ctx := context.Background()
	st, sessionID, gameID, users := newCompetitiveSessionFixture(t, 2)
	winner, loser := users[0], users[1]

	rated, err := st.RecordMatchResult(ctx, sessionID, "COMPLETED", []uuid.UUID{winner}, nil, time.Now())
	if err != nil {
		t.Fatalf("RecordMatchResult: %v", err)
	}
	if rated == nil {
		t.Fatal("1v1 match was not rated")
	}

	players, _ := replayAndLoad(t, st, gameID, rated.ModeKey)

	w, ok := players[PlayerRatingKey(winner)]
	if !ok {
		t.Fatal("winner has no rating")
	}
	l, ok := players[PlayerRatingKey(loser)]
	if !ok {
		t.Fatal("loser has no rating")
	}

	// Direction and ordering only — never an exact mu. The engine's constants
	// are allowed to change; a test pinned to 25.3841 would fail for the wrong
	// reason.
	if w.Mu <= l.Mu {
		t.Fatalf("winner mu %v is not above loser mu %v", w.Mu, l.Mu)
	}
	if w.MatchesPlayed != 1 || l.MatchesPlayed != 1 {
		t.Fatalf("matches played = %d/%d, want 1 each", w.MatchesPlayed, l.MatchesPlayed)
	}
}

// Free-for-all — placement order from reportPlayerFinished (RecordPlayerFinish
// here, its store-level name). No winner is reported at all: placement alone
// must be enough for deriveRanks to order the three sides.
func TestPipelineFreeForAllOrdersByPlacement(t *testing.T) {
	ctx := context.Background()
	st, sessionID, gameID, users := newCompetitiveSessionFixture(t, 3)

	for i, userID := range users {
		placement := i + 1 // users[0] finishes 1st, users[1] 2nd, users[2] 3rd.
		if err := st.RecordPlayerFinish(ctx, sessionID, userID, "FINISHED", &placement, nil); err != nil {
			t.Fatalf("RecordPlayerFinish(%s, placement %d): %v", userID, placement, err)
		}
	}

	rated, err := st.RecordMatchResult(ctx, sessionID, "COMPLETED", nil, nil, time.Now())
	if err != nil {
		t.Fatalf("RecordMatchResult: %v", err)
	}
	if rated == nil {
		t.Fatal("free-for-all match was not rated")
	}

	players, _ := replayAndLoad(t, st, gameID, rated.ModeKey)

	mus := make([]float64, len(users))
	for i, u := range users {
		r, ok := players[PlayerRatingKey(u)]
		if !ok {
			t.Fatalf("placement %d finisher %s has no rating", i+1, u)
		}
		if r.MatchesPlayed != 1 {
			t.Fatalf("placement %d finisher matches played = %d, want 1", i+1, r.MatchesPlayed)
		}
		mus[i] = r.Mu
	}
	for i := 1; i < len(mus); i++ {
		if mus[i-1] <= mus[i] {
			t.Fatalf("placement order not reflected in mu: %v (want strictly descending by finish order)", mus)
		}
	}
}

// Teams — winning team vs losing team; both winners rise, both losers fall.
// The team split comes from the seat template's affinity key (see
// newTeamsSessionFixture), read back off the seated session rather than
// assumed from join order, so the test doesn't quietly depend on
// matchmaking's seat-fill order.
func TestPipelineTeamsMovesBothSidesTogether(t *testing.T) {
	ctx := context.Background()
	st, sessionID, gameID, users := newTeamsSessionFixture(t)
	prior := ratingPrior(t)

	seats, err := st.ListSessionSeatAssignments(ctx, sessionID)
	if err != nil {
		t.Fatalf("ListSessionSeatAssignments: %v", err)
	}
	teamOf := make(map[uuid.UUID]string, len(seats))
	for _, seat := range seats {
		idx := strings.Index(seat.SeatKey, "-Seat-")
		if idx < 0 {
			t.Fatalf("seat key %q is not team-shaped, want a \"Team-N-Seat-N\" key", seat.SeatKey)
		}
		teamOf[seat.UserID] = seat.SeatKey[:idx]
	}

	var winners, losers []uuid.UUID
	winningTeam := ""
	for _, u := range users {
		team, ok := teamOf[u]
		if !ok {
			t.Fatalf("user %s was not seated", u)
		}
		if winningTeam == "" {
			winningTeam = team
		}
		if team == winningTeam {
			winners = append(winners, u)
		} else {
			losers = append(losers, u)
		}
	}
	if len(winners) != 2 || len(losers) != 2 {
		t.Fatalf("expected two teams of two, got %d winners and %d losers", len(winners), len(losers))
	}

	rated, err := st.RecordMatchResult(ctx, sessionID, "COMPLETED", winners, nil, time.Now())
	if err != nil {
		t.Fatalf("RecordMatchResult: %v", err)
	}
	if rated == nil {
		t.Fatal("teams match was not rated")
	}

	players, _ := replayAndLoad(t, st, gameID, rated.ModeKey)

	for _, u := range winners {
		r, ok := players[PlayerRatingKey(u)]
		if !ok {
			t.Fatalf("winning player %s has no rating", u)
		}
		if r.Mu <= prior {
			t.Fatalf("winning player %s mu = %v, want above the prior %v", u, r.Mu, prior)
		}
		if r.MatchesPlayed != 1 {
			t.Fatalf("winning player %s matches played = %d, want 1", u, r.MatchesPlayed)
		}
	}
	for _, u := range losers {
		r, ok := players[PlayerRatingKey(u)]
		if !ok {
			t.Fatalf("losing player %s has no rating", u)
		}
		if r.Mu >= prior {
			t.Fatalf("losing player %s mu = %v, want below the prior %v", u, r.Mu, prior)
		}
		if r.MatchesPlayed != 1 {
			t.Fatalf("losing player %s matches played = %d, want 1", u, r.MatchesPlayed)
		}
	}

	// The above only proves "every winner rose, every loser fell" — a pattern
	// four solo sides (two tied at rank 0, two tied at rank 1) would also
	// produce. Pin down that the seat template's affinity actually grouped
	// each team onto one rated side, not four: the persisted rating input
	// must have exactly two sides of two entrants each.
	inputs, err := st.ListRatingInputs(ctx, gameID, rated.ModeKey)
	if err != nil {
		t.Fatalf("ListRatingInputs: %v", err)
	}
	row := findRatingInputBySession(t, inputs, sessionID)
	if len(row.Sides) != 2 {
		t.Fatalf("rated sides = %d, want 2 (teammates must share a side, not four solo sides)", len(row.Sides))
	}
	for _, side := range row.Sides {
		if len(side.Entrants) != 2 {
			t.Fatalf("side rank %d has %d entrants, want 2 (both teammates on one side)", side.Rank, len(side.Entrants))
		}
	}
}

// Co-op — crew vs the reported scenario keys; a cleared hard scenario raises
// the crew and lowers scenario:hard.
func TestPipelineCooperativeRatesCrewAgainstScenario(t *testing.T) {
	ctx := context.Background()
	st, sessionID, gameID, users := newCooperativeSessionFixture(t)
	prior := ratingPrior(t)

	rated, err := st.RecordMatchResult(ctx, sessionID, "COMPLETED", users,
		map[string]any{"scenarios": []any{"hard"}}, time.Now())
	if err != nil {
		t.Fatalf("RecordMatchResult: %v", err)
	}
	if rated == nil {
		t.Fatal("cooperative match was not rated")
	}

	players, entities := replayAndLoad(t, st, gameID, rated.ModeKey)

	for _, u := range users {
		r, ok := players[PlayerRatingKey(u)]
		if !ok {
			t.Fatalf("crew member %s has no rating", u)
		}
		if r.Mu <= prior {
			t.Fatalf("crew member %s mu = %v, want above the prior %v after clearing the scenario", u, r.Mu, prior)
		}
		if r.MatchesPlayed != 1 {
			t.Fatalf("crew member %s matches played = %d, want 1", u, r.MatchesPlayed)
		}
	}

	hard, ok := entities["scenario:hard"]
	if !ok {
		t.Fatal("scenario:hard was never rated")
	}
	if hard.Mu >= prior {
		t.Fatalf("scenario:hard mu = %v, want below the prior %v after the crew cleared it", hard.Mu, prior)
	}
}

// Creative / no outcome — a game that reports nothing at all leaves every
// rating untouched. This also covers the invariant implied by the no-result
// row: an unrateable match writes no player_ratings rows at all.
func TestPipelineNoReportedResultLeavesRatingsUntouched(t *testing.T) {
	ctx := context.Background()
	st, sessionID, gameID, users := newCompetitiveSessionFixture(t, 2)

	// No RecordPlayerFinish, no RecordMatchResult: nothing was ever reported.
	// gameAndModeForSession reads the (game, mode) back off the session row
	// rather than assuming the fixture's "arena" literal, so this test
	// doesn't depend on a result ever having been recorded to learn the mode
	// key the way rated.ModeKey does elsewhere in this file.
	_, modeKey := gameAndModeForSession(t, st, ctx, sessionID)

	players, entities := replayAndLoad(t, st, gameID, modeKey)

	if len(players) != 0 {
		t.Fatalf("players = %v, want none for a match with no reported result", players)
	}
	if len(entities) != 0 {
		t.Fatalf("entities = %v, want none for a match with no reported result", entities)
	}

	// On its own, the assertion above passes identically whether the
	// record->replay->save path is alive and correctly wrote nothing, or is
	// entirely dead: rating_match_inputs is empty by construction here either
	// way. Anchor it: report a real result on this same session and confirm
	// the pipeline actually produces rows when there is something to rate.
	rated, err := st.RecordMatchResult(ctx, sessionID, "COMPLETED", []uuid.UUID{users[0]}, nil, time.Now())
	if err != nil {
		t.Fatalf("RecordMatchResult: %v", err)
	}
	if rated == nil {
		t.Fatal("match was not rated once a result was reported")
	}

	players, entities = replayAndLoad(t, st, gameID, rated.ModeKey)
	if len(players) != 2 {
		t.Fatalf("players = %v, want 2 once a result was reported", players)
	}
}

// Idempotency — the same result reported twice moves ratings exactly once.
func TestPipelineRepeatedReportDoesNotMoveRatingsTwice(t *testing.T) {
	ctx := context.Background()
	st, sessionID, gameID, users := newCompetitiveSessionFixture(t, 2)
	winner := users[0]
	reportedAt := time.Now()

	rated, err := st.RecordMatchResult(ctx, sessionID, "COMPLETED", []uuid.UUID{winner}, nil, reportedAt)
	if err != nil {
		t.Fatalf("first report: %v", err)
	}
	if rated == nil {
		t.Fatal("match was not rated")
	}

	beforePlayers, beforeEntities := replayAndLoad(t, st, gameID, rated.ModeKey)

	// Anchor the snapshot: rated != nil above only proves a rating input row
	// was written, not that replay actually produced ratings from it. Without
	// this, a dead ReplayMode/SaveAll/LoadPlayerRatings path would leave
	// beforePlayers empty and afterPlayers empty, and the DeepEqual below
	// would pass trivially over two empty maps.
	if len(beforePlayers) != 2 {
		t.Fatalf("beforePlayers = %v, want 2 rated players from the first report", beforePlayers)
	}
	for _, u := range users {
		r, ok := beforePlayers[PlayerRatingKey(u)]
		if !ok {
			t.Fatalf("player %s has no rating after the first report", u)
		}
		if r.MatchesPlayed != 1 {
			t.Fatalf("player %s matches played = %d, want 1 after the first report", u, r.MatchesPlayed)
		}
	}

	if _, err := st.RecordMatchResult(ctx, sessionID, "COMPLETED", []uuid.UUID{winner}, nil, reportedAt); err != nil {
		t.Fatalf("second report: %v", err)
	}

	afterPlayers, afterEntities := replayAndLoad(t, st, gameID, rated.ModeKey)

	if !reflect.DeepEqual(beforePlayers, afterPlayers) {
		t.Fatalf("player ratings moved on a repeated report:\n before %v\n after  %v", beforePlayers, afterPlayers)
	}
	if !reflect.DeepEqual(beforeEntities, afterEntities) {
		t.Fatalf("entity ratings moved on a repeated report:\n before %v\n after  %v", beforeEntities, afterEntities)
	}
}

// Disconnect — the disconnected player's rating is untouched while the
// others still move. "Untouched" here means no player_ratings row at all:
// RecordMatchResult excludes a DISCONNECT participant from every side (see
// TestRecordMatchResultExcludesDisconnectedPlayers), so a fresh user with no
// prior history simply never gets rated, rather than being rated and held
// constant.
func TestPipelineDisconnectedPlayerKeepsTheirRating(t *testing.T) {
	ctx := context.Background()
	st, sessionID, gameID, users := newCompetitiveSessionFixture(t, 3)
	winner, loser, disconnected := users[0], users[1], users[2]
	prior := ratingPrior(t)

	if err := st.RecordPlayerFinish(ctx, sessionID, disconnected, "DISCONNECT", nil, nil); err != nil {
		t.Fatalf("RecordPlayerFinish: %v", err)
	}

	rated, err := st.RecordMatchResult(ctx, sessionID, "COMPLETED", []uuid.UUID{winner}, nil, time.Now())
	if err != nil {
		t.Fatalf("RecordMatchResult: %v", err)
	}
	if rated == nil {
		t.Fatal("match was not rated")
	}

	players, _ := replayAndLoad(t, st, gameID, rated.ModeKey)

	if _, ok := players[PlayerRatingKey(disconnected)]; ok {
		t.Fatalf("disconnected player %s has a rating; want none written", disconnected)
	}

	w, ok := players[PlayerRatingKey(winner)]
	if !ok {
		t.Fatal("winner has no rating")
	}
	if w.Mu <= prior {
		t.Fatalf("winner mu = %v, want above the prior %v", w.Mu, prior)
	}
	if w.MatchesPlayed != 1 {
		t.Fatalf("winner matches played = %d, want 1", w.MatchesPlayed)
	}

	l, ok := players[PlayerRatingKey(loser)]
	if !ok {
		t.Fatal("loser has no rating")
	}
	if l.Mu >= prior {
		t.Fatalf("loser mu = %v, want below the prior %v", l.Mu, prior)
	}
	if l.MatchesPlayed != 1 {
		t.Fatalf("loser matches played = %d, want 1", l.MatchesPlayed)
	}
}

// All-winners with no scenario context — nothing moves. deriveRanks refuses
// to rate an outcome where every side is marked a winner (it carries no
// ranking), so RecordMatchResult must report the match as unrated and no
// rating input is ever appended for replay to find.
func TestPipelineEveryoneWinsLeavesRatingsUntouched(t *testing.T) {
	ctx := context.Background()
	st, sessionID, gameID, users := newCompetitiveSessionFixture(t, 2)

	rated, err := st.RecordMatchResult(ctx, sessionID, "COMPLETED", users, nil, time.Now())
	if err != nil {
		t.Fatalf("RecordMatchResult: %v", err)
	}
	if rated != nil {
		t.Fatalf("all-winners match was rated (mode %s); want it skipped since the outcome carries no ranking", rated.ModeKey)
	}

	_, modeKey := gameAndModeForSession(t, st, ctx, sessionID)
	players, entities := replayAndLoad(t, st, gameID, modeKey)
	if len(players) != 0 {
		t.Fatalf("players = %v, want none", players)
	}
	if len(entities) != 0 {
		t.Fatalf("entities = %v, want none", entities)
	}
}
