package rating

import (
	"math"
	"testing"

	openskillrating "github.com/intinig/go-openskill/rating"
	"github.com/intinig/go-openskill/types"
)

// TestDefaultBetaMatchesPackageDefault guards the restatement in DefaultBeta.
// The constant exists so our default is a number we own rather than one we
// inherit without noticing; that is only true while it agrees with what
// go-openskill would have used on its own.
func TestDefaultBetaMatchesPackageDefault(t *testing.T) {
	prior := openskillrating.New()
	teams := []types.Team{
		{{Mu: prior.Mu, Sigma: prior.Sigma}},
		{{Mu: prior.Mu + 5, Sigma: prior.Sigma}},
	}

	beta := DefaultBeta
	explicit := openskillrating.PredictWin(teams, &types.OpenSkillOptions{Beta: &beta})
	defaulted := openskillrating.PredictWin(teams, nil)

	for i := range explicit {
		if math.Abs(explicit[i]-defaulted[i]) > 1e-12 {
			t.Fatalf("DefaultBeta (%v) no longer matches the package default: %v vs %v",
				DefaultBeta, explicit, defaulted)
		}
	}
}

func TestNewWengLinBetaRejectsNonPositiveBeta(t *testing.T) {
	for _, beta := range []float64{0, -1, math.NaN(), math.Inf(1)} {
		if _, err := NewWengLinBeta("plackett-luce", beta); err == nil {
			t.Errorf("NewWengLinBeta(beta=%v) = nil error, want an error", beta)
		}
	}
}

// TestEngineIDCarriesMeasuredBeta is what makes a constants change detectable:
// stored ratings, the replay sweep and every backtest report key off this
// string, so a beta that moved without moving the id would let ratings from
// two different models pile up in one mode.
func TestEngineIDCarriesMeasuredBeta(t *testing.T) {
	defaulted, _ := NewWengLinBeta("plackett-luce", DefaultBeta)
	if got := defaulted.ID(); got != "weng-lin/plackett-luce@1" {
		t.Errorf("default beta ID() = %q, want the unsuffixed id", got)
	}

	measured, _ := NewWengLinBeta("plackett-luce", 6.25)
	if got := measured.ID(); got != "weng-lin/plackett-luce@1+beta=6.25" {
		t.Errorf("measured ID() = %q, want weng-lin/plackett-luce@1+beta=6.25", got)
	}

	other, _ := NewWengLinBeta("plackett-luce", 6.26)
	if measured.ID() == other.ID() {
		t.Errorf("two betas share the id %q", measured.ID())
	}
}

// TestBetaGovernsHowMuchOneResultMoves is the behavioural claim the whole
// ticket rests on: beta is the amount of noise between skill and the result,
// so a mode fitted with a large beta treats one upset as weak evidence and a
// mode with a small beta treats it as strong.
func TestBetaGovernsHowMuchOneResultMoves(t *testing.T) {
	lowNoise, _ := NewWengLinBeta("plackett-luce", DefaultBeta/4)
	highNoise, _ := NewWengLinBeta("plackett-luce", DefaultBeta*4)

	move := func(e Engine) float64 {
		prior := e.Prior()
		got, err := e.Rate([]Side{
			solo("player:a", prior, 0),
			solo("player:b", prior, 1),
		})
		if err != nil {
			t.Fatalf("Rate: %v", err)
		}
		return got[0][0].Mu - prior.Mu
	}

	small, large := move(highNoise), move(lowNoise)
	if !(large > small) {
		t.Errorf("low-beta mode moved %v and high-beta mode moved %v; want the low-beta mode to move further", large, small)
	}
}

// TestBetaGovernsForecastConfidence is the same claim on the read side. The
// two have to agree: the harness scores a forecast against the ratings the
// same engine produced, so a beta that reached Rate but not WinProbabilities
// would look like a badly calibrated model rather than a wiring bug.
func TestBetaGovernsForecastConfidence(t *testing.T) {
	prior := Rating{Mu: UnratedMu, Sigma: UnratedSigma}
	strong := Rating{Mu: UnratedMu + 10, Sigma: UnratedSigma}

	forecast := func(beta float64) float64 {
		e, err := NewWengLinBeta("plackett-luce", beta)
		if err != nil {
			t.Fatalf("NewWengLinBeta: %v", err)
		}
		probs, err := e.(Predictor).WinProbabilities([]Side{
			solo("player:strong", strong, 0),
			solo("player:weak", prior, 1),
		})
		if err != nil {
			t.Fatalf("WinProbabilities: %v", err)
		}
		return probs[0]
	}

	confident, hedged := forecast(DefaultBeta/4), forecast(DefaultBeta*4)
	if !(confident > hedged) {
		t.Errorf("low beta forecast %v, high beta forecast %v; want the low-beta mode more confident in the stronger player", confident, hedged)
	}
	if !(hedged > 0.5) {
		t.Errorf("high beta forecast %v; even a luck-heavy mode should favour the stronger player", hedged)
	}
}
