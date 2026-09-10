package rating

import "testing"

// The unrated numbers are constants so the read path never builds an engine to
// answer a lookup. That is only safe while they still equal what the engine
// would have handed back, which is what this asserts.
func TestUnratedSkillMatchesEnginePrior(t *testing.T) {
	engine, err := NewWengLin("plackett-luce")
	if err != nil {
		t.Fatalf("NewWengLin: %v", err)
	}
	prior := engine.Prior()

	got := UnratedSkill()
	if got.Rating != prior.Mu {
		t.Errorf("UnratedSkill().Rating = %v, engine prior mu = %v", got.Rating, prior.Mu)
	}
	if got.Uncertainty != prior.Sigma {
		t.Errorf("UnratedSkill().Uncertainty = %v, engine prior sigma = %v", got.Uncertainty, prior.Sigma)
	}
	if got.MatchesPlayed != 0 {
		t.Errorf("UnratedSkill().MatchesPlayed = %d, want 0", got.MatchesPlayed)
	}
}
