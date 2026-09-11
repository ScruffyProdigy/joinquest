package ratingbacktest

import (
	"context"
	"fmt"
	"math"
	"math/rand"
	"testing"

	"github.com/scruffyprodigy/joinquest/internal/rating"
)

// syntheticDuels generates a mode's history from a known performance variance.
//
// Players are given fixed true skills and matched at random; the winner is
// drawn from the same probit form the engine forecasts with, so trueBeta means
// exactly what beta means to the model being fitted. That is the point of
// generating history rather than fixturing it: a small trueBeta produces a
// mode where the better player nearly always wins, a large one produces a mode
// that is mostly luck, and an estimator that cannot tell those apart has not
// measured anything.
//
// The seed is fixed, so a failure is reproducible rather than a coin flip in
// CI.
func syntheticDuels(t *testing.T, trueBeta float64, matches int, seed int64) []rating.Input {
	t.Helper()

	const players = 40
	skills := make([]float64, players)
	rng := rand.New(rand.NewSource(seed))
	for i := range skills {
		// A spread comparable to the rating scale, so skill gaps are the kind
		// of size the engine's prior expects to see.
		skills[i] = rng.NormFloat64() * rating.UnratedSigma
	}

	inputs := make([]rating.Input, 0, matches)
	for i := 0; i < matches; i++ {
		a := rng.Intn(players)
		b := rng.Intn(players)
		for b == a {
			b = rng.Intn(players)
		}

		// P(a beats b) under the same model the engine uses: the skill gap
		// divided by the combined performance noise of both sides.
		p := normalCDF((skills[a] - skills[b]) / (math.Sqrt2 * trueBeta))
		aWins := rng.Float64() < p

		rankA, rankB := 1, 0
		if aWins {
			rankA, rankB = 0, 1
		}
		inputs = append(inputs, rating.Input{
			SessionID: fmt.Sprintf("s%d", i),
			Sides: []rating.Side{
				{Entrants: []rating.Entrant{{Key: fmt.Sprintf("player:%d", a)}}, Rank: rankA},
				{Entrants: []rating.Entrant{{Key: fmt.Sprintf("player:%d", b)}}, Rank: rankB},
			},
		})
	}
	return inputs
}

func normalCDF(x float64) float64 {
	return 0.5 * math.Erfc(-x/math.Sqrt2)
}

func betaEngine(t *testing.T) BetaEngine {
	t.Helper()
	return func(beta float64) (rating.Engine, error) {
		return rating.NewWengLinBeta("plackett-luce", beta)
	}
}

func estimate(t *testing.T, inputs []rating.Input, grid BetaGrid) BetaEstimate {
	t.Helper()
	h, err := New(&fakeSource{inputs: inputs}, DefaultSplit)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	est, err := h.EstimateBeta(context.Background(), Target{GameID: "g", ModeKey: "duel"}, betaEngine(t), grid)
	if err != nil {
		t.Fatalf("EstimateBeta: %v", err)
	}
	return est
}

// TestEstimateBetaSeparatesSkillFromLuck is the claim the ticket rests on: the
// fit measures the thing it says it measures. Two synthetic modes are
// generated from betas a factor of eight apart — one where skill nearly
// decides the match, one where it barely moves the odds — and the estimates
// have to come back clearly apart and on the right sides.
//
// The assertion is deliberately about the ordering and the gap rather than
// about hitting each true beta on the nose. The grid is coarse on purpose (see
// DefaultBetaMultipliers), so recovering the exact generating value is not
// something this estimator promises; telling a skill-driven mode from a
// luck-driven one is.
func TestEstimateBetaSeparatesSkillFromLuck(t *testing.T) {
	const matches = 3000
	grid := BetaGrid{Default: rating.DefaultBeta}

	skillDriven := estimate(t, syntheticDuels(t, rating.DefaultBeta/4, matches, 1), grid)
	luckDriven := estimate(t, syntheticDuels(t, rating.DefaultBeta*4, matches, 2), grid)

	if !skillDriven.Measured {
		t.Fatalf("skill-driven mode came back unmeasured: %s", skillDriven.Unmeasured)
	}
	if !luckDriven.Measured {
		t.Fatalf("luck-driven mode came back unmeasured: %s", luckDriven.Unmeasured)
	}

	if !(skillDriven.Beta < luckDriven.Beta) {
		t.Fatalf("skill-driven beta %v is not below luck-driven beta %v: the fit does not tell them apart",
			skillDriven.Beta, luckDriven.Beta)
	}
	// Clearly distinguishable, not merely ordered: adjacent grid points sit a
	// third to a half apart, so a factor of two is well outside the grid's own
	// resolution.
	if ratio := luckDriven.Beta / skillDriven.Beta; ratio < 2 {
		t.Errorf("betas %v and %v differ by only %.2fx; want a clear separation",
			skillDriven.Beta, luckDriven.Beta, ratio)
	}
	if !(skillDriven.Beta < rating.DefaultBeta) {
		t.Errorf("skill-driven beta %v is not below the default %v", skillDriven.Beta, rating.DefaultBeta)
	}
	if !(luckDriven.Beta > rating.DefaultBeta) {
		t.Errorf("luck-driven beta %v is not above the default %v", luckDriven.Beta, rating.DefaultBeta)
	}

	// Fitting has to buy something, or there was no reason to fit: on history
	// generated well away from the default, the winning candidate must
	// forecast better than the constant it replaces.
	for _, est := range []BetaEstimate{skillDriven, luckDriven} {
		if !(est.LogLoss < est.DefaultLogLoss) {
			t.Errorf("beta %v scored %v against the default's %v; the fit bought nothing",
				est.Beta, est.LogLoss, est.DefaultLogLoss)
		}
	}
}

// TestEstimateBetaRisesWithTheLuckInTheMode is the other half of the same
// claim. An estimator that always ran to one end of its grid would pass the
// separation test while measuring nothing but the grid, so the estimates have
// to track the generating beta across its whole range, not merely sit apart at
// two points.
//
// It asserts the ordering rather than the value. The fitted beta lives on the
// engine's rating scale, and the generator's skills live on one this test
// chose; the two are related by a constant this method never claims to
// recover, so "beta went up when the mode got luckier" is the strongest true
// statement available — and it is the statement matchmaking needs, since what
// it asks of beta is which modes are worth matching strictly.
func TestEstimateBetaRisesWithTheLuckInTheMode(t *testing.T) {
	grid := BetaGrid{Default: rating.DefaultBeta}
	multipliers := []float64{0.125, 0.25, 0.5, 1, 2}

	var prev BetaEstimate
	for i, m := range multipliers {
		est := estimate(t, syntheticDuels(t, rating.DefaultBeta*m, 3000, int64(i)+100), grid)
		if !est.Measured {
			t.Fatalf("mode generated at %.3fx came back unmeasured: %s", m, est.Unmeasured)
		}
		if i > 0 && !(est.Beta >= prev.Beta) {
			t.Errorf("mode generated at %.3fx fitted to %v, below the %v fitted for the less lucky mode before it",
				m, est.Beta, prev.Beta)
		}
		// A luckier mode is genuinely harder to predict, so its held-out log
		// loss must rise too. If it did not, the modes are not what the
		// generator claims and the ordering above would prove nothing.
		if i > 0 && !(est.LogLoss > prev.LogLoss) {
			t.Errorf("mode generated at %.3fx scored log loss %v, not worse than the %v before it",
				m, est.LogLoss, prev.LogLoss)
		}
		prev = est
	}
}

// TestEstimateBetaFlagsAWinnerAtTheGridEdge keeps a bound from reading as a
// fit: a mode luckier than the widest candidate pins to that candidate, and
// the pinning has to be visible.
func TestEstimateBetaFlagsAWinnerAtTheGridEdge(t *testing.T) {
	est := estimate(t, syntheticDuels(t, rating.DefaultBeta*16, 3000, 200), BetaGrid{Default: rating.DefaultBeta})

	if !est.Measured {
		t.Fatalf("came back unmeasured: %s", est.Unmeasured)
	}
	if !est.AtGridEdge {
		t.Errorf("a mode far luckier than the grid fitted to %v (%.2fx) without flagging the edge",
			est.Beta, est.Beta/rating.DefaultBeta)
	}

	// And the flag must not fire on a fit the grid comfortably brackets.
	inner := estimate(t, syntheticDuels(t, rating.DefaultBeta/2, 3000, 201), BetaGrid{Default: rating.DefaultBeta})
	if inner.AtGridEdge {
		t.Errorf("a fit at %v (%.2fx) was flagged as sitting at the grid edge",
			inner.Beta, inner.Beta/rating.DefaultBeta)
	}
}

// TestEstimateBetaReportsThinHistoryAsUnmeasured is the acceptance criterion
// that keeps a young mode from being handed a number: below the floor the fit
// is reported, never adopted.
func TestEstimateBetaReportsThinHistoryAsUnmeasured(t *testing.T) {
	est := estimate(t, syntheticDuels(t, rating.DefaultBeta/4, 100, 4), BetaGrid{Default: rating.DefaultBeta})

	if est.Measured {
		t.Errorf("a mode with %d scored held-out matches was adopted; the floor is %d", est.ProbMatches, est.MinProbMatches)
	}
	if est.Beta != rating.DefaultBeta {
		t.Errorf("unmeasured mode got beta %v, want the default %v", est.Beta, rating.DefaultBeta)
	}
	if est.Unmeasured == "" {
		t.Error("unmeasured estimate gave no reason")
	}
	// The sample size still has to be reported: "we did not measure this" and
	// "we have no idea how close it came" are different statements.
	if est.ProbMatches == 0 {
		t.Error("no sample size reported alongside the unmeasured estimate")
	}
	if len(est.Candidates) == 0 {
		t.Error("no candidate curve reported: a thin fit is still worth looking at, it is just not worth adopting")
	}
}

// TestEstimateBetaAlwaysCarriesItsSampleSize covers the acceptance criterion
// directly, on the measured path too.
func TestEstimateBetaAlwaysCarriesItsSampleSize(t *testing.T) {
	est := estimate(t, syntheticDuels(t, rating.DefaultBeta, 3000, 5), BetaGrid{Default: rating.DefaultBeta})

	if est.ProbMatches <= 0 {
		t.Fatalf("ProbMatches = %d, want the number of scored held-out matches", est.ProbMatches)
	}
	if est.ProbMatches > est.HeldOut {
		t.Errorf("ProbMatches %d exceeds HeldOut %d", est.ProbMatches, est.HeldOut)
	}
	if est.HeldOut > est.Matches {
		t.Errorf("HeldOut %d exceeds Matches %d", est.HeldOut, est.Matches)
	}
	for _, c := range est.Candidates {
		if c.Score.ProbMatches != est.ProbMatches {
			t.Errorf("candidate beta %v scored %d matches, but the estimate reports %d; every candidate must be judged on identical history",
				c.Beta, c.Score.ProbMatches, est.ProbMatches)
		}
	}
}

// TestEstimateBetaAlwaysScoresTheDefault keeps "what did fitting buy?"
// answerable even when the caller's grid does not include the fallback.
func TestEstimateBetaAlwaysScoresTheDefault(t *testing.T) {
	est := estimate(t, syntheticDuels(t, rating.DefaultBeta, 1500, 6), BetaGrid{
		Default:     rating.DefaultBeta,
		Multipliers: []float64{0.25, 4}, // deliberately excludes 1
	})

	if est.DefaultLogLoss == 0 {
		t.Fatal("the default was not scored")
	}
	var sawDefault bool
	for _, c := range est.Candidates {
		if c.Beta == rating.DefaultBeta {
			sawDefault = true
		}
	}
	if !sawDefault {
		t.Error("the default beta is missing from the candidate curve")
	}
}

// TestEstimateBetaCandidatesAreAscendingAndUnique pins the reproducibility the
// tie-break depends on.
func TestEstimateBetaCandidatesAreAscendingAndUnique(t *testing.T) {
	est := estimate(t, syntheticDuels(t, rating.DefaultBeta, 300, 7), BetaGrid{
		Default:     rating.DefaultBeta,
		Multipliers: []float64{2, 0.5, 1, 2, 1},
	})

	if len(est.Candidates) != 3 {
		t.Fatalf("got %d candidates from a grid of 3 distinct multipliers, want 3", len(est.Candidates))
	}
	for i := 1; i < len(est.Candidates); i++ {
		if !(est.Candidates[i-1].Beta < est.Candidates[i].Beta) {
			t.Errorf("candidates are not strictly ascending at %d: %v then %v",
				i, est.Candidates[i-1].Beta, est.Candidates[i].Beta)
		}
	}
}

// TestEstimateBetaRejectsAnIncoherentGrid keeps a caller from asking for a
// search that has no meaning.
func TestEstimateBetaRejectsAnIncoherentGrid(t *testing.T) {
	h, err := New(&fakeSource{}, DefaultSplit)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	for name, grid := range map[string]BetaGrid{
		"zero default":     {Default: 0},
		"negative default": {Default: -1},
		"zero multiplier":  {Default: rating.DefaultBeta, Multipliers: []float64{0}},
		"negative floor":   {Default: rating.DefaultBeta, MinProbMatches: -1},
	} {
		if _, err := h.EstimateBeta(context.Background(), Target{}, betaEngine(t), grid); err == nil {
			t.Errorf("%s: got nil error, want a rejection", name)
		}
	}
	if _, err := h.EstimateBeta(context.Background(), Target{}, nil, BetaGrid{Default: rating.DefaultBeta}); err == nil {
		t.Error("nil BetaEngine: got nil error, want a rejection")
	}
}

// TestEstimateBetaNeedsAForecast covers the engine that can rank but not
// forecast: beta cannot be fitted on ordering, because it does not change the
// order at all.
func TestEstimateBetaNeedsAForecast(t *testing.T) {
	h, err := New(&fakeSource{inputs: syntheticDuels(t, rating.DefaultBeta, 1500, 8)}, DefaultSplit)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	est, err := h.EstimateBeta(context.Background(), Target{GameID: "g", ModeKey: "duel"},
		func(beta float64) (rating.Engine, error) { return rankOnlyEngine{beta: beta}, nil },
		BetaGrid{Default: rating.DefaultBeta})
	if err != nil {
		t.Fatalf("EstimateBeta: %v", err)
	}
	if est.Measured {
		t.Error("a non-forecasting engine produced a measured beta")
	}
	if est.Beta != rating.DefaultBeta {
		t.Errorf("got beta %v, want the default %v", est.Beta, rating.DefaultBeta)
	}
	if est.Unmeasured == "" {
		t.Error("no reason given")
	}
}

// rankOnlyEngine rates without forecasting — deliberately not a
// rating.Predictor.
type rankOnlyEngine struct{ beta float64 }

func (rankOnlyEngine) Prior() rating.Rating {
	return rating.Rating{Mu: rating.UnratedMu, Sigma: rating.UnratedSigma}
}

func (e rankOnlyEngine) Rate(sides []rating.Side) ([][]rating.Rating, error) {
	out := make([][]rating.Rating, len(sides))
	for i, side := range sides {
		row := make([]rating.Rating, len(side.Entrants))
		for j, entrant := range side.Entrants {
			row[j] = entrant.Rating
		}
		out[i] = row
	}
	return out, nil
}

func (e rankOnlyEngine) ID() string { return fmt.Sprintf("rank-only@1+beta=%v", e.beta) }
