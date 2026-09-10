package coldstart

import (
	"fmt"
	"math"
	"math/rand"
	"testing"

	"github.com/scruffyprodigy/joinquest/internal/rating"
)

// correlatedPopulation builds n players whose target-mode rating is their
// source-mode rating scaled by rho plus independent noise, both distributions
// centred on the flat prior's mu.
//
// Centring on rating.UnratedMu is what makes the comparison against the flat
// prior a fair fight rather than a rigged one: the flat prior is exactly right
// about the population's centre, so anything seeding wins by is won on placing
// individuals, which is the only thing it claims to do.
func correlatedPopulation(t *testing.T, n int, rho, spread float64) []Observation {
	t.Helper()

	rng := rand.New(rand.NewSource(20260910))
	obs := make([]Observation, n)
	for i := range obs {
		source := rng.NormFloat64() * spread
		noise := rng.NormFloat64() * spread * math.Sqrt(1-rho*rho)
		obs[i] = Observation{
			PlayerKey: fmt.Sprintf("player:%04d", i),
			SourceMu:  rating.UnratedMu + source,
			TargetMu:  rating.UnratedMu + rho*source + noise,
		}
	}
	return obs
}

// The headline claim: over a population where one mode really does predict
// the other, the measured correlation recovers the relationship and the
// held-out error beats the flat prior's. A harness that cannot show this has
// no business setting a sigma.
func TestFitRecoversStrongCorrelationAndBeatsFlatPrior(t *testing.T) {
	stats := Fit("arena", "duel", correlatedPopulation(t, 400, 0.9, 5))

	if math.Abs(stats.Correlation-0.9) > 0.05 {
		t.Errorf("Correlation = %v, want approximately 0.9", stats.Correlation)
	}
	if stats.ResidualSD >= stats.FlatResidualSD {
		t.Errorf("ResidualSD = %v, want better than flat prior's %v", stats.ResidualSD, stats.FlatResidualSD)
	}
	if !stats.Usable() {
		t.Errorf("Usable() = false for a strongly correlated pair: %+v", stats)
	}
}

// The mirror image, and the more important one. Two modes with no
// relationship must not produce a seed. If this test ever passes by accident
// the whole package ships confident noise, which is worse than shipping
// nothing.
func TestFitRefusesUncorrelatedModes(t *testing.T) {
	stats := Fit("arena", "duel", correlatedPopulation(t, 400, 0.0, 5))

	if stats.Usable() {
		t.Errorf("Usable() = true for an uncorrelated pair: correlation %v, residual %v vs flat %v",
			stats.Correlation, stats.ResidualSD, stats.FlatResidualSD)
	}
}

// A pair below the evidence threshold falls back to the flat prior even when
// the handful of players it does have look beautifully correlated — which,
// at that size, they will roughly one time in five.
func TestFitRefusesTooFewPairedPlayers(t *testing.T) {
	obs := correlatedPopulation(t, MinPairedPlayers-1, 0.95, 5)
	stats := Fit("arena", "duel", obs)

	if stats.PairedPlayers >= MinPairedPlayers {
		t.Fatalf("test population has %d players, want fewer than %d", stats.PairedPlayers, MinPairedPlayers)
	}
	if stats.Usable() {
		t.Errorf("Usable() = true with only %d paired players", stats.PairedPlayers)
	}
}

// Fit is called on a schedule, so two recomputes over unchanged history have
// to agree exactly. A residual that drifted between runs would make every
// stored sigma a moving target and the audit record unreconcilable.
func TestFitIsDeterministic(t *testing.T) {
	obs := correlatedPopulation(t, 200, 0.8, 5)

	first := Fit("arena", "duel", obs)
	second := Fit("arena", "duel", obs)

	if first != second {
		t.Errorf("Fit is not deterministic:\nfirst  = %+v\nsecond = %+v", first, second)
	}
}

// Fold assignment runs over the key-sorted slice, so the order the store
// happened to return rows in must not reach the measurement.
func TestFitIgnoresInputOrder(t *testing.T) {
	obs := correlatedPopulation(t, 200, 0.8, 5)
	reversed := make([]Observation, len(obs))
	for i, o := range obs {
		reversed[len(obs)-1-i] = o
	}

	if got, want := Fit("arena", "duel", reversed), Fit("arena", "duel", obs); got != want {
		t.Errorf("Fit depends on input order:\nsorted   = %+v\nreversed = %+v", want, got)
	}
}

// strongPair is a usable PairStats with round numbers, for the seeding tests
// that are about the formula rather than about the measurement.
func strongPair(source string, residual float64) PairStats {
	return PairStats{
		SourceMode:    source,
		TargetMode:    "duel",
		PairedPlayers: 200,
		Correlation:   0.8,
		SourceMean:    25,
		SourceSD:      5,
		TargetMean:    25,
		TargetSD:      5,
		ResidualSD:    residual,
		// Near the flat prior's own spread, which is what the flat baseline
		// measures over a population with real variation in it — so these
		// fixtures exercise Usable's "beats the flat prior" clause rather
		// than sliding past it on an unrealistically weak baseline.
		FlatResidualSD: 8,
	}
}

// The asymmetry the ticket asks for, stated as a test: a player who is as far
// above the population mean as another is below it must be seeded closer to
// the mean than that other player, because being wrong upward costs more.
func TestSeedIsMoreCautiousUpwardThanDownward(t *testing.T) {
	stats := map[string]PairStats{"arena": strongPair("arena", 3)}

	above, ok := SeedFor([]SourceRating{{ModeKey: "arena", Rating: rating.Rating{Mu: 35, Sigma: 2}}}, stats)
	if !ok {
		t.Fatal("SeedFor returned no seed for an above-average player")
	}
	below, ok := SeedFor([]SourceRating{{ModeKey: "arena", Rating: rating.Rating{Mu: 15, Sigma: 2}}}, stats)
	if !ok {
		t.Fatal("SeedFor returned no seed for a below-average player")
	}

	up := above.Rating.Mu - stats["arena"].TargetMean
	down := stats["arena"].TargetMean - below.Rating.Mu
	if up >= down {
		t.Errorf("upward seed moved %v from the mean and downward moved %v; upward must be the smaller", up, down)
	}
	if want := down * UpwardCaution; math.Abs(up-want) > 1e-9 {
		t.Errorf("upward deviation = %v, want %v (downward deviation shrunk by UpwardCaution)", up, want)
	}
}

// The floor is the promise that a seeded player still moves on their first
// match. A pair whose backtest came out implausibly tight must not be able to
// buy more confidence than the floor allows.
func TestSeedSigmaNeverBeatsTheFloor(t *testing.T) {
	stats := map[string]PairStats{"arena": strongPair("arena", 0.01)}

	seed, ok := SeedFor([]SourceRating{{ModeKey: "arena", Rating: rating.Rating{Mu: 30, Sigma: 2}}}, stats)
	if !ok {
		t.Fatal("SeedFor returned no seed")
	}
	if seed.Rating.Sigma != SeedSigmaFloor {
		t.Errorf("seed sigma = %v, want the floor %v", seed.Rating.Sigma, SeedSigmaFloor)
	}
	if seed.Rating.Sigma >= rating.UnratedSigma {
		t.Errorf("seed sigma = %v, want tighter than the flat prior's %v", seed.Rating.Sigma, rating.UnratedSigma)
	}
}

// Sigma comes from the measurement whenever the measurement is the weaker
// claim, which is the case the floor exists to leave alone.
func TestSeedSigmaComesFromTheMeasuredResidual(t *testing.T) {
	residual := SeedSigmaFloor + 1
	stats := map[string]PairStats{"arena": strongPair("arena", residual)}

	seed, ok := SeedFor([]SourceRating{{ModeKey: "arena", Rating: rating.Rating{Mu: 30, Sigma: 2}}}, stats)
	if !ok {
		t.Fatal("SeedFor returned no seed")
	}
	if seed.Rating.Sigma != residual {
		t.Errorf("seed sigma = %v, want the measured residual %v", seed.Rating.Sigma, residual)
	}
}

// A player whose own rating in the source mode has not settled is not
// evidence about anything yet, however good the pair looks.
func TestSeedForSkipsUnconvergedSources(t *testing.T) {
	stats := map[string]PairStats{"arena": strongPair("arena", 3)}
	unconverged := rating.Rating{Mu: 35, Sigma: ConvergedSigma + 0.1}

	if _, ok := SeedFor([]SourceRating{{ModeKey: "arena", Rating: unconverged}}, stats); ok {
		t.Error("SeedFor seeded from a source rating that has not converged")
	}
}

func TestSeedForFallsBackWhenPairIsUnusable(t *testing.T) {
	thin := strongPair("arena", 3)
	thin.PairedPlayers = MinPairedPlayers - 1

	stats := map[string]PairStats{"arena": thin}
	if _, ok := SeedFor([]SourceRating{{ModeKey: "arena", Rating: rating.Rating{Mu: 35, Sigma: 2}}}, stats); ok {
		t.Error("SeedFor seeded from a pair below MinPairedPlayers")
	}
}

// Best-measured wins, not highest-correlated and not first-seen.
func TestSeedForPicksTheLowestResidualSource(t *testing.T) {
	loose := strongPair("arena", 4)
	tight := strongPair("blitz", 2)
	tight.Correlation = 0.5 // deliberately the weaker correlation

	stats := map[string]PairStats{"arena": loose, "blitz": tight}
	sources := []SourceRating{
		{ModeKey: "arena", Rating: rating.Rating{Mu: 30, Sigma: 2}},
		{ModeKey: "blitz", Rating: rating.Rating{Mu: 30, Sigma: 2}},
	}

	seed, ok := SeedFor(sources, stats)
	if !ok {
		t.Fatal("SeedFor returned no seed")
	}
	if seed.SourceMode != "blitz" {
		t.Errorf("seeded from %q, want the better-measured %q", seed.SourceMode, "blitz")
	}
}

// Two source modes measured identically must resolve the same way on every
// read, or the seed recorded in the audit trail is not the seed that would be
// served again a moment later.
func TestSeedForBreaksTiesDeterministically(t *testing.T) {
	stats := map[string]PairStats{"arena": strongPair("arena", 3), "blitz": strongPair("blitz", 3)}
	sources := []SourceRating{
		{ModeKey: "blitz", Rating: rating.Rating{Mu: 30, Sigma: 2}},
		{ModeKey: "arena", Rating: rating.Rating{Mu: 30, Sigma: 2}},
	}

	for i := 0; i < 8; i++ {
		seed, ok := SeedFor(sources, stats)
		if !ok {
			t.Fatal("SeedFor returned no seed")
		}
		if seed.SourceMode != "arena" {
			t.Fatalf("tie resolved to %q, want the lowest mode key %q", seed.SourceMode, "arena")
		}
	}
}

// The seed carries the measurement it was made from, so an audit months later
// can tell a seed made on strong evidence from one made on thin evidence that
// has since been recomputed away.
func TestSeedRecordsItsProvenance(t *testing.T) {
	pair := strongPair("arena", 3)
	stats := map[string]PairStats{"arena": pair}

	seed, ok := SeedFor([]SourceRating{{ModeKey: "arena", Rating: rating.Rating{Mu: 31, Sigma: 2}}}, stats)
	if !ok {
		t.Fatal("SeedFor returned no seed")
	}
	if seed.SourceMode != "arena" || seed.SourceMu != 31 {
		t.Errorf("provenance = %q at mu %v, want arena at 31", seed.SourceMode, seed.SourceMu)
	}
	if seed.Correlation != pair.Correlation || seed.PairedPlayers != pair.PairedPlayers {
		t.Errorf("seed recorded correlation %v over %d players, want %v over %d",
			seed.Correlation, seed.PairedPlayers, pair.Correlation, pair.PairedPlayers)
	}
}

// No source modes at all is the ordinary case for a player new to the game,
// and it has to be a quiet fallback rather than anything louder.
func TestSeedForWithNoSourcesFallsBack(t *testing.T) {
	if _, ok := SeedFor(nil, map[string]PairStats{"arena": strongPair("arena", 3)}); ok {
		t.Error("SeedFor produced a seed for a player with no other ratings")
	}
}
