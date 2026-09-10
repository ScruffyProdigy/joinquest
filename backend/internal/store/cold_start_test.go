package store

import (
	"context"
	"encoding/json"
	"fmt"
	"math/rand"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/scruffyprodigy/joinquest/internal/coldstart"
	"github.com/scruffyprodigy/joinquest/internal/gameclient"
	"github.com/scruffyprodigy/joinquest/internal/rating"
)

// newColdStartFixture registers a game and n users, and returns them with no
// matches played.
//
// Deliberately lighter than newSessionFixture: cold-start seeding is fitted
// over player_ratings, not over match history, so these tests write the
// ratings they need directly through SaveRatings. Driving real matches would
// make a thirty-player population slow to build and would test the queue
// rather than the seeding.
func newColdStartFixture(t *testing.T, n int) (*Store, uuid.UUID, []uuid.UUID) {
	t.Helper()
	st := openTestStore(t)
	cleaner := st.NewTestCleaner(t)
	ctx := context.Background()

	slug := "coldstart-fixture-" + uuid.NewString()
	manifest := &gameclient.Manifest{
		Modes: []gameclient.ModeManifest{
			{Key: "arena", DisplayName: "Arena", SeatTemplate: json.RawMessage(`{"count":2}`)},
			{Key: "duel", DisplayName: "Duel", SeatTemplate: json.RawMessage(`{"count":2}`)},
		},
		Status:     gameclient.StatusResponse{Game: "Cold Start Fixture", Version: "1.0.0"},
		ETag:       `"coldstart-fixture"`,
		RawJSON:    []byte(`{"modes":[{"key":"arena"},{"key":"duel"}]}`),
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

	users := make([]uuid.UUID, n)
	for i := range users {
		user, err := st.CreateUser(ctx, CreateUserParams{Email: fmt.Sprintf("coldstart-%d-%s@example.com", i, uuid.NewString())})
		if err != nil {
			t.Fatalf("CreateUser: %v", err)
		}
		cleaner.TrackUser(user.ID)
		users[i] = user.ID
	}
	return st, result.Game.ID, users
}

// saveModeRatings caches one rating per user in a mode, as a replay would.
func saveModeRatings(t *testing.T, st *Store, gameID uuid.UUID, modeKey string, values map[string]RatingValue) {
	t.Helper()
	if err := st.SaveRatings(context.Background(), gameID, modeKey, "test-engine@1", time.Now(), values, nil); err != nil {
		t.Fatalf("SaveRatings(%s): %v", modeKey, err)
	}
}

// seedCorrelatedModes gives every user a converged rating in both modes, the
// second correlated with the first.
func seedCorrelatedModes(t *testing.T, st *Store, gameID uuid.UUID, users []uuid.UUID, rho float64) {
	t.Helper()
	rng := rand.New(rand.NewSource(99))

	arena := make(map[string]RatingValue, len(users))
	duel := make(map[string]RatingValue, len(users))
	for _, id := range users {
		source := rng.NormFloat64() * 5
		target := rho*source + rng.NormFloat64()*5*(1-rho*rho)
		// Sigma comfortably under coldstart.ConvergedSigma, so these count as
		// settled ratings rather than as players still near the prior.
		arena[PlayerRatingKey(id)] = RatingValue{Mu: 25 + source, Sigma: 2, MatchesPlayed: 20}
		duel[PlayerRatingKey(id)] = RatingValue{Mu: 25 + target, Sigma: 2, MatchesPlayed: 20}
	}
	saveModeRatings(t, st, gameID, "arena", arena)
	saveModeRatings(t, st, gameID, "duel", duel)
}

func TestListConvergedRatingsExcludesUnsettledRatings(t *testing.T) {
	st, gameID, users := newColdStartFixture(t, 3)
	ctx := context.Background()

	saveModeRatings(t, st, gameID, "arena", map[string]RatingValue{
		PlayerRatingKey(users[0]): {Mu: 30, Sigma: 2, MatchesPlayed: 20},
		PlayerRatingKey(users[1]): {Mu: 20, Sigma: coldstart.ConvergedSigma, MatchesPlayed: 9},
		// Still near the prior: its mu is mostly the prior's, so including it
		// would correlate players through what they share rather than through
		// anything either has shown.
		PlayerRatingKey(users[2]): {Mu: 25, Sigma: coldstart.ConvergedSigma + 0.5, MatchesPlayed: 1},
	})

	got, err := st.ListConvergedRatings(ctx, gameID, coldstart.ConvergedSigma)
	if err != nil {
		t.Fatalf("ListConvergedRatings: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("returned %d rating(s), want the 2 at or below the threshold: %+v", len(got), got)
	}
	for _, r := range got {
		if r.UserID == users[2] {
			t.Errorf("an unconverged rating (sigma above %v) was returned", coldstart.ConvergedSigma)
		}
	}
}

func TestModePairStatsRoundTrip(t *testing.T) {
	st, gameID, _ := newColdStartFixture(t, 1)
	ctx := context.Background()

	want := coldstart.PairStats{
		SourceMode: "arena", TargetMode: "duel",
		PairedPlayers: 120, Correlation: 0.83,
		SourceMean: 25.5, SourceSD: 4.25, TargetMean: 24.75, TargetSD: 5.125,
		ResidualSD: 3.5, FlatResidualSD: 6.25, Bias: -0.125,
		AbsErrorP50: 2.25, AbsErrorP90: 6.5,
	}
	if err := st.SaveModePairStats(ctx, gameID, time.Now(), []coldstart.PairStats{want}); err != nil {
		t.Fatalf("SaveModePairStats: %v", err)
	}

	loaded, err := st.LoadModePairStats(ctx, gameID, "duel")
	if err != nil {
		t.Fatalf("LoadModePairStats: %v", err)
	}
	got, ok := loaded["arena"]
	if !ok {
		t.Fatalf("no pair keyed by source mode %q: %+v", "arena", loaded)
	}
	if got != want {
		t.Errorf("round trip changed the measurement:\n got = %+v\nwant = %+v", got, want)
	}
}

// The lookup is by target mode, and the two directions of a pair are separate
// rows — reading one direction must not hand back the other.
func TestLoadModePairStatsIsDirectional(t *testing.T) {
	st, gameID, _ := newColdStartFixture(t, 1)
	ctx := context.Background()

	forward := coldstart.PairStats{SourceMode: "arena", TargetMode: "duel", PairedPlayers: 100, Correlation: 0.9, SourceSD: 5, TargetSD: 5, ResidualSD: 3, FlatResidualSD: 6}
	backward := coldstart.PairStats{SourceMode: "duel", TargetMode: "arena", PairedPlayers: 100, Correlation: 0.9, SourceSD: 5, TargetSD: 5, ResidualSD: 4, FlatResidualSD: 6}
	if err := st.SaveModePairStats(ctx, gameID, time.Now(), []coldstart.PairStats{forward, backward}); err != nil {
		t.Fatalf("SaveModePairStats: %v", err)
	}

	intoDuel, err := st.LoadModePairStats(ctx, gameID, "duel")
	if err != nil {
		t.Fatalf("LoadModePairStats: %v", err)
	}
	if len(intoDuel) != 1 || intoDuel["arena"].ResidualSD != 3 {
		t.Errorf("pairs into duel = %+v, want only arena->duel with residual 3", intoDuel)
	}
}

// A pair that has stopped being derivable must stop being served, which is why
// a save replaces the game's rows rather than upserting onto them.
func TestSaveModePairStatsReplacesPreviousPairs(t *testing.T) {
	st, gameID, _ := newColdStartFixture(t, 1)
	ctx := context.Background()

	first := []coldstart.PairStats{
		{SourceMode: "arena", TargetMode: "duel", PairedPlayers: 100, Correlation: 0.9, SourceSD: 5, TargetSD: 5, ResidualSD: 3, FlatResidualSD: 6},
		{SourceMode: "gauntlet", TargetMode: "duel", PairedPlayers: 100, Correlation: 0.8, SourceSD: 5, TargetSD: 5, ResidualSD: 4, FlatResidualSD: 6},
	}
	if err := st.SaveModePairStats(ctx, gameID, time.Now(), first); err != nil {
		t.Fatalf("SaveModePairStats: %v", err)
	}
	if err := st.SaveModePairStats(ctx, gameID, time.Now(), first[:1]); err != nil {
		t.Fatalf("SaveModePairStats (second): %v", err)
	}

	loaded, err := st.LoadModePairStats(ctx, gameID, "duel")
	if err != nil {
		t.Fatalf("LoadModePairStats: %v", err)
	}
	if _, stale := loaded["gauntlet"]; stale {
		t.Error("a pair dropped by the newer recompute is still being served")
	}
}

func TestListSeedSourcesExcludesTargetModeAndUnsettledRatings(t *testing.T) {
	st, gameID, users := newColdStartFixture(t, 1)
	ctx := context.Background()
	userID := users[0]

	saveModeRatings(t, st, gameID, "arena", map[string]RatingValue{
		PlayerRatingKey(userID): {Mu: 31, Sigma: 2, MatchesPlayed: 20},
	})
	saveModeRatings(t, st, gameID, "duel", map[string]RatingValue{
		PlayerRatingKey(userID): {Mu: 28, Sigma: 2, MatchesPlayed: 20},
	})

	got, err := st.ListSeedSources(ctx, gameID, "duel", []uuid.UUID{userID}, coldstart.ConvergedSigma)
	if err != nil {
		t.Fatalf("ListSeedSources: %v", err)
	}
	sources := got[userID]
	if len(sources) != 1 || sources[0].ModeKey != "arena" {
		t.Fatalf("sources = %+v, want only arena", sources)
	}
	if sources[0].Rating.Mu != 31 {
		t.Errorf("source mu = %v, want 31", sources[0].Rating.Mu)
	}
}

// First write wins: the row says what the player's earliest matches in this
// mode were built on, and a later read must not overwrite that.
func TestRecordRatingSeedKeepsTheFirstSeed(t *testing.T) {
	st, gameID, users := newColdStartFixture(t, 1)
	ctx := context.Background()
	userID := users[0]

	first := coldstart.Seed{
		Rating:     rating.Rating{Mu: 27, Sigma: 5},
		SourceMode: "arena", SourceMu: 31,
		Correlation: 0.85, PairedPlayers: 120,
	}
	second := coldstart.Seed{
		Rating:     rating.Rating{Mu: 20, Sigma: 6},
		SourceMode: "gauntlet", SourceMu: 12,
		Correlation: 0.4, PairedPlayers: 40,
	}
	for _, seed := range []coldstart.Seed{first, second} {
		if err := st.RecordRatingSeed(ctx, userID, gameID, "duel", seed, time.Now()); err != nil {
			t.Fatalf("RecordRatingSeed: %v", err)
		}
	}

	n, err := st.CountRatingSeeds(ctx, gameID, "duel")
	if err != nil {
		t.Fatalf("CountRatingSeeds: %v", err)
	}
	if n != 1 {
		t.Fatalf("recorded %d seed events for one player, want 1", n)
	}

	var sourceMode string
	var seededMu float64
	if err := st.db.QueryRowContext(ctx, `
		SELECT source_mode_key, seeded_mu FROM rating_seed_events
		WHERE user_id = $1 AND game_id = $2 AND mode_key = $3
	`, userID, gameID, "duel").Scan(&sourceMode, &seededMu); err != nil {
		t.Fatalf("read back seed event: %v", err)
	}
	if sourceMode != "arena" || seededMu != 27 {
		t.Errorf("stored seed is from %q at mu %v, want the first seed: arena at 27", sourceMode, seededMu)
	}
}

// The whole path against Postgres: real ratings in two modes, a scheduled
// refit over them, and a player who has played only one of the two coming out
// seeded rather than at the flat prior.
func TestRecomputeThenSeedOverRealRatings(t *testing.T) {
	// One more player than the evidence threshold needs, so the pair clears
	// MinPairedPlayers without the test resting on the exact boundary.
	st, gameID, users := newColdStartFixture(t, coldstart.MinPairedPlayers+10)
	ctx := context.Background()

	paired, newcomer := users[:len(users)-1], users[len(users)-1]
	seedCorrelatedModes(t, st, gameID, paired, 0.9)

	// The newcomer has a settled rating in arena only. Written after the
	// paired population so it does not join the pairs the fit measures.
	arena, err := st.LoadPlayerRatings(ctx, gameID, "arena")
	if err != nil {
		t.Fatalf("LoadPlayerRatings: %v", err)
	}
	arena[PlayerRatingKey(newcomer)] = RatingValue{Mu: 34, Sigma: 2, MatchesPlayed: 20}
	saveModeRatings(t, st, gameID, "arena", arena)

	report, err := coldstart.NewRecomputer(st).RecomputeGame(ctx, gameID)
	if err != nil {
		t.Fatalf("RecomputeGame: %v", err)
	}
	if report.Usable == 0 {
		t.Fatalf("refit found no usable pair over strongly correlated modes: %+v", report)
	}

	seeds, err := coldstart.NewSeeder(st, true).Seed(ctx, gameID, "duel", []uuid.UUID{newcomer})
	if err != nil {
		t.Fatalf("Seed: %v", err)
	}
	seed, ok := seeds[newcomer]
	if !ok {
		t.Fatal("a player with a settled arena rating was not seeded into duel")
	}

	// Above the mean, because they are a strong arena player — but pulled
	// back toward it, and never as confident as a played rating.
	if seed.Rating.Mu <= 25 {
		t.Errorf("seeded mu = %v, want above the population mean", seed.Rating.Mu)
	}
	if seed.Rating.Mu >= 34 {
		t.Errorf("seeded mu = %v, want regressed below the source rating of 34", seed.Rating.Mu)
	}
	if seed.Rating.Sigma < coldstart.SeedSigmaFloor {
		t.Errorf("seeded sigma = %v, tighter than the floor %v", seed.Rating.Sigma, coldstart.SeedSigmaFloor)
	}

	n, err := st.CountRatingSeeds(ctx, gameID, "duel")
	if err != nil {
		t.Fatalf("CountRatingSeeds: %v", err)
	}
	if n != 1 {
		t.Errorf("audit trail holds %d seed(s), want 1", n)
	}
}
