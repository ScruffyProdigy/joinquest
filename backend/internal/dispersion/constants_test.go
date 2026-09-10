package dispersion

import (
	"math"
	"testing"

	"github.com/scruffyprodigy/joinquest/internal/rating"
)

// Beta is the rating model's own unit for a gap that matters, not a multiplier
// chosen to feel reasonable. It is derived from the engine's prior rather than
// restated as a literal, so swapping the engine cannot silently decouple the
// band from the scale it is measured on. This is the guard for that.
func TestBetaTracksTheEnginePriorSpread(t *testing.T) {
	if want := rating.UnratedSigma / 2; math.Abs(Beta-want) > 1e-12 {
		t.Errorf("Beta = %v, want rating.UnratedSigma/2 = %v", Beta, want)
	}
	if want := 2 * Beta; math.Abs(HardCap-want) > 1e-12 {
		t.Errorf("HardCap = %v, want 2*Beta = %v", HardCap, want)
	}
}

// The case the hard cap exists for, and the reason it is not redundant with SD:
// one distant player riding along in a lobby that is otherwise tight barely
// moves the standard deviation, so SD alone would accept them.
func TestALobbyInsideTheBandIsStillRejectedForASingleDistantOutlier(t *testing.T) {
	lobby := []float64{25, 25, 25, 25, 25, 25, 25, 34.6}
	spread := Of(lobby)

	if spread.SD >= Beta {
		t.Fatalf("test is not exercising the cap: SD %v must sit inside the band %v", spread.SD, Beta)
	}
	if spread.Range <= HardCap {
		t.Fatalf("test is not exercising the cap: Range %v must exceed the cap %v", spread.Range, HardCap)
	}

	if spread.Acceptable(Beta) {
		t.Errorf("Acceptable = true for %+v; the outlier must be rejected by the cap", spread)
	}
}

func TestALobbyInsideBothTheBandAndTheCapIsAcceptable(t *testing.T) {
	spread := Of([]float64{22, 24, 26, 28})

	if !spread.Acceptable(Beta) {
		t.Errorf("Acceptable = false for %+v, which sits inside both band and cap", spread)
	}
}
