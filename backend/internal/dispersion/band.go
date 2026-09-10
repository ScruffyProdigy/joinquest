package dispersion

import (
	"time"

	"github.com/scruffyprodigy/joinquest/internal/rating"
)

// PopulationSpread is the widest the band ever opens: the spread of the rating
// population itself, which is the engine's prior sigma by construction. A band
// this wide accepts any lobby, which is the correct behaviour for a queue that
// cannot offer a choice — demanding tightness of a pool with nothing in it is
// precisely how a queue stalls.
const PopulationSpread = rating.UnratedSigma

// Band is the dispersion a lobby must sit inside to fire early, given the rate
// players are arriving on this line, the wait budget still affordable, and k.
//
// It is the looser of two numbers: what the rating model can call meaningful
// (Beta), and what the candidate pool can actually deliver. As arrivals rise
// the second falls below the first and Beta binds, so the band narrows with
// population and then bottoms out somewhere defensible instead of chasing zero.
//
// The pool term is an order-statistics approximation, not a law: choosing the
// tightest k out of N candidates drawn from a population of spread S leaves a
// subset spread on the order of S*k/N. It gives the right shape — monotone in
// arrivals, wide open when N <= k — and the constants get re-derived from
// measured dispersion once JQ-225 is recording it.
//
// A divergence from JQ-142's design worth knowing about. The design writes the
// pool as "N ~= lambda * t_needed", but t_needed is itself k/lambda, so that N
// is exactly k and the ratio k/N is always 1 — the formula is circular as
// written and cannot be implemented literally. What is meant is the pool the
// wait can actually accumulate, so N is taken over the affordable budget rather
// than over the time to gather k. That preserves every property the design
// argues for and is the only non-circular reading; it is flagged back to JQ-142
// rather than settled here.
func Band(lambda float64, budget time.Duration, k int) float64 {
	if k < 1 {
		k = 1
	}
	if lambda <= 0 || budget <= 0 {
		return PopulationSpread
	}

	expected := lambda * budget.Seconds()
	if expected <= float64(k) {
		// The pool cannot be chosen from, only accepted.
		return PopulationSpread
	}

	achievable := PopulationSpread * float64(k) / expected
	if achievable < Beta {
		return Beta
	}
	return achievable
}
