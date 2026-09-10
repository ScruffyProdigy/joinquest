package dispersion

import (
	"testing"
	"time"
)

// A wide lobby: SD well outside any band, so nothing but a gate or an expiry
// can make it fire.
func wideLobby() Spread { return Of([]float64{5, 15, 35, 45}) }

// A mode whose skill signal is meaningless should not pay a millisecond for it.
// The bypass is a bypass, not a wider band.
func TestSkillMatchingOffFiresImmediatelyWhateverTheLobbyLooksLike(t *testing.T) {
	got := Decide(Input{
		SkillMatchingEnabled: false,
		Lambda:               10,
		Seats:                4,
		Spread:               wideLobby(),
	})

	if !got.Fire {
		t.Errorf("Decide(disabled) = %+v, want a fire", got)
	}
	if got.Reason != ReasonDisabled {
		t.Errorf("Reason = %q, want %q", got.Reason, ReasonDisabled)
	}
}

// The lambda gate is the launch switch. At launch lambda is near zero
// everywhere, so the queue fires immediately and unbiased -- behaviourally
// identical to a queue with no skill matching at all. No flag, no threshold,
// no configured population floor.
func TestAQueueNobodyIsJoiningFiresOnTheFirstTickAndIgnoresRatings(t *testing.T) {
	got := Decide(Input{
		SkillMatchingEnabled: true,
		Lambda:               0,
		Seats:                4,
		OldestWait:           0,
		Spread:               wideLobby(),
	})

	if !got.Fire {
		t.Errorf("Decide(lambda=0) = %+v, want a fire", got)
	}
	if got.Reason != ReasonRateTooThin {
		t.Errorf("Reason = %q, want %q", got.Reason, ReasonRateTooThin)
	}
}

// Every seat at the prior scores dispersion zero. Vacuous, and also correct:
// it is the best estimate available, and the alternative is excluding unrated
// players from matchmaking entirely.
func TestALobbyWhereNobodyIsRatedFiresOnTheBand(t *testing.T) {
	got := Decide(Input{
		SkillMatchingEnabled: true,
		Lambda:               1,
		Seats:                4,
		OldestWait:           0,
		Spread:               Of([]float64{25, 25, 25, 25}),
	})

	if !got.Fire {
		t.Errorf("Decide(all unrated) = %+v, want a fire", got)
	}
	if got.Reason != ReasonBandSatisfied {
		t.Errorf("Reason = %q, want %q", got.Reason, ReasonBandSatisfied)
	}
}

// A party spanning more than the band can never satisfy it, because its members
// enter dispersion individually and are never split. The ceiling is what stops
// that being a deadlock -- this is the guarantee it is load-bearing for.
func TestAPartyWiderThanTheBandFiresWhenTheBudgetRunsOutRatherThanDeadlocking(t *testing.T) {
	in := Input{
		SkillMatchingEnabled: true,
		Lambda:               1,
		Seats:                4,
		OldestWait:           0,
		Spread:               wideLobby(),
	}

	if got := Decide(in); got.Fire {
		t.Fatalf("test is not exercising the ceiling: Decide(fresh) = %+v, want a deferral", got)
	}

	in.OldestWait = TCeiling
	got := Decide(in)
	if !got.Fire {
		t.Errorf("Decide(budget spent) = %+v, want a fire", got)
	}
	if got.Reason != ReasonBudgetExhausted {
		t.Errorf("Reason = %q, want %q", got.Reason, ReasonBudgetExhausted)
	}
}

// The budget is anchored to the oldest waiter, not the newest, because a second
// does not cost every player the same. A fresh queue can afford the full hold;
// a queue with somebody already deep into their wait can afford none. The
// endpoints are what matter, not the shape between them.
func TestTheBudgetIsAnchoredToTheOldestWaiterNotTheNewest(t *testing.T) {
	// k = round(4/3) = 1, so at lambda = 0.5 one more candidate is 2s away.
	in := Input{
		SkillMatchingEnabled: true,
		Lambda:               0.5,
		Seats:                4,
		Spread:               wideLobby(),
	}

	in.OldestWait = 0
	if got := Decide(in); got.Fire {
		t.Errorf("Decide(fresh queue) = %+v, want a deferral: the full budget is affordable", got)
	}

	in.OldestWait = 14 * time.Second
	got := Decide(in)
	if !got.Fire {
		t.Errorf("Decide(oldest waiter at 14s) = %+v, want a fire: 1s left cannot buy a 2s wait", got)
	}
	if got.Reason != ReasonRateTooThin {
		t.Errorf("Reason = %q, want %q", got.Reason, ReasonRateTooThin)
	}
}

// Hitting the band fires early. The deferral is an upper bound, never a delay
// that gets spent for its own sake.
func TestAGoodLobbyFiresEarlyRatherThanServingOutTheBudget(t *testing.T) {
	got := Decide(Input{
		SkillMatchingEnabled: true,
		Lambda:               1,
		Seats:                4,
		OldestWait:           0,
		Spread:               Of([]float64{24, 25, 26, 27}),
	})

	if !got.Fire {
		t.Errorf("Decide(tight lobby) = %+v, want an early fire", got)
	}
	if got.Reason != ReasonBandSatisfied {
		t.Errorf("Reason = %q, want %q", got.Reason, ReasonBandSatisfied)
	}
}
