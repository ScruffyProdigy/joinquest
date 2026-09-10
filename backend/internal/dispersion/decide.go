package dispersion

import "time"

// Why a lobby fired, or why it has not yet.
//
// This is not decoration. "Fired because it was a good lobby" and "fired
// because we ran out of budget and took what was there" are opposite facts
// about matchmaking quality, and a fire event that does not distinguish them is
// one that cannot be analysed later. JQ-225 records the achieved dispersion;
// this records what produced it.
const (
	// ReasonDisabled: the mode has skill matching switched off. No rating was
	// read, no band was computed.
	ReasonDisabled = "disabled"
	// ReasonRateTooThin: waiting could not have paid. Not enough players are
	// arriving to accumulate candidates inside the affordable budget, so the
	// lobby fired unbiased.
	ReasonRateTooThin = "rate-too-thin"
	// ReasonBandSatisfied: the lobby was good enough. The happy path.
	ReasonBandSatisfied = "band-satisfied"
	// ReasonBudgetExhausted: the wait budget ran out and the best lobby
	// available fired regardless of its dispersion. This is the guarantee that
	// skill matching can never be a way to never fill a queue.
	ReasonBudgetExhausted = "budget-exhausted"
	// ReasonWaitingForBand: not a fire. The map is full but the lobby is worse
	// than the band and the budget can still afford to look for better.
	ReasonWaitingForBand = "waiting-for-band"
)

// Input is everything the fire condition's skill term needs.
//
// It carries no clock. OldestWait is a duration the caller measured, not a
// timestamp to subtract from time.Now(), so every decision here is reproducible
// from its inputs alone.
type Input struct {
	// SkillMatchingEnabled is the mode's kill switch. A mode whose skill signal
	// is weak or meaningless opts out here, which is a different question from
	// a mode with a thin population -- that one is answered by Lambda on its
	// own, and needs no switch.
	SkillMatchingEnabled bool
	// Lambda is arrivals per second on this line, from queuewait.Flow. For a
	// composition mode it is the scarcest path's rate, not the mode's total: a
	// lobby cannot complete faster than its slowest bucket fills.
	Lambda float64
	// Seats is the lobby's full size, which sets k.
	Seats int
	// OldestWait is how long the longest-waiting player on this line has been
	// waiting -- the anchor the budget is measured against.
	OldestWait time.Duration
	// Spread is the dispersion of the lobby currently sitting on the map.
	Spread Spread
}

// Decision is the second term on the fire condition. Today a lobby fires when
// the map is full; with this it fires when the map is full AND Decide says so.
type Decision struct {
	Fire   bool
	Reason string
}

// Decide answers whether a full map should fire now or wait for a better lobby.
//
// The order of the gates is the design, not an optimisation. Cheapest and most
// absolute first: a disabled mode never touches a rating, and a queue nobody is
// joining is answered without one too. Only a queue where waiting could
// actually pay gets as far as comparing dispersion against a band.
//
// The deferral it can return is an upper bound rather than a delay. A lobby
// that satisfies the band fires on the tick it does so, and one that never
// satisfies it fires when the budget runs out -- so the worst case is bounded
// by TCeiling against the oldest waiter, and lambda -> 0 collapses to today's
// behaviour exactly.
func Decide(in Input) Decision {
	if !in.SkillMatchingEnabled {
		return Decision{Fire: true, Reason: ReasonDisabled}
	}

	budget := TCeiling - in.OldestWait
	if budget <= 0 {
		// The oldest waiter has already spent the whole budget. Whatever is on
		// the map is what they get, and the queue cannot deadlock.
		return Decision{Fire: true, Reason: ReasonBudgetExhausted}
	}

	k := CandidatesWanted(in.Seats)
	if in.Lambda <= 0 {
		return Decision{Fire: true, Reason: ReasonRateTooThin}
	}
	// Time to accumulate k more candidates. Below one tick there is no
	// sub-tick holding, so the floor is a tick.
	needed := time.Duration(float64(k) / in.Lambda * float64(time.Second))
	if needed < Tick {
		needed = Tick
	}
	if needed > budget {
		// Waiting cannot pay: the candidates would arrive after the budget is
		// gone. Fire now, unbiased, rather than spending the wait for nothing.
		return Decision{Fire: true, Reason: ReasonRateTooThin}
	}

	if in.Spread.Acceptable(Band(in.Lambda, budget, k)) {
		return Decision{Fire: true, Reason: ReasonBandSatisfied}
	}
	return Decision{Fire: false, Reason: ReasonWaitingForBand}
}
