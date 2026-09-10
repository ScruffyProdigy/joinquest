package ratingbacktest

import (
	"fmt"
	"math"
	"math/rand"

	"github.com/scruffyprodigy/joinquest/internal/rating"
)

// The engines below exist so the harness can be tested against models whose
// relative quality is known in advance. That is the whole basis of
// TestHarnessDistinguishesBetterFromWorse: a harness that reports a tie
// between a model that learns and one that cannot is worse than no harness at
// all, because it looks like evidence. Proving it separates known-different
// engines is the only way to know it would separate two real ones.

// staticEngine never learns: it hands back exactly the ratings it was given.
//
// Over history generated from real latent skill it should score at chance,
// since every side always looks identical to it. It deliberately does not
// implement rating.Predictor, which also exercises the harness's handling of
// an engine that cannot forecast.
type staticEngine struct{}

func (staticEngine) Prior() rating.Rating { return rating.Rating{Mu: 25, Sigma: 25.0 / 3.0} }
func (staticEngine) ID() string           { return "test/static@1" }

func (staticEngine) Rate(sides []rating.Side) ([][]rating.Rating, error) {
	out := make([][]rating.Rating, len(sides))
	for i, side := range sides {
		row := make([]rating.Rating, len(side.Entrants))
		for j, e := range side.Entrants {
			row[j] = e.Rating
		}
		out[i] = row
	}
	return out, nil
}

// invertedEngine is a real model wired up backwards: it updates as though the
// losers had won. It learns just as hard as the engine it wraps, in precisely
// the wrong direction, so it should score below chance rather than at it —
// which is a sharper test than a model that merely knows nothing.
//
// It forecasts through the wrapped engine's Predictor over its own inverted
// ratings, so its probabilities are confidently wrong. That is what makes it
// separable on log-loss and not only on accuracy.
type invertedEngine struct {
	inner rating.Engine
}

func (e invertedEngine) Prior() rating.Rating { return e.inner.Prior() }
func (e invertedEngine) ID() string           { return "test/inverted@1" }

func (e invertedEngine) Rate(sides []rating.Side) ([][]rating.Rating, error) {
	// Reverse the ranks: best becomes worst, preserving ties.
	worst := 0
	for _, s := range sides {
		if s.Rank > worst {
			worst = s.Rank
		}
	}
	flipped := make([]rating.Side, len(sides))
	for i, s := range sides {
		flipped[i] = rating.Side{Entrants: s.Entrants, Rank: worst - s.Rank}
	}
	return e.inner.Rate(flipped)
}

func (e invertedEngine) WinProbabilities(sides []rating.Side) ([]float64, error) {
	p, ok := e.inner.(rating.Predictor)
	if !ok {
		return nil, fmt.Errorf("inner engine does not predict")
	}
	return p.WinProbabilities(sides)
}

// syntheticHistory generates duels between players with known latent skills.
//
// The winner is drawn from a Bradley-Terry model on the skill gap, so the
// history really was produced by skill plus noise: a model that recovers
// player strength must beat one that does not, and the harness is being asked
// to notice a difference that is genuinely there. Seeding the generator makes
// the fixture itself reproducible, so a failure here is a change in the
// harness and never a change in the dice.
func syntheticHistory(players, matches int, spread float64, seed int64) (inputs []rating.Input, skills []float64) {
	rng := rand.New(rand.NewSource(seed))

	skills = make([]float64, players)
	for i := range skills {
		// Evenly spaced latent skills across [-spread, +spread].
		skills[i] = -spread + 2*spread*float64(i)/float64(players-1)
	}

	inputs = make([]rating.Input, 0, matches)
	for m := range matches {
		a := rng.Intn(players)
		b := rng.Intn(players - 1)
		if b >= a {
			b++
		}

		pAWins := 1 / (1 + math.Exp(-(skills[a] - skills[b])))
		winner, loser := a, b
		if rng.Float64() >= pAWins {
			winner, loser = b, a
		}

		inputs = append(inputs, rating.Input{
			SessionID: fmt.Sprintf("s%04d", m),
			Sides: []rating.Side{
				{Rank: 0, Entrants: []rating.Entrant{{Key: fmt.Sprintf("player:p%02d", winner)}}},
				{Rank: 1, Entrants: []rating.Entrant{{Key: fmt.Sprintf("player:p%02d", loser)}}},
			},
		})
	}
	return inputs, skills
}
