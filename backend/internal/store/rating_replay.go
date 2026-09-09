package store

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/scruffyprodigy/joinquest/internal/rating"
)

// ratingStoreAdapter adapts *Store to rating.Store, following the
// liveCountsSource pattern in graph/resolver.go: a thin conversion layer
// between the database's shapes (uuid.UUID game ids, RatingInput,
// RatingValue) and the narrow port the consuming package (rating) defines
// for itself, so replay stays unit-testable without Postgres.
//
// It carries state between ListInputs and SaveAll within a single replay:
// matches-played counts aren't part of the rating.Store contract (a fake
// store in rating's own tests has no need of them), so the adapter derives
// them itself from the input log it already fetched.
type ratingStoreAdapter struct {
	st *Store

	matchesPlayed map[string]int
}

// RatingSource adapts the store to rating.Store for one replay run.
func (s *Store) RatingSource() rating.Store {
	return &ratingStoreAdapter{st: s}
}

func (a *ratingStoreAdapter) ListInputs(ctx context.Context, gameID, modeKey string) ([]rating.Input, error) {
	id, err := uuid.Parse(gameID)
	if err != nil {
		return nil, fmt.Errorf("store: parse game id %q: %w", gameID, err)
	}

	rows, err := a.st.ListRatingInputs(ctx, id, modeKey)
	if err != nil {
		return nil, err
	}

	counts := make(map[string]int)
	out := make([]rating.Input, len(rows))
	for i, row := range rows {
		sides := make([]rating.Side, len(row.Sides))
		for j, side := range row.Sides {
			// A side can never legitimately hold the same entrant key twice: dropping
			// duplicates here is unconditionally correct, not just a fix for merge's
			// current REPLACE (see carrySourceRatingHistoryTx). Without it, a duplicated
			// key double-counts one player as two teammates on the same side, or — split
			// across opposing sides — makes the merged player's placement depend on slice
			// order in the engine's ratings map, which is silently nondeterministic from
			// the caller's point of view.
			seen := make(map[string]bool, len(side.Entrants))
			entrants := make([]rating.Entrant, 0, len(side.Entrants))
			for _, e := range side.Entrants {
				if seen[e.Key] {
					continue
				}
				seen[e.Key] = true
				entrants = append(entrants, rating.Entrant{Key: e.Key})
				counts[e.Key]++
			}
			sides[j] = rating.Side{Entrants: entrants, Rank: side.Rank}
		}
		out[i] = rating.Input{SessionID: row.SessionID.String(), Sides: sides}
	}

	a.matchesPlayed = counts
	return out, nil
}

func (a *ratingStoreAdapter) SaveAll(ctx context.Context, gameID, modeKey, engineID string, players, entities map[string]rating.Rating) error {
	id, err := uuid.Parse(gameID)
	if err != nil {
		return fmt.Errorf("store: parse game id %q: %w", gameID, err)
	}

	playerValues := make(map[string]RatingValue, len(players))
	for key, r := range players {
		playerValues[key] = RatingValue{Mu: r.Mu, Sigma: r.Sigma, MatchesPlayed: a.matchesPlayed[key]}
	}
	entityValues := make(map[string]RatingValue, len(entities))
	for key, r := range entities {
		entityValues[key] = RatingValue{Mu: r.Mu, Sigma: r.Sigma, MatchesPlayed: a.matchesPlayed[key]}
	}

	return a.st.SaveRatings(ctx, id, modeKey, engineID, time.Now().UTC(), playerValues, entityValues)
}
