package coldstart

import (
	"context"
	"fmt"
	"log"
	"sort"
	"time"

	"github.com/google/uuid"
)

// ConvergedRating is one player's settled rating in one mode of a game.
//
// It lives here rather than in the store so that *store.Store satisfies the
// ports below with no adapter in between. rating.Store takes the opposite
// approach — string ids, a hand-written adapter — because the engine is
// deliberately ignorant of what a player is. Nothing here has that reason to
// be ignorant: seeding is about players specifically, and an adapter that
// only re-typed the same fields would be a layer to keep in sync for no
// separation it actually buys.
type ConvergedRating struct {
	UserID  uuid.UUID
	ModeKey string
	Mu      float64
}

// FitStore is the port the scheduled recompute reads and writes through.
type FitStore interface {
	// ListGamesWithRatings returns every game with cached player ratings.
	ListGamesWithRatings(ctx context.Context) ([]uuid.UUID, error)

	// ListConvergedRatings returns every rating in a game at or below
	// maxSigma, across all its modes.
	ListConvergedRatings(ctx context.Context, gameID uuid.UUID, maxSigma float64) ([]ConvergedRating, error)

	// SaveModePairStats replaces the game's measured pairs.
	SaveModePairStats(ctx context.Context, gameID uuid.UUID, at time.Time, stats []PairStats) error
}

// Recomputer refits every mode pair in every game on a schedule.
//
// A full refit rather than an incremental update. The fit is over a
// population's moments and a cross-validated residual, both of which every
// new paired player changes slightly, so there is no correct way to fold one
// player in without redoing the arithmetic. The work is proportional to the
// number of converged ratings, not to match history, and it runs on a slow
// tick well away from any request path.
type Recomputer struct {
	store FitStore
	now   func() time.Time
}

// NewRecomputer builds a Recomputer over store.
func NewRecomputer(store FitStore) *Recomputer {
	return &Recomputer{store: store, now: time.Now}
}

// GameReport summarises one game's refit.
type GameReport struct {
	GameID uuid.UUID
	// Pairs is how many ordered mode pairs were measured, and Usable how
	// many of those cleared PairStats.Usable. The two are reported
	// separately because "we measured nothing" and "we measured and none of
	// it works" are different situations with different fixes.
	Pairs  int
	Usable int
}

// RecomputeAll refits every game and returns one report per game.
//
// A game whose refit fails is logged and skipped rather than aborting the
// pass: one game's ratings being briefly unreadable is not a reason to leave
// every other game's pairs stale, and the next tick retries it anyway.
func (r *Recomputer) RecomputeAll(ctx context.Context) ([]GameReport, error) {
	games, err := r.store.ListGamesWithRatings(ctx)
	if err != nil {
		return nil, fmt.Errorf("coldstart: list games: %w", err)
	}

	reports := make([]GameReport, 0, len(games))
	for _, gameID := range games {
		report, err := r.RecomputeGame(ctx, gameID)
		if err != nil {
			log.Printf("coldstart: recompute %s: %v", gameID, err)
			continue
		}
		reports = append(reports, report)
	}
	return reports, nil
}

// RecomputeGame refits every ordered pair of modes in one game and replaces
// that game's stored pairs with the result.
func (r *Recomputer) RecomputeGame(ctx context.Context, gameID uuid.UUID) (GameReport, error) {
	ratings, err := r.store.ListConvergedRatings(ctx, gameID, ConvergedSigma)
	if err != nil {
		return GameReport{}, fmt.Errorf("coldstart: list converged ratings: %w", err)
	}

	stats := FitPairs(ratings)

	usable := 0
	for _, p := range stats {
		if p.Usable() {
			usable++
		}
	}

	if err := r.store.SaveModePairStats(ctx, gameID, r.now(), stats); err != nil {
		return GameReport{}, fmt.Errorf("coldstart: save pairs: %w", err)
	}

	return GameReport{GameID: gameID, Pairs: len(stats), Usable: usable}, nil
}

// FitPairs measures every ordered pair of modes present in one game's
// converged ratings.
//
// Both directions of every pair are fitted, because they are two different
// measurements: predicting a widely-spread mode from a tight one is not as
// precise as the reverse, and the residual that sets a seed's sigma differs
// accordingly.
//
// A pair with no players in common is left out entirely — there is nothing to
// report about it, and a row of zeroes would read as a measurement that came
// out empty rather than as one that was never possible. A pair that does have
// players in common is kept even when the fit is unusable, so that "we
// measured this and it does not work" is on the record and distinguishable
// from a recompute that never ran.
//
// Modes are iterated in sorted order so the returned slice is stable, which
// is what lets two recomputes over unchanged ratings write identical rows.
func FitPairs(ratings []ConvergedRating) []PairStats {
	byMode := make(map[string]map[uuid.UUID]float64)
	for _, r := range ratings {
		if byMode[r.ModeKey] == nil {
			byMode[r.ModeKey] = make(map[uuid.UUID]float64)
		}
		byMode[r.ModeKey][r.UserID] = r.Mu
	}

	modes := make([]string, 0, len(byMode))
	for mode := range byMode {
		modes = append(modes, mode)
	}
	sort.Strings(modes)

	var out []PairStats
	for _, source := range modes {
		for _, target := range modes {
			if source == target {
				continue
			}
			obs := pairedObservations(byMode[source], byMode[target])
			if len(obs) == 0 {
				continue
			}
			out = append(out, Fit(source, target, obs))
		}
	}
	return out
}

// pairedObservations returns the players holding a rating in both modes.
//
// Iteration is over the source map and the result is sorted by player key, so
// nothing about Go's map ordering reaches the fit — Fit sorts again before
// splitting folds, and the two together mean the measurement depends on the
// data alone.
func pairedObservations(source, target map[uuid.UUID]float64) []Observation {
	var obs []Observation
	for userID, sourceMu := range source {
		targetMu, ok := target[userID]
		if !ok {
			continue
		}
		obs = append(obs, Observation{
			PlayerKey: userID.String(),
			SourceMu:  sourceMu,
			TargetMu:  targetMu,
		})
	}
	sort.Slice(obs, func(i, j int) bool { return obs[i].PlayerKey < obs[j].PlayerKey })
	return obs
}

// Start refits on a tick until ctx is cancelled.
//
// This is a plain ticker rather than a worker package of its own, unlike
// ratingworker and formingworker. Those exist to coalesce per-entity work
// scheduled from a request path; there is no such work here. The recompute is
// one periodic full pass over every game, nothing schedules it, and wrapping
// that in a dirty set and a drain loop would be machinery with nothing to do.
//
// It refits once before entering the loop so a process that has just started
// is not serving pairs from before its last restart for a whole interval.
func (r *Recomputer) Start(ctx context.Context, every time.Duration) {
	r.runOnce(ctx)

	ticker := time.NewTicker(every)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			r.runOnce(ctx)
		}
	}
}

func (r *Recomputer) runOnce(ctx context.Context) {
	reports, err := r.RecomputeAll(ctx)
	if err != nil {
		log.Printf("coldstart: recompute: %v", err)
		return
	}

	// One line per pass, and only when there was something to say. A mode
	// pair crossing MinPairedPlayers for the first time is the moment
	// seeding starts affecting real players, and this is the only place that
	// shows up.
	pairs, usable := 0, 0
	for _, rep := range reports {
		pairs += rep.Pairs
		usable += rep.Usable
	}
	if pairs > 0 {
		log.Printf("coldstart: refit %d mode pair(s) across %d game(s); %d usable for seeding", pairs, len(reports), usable)
	}
}
