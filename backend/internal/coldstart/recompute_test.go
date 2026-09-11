package coldstart

import (
	"context"
	"fmt"
	"math/rand"
	"testing"
	"time"

	"github.com/google/uuid"
)

// fitStore is an in-memory FitStore, so the recompute's shape can be tested
// without Postgres. The store's own SQL is covered in
// internal/store/cold_start_test.go.
type fitStore struct {
	games    []uuid.UUID
	ratings  map[uuid.UUID][]ConvergedRating
	maxSigma float64
	saved    map[uuid.UUID][]PairStats
	saveErr  error
}

func newFitStore() *fitStore {
	return &fitStore{
		ratings: make(map[uuid.UUID][]ConvergedRating),
		saved:   make(map[uuid.UUID][]PairStats),
	}
}

func (f *fitStore) ListGamesWithRatings(context.Context) ([]uuid.UUID, error) {
	return f.games, nil
}

func (f *fitStore) ListConvergedRatings(_ context.Context, gameID uuid.UUID, maxSigma float64) ([]ConvergedRating, error) {
	f.maxSigma = maxSigma
	return f.ratings[gameID], nil
}

func (f *fitStore) SaveModePairStats(_ context.Context, gameID uuid.UUID, _ time.Time, stats []PairStats) error {
	if f.saveErr != nil {
		return f.saveErr
	}
	f.saved[gameID] = stats
	return nil
}

// correlatedGame builds a game whose two modes are related, plus a third mode
// nobody who plays the first two has touched.
func correlatedGame(n int, rho float64) []ConvergedRating {
	rng := rand.New(rand.NewSource(4242))
	var out []ConvergedRating
	for i := 0; i < n; i++ {
		userID := uuid.NewSHA1(uuid.Nil, []byte(fmt.Sprintf("player-%d", i)))
		source := rng.NormFloat64() * 5
		target := rho*source + rng.NormFloat64()*5*(1-rho*rho)
		out = append(out,
			ConvergedRating{UserID: userID, ModeKey: "arena", Mu: 25 + source},
			ConvergedRating{UserID: userID, ModeKey: "duel", Mu: 25 + target},
		)
	}
	// A mode played only by someone who has played nothing else: it shares no
	// player with arena or duel, so no pair involving it is measurable.
	out = append(out, ConvergedRating{
		UserID:  uuid.NewSHA1(uuid.Nil, []byte("lonely")),
		ModeKey: "gauntlet",
		Mu:      25,
	})
	return out
}

// Both directions of a pair are measured, because they are two different
// questions with two different answers.
func TestFitPairsMeasuresBothDirections(t *testing.T) {
	stats := FitPairs(correlatedGame(200, 0.85))

	var forward, backward bool
	for _, p := range stats {
		if p.SourceMode == "arena" && p.TargetMode == "duel" {
			forward = true
		}
		if p.SourceMode == "duel" && p.TargetMode == "arena" {
			backward = true
		}
		if p.SourceMode == p.TargetMode {
			t.Errorf("FitPairs produced a self-pair for %q", p.SourceMode)
		}
	}
	if !forward || !backward {
		t.Errorf("arena->duel measured = %v, duel->arena measured = %v; want both", forward, backward)
	}
}

// A pair with nobody in common is not a measurement that came out empty, it
// is one that was never possible — so it is left out rather than stored as a
// row of zeroes that reads like a finding.
func TestFitPairsSkipsModesWithNoSharedPlayers(t *testing.T) {
	for _, p := range FitPairs(correlatedGame(60, 0.85)) {
		if p.SourceMode == "gauntlet" || p.TargetMode == "gauntlet" {
			t.Errorf("measured a pair with no shared players: %s -> %s over %d players",
				p.SourceMode, p.TargetMode, p.PairedPlayers)
		}
	}
}

// The recompute runs on a schedule, so two passes over unchanged ratings must
// write identical rows — otherwise every stored sigma moves for reasons that
// have nothing to do with the players.
func TestFitPairsIsDeterministic(t *testing.T) {
	ratings := correlatedGame(120, 0.7)

	first, second := FitPairs(ratings), FitPairs(ratings)
	if len(first) != len(second) {
		t.Fatalf("pair counts differ: %d then %d", len(first), len(second))
	}
	for i := range first {
		if first[i] != second[i] {
			t.Errorf("pair %d differs:\nfirst  = %+v\nsecond = %+v", i, first[i], second[i])
		}
	}
}

func TestRecomputeGameSavesAndCountsUsablePairs(t *testing.T) {
	gameID := uuid.New()
	fs := newFitStore()
	fs.games = []uuid.UUID{gameID}
	fs.ratings[gameID] = correlatedGame(300, 0.9)

	report, err := NewRecomputer(fs).RecomputeGame(context.Background(), gameID)
	if err != nil {
		t.Fatalf("RecomputeGame: %v", err)
	}
	if report.Pairs != len(fs.saved[gameID]) {
		t.Errorf("report says %d pairs, store was given %d", report.Pairs, len(fs.saved[gameID]))
	}
	if report.Usable == 0 {
		t.Errorf("no usable pairs from a strongly correlated game: %+v", fs.saved[gameID])
	}
	if fs.maxSigma != ConvergedSigma {
		t.Errorf("read ratings at sigma <= %v, want ConvergedSigma %v", fs.maxSigma, ConvergedSigma)
	}
}

// A pair that has been measured and does not work is stored anyway. A missing
// row cannot be told apart from a recompute that never ran, and "we checked,
// and these two modes predict nothing about each other" is the answer to why
// a mode is not being seeded.
func TestRecomputeStoresMeasuredButUnusablePairs(t *testing.T) {
	gameID := uuid.New()
	fs := newFitStore()
	fs.games = []uuid.UUID{gameID}
	fs.ratings[gameID] = correlatedGame(300, 0.0)

	report, err := NewRecomputer(fs).RecomputeGame(context.Background(), gameID)
	if err != nil {
		t.Fatalf("RecomputeGame: %v", err)
	}
	if report.Pairs == 0 {
		t.Fatal("uncorrelated modes produced no stored pairs; the recompute left no record that it looked")
	}
	if report.Usable != 0 {
		t.Errorf("%d usable pairs from uncorrelated modes, want none", report.Usable)
	}
}

// One game failing must not cost every other game its refit — the next tick
// retries it either way.
func TestRecomputeAllContinuesPastAFailingGame(t *testing.T) {
	bad, good := uuid.New(), uuid.New()
	fs := newFitStore()
	fs.games = []uuid.UUID{bad, good}
	fs.ratings[bad] = correlatedGame(300, 0.9)
	fs.ratings[good] = correlatedGame(300, 0.9)
	fs.saveErr = fmt.Errorf("boom")

	// Every save fails, so nothing is reported; the point is that RecomputeAll
	// returns rather than aborting on the first game.
	reports, err := NewRecomputer(fs).RecomputeAll(context.Background())
	if err != nil {
		t.Fatalf("RecomputeAll: %v", err)
	}
	if len(reports) != 0 {
		t.Errorf("reports = %+v, want none when every save failed", reports)
	}

	// With saving restored, both games are refitted on the next pass.
	fs.saveErr = nil
	reports, err = NewRecomputer(fs).RecomputeAll(context.Background())
	if err != nil {
		t.Fatalf("RecomputeAll: %v", err)
	}
	if len(reports) != 2 {
		t.Errorf("refitted %d game(s), want 2", len(reports))
	}
}
