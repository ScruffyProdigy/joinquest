package ratingbacktest

import (
	"context"
	"fmt"
	"math"
	"sort"

	"github.com/scruffyprodigy/joinquest/internal/rating"
)

// MinProbMatches is how many scored held-out matches a mode must produce
// before its fitted beta is adopted rather than merely reported.
//
// Beta is one parameter of a binary-outcome model, and the grid below spaces
// its candidates roughly half an order of magnitude apart precisely because a
// few hundred outcomes can separate neighbours that far apart and cannot
// separate a 5% difference. Two hundred is the point where the winning
// candidate stops changing from one re-run to the next on synthetic history
// generated at a known beta; below it the argmin wanders, and adopting the
// wander would dress noise up as a measurement.
//
// It is a floor picked to be revisited, not a derived quantity — the honest
// state of the open question on JQ-227. What protects a reader from it being
// wrong is that BetaEstimate.ProbMatches travels with every estimate, so a
// thin mode reads as thin whatever the floor happens to be.
const MinProbMatches = 200

// DefaultBetaMultipliers is the grid of candidate betas, as multiples of the
// mode's fallback beta.
//
// Geometric rather than linear, because beta is a scale: the difference
// between 0.5x and 0.75x is the same kind of difference as between 4x and 6x,
// and a linear grid would spend most of its candidates on the luck-heavy end
// where the likelihood is flattest. The span reaches from a mode where a small
// skill gap is nearly decisive to one that is close to a coin flip, which is
// the range the catalog actually has to cover — an RPSLR duel and a high-luck
// party game are both meant to land inside it.
//
// The grid is deliberately coarse. A finer one would report differences it
// cannot support at the sample sizes any mode here has, and the point of the
// exercise is to tell a skill-driven mode from a luck-driven one, not to
// resolve beta to three digits.
var DefaultBetaMultipliers = []float64{
	0.125, 0.1875, 0.25, 0.375, 0.5, 0.75, 1, 1.5, 2, 3, 4, 6, 8,
}

// BetaEngine builds the engine to score at one candidate beta.
//
// The caller supplies it for the same reason Run takes engines rather than
// model names: this package scores models, it does not choose them. Naming
// "plackett-luce" here would put the catalog's model choice inside the
// measuring instrument.
type BetaEngine func(beta float64) (rating.Engine, error)

// BetaGrid is the search this estimator runs.
type BetaGrid struct {
	// Default is the beta a mode falls back to when nothing is measured. It
	// anchors Multipliers and is always scored, so every estimate can say
	// what fitting actually bought over the assertion it replaces.
	Default float64

	// Multipliers are the candidates, as multiples of Default. Empty means
	// DefaultBetaMultipliers.
	Multipliers []float64

	// MinProbMatches is the adoption floor. Zero means MinProbMatches.
	MinProbMatches int
}

func (g BetaGrid) validate() error {
	if !(g.Default > 0) || math.IsInf(g.Default, 0) {
		return fmt.Errorf("backtest: beta grid Default must be a positive finite number, got %v", g.Default)
	}
	if g.MinProbMatches < 0 {
		return fmt.Errorf("backtest: beta grid MinProbMatches must not be negative, got %d", g.MinProbMatches)
	}
	for _, m := range g.Multipliers {
		if !(m > 0) || math.IsInf(m, 0) {
			return fmt.Errorf("backtest: beta grid multiplier must be a positive finite number, got %v", m)
		}
	}
	return nil
}

// betas returns the candidate betas in ascending order, deduplicated.
//
// Ascending and deduplicated so the search is reproducible: the winner is the
// first candidate holding the lowest log loss, and on an exact tie that is the
// smaller beta — which is arbitrary but fixed, where iteration in caller order
// would let the same history produce two different answers.
func (g BetaGrid) betas() []float64 {
	mults := g.Multipliers
	if len(mults) == 0 {
		mults = DefaultBetaMultipliers
	}

	seen := make(map[float64]bool, len(mults)+1)
	out := make([]float64, 0, len(mults)+1)
	add := func(b float64) {
		if b > 0 && !seen[b] {
			seen[b] = true
			out = append(out, b)
		}
	}
	for _, m := range mults {
		add(g.Default * m)
	}
	// The fallback is always in the running. An estimate that cannot say how
	// the value it replaces would have scored is not a comparison.
	add(g.Default)

	sort.Float64s(out)
	return out
}

func (g BetaGrid) floor() int {
	if g.MinProbMatches == 0 {
		return MinProbMatches
	}
	return g.MinProbMatches
}

// BetaCandidate is one grid point and how it did over the held-out matches.
type BetaCandidate struct {
	Beta     float64
	EngineID string
	Score    Score
}

// BetaEstimate is one mode's fitted performance variance, and everything a
// reader needs in order to decide whether to believe it.
type BetaEstimate struct {
	Target Target

	// Beta is the value to use for this mode: the fitted one when Measured,
	// and Default otherwise. It is never absent, so a caller reading only
	// this field gets a usable number rather than a zero.
	Beta float64

	// Default is the fallback the fit was measured against.
	Default float64

	// Measured says whether the fit is trustworthy enough to adopt. When it
	// is false Unmeasured says why, and Beta holds Default — the mode keeps
	// the assertion rather than taking on a number fitted to noise.
	Measured   bool
	Unmeasured string

	// Matches, HeldOut and ProbMatches size the evidence. ProbMatches is the
	// one that matters: it counts the held-out matches with a single outright
	// winner, which are the only ones a win-probability forecast can be
	// scored against, and it is therefore the sample the fit was actually
	// selected on. It travels with the estimate everywhere the estimate
	// goes, so a thin mode reads as thin.
	Matches     int
	HeldOut     int
	ProbMatches int

	// MinProbMatches is the floor that was in force, so a report read later
	// says what "too little history" meant at the time.
	MinProbMatches int

	// LogLoss is the winning candidate's mean held-out log loss, and
	// DefaultLogLoss is the fallback's over the identical history. The two
	// together are what says whether measuring bought anything: a mode where
	// they are equal has learned that the assertion was already right, which
	// is a result and not a failure.
	LogLoss        float64
	DefaultLogLoss float64

	// AtGridEdge says the winner is the smallest or largest candidate tried,
	// so the value that would actually minimise log loss may lie outside the
	// grid entirely and this estimate is a bound rather than a fit. A mode
	// luckier than the widest candidate pins to it silently otherwise, and
	// reads as a confident measurement of the grid's own edge.
	AtGridEdge bool

	// Candidates is the whole grid, ascending, with each point's score. The
	// curve is worth more than its minimum: a flat one means the history
	// cannot separate a skill-driven mode from a luck-driven one no matter
	// how many matches it holds, and the argmin of a flat curve is noise
	// wearing a decimal point.
	Candidates []BetaCandidate
}

// EstimateBeta fits one mode's performance variance from its own retained
// history.
//
// The method is the same prequential walk Run scores engines with, repeated
// once per candidate beta: each candidate rates the training prefix, then
// forecasts and rates its way through the held-out suffix, and is scored only
// on forecasts made before it saw the result. The candidate with the lowest
// held-out log loss wins. Beta is chosen on how well it predicts, because
// predicting is what beta is for.
//
// Fitting on the held-out suffix rather than on the training prefix is what
// keeps this from being circular. Beta does not only tune the forecast — it
// changes the ratings the forecast is made from, since it sets how far one
// result moves a player. A candidate is therefore run end to end under its own
// constants, and judged on matches it had not seen.
//
// A mode with too little history is not fitted. It keeps Default and is
// reported as unmeasured, with the sample size that fell short.
func (h *Harness) EstimateBeta(ctx context.Context, target Target, newEngine BetaEngine, grid BetaGrid) (BetaEstimate, error) {
	if newEngine == nil {
		return BetaEstimate{}, fmt.Errorf("backtest: a BetaEngine is required")
	}
	if err := grid.validate(); err != nil {
		return BetaEstimate{}, err
	}

	inputs, err := h.source.ListInputs(ctx, target.GameID, target.ModeKey)
	if err != nil {
		return BetaEstimate{}, fmt.Errorf("backtest: list inputs for %s/%s: %w", target.GameID, target.ModeKey, err)
	}
	train := h.split.trainCount(len(inputs))

	est := BetaEstimate{
		Target:         target,
		Beta:           grid.Default,
		Default:        grid.Default,
		Matches:        len(inputs),
		HeldOut:        len(inputs) - train,
		MinProbMatches: grid.floor(),
	}

	betas := grid.betas()
	est.Candidates = make([]BetaCandidate, 0, len(betas))

	best, bestLogLoss := -1, math.Inf(1)
	for _, beta := range betas {
		engine, err := newEngine(beta)
		if err != nil {
			return BetaEstimate{}, fmt.Errorf("backtest: build engine at beta %v: %w", beta, err)
		}
		score, err := scoreEngine(engine, inputs, train)
		if err != nil {
			return BetaEstimate{}, fmt.Errorf("backtest: score beta %v over %s/%s: %w", beta, target.GameID, target.ModeKey, err)
		}
		if !score.Probabilistic {
			// Without a forecast there is no log loss, and beta cannot be
			// fitted on ordering alone: it does not change the order the
			// ratings imply, only how confident that order is. Saying so
			// beats returning the grid's arbitrary first point.
			est.Unmeasured = "engine does not forecast, so beta has nothing to be fitted on"
			est.Candidates = nil
			return est, nil
		}

		est.Candidates = append(est.Candidates, BetaCandidate{Beta: beta, EngineID: engine.ID(), Score: score})
		if beta == grid.Default {
			est.ProbMatches = score.ProbMatches
			est.DefaultLogLoss = score.LogLoss
		}
		if score.ProbMatches > 0 && score.LogLoss < bestLogLoss {
			best, bestLogLoss = len(est.Candidates)-1, score.LogLoss
		}
	}

	switch {
	case best < 0:
		// Every candidate scored zero forecastable matches: a mode of solo
		// results, or one whose held-out suffix is all shared first places.
		est.Unmeasured = "no held-out match had a single outright winner to forecast"
	case est.ProbMatches < est.MinProbMatches:
		est.Unmeasured = fmt.Sprintf("only %d scored held-out match(es), below the floor of %d", est.ProbMatches, est.MinProbMatches)
	default:
		est.Measured = true
		est.Beta = est.Candidates[best].Beta
		est.LogLoss = est.Candidates[best].Score.LogLoss
		est.AtGridEdge = best == 0 || best == len(est.Candidates)-1
	}
	return est, nil
}
