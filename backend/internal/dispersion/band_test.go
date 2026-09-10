package dispersion

import (
	"math"
	"testing"
	"time"

	"github.com/scruffyprodigy/joinquest/internal/rating"
)

// A queue nobody is joining can deliver no choice at all, so demanding
// tightness of it is exactly how a queue stalls. The band opens to the whole
// population spread.
func TestBandOpensToThePopulationSpreadWhenNobodyIsArriving(t *testing.T) {
	got := Band(0, TCeiling, 2)

	if want := rating.UnratedSigma; math.Abs(got-want) > 1e-9 {
		t.Errorf("Band(lambda=0) = %v, want the population spread %v", got, want)
	}
}

// Fewer expected arrivals than seats to fill means the pool cannot be chosen
// from, only accepted, so the band stays wide open.
func TestBandStaysOpenWhenFewerCandidatesArriveThanSeatsToFill(t *testing.T) {
	// 0.05/s over 15s is 0.75 expected arrivals against k = 2.
	got := Band(0.05, TCeiling, 2)

	if want := rating.UnratedSigma; math.Abs(got-want) > 1e-9 {
		t.Errorf("Band(thin) = %v, want the population spread %v", got, want)
	}
}

// Once the pool is deep enough to choose from, the achievable tightness falls
// below what the rating model can call meaningful, and Beta binds instead.
// The band bottoms out there rather than chasing zero.
func TestBandBottomsOutAtBetaRatherThanChasingZero(t *testing.T) {
	if got := Band(0.5, TCeiling, 2); math.Abs(got-Beta) > 1e-9 {
		t.Errorf("Band(busy) = %v, want Beta %v", got, Beta)
	}
	if got := Band(1000, TCeiling, 2); math.Abs(got-Beta) > 1e-9 {
		t.Errorf("Band(very busy) = %v, want Beta %v", got, Beta)
	}
}

func TestBandNarrowsMonotonicallyAsArrivalsRise(t *testing.T) {
	prev := math.Inf(1)
	for _, lambda := range []float64{0, 0.05, 0.1, 0.15, 0.2, 0.3, 0.5, 1, 5} {
		got := Band(lambda, TCeiling, 2)
		if got > prev {
			t.Errorf("Band(%v) = %v widened from %v; the band must never widen as arrivals rise", lambda, got, prev)
		}
		if got < Beta {
			t.Errorf("Band(%v) = %v fell below Beta %v", lambda, got, Beta)
		}
		prev = got
	}
}

// A budget already spent buys no candidates, whatever the arrival rate.
func TestBandOpensWhenThereIsNoBudgetLeftToSpend(t *testing.T) {
	got := Band(5, 0*time.Second, 2)

	if want := rating.UnratedSigma; math.Abs(got-want) > 1e-9 {
		t.Errorf("Band(no budget) = %v, want the population spread %v", got, want)
	}
}
