package rating

// Skill is the service-facing view of a rating: where we think a player sits,
// and how sure we are.
//
// It deliberately does not name mu and sigma. Those are Weng-Lin's parameters,
// and a developer-facing contract spelled in them is a contract we cannot
// change engines without breaking. The names here describe meaning — centre and
// spread — which a different model can still satisfy. The numbers happen to be
// the engine's today; the promise is only that they mean this.
//
// It deliberately carries no match count either. A game can already count how
// many times it has seen a player id, so the number would be redundant on its
// own terms — and worse, it invites reading "we have not rated them here" as
// "we know nothing about them", which stops being true the moment a rating can
// be seeded from a player's record elsewhere in the catalog. Uncertainty is the
// field that answers how much to trust the estimate, and it stays honest under
// that change.
type Skill struct {
	Rating      float64
	Uncertainty float64
}

// UnratedMu and UnratedSigma are what a player with no rating in a game and
// mode reports. They mirror the engine's Prior() rather than calling it, so
// nothing on the read path has to build an engine to answer a lookup;
// TestUnratedSkillMatchesEnginePrior fails if the engine's prior ever drifts
// away from them.
const (
	UnratedMu    = 25.0
	UnratedSigma = 25.0 / 3.0
)

// SkillOf projects a stored rating into the service-facing shape.
func SkillOf(mu, sigma float64) Skill {
	return Skill{Rating: mu, Uncertainty: sigma}
}

// UnratedSkill is what we report for a player we have never rated here: the
// prior, at full uncertainty.
//
// The prior is returned rather than nothing so a game never has to special-case
// an absent number to pick a difficulty, and it is not flagged as provisional
// because a game meeting a player for the first time already knows it has not
// seen them before.
func UnratedSkill() Skill {
	return SkillOf(UnratedMu, UnratedSigma)
}
