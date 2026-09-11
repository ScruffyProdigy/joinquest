package rating

import (
	"fmt"

	openskillrating "github.com/intinig/go-openskill/rating"
	"github.com/intinig/go-openskill/types"
)

// Predictor is an optional capability an Engine may also implement: turning
// ratings into a probability per side, before the match is played.
//
// It is deliberately not part of Engine. Rating a finished match and
// forecasting an unplayed one are different asks, and a model can be perfectly
// good at the first without offering the second — a rank-only or heuristic
// engine has ratings to order but no calibrated probability behind them.
// Folding this into Engine would either bar such a model from the codebase or
// invite it to return a made-up number, and a made-up probability is worse
// than none: the backtest harness would score it as though it meant something.
// As a separate interface, an engine that cannot answer simply does not
// implement it, and the harness reports "no probabilistic score" rather than a
// confident fiction.
type Predictor interface {
	// WinProbabilities returns each side's probability of placing first, in
	// the order the sides were given, summing to 1. Side.Rank is ignored:
	// this is the forecast made before the outcome is known.
	WinProbabilities(sides []Side) ([]float64, error)
}

// WinProbabilities implements Predictor for the Weng-Lin engine, through the
// same go-openskill fork that backs Rate — so the forecast and the update come
// from one model's constants rather than from two that can drift apart.
func (w *wengLin) WinProbabilities(sides []Side) ([]float64, error) {
	if len(sides) < 2 {
		return nil, fmt.Errorf("rating: win probabilities need at least 2 sides, got %d", len(sides))
	}

	teams := make([]types.Team, len(sides))
	for i, side := range sides {
		if len(side.Entrants) == 0 {
			return nil, fmt.Errorf("rating: side %d has no entrants", i)
		}
		team := make(types.Team, len(side.Entrants))
		for j, entrant := range side.Entrants {
			team[j] = types.Rating{Mu: entrant.Rating.Mu, Sigma: entrant.Rating.Sigma}
		}
		teams[i] = team
	}

	// The engine's own beta, not the package default: a forecast made under
	// different constants than the update that follows it would be scored
	// against ratings it does not describe, and the backtest harness reads
	// that disagreement as the model being wrong.
	probs := openskillrating.PredictWin(teams, w.options())
	if len(probs) != len(sides) {
		return nil, fmt.Errorf("rating: predict returned %d probabilities, want %d", len(probs), len(sides))
	}
	return probs, nil
}
