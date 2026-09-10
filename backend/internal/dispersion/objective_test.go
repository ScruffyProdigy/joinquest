package dispersion

import "testing"

// teamAverageGap is the objective this package deliberately does NOT use: the
// difference between the two sides' mean rating. It lives in the test as the
// rejected alternative, so the case where the two disagree is written down
// rather than asserted about in prose.
func teamAverageGap(a, b []float64) float64 {
	mean := func(xs []float64) float64 {
		sum := 0.0
		for _, x := range xs {
			sum += x
		}
		return sum / float64(len(xs))
	}
	gap := mean(a) - mean(b)
	if gap < 0 {
		return -gap
	}
	return gap
}

// The lobby team-average balance prefers is the one nobody enjoys: a strong
// player carrying a weak one against two average players reads as perfectly
// balanced, because the two sides average the same. Every seat in it is
// mismatched.
func TestWholeLobbyDispersionPrefersADifferentLobbyThanTeamAverageBalance(t *testing.T) {
	// Sides average 25 and 25 — perfectly "balanced" — but the seats span 30.
	carriedTeamA, carriedTeamB := []float64{40, 10}, []float64{25, 25}
	// Sides average 23 and 27 — "unbalanced" by 4 — but every seat is within 6.
	tightTeamA, tightTeamB := []float64{22, 24}, []float64{26, 28}

	carried := Of(append(append([]float64{}, carriedTeamA...), carriedTeamB...))
	tight := Of(append(append([]float64{}, tightTeamA...), tightTeamB...))

	if teamAverageGap(carriedTeamA, carriedTeamB) >= teamAverageGap(tightTeamA, tightTeamB) {
		t.Fatal("test is not exercising the disagreement: team-average balance must prefer the carried lobby")
	}
	if !tight.Tighter(carried) {
		t.Errorf("Tighter: dispersion must prefer the tight lobby; tight = %+v, carried = %+v", tight, carried)
	}
}
