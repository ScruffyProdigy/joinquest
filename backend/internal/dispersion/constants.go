package dispersion

import (
	"time"

	"github.com/scruffyprodigy/joinquest/internal/rating"
)

// The starting constants, with the reasoning attached rather than left in the
// design doc, because "documented in the code rather than tuned by feel" is
// what this is required to be. Every one of them is a starting value to be
// re-derived once JQ-225 has recorded achieved dispersion and realised wait
// across a few hundred formed matches per mode — not a measured one.
const (
	// Beta is the rating model's own unit for a skill gap that matters:
	// go-openskill documents sigma/2 as "the distance that guarantees about an
	// 80% chance of winning". It anchors the dispersion target for that reason
	// rather than because a multiplier was chosen to feel reasonable, and it is
	// derived from the engine's prior rather than restated as a literal so a
	// change of engine cannot silently decouple the band from its scale.
	//
	// A lobby spread by about Beta has its extremes at roughly an 80/20
	// matchup. Demanding tighter is asking for a distinction the model does not
	// claim to make.
	Beta = rating.UnratedSigma / 2

	// HardCap is the widest gap a lobby may contain regardless of how good its
	// standard deviation looks. Two Beta apart is around a 95/5 matchup at the
	// extremes — the line past which somebody in the lobby is definitely not
	// enjoying it.
	HardCap = 2 * Beta

	// TCeiling is the total skill-induced wait budget, measured against the
	// oldest waiter rather than the newest. Short because JoinQuest's matches
	// are short and its promise is "jump in": queue overhead should stay a
	// small fraction of match length.
	//
	// Known wrong, deliberately: a flat ceiling is the wrong shape across a
	// catalog whose modes run for 90 seconds and for twenty minutes. The right
	// version scales with the mode's expected duration. GameMode.TypicalMinutes
	// would carry it — this is flat because nobody has decided the scaling yet,
	// not because the catalog cannot express it.
	TCeiling = 15 * time.Second

	// Tick is the solve interval, and therefore the floor on hold granularity.
	// There is no sub-tick holding: a computed hold below one tick simply means
	// one tick.
	Tick = time.Second
)

// Acceptable reports whether a lobby is good enough to fire early.
//
// Both tests must pass, and they reject different lobbies: band is the
// objective, cap is the outlier rejection the objective cannot make on its own.
func (s Spread) Acceptable(band float64) bool {
	return s.SD <= band && s.Range <= HardCap
}

// CandidatesWanted is k: how many additional candidates are worth waiting for.
//
// It scales with lobby size rather than being flat, because a fixed k means
// something very different at 2 seats than at 10. The ratio it holds near a
// third is the part that matters: the advertised wait estimate runs about
// seats/lambda and a hold runs about k/lambda, so a hold as a fraction of the
// advertised wait is about k/seats regardless of arrival rate. Keeping k small
// relative to seat count therefore caps the overshoot in relative terms rather
// than only in seconds, which is a constraint rather than a preference.
//
// Never zero. A k of zero would be a deferral waiting for nobody: it can only
// expire, so it is pure cost.
func CandidatesWanted(seats int) int {
	if k := (seats + 1) / 3; k >= 1 {
		return k
	}
	return 1
}
