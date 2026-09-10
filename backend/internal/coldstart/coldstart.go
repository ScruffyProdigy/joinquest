// Package coldstart seeds a player's rating in a mode they have not played
// from their rating in another mode of the same game.
//
// A player who has a settled rating in Word Hunt Arena and queues for Word
// Hunt Duel currently starts from the flat prior, as though the lobby had
// never seen them. That throws away the strongest transfer signal there is:
// same rules, same interface, same core loop, differing only in pacing and
// player count. This package turns that signal into two numbers — a centre
// and a spread — and refuses to produce either when the history cannot
// support them.
//
// # What is measured rather than assumed
//
// Everything. The relationship between two modes is an ordinary Pearson
// correlation over players holding converged ratings in both, and the spread
// of a seed is the spread of the errors that same seeding made when it was
// backtested against players who did play both (see Fit). No constant in this
// package chooses how good the transfer is; the constants only decide when
// there is too little evidence to say.
//
// # Why the guards lean the way they do
//
// The two failure modes are not symmetric, so the guards are not either.
//
// Bias sigma large. A seed that is too wide washes out after a handful of
// matches and harms nobody in the meantime. A seed that is too tight is
// sticky: a player seeded wrongly high has to lose repeatedly to come back
// down, and every one of those matches is a bad experience for them and for
// whoever was matched against them. So the spread is floored (SeedSigmaFloor)
// and the seed is abandoned outright when the measured error is no better
// than the flat prior's.
//
// Be conservative upward. Seeded too high, a player is outmatched and has no
// way out but losing; seeded too low, they get an easy match and climb out
// within a few. Upward seeds are therefore shrunk toward the population mean
// by UpwardCaution and downward ones are not — a deliberate, documented
// asymmetry, not an oversight.
//
// # A floor this package cannot lift
//
// Rating is keyed per (player, game, mode), so a player's rating already
// averages over whatever roles they played inside that mode. For a game with
// asymmetric roles — Vent Crew's saboteur, Salt the Well's traitor — that
// average spans roles a player may be unevenly good at. No cross-mode seed
// can be more precise than that average is, however high the correlation
// climbs, which is a second reason the sigma floor is not merely defensive.
//
// # Scope
//
// Within one game only. Cross-game and cross-genre transfer need latent
// factors and belong to their own ticket; two modes of one game need one
// correlation per ordered pair and nothing more.
package coldstart

import (
	"math"
	"sort"

	"github.com/scruffyprodigy/joinquest/internal/rating"
)

// ConvergedSigma is how far a rating's uncertainty must have fallen before
// that rating may be used as evidence — either as a source to seed from, or
// as one half of a pair the correlation is measured over.
//
// Half the flat prior's spread, i.e. a rating that has at least halved its
// own uncertainty. Sigma is used rather than a match count because it is the
// engine's own statement about how much it knows, and it stays meaningful if
// the engine is ever swapped; a match count is a proxy for it that would have
// to be recalibrated alongside any such change.
//
// Including unconverged players in the correlation would be worse than
// excluding them: their mu is still mostly the prior, so pairs of them
// correlate with each other through the prior they share rather than through
// any skill they have shown, which inflates the estimate in exactly the
// direction that causes harm.
const ConvergedSigma = rating.UnratedSigma / 2

// SeedSigmaFloor is the tightest a seeded rating may ever be, whatever the
// backtest says.
//
// Half the flat prior's spread: a seed may claim to be twice as informative
// as knowing nothing, and no more. The number exists because the measured
// residual is an average over a population, while the guarantee has to hold
// for the individual in front of us — the player whose two modes happen to
// have nothing to do with each other is inside that average, not excluded
// from it. It is also where the role-averaging limit described in the package
// comment lands: a rating that spans roles a player is uneven at cannot
// support a tighter claim than this no matter how strong the pair
// correlation looks.
//
// Sitting at half the prior, a seeded player still moves substantially on
// their first match, which is the property that makes a wrong seed
// self-correcting rather than sticky.
const SeedSigmaFloor = rating.UnratedSigma / 2

// MinPairedPlayers is the fewest players holding converged ratings in both
// modes that will produce a correlation at all. Below it the pair falls back
// to the flat prior.
//
// A Pearson correlation's standard error is roughly 1/sqrt(n-3): about 0.19
// at n=30, and about 0.35 at n=11. At the smaller number a true correlation
// of zero routinely measures as 0.35 and a seed built on it is confident
// noise — which is the one outcome worse than not seeding, because it looks
// like a measurement. Thirty is where the error falls under 0.2, meaning a
// pair that measures as strongly correlated is unlikely to be uncorrelated in
// truth. It is a threshold on evidence, not on sample-size etiquette.
const MinPairedPlayers = 30

// UpwardCaution shrinks the part of a seed that sits above the target mode's
// population mean, and leaves the part below it alone.
//
// The asymmetry is the point; see the package comment. 0.8 is deliberately
// mild: the regression in predict already pulls every seed toward the mean by
// the correlation itself, and this is a second, smaller pull applied only in
// the direction where being wrong costs more. A value low enough to be a real
// safety margin would also flatten every genuinely strong player into the
// middle, which is its own bad matchmaking.
const UpwardCaution = 0.8

// PairStats is everything measured about seeding one mode from another, in
// that direction. Source and Target are not interchangeable: the regression
// coefficient and the residual spread both differ depending on which way the
// prediction runs, so a pair of modes yields two of these.
type PairStats struct {
	// SourceMode is the mode a player has already played; TargetMode is the
	// one being seeded.
	SourceMode string
	TargetMode string

	// PairedPlayers is how many players held converged ratings in both modes
	// when this was fitted. It travels with the numbers because it is what
	// says how much they are worth, and because MinPairedPlayers is checked
	// against it at serve time as well as at fit time.
	PairedPlayers int

	// Correlation is the Pearson correlation between paired players' mu in
	// the two modes.
	Correlation float64

	// SourceMean, SourceSD, TargetMean and TargetSD describe the paired
	// population in each mode. The regression needs all four: it maps a
	// player's standing within the source distribution onto the target one,
	// so two modes whose ratings are on different scales still compose.
	SourceMean float64
	SourceSD   float64
	TargetMean float64
	TargetSD   float64

	// ResidualSD is the spread of the errors this seeding made when it was
	// backtested against held-out players who had in fact played both modes.
	// This — not a chosen constant — is where a seed's sigma comes from.
	ResidualSD float64

	// FlatResidualSD is the same error spread for the flat prior, which
	// predicts rating.UnratedMu for everyone. It is kept so that "does
	// seeding beat doing nothing?" is answerable from the stored row rather
	// than by rerunning the fit, and so a pair that loses to the flat prior
	// can be refused at serve time.
	FlatResidualSD float64

	// Bias is the mean signed error over the same held-out players, positive
	// when the seeding ran high. It is reported rather than corrected: a
	// systematic overshoot is a fact about the fit that a human should see,
	// and silently subtracting it would hide a broken pair behind a
	// plausible-looking centre.
	Bias float64

	// AbsErrorP50 and AbsErrorP90 are the median and 90th percentile of the
	// absolute held-out error. ResidualSD alone reads as reassuring when a
	// pair is mostly fine and catastrophically wrong for a tail of players,
	// which is precisely the population a mis-seed hurts most.
	AbsErrorP50 float64
	AbsErrorP90 float64
}

// Usable reports whether this pair has enough evidence behind it to seed
// from, and is the single gate every caller goes through.
//
// Each clause refuses a different way of being wrong:
//
//   - Too few paired players: the correlation is noise (MinPairedPlayers).
//   - A non-positive correlation: two modes of one game that measure as
//     unrelated or inversely related have almost certainly measured nothing,
//     and a negative coefficient would seed strong players low and weak
//     players high. The flat prior is both safer and more honest.
//   - A degenerate source spread: every paired player sits at the same place
//     in the source mode, so there is no standing to map onto the target.
//   - A residual no better than the flat prior's: the seeding has been
//     measured and does not work here. This is the clause that lets a bad
//     pair be discovered rather than argued about.
//   - A residual at or beyond the flat prior's own spread: the seed would be
//     no more informative than knowing nothing, so the flat prior says the
//     same thing more simply.
func (p PairStats) Usable() bool {
	return p.PairedPlayers >= MinPairedPlayers &&
		p.Correlation > 0 &&
		p.SourceSD > 0 &&
		p.ResidualSD > 0 &&
		p.FlatResidualSD > 0 &&
		p.ResidualSD < p.FlatResidualSD &&
		p.ResidualSD < rating.UnratedSigma
}

// Seed is a starting rating derived from another mode, together with enough
// of its provenance to be audited later.
//
// The provenance is not decoration. A seeded rating is a claim the lobby made
// about a player before watching them play, and when a mode's matchmaking
// turns out to be lopsided, the first question is which players were seeded,
// from where, and on how strong a measurement. Recording the correlation and
// the paired-player count as they stood at the moment of seeding answers that
// even after the next scheduled recompute has moved both.
type Seed struct {
	Rating rating.Rating

	// SourceMode is the mode the seed was derived from, and SourceMu the
	// player's rating there at the time.
	SourceMode string
	SourceMu   float64

	// Correlation and PairedPlayers are the pair's measurement as it stood
	// when this seed was produced.
	Correlation   float64
	PairedPlayers int
}

// SourceRating is one mode a player has already played, as the seeder sees it.
type SourceRating struct {
	ModeKey string
	Rating  rating.Rating
}

// SeedFor picks the best available source mode for a player and returns the
// seed it produces, or ok=false when nothing qualifies and the caller should
// fall back to the flat prior.
//
// stats is keyed by source mode key and holds only pairs whose target is the
// mode being seeded. sources is every rating the player already holds in this
// game; modes that are unconverged, absent from stats, or backed by an
// unusable pair are skipped.
//
// When a player qualifies through more than one mode the one with the
// smallest ResidualSD wins — the pair that has been measured to predict this
// target most accurately, which is not always the one with the highest
// correlation, since a strong relationship between two widely-spread modes
// can still place a player less precisely than a weaker one between two tight
// ones. Ties break on mode key so that the same inputs always produce the
// same seed; a seed that varied between two reads of the same state would
// make the audit record meaningless.
//
// Deliberately no blending across several source modes. Two modes of one game
// are strongly correlated with each other as well as with the target, so
// averaging their predictions would count one player's evidence twice and
// produce a seed that is falsely precise — the sigma would shrink as though
// the sources were independent when they are anything but. Taking the single
// best-measured source is the honest version.
func SeedFor(sources []SourceRating, stats map[string]PairStats) (Seed, bool) {
	ordered := append([]SourceRating(nil), sources...)
	sort.Slice(ordered, func(i, j int) bool { return ordered[i].ModeKey < ordered[j].ModeKey })

	var best Seed
	var bestResidual float64
	found := false

	for _, src := range ordered {
		if src.Rating.Sigma > ConvergedSigma {
			continue
		}
		st, ok := stats[src.ModeKey]
		if !ok || !st.Usable() {
			continue
		}
		if found && st.ResidualSD >= bestResidual {
			continue
		}

		best = Seed{
			Rating:        rating.Rating{Mu: predict(st, src.Rating.Mu), Sigma: seedSigma(st)},
			SourceMode:    src.ModeKey,
			SourceMu:      src.Rating.Mu,
			Correlation:   st.Correlation,
			PairedPlayers: st.PairedPlayers,
		}
		bestResidual = st.ResidualSD
		found = true
	}

	return best, found
}

// predict maps a player's standing in the source mode onto the target mode.
//
// The first term is the ordinary least-squares prediction from a bivariate
// fit: start at the target mode's population mean and move toward the
// player's source-mode standing by the correlation, rescaled between the two
// modes' spreads. A correlation of 1 reproduces their source standing exactly;
// a correlation near 0 leaves them at the population mean, which is the flat
// prior in all but name. That the seed collapses gracefully to "we know
// nothing" as the evidence weakens is a property of the formula, not
// something bolted on.
//
// The second term is UpwardCaution, applied only above the mean. See the
// package comment for why the two directions are not treated alike.
func predict(p PairStats, sourceMu float64) float64 {
	deviation := p.Correlation * (p.TargetSD / p.SourceSD) * (sourceMu - p.SourceMean)
	if deviation > 0 {
		deviation *= UpwardCaution
	}
	return p.TargetMean + deviation
}

// seedSigma turns the pair's measured error spread into the seed's
// uncertainty: the backtested residual, floored at SeedSigmaFloor and never
// wider than the flat prior.
//
// The upper clamp is not merely tidiness. A seed wider than rating.UnratedSigma
// would claim to know less than nothing while still moving the player's centre
// away from the prior, which is a strictly worse position than not seeding at
// all — and Usable already refuses those pairs, so reaching the clamp means
// something upstream changed and the safe reading is the prior's.
func seedSigma(p PairStats) float64 {
	return math.Min(math.Max(p.ResidualSD, SeedSigmaFloor), rating.UnratedSigma)
}
