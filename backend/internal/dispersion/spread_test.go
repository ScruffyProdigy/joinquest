package dispersion

import (
	"math"
	"testing"
)

func TestOfReportsPopulationSDAndRange(t *testing.T) {
	// Four seats at 20, 24, 26, 30: mean 25, deviations -5, -1, 1, 5.
	// Population variance is (25 + 1 + 1 + 25) / 4 = 13.
	got := Of([]float64{20, 24, 26, 30})

	if want := math.Sqrt(13); math.Abs(got.SD-want) > 1e-9 {
		t.Errorf("SD = %v, want %v", got.SD, want)
	}
	if want := 10.0; math.Abs(got.Range-want) > 1e-9 {
		t.Errorf("Range = %v, want %v", got.Range, want)
	}
}

func TestOfIsZeroWhenEverySeatSitsAtTheSameRating(t *testing.T) {
	// The all-unrated lobby: every seat at the prior. Dispersion 0 is vacuous
	// and also correct — it is the best estimate available.
	got := Of([]float64{25, 25, 25, 25})

	if got.SD != 0 || got.Range != 0 {
		t.Errorf("Of(identical) = %+v, want zero spread", got)
	}
}
