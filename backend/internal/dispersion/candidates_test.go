package dispersion

import "testing"

// k is round(seats/3): small relative to seat count, and scaling with it. The
// endpoints are what matter — k must never be zero, or a hold would wait for
// nobody, and it must stay near a third of the lobby, which is what bounds the
// overshoot against the advertised wait estimate.
func TestCandidatesWantedStaysNearAThirdOfTheLobbyAndNeverZero(t *testing.T) {
	for _, tc := range []struct {
		seats int
		want  int
	}{
		{seats: 1, want: 1}, // round(0.33) = 0, floored to 1: never wait for nobody
		{seats: 2, want: 1}, // a duel
		{seats: 4, want: 1}, // round(1.33)
		{seats: 5, want: 2}, // round(1.67)
		{seats: 6, want: 2},
		{seats: 10, want: 3}, // round(3.33)
	} {
		if got := CandidatesWanted(tc.seats); got != tc.want {
			t.Errorf("CandidatesWanted(%d) = %d, want %d", tc.seats, got, tc.want)
		}
	}
}

func TestCandidatesWantedIsOneForANonsenseSeatCount(t *testing.T) {
	if got := CandidatesWanted(0); got != 1 {
		t.Errorf("CandidatesWanted(0) = %d, want 1", got)
	}
}
