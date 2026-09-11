package rating

import (
	"fmt"
	"math"
	"strconv"

	openskillrating "github.com/intinig/go-openskill/rating"
	"github.com/intinig/go-openskill/types"
)

// DefaultBeta is the performance variance a mode uses until its own history
// says otherwise.
//
// Beta is the distance in skill that buys about an 80% chance of winning, so
// it encodes how strongly a rating gap predicts the result at all: a small
// beta says the better player nearly always wins, a large one says the mode is
// mostly luck. This value — half the prior sigma — is the go-openskill
// package's own default, restated here as a number we own rather than one we
// inherit silently. It is asserted rather than measured, and it is the same
// assertion for every mode in the catalog, which is why a mode that has enough
// history to do better should be measured instead (JQ-227).
//
// TestDefaultBetaMatchesPackageDefault fails if the package's default ever
// drifts away from this, so the restatement cannot go quietly stale.
const DefaultBeta = UnratedSigma / 2

// wengLin implements Engine using the go-openskill Weng-Lin (OpenSkill) package.
type wengLin struct {
	model string
	beta  float64
}

// NewWengLin returns an Engine backed by the go-openskill implementation,
// using DefaultBeta.
//
// model must be "plackett-luce" — currently the only model the package
// implements. Names like "bradley-terry" and "thurstone-mosteller" are not
// a recognized-but-unavailable case; they are rejected with the same plain
// unknown-model error as any made-up string, e.g. "elo". The string
// parameter stays even though only one value is accepted: NewWengLin has no
// caller outside this codebase — no public API, no game-developer surface,
// nothing external that reads a model name — so configurability across
// formulas would only be us talking to ourselves. If a second model is ever
// pulled in, swapping it in means writing a new Engine implementation
// behind this same interface, which the interface already supports; a name
// is added the day the formula behind it exists, not before.
func NewWengLin(model string) (Engine, error) {
	return NewWengLinBeta(model, DefaultBeta)
}

// NewWengLinBeta returns an Engine using an explicit performance variance.
//
// This is the per-mode constructor: a mode whose beta has been measured from
// its own history is rated by an engine built here, and every other mode falls
// back to NewWengLin. Beta must be positive — a zero or negative beta claims a
// skill gap of any size decides the match with certainty, which is not a
// weaker model but an incoherent one, and the arithmetic downstream would
// produce plausible numbers rather than fail.
func NewWengLinBeta(model string, beta float64) (Engine, error) {
	switch model {
	case "plackett-luce":
	default:
		return nil, fmt.Errorf("rating: unknown model %q", model)
	}
	if !(beta > 0) || math.IsInf(beta, 0) {
		return nil, fmt.Errorf("rating: beta must be a positive finite number, got %v", beta)
	}
	return &wengLin{model: model, beta: beta}, nil
}

// options carries this engine's constants into every go-openskill call, so a
// forecast and the update that follows it are made under the same beta rather
// than one explicit and one defaulted.
func (w *wengLin) options() *types.OpenSkillOptions {
	beta := w.beta
	return &types.OpenSkillOptions{Beta: &beta}
}

// Prior returns the package's default starting rating.
//
// Beta does not enter here. It describes how much noise sits between skill and
// the result, not where an unrated player is assumed to sit or how sure we are
// of it, so two modes with different betas still start their players in the
// same place — which is what lets UnratedSkill answer a lookup without knowing
// the mode's constants.
func (w *wengLin) Prior() Rating {
	r := openskillrating.New()
	return Rating{Mu: r.Mu, Sigma: r.Sigma}
}

// Rate converts sides into the package's teams-plus-ranks shape, rates
// them, and maps the result back preserving the input's shape and order.
func (w *wengLin) Rate(sides []Side) ([][]Rating, error) {
	teams := make([]types.Team, len(sides))
	ranks := make([]int, len(sides))
	for i, side := range sides {
		team := make(types.Team, len(side.Entrants))
		for j, entrant := range side.Entrants {
			team[j] = types.Rating{Mu: entrant.Rating.Mu, Sigma: entrant.Rating.Sigma}
		}
		teams[i] = team
		ranks[i] = side.Rank
	}

	opts := w.options()
	opts.Rank = ranks
	rated := openskillrating.Rate(teams, opts)

	out := make([][]Rating, len(rated))
	for i, team := range rated {
		row := make([]Rating, len(team))
		for j, r := range team {
			row[j] = Rating{Mu: r.Mu, Sigma: r.Sigma}
		}
		out[i] = row
	}
	return out, nil
}

// ID names the model and its constants version, for example
// "weng-lin/plackett-luce@1". The "@1" suffix is a hand-bumped constants
// version, not derived from the go-openskill package, so replayed history
// records exactly which constants produced a given rating.
//
// A measured beta is appended to that — "weng-lin/plackett-luce@1+beta=6.25" —
// which makes a change of beta a change of engine identity, and therefore
// visible to everything already built to notice one: stored ratings carry it,
// ListModesNeedingReplay compares against it, and a backtest report says which
// constants it actually scored. Ratings computed under different betas are not
// comparable, so a mode's history has to be replayed whole when its beta
// moves, and this is what makes that detectable rather than silent.
//
// The default beta appends nothing. Every mode in the catalog uses it until it
// is measured, so spelling it out would rename every stored engine id — and
// force a replay of every mode — to record constants that did not change.
func (w *wengLin) ID() string {
	id := "weng-lin/" + w.model + "@1"
	if w.beta != DefaultBeta {
		// 'g' with -1 precision is the shortest form that round-trips, so two
		// betas are the same id only if they are the same float64. A coarser
		// format would silently merge two distinct sets of constants under one
		// name, which is the exact confusion this suffix exists to prevent.
		id += "+beta=" + strconv.FormatFloat(w.beta, 'g', -1, 64)
	}
	return id
}
