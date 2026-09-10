package dispersion

import "math"

// Spread is how far apart a candidate lobby's seats sit on the rating scale.
//
// Two numbers rather than one, because they reject different lobbies. SD is
// what the search minimises: it describes the lobby as a whole, which is the
// objective — teammates and opponents together, never the gap between team
// averages. Range is the outlier check SD cannot make, because a single distant
// player barely moves the standard deviation of a lobby that is otherwise
// tight, and that player is exactly the one the objective exists to protect.
type Spread struct {
	// SD is the population standard deviation of mu across every seat.
	SD float64
	// Range is max - min: the widest gap the lobby contains.
	Range float64
}

// Of measures the spread of a lobby from its seated players' mu.
//
// Population rather than sample standard deviation: these seats are the whole
// lobby, not a draw from it, so there is nothing to correct for. An empty or
// single-seat lobby has no spread, which reads downstream as "nothing to
// object to" — the honest answer, and the one that cannot divide by zero.
func Of(mus []float64) Spread {
	if len(mus) < 2 {
		return Spread{}
	}

	sum, min, max := 0.0, mus[0], mus[0]
	for _, mu := range mus {
		sum += mu
		min = math.Min(min, mu)
		max = math.Max(max, mu)
	}
	mean := sum / float64(len(mus))

	variance := 0.0
	for _, mu := range mus {
		d := mu - mean
		variance += d * d
	}
	variance /= float64(len(mus))

	return Spread{SD: math.Sqrt(variance), Range: max - min}
}

// Tighter reports whether s is the better lobby of the two.
//
// SD decides it, because SD is the objective. Range is a rejection test rather
// than a ranking one: a lobby that fails the cap is not "worse", it is not
// eligible, and Acceptable is where that is asked.
func (s Spread) Tighter(other Spread) bool {
	return s.SD < other.SD
}
