package rating

// Skill is the service-facing view of a rating: where we think a player sits,
// how sure we are, and how much play that rests on.
//
// It deliberately does not name mu and sigma. Those are Weng-Lin's parameters,
// and a developer-facing contract spelled in them is a contract we cannot
// change engines without breaking. The names here describe meaning — centre,
// spread, evidence — which a different model can still satisfy. The numbers
// happen to be the engine's today; the promise is only that they mean this.
type Skill struct {
	Rating        float64
	Uncertainty   float64
	MatchesPlayed int
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
func SkillOf(mu, sigma float64, matchesPlayed int) Skill {
	return Skill{Rating: mu, Uncertainty: sigma, MatchesPlayed: matchesPlayed}
}

// UnratedSkill is what we report for a player we have never rated here: the
// prior, with zero matches behind it.
//
// The prior is returned rather than nothing so a game never has to special-case
// an absent number to pick a difficulty. It is not flagged as provisional
// either, because MatchesPlayed of 0 already says so — and a game meeting a
// player for the first time is the ordinary case, not an error state.
func UnratedSkill() Skill {
	return SkillOf(UnratedMu, UnratedSigma, 0)
}
