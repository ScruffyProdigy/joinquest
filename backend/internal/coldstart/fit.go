package coldstart

import (
	"math"
	"sort"

	"github.com/scruffyprodigy/joinquest/internal/rating"
)

// folds is how many parts the paired population is cut into when measuring
// how well the seeding predicts a player it has not seen.
//
// Five is the usual compromise, and the reason it has to be more than one is
// the whole point of this file: a fit scored against the same players it was
// fitted on reports how well a line was drawn through known points, not how
// well the next player is placed. At MinPairedPlayers the training side of
// each fold still holds twenty-four players, enough for the moments to mean
// something.
const folds = 5

// Observation is one player who holds converged ratings in both modes of a
// pair — the unit the correlation and the backtest are both computed over.
type Observation struct {
	// PlayerKey identifies the player. It is used only to order the
	// observations before they are split into folds, so that the same
	// population always lands in the same folds and a refit does not shuffle
	// the measured residual for reasons unrelated to the data.
	PlayerKey string

	// SourceMu and TargetMu are the player's rating centres in the two modes.
	SourceMu float64
	TargetMu float64
}

// Fit measures one direction of one mode pair: the correlation over the whole
// paired population, and the error distribution that seeding produced when it
// was held out from part of it.
//
// The returned PairStats is always populated and never an error. A pair with
// too little history, no relationship, or a residual worse than the flat
// prior's comes back with numbers that PairStats.Usable rejects, and it is
// worth storing exactly that: a row saying "measured, and it does not work
// here" is the answer to why a mode is not being seeded, where a missing row
// is indistinguishable from a recompute that never ran.
//
// # Why the backtest scores the seeding, not the regression
//
// The held-out predictions run through predict, the same function that serves
// a live seed, UpwardCaution included. Scoring the bare regression instead
// would set sigma from the spread of a thing we do not ship. The asymmetric
// shrink deliberately trades a little accuracy for safety on the upside, and
// that cost belongs in the measured residual rather than hidden from it.
//
// # Why the residual is an RMSE and not a standard deviation
//
// ResidualSD is the root mean square of the held-out errors about zero, not
// their spread about their own mean. A seeding that is wrong by four points
// every single time has a standard deviation of zero and is not a confident
// prediction; the player still lands four points from where they belong.
// Taking the RMSE folds any systematic bias into the sigma it earns, and Bias
// reports the same quantity separately so a human can see whether a pair is
// noisy or simply skewed.
func Fit(source, target string, obs []Observation) PairStats {
	stats := PairStats{
		SourceMode:    source,
		TargetMode:    target,
		PairedPlayers: len(obs),
	}

	ordered := append([]Observation(nil), obs...)
	sort.Slice(ordered, func(i, j int) bool { return ordered[i].PlayerKey < ordered[j].PlayerKey })

	m, ok := moments(ordered)
	if !ok {
		return stats
	}
	stats.SourceMean, stats.SourceSD = m.SourceMean, m.SourceSD
	stats.TargetMean, stats.TargetSD = m.TargetMean, m.TargetSD
	stats.Correlation = m.Correlation

	if len(ordered) < folds {
		// Nothing can be held out meaningfully, so no residual is reported
		// and Usable refuses the pair. Such a pair is already below
		// MinPairedPlayers by a wide margin; this guard is here so that
		// Fit is safe on any input rather than only on inputs that passed
		// an earlier check.
		return stats
	}

	seedErrs, flatErrs := heldOutErrors(ordered, stats)
	if len(seedErrs) == 0 {
		return stats
	}

	stats.ResidualSD = rms(seedErrs)
	stats.FlatResidualSD = rms(flatErrs)
	stats.Bias = mean(seedErrs)
	stats.AbsErrorP50 = absQuantile(seedErrs, 0.50)
	stats.AbsErrorP90 = absQuantile(seedErrs, 0.90)
	return stats
}

// heldOutErrors walks the folds, fitting on every other fold and predicting
// the one left out, and returns the seeding's signed errors alongside the flat
// prior's over exactly the same players.
//
// Scoring both baselines over the identical held-out set is what makes
// PairStats.Usable's "beats the flat prior" clause a comparison rather than
// two unrelated numbers. Observations are assigned to folds by position in the
// key-sorted slice, so the split is deterministic: the same population
// measures the same way on every recompute, and a change in ResidualSD is a
// change in the data.
func heldOutErrors(obs []Observation, full PairStats) (seedErrs, flatErrs []float64) {
	for f := 0; f < folds; f++ {
		var train, test []Observation
		for i, o := range obs {
			if i%folds == f {
				test = append(test, o)
			} else {
				train = append(train, o)
			}
		}
		if len(test) == 0 {
			continue
		}

		m, ok := moments(train)
		if !ok {
			continue
		}
		fold := PairStats{
			SourceMode: full.SourceMode,
			TargetMode: full.TargetMode,
			SourceMean: m.SourceMean,
			SourceSD:   m.SourceSD,
			TargetMean: m.TargetMean,
			TargetSD:   m.TargetSD,
			// A fold that measures a negative correlation is left as it is
			// rather than clamped to zero. Usable refuses negative pairs at
			// serve time, but a fold measuring one is evidence the pair is
			// unstable, and letting that show up as a large residual is how
			// the instability reaches the decision.
			Correlation: m.Correlation,
		}

		for _, o := range test {
			seedErrs = append(seedErrs, predict(fold, o.SourceMu)-o.TargetMu)
			flatErrs = append(flatErrs, rating.UnratedMu-o.TargetMu)
		}
	}
	return seedErrs, flatErrs
}

// pairMoments is the summary of a paired population that predict needs.
type pairMoments struct {
	SourceMean, SourceSD float64
	TargetMean, TargetSD float64
	Correlation          float64
}

// moments computes the means, spreads and correlation of a paired population.
// ok is false when there are fewer than two observations or either mode's
// spread is zero — a population with no variation to speak of, from which no
// standing can be mapped onto anything.
func moments(obs []Observation) (pairMoments, bool) {
	n := float64(len(obs))
	if len(obs) < 2 {
		return pairMoments{}, false
	}

	var sumS, sumT float64
	for _, o := range obs {
		sumS += o.SourceMu
		sumT += o.TargetMu
	}
	meanS, meanT := sumS/n, sumT/n

	var varS, varT, cov float64
	for _, o := range obs {
		ds, dt := o.SourceMu-meanS, o.TargetMu-meanT
		varS += ds * ds
		varT += dt * dt
		cov += ds * dt
	}

	// Population (not sample) denominators throughout: the correlation is a
	// ratio in which n cancels, and SourceSD and TargetSD are only ever used
	// as the ratio TargetSD/SourceSD inside predict, where it cancels again.
	// Consistency between them matters; the choice of denominator does not.
	sdS, sdT := math.Sqrt(varS/n), math.Sqrt(varT/n)
	if sdS == 0 || sdT == 0 {
		return pairMoments{}, false
	}

	return pairMoments{
		SourceMean:  meanS,
		SourceSD:    sdS,
		TargetMean:  meanT,
		TargetSD:    sdT,
		Correlation: cov / math.Sqrt(varS*varT),
	}, true
}

// rms is the root mean square of the errors — their size about zero rather
// than about their own mean. See Fit's comment on why.
func rms(errs []float64) float64 {
	if len(errs) == 0 {
		return 0
	}
	var total float64
	for _, e := range errs {
		total += e * e
	}
	return math.Sqrt(total / float64(len(errs)))
}

func mean(errs []float64) float64 {
	if len(errs) == 0 {
		return 0
	}
	var total float64
	for _, e := range errs {
		total += e
	}
	return total / float64(len(errs))
}

// absQuantile is the q-th quantile of the absolute errors, by nearest rank.
// Nearest rank rather than an interpolating definition because the number is
// read as "the error at the 90th percentile player", and a real player's error
// is a more useful thing to report than a value between two of them.
func absQuantile(errs []float64, q float64) float64 {
	if len(errs) == 0 {
		return 0
	}
	abs := make([]float64, len(errs))
	for i, e := range errs {
		abs[i] = math.Abs(e)
	}
	sort.Float64s(abs)

	idx := int(math.Ceil(q*float64(len(abs)))) - 1
	if idx < 0 {
		idx = 0
	}
	if idx >= len(abs) {
		idx = len(abs) - 1
	}
	return abs[idx]
}
