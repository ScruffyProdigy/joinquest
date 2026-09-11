// Package ratingbacktest scores rating models against held-out match history.
//
// internal/rating is built so the choice of model is reversible:
// rating_match_inputs is the source of truth and the stored ratings are a
// derived cache, so any engine can be run over the same history. This package
// turns that property into a number. Without it, "is Thurstone-Mosteller
// better than Plackett-Luce here?" and "what sigma should a cold-start prior
// use?" are matters of taste; with it they are measurements.
//
// The method is prequential (walk-forward) evaluation. History is split into a
// training prefix and a held-out suffix; the engine learns from the prefix,
// and then for each held-out match in order it forecasts the outcome from the
// ratings as they stand at that moment, is scored, and only then is allowed to
// see the result. Freezing the ratings at the end of training instead would
// penalise every later held-out match for staleness rather than for the
// model's quality, which measures the length of the suffix, not the engine.
//
// Nothing here writes. Source is the read-only half of rating.Store, so a
// dry run cannot touch player_ratings or nonplayer_ratings — that is a
// property of the type, not a rule someone has to remember.
package ratingbacktest

import (
	"context"
	"fmt"
	"math"

	"github.com/scruffyprodigy/joinquest/internal/rating"
)

// logLossFloor bounds -log(p) for a forecast that assigned the actual winner
// no chance at all. Without it a single p=0 makes the whole mode's log-loss
// +Inf, which destroys the comparison it was supposed to inform: two engines
// that are both infinitely bad are indistinguishable, and one bad match hides
// everything else the engine got right. The floor makes such a match very
// expensive (about 34.5 nats) rather than fatal.
const logLossFloor = 1e-15

// Source is the read-only port the harness needs: the same ListInputs half of
// rating.Store, minus SaveAll.
//
// The narrowing is the point. *store.Store's rating adapter satisfies this
// interface as it stands, so the harness can be pointed at production history
// with no new plumbing — and still has no method through which to write a
// rating back. "A dry run must not mutate the live cache" is therefore
// something the compiler enforces rather than something a reviewer checks.
type Source interface {
	// ListInputs returns every rating input for a game/mode in replay order.
	// The order must be total, exactly as rating.Store requires: it drives
	// the sequence of Engine.Rate calls and therefore the score.
	ListInputs(ctx context.Context, gameID, modeKey string) ([]rating.Input, error)
}

// Target names one (game, mode) whose history is to be scored.
type Target struct {
	GameID  string
	ModeKey string
}

// Split says where a mode's history divides into a training prefix and a
// held-out suffix.
//
// TrainMatches wins when both are set. A fraction is the useful default across
// modes of wildly different sizes; an exact count is what makes two runs
// comparable when history has grown between them.
type Split struct {
	// TrainFraction is the share of matches used for training, in (0, 1).
	// The prefix is truncated toward zero, so the held-out suffix is never
	// empty for a fraction below 1 — a split that leaves nothing to score
	// would report a confident-looking zero.
	TrainFraction float64

	// TrainMatches, when positive, fixes the training prefix at exactly this
	// many matches and ignores TrainFraction. A mode with no more history
	// than this trains on everything and scores nothing, which the report
	// shows as zero held-out matches rather than as a score.
	TrainMatches int
}

// DefaultSplit trains on the first 80% of a mode's history.
var DefaultSplit = Split{TrainFraction: 0.8}

func (s Split) validate() error {
	if s.TrainMatches < 0 {
		return fmt.Errorf("backtest: TrainMatches must not be negative, got %d", s.TrainMatches)
	}
	if s.TrainMatches > 0 {
		return nil
	}
	if !(s.TrainFraction > 0 && s.TrainFraction < 1) {
		return fmt.Errorf("backtest: TrainFraction must be in (0, 1), got %v", s.TrainFraction)
	}
	return nil
}

// trainCount is how many of total matches form the training prefix.
func (s Split) trainCount(total int) int {
	if s.TrainMatches > 0 {
		return min(s.TrainMatches, total)
	}
	n := int(math.Floor(s.TrainFraction * float64(total)))
	if n < 0 {
		n = 0
	}
	return min(n, total)
}

// Score is one engine's performance over one mode's held-out matches.
type Score struct {
	// HeldOut is the number of matches the engine was scored on. It travels
	// with every score deliberately: an accuracy computed over four matches
	// and one computed over forty thousand are the same number and not the
	// same evidence, and a thin mode has to read as thin.
	HeldOut int

	// Pairs is the number of side-pairs with distinct ranks that Accuracy was
	// computed over — one per match for a duel, more for a multi-side mode.
	Pairs int

	// Accuracy is the share of those pairs the engine ordered correctly.
	// A pair the engine rates dead level counts as half, not as wrong: an
	// engine that has learned nothing scores 0.5, which reads as "no
	// information" rather than as "actively misleading". An engine that
	// orders sides backwards scores below 0.5, and the distinction between
	// those two failures is worth keeping.
	Accuracy float64

	// Probabilistic is whether the engine implements rating.Predictor. When
	// false, LogLoss and Brier are absent rather than zero-and-meaningless.
	Probabilistic bool

	// ProbMatches is how many held-out matches the probabilistic metrics were
	// computed over: those with two or more sides and a single outright
	// winner. A match whose first place is shared has no one outcome for a
	// win-probability forecast to be scored against, so it is excluded and
	// counted out here rather than folded in under an invented convention.
	ProbMatches int

	// LogLoss is the mean -log(probability assigned to the actual winner)
	// over ProbMatches. Lower is better. It punishes confident mistakes far
	// harder than Accuracy does, which is the whole reason to want it.
	LogLoss float64

	// Brier is the mean squared error of the full probability vector against
	// the one-hot outcome, over ProbMatches. Lower is better. It is bounded
	// where LogLoss is not, so the two disagreeing is itself informative:
	// it means a small number of very confident misses are driving LogLoss.
	Brier float64
}

// EngineScore pairs an engine's identity with how it did.
type EngineScore struct {
	// EngineID is Engine.ID() — the model and its constants, so a report
	// read months later says which thing was actually measured.
	EngineID string
	Score    Score
}

// ModeReport is every engine's score over one mode, plus what the reader
// needs in order to know how much the scores are worth.
type ModeReport struct {
	Target Target

	// Matches, TrainMatches and HeldOutMatches describe the history the
	// scores were computed over.
	Matches        int
	TrainMatches   int
	HeldOutMatches int

	// IdentifiabilityReport is rating.Identifiability over this mode's full
	// history. It sits beside the scores because it qualifies them: a
	// modifier with PlayersWithMultipleValues of zero is confounded with
	// player skill, so every engine ran over history that cannot tell the
	// two apart, and a comparison between them is suspect no matter how
	// clean the numbers look.
	rating.IdentifiabilityReport

	// Engines holds one score per engine, in the order the engines were
	// supplied — all of them over this identical history, which is what
	// makes them comparable at all.
	Engines []EngineScore
}

// Report is one backtest run across every requested mode.
type Report struct {
	// Split records how history was divided, so a report carries the setting
	// that produced it rather than relying on whoever ran it to remember.
	Split Split
	Modes []ModeReport
}

// Harness scores engines against retained history.
type Harness struct {
	source Source
	split  Split
}

// New builds a Harness reading through source and splitting history at split.
func New(source Source, split Split) (*Harness, error) {
	if source == nil {
		return nil, fmt.Errorf("backtest: source is required")
	}
	if err := split.validate(); err != nil {
		return nil, err
	}
	return &Harness{source: source, split: split}, nil
}

// Run scores every engine against every target's history and returns one
// report covering all of them.
//
// Each target's history is fetched exactly once and every engine is run over
// that same slice, so a comparison between engines can never be an artefact of
// two reads seeing different data.
func (h *Harness) Run(ctx context.Context, targets []Target, engines ...rating.Engine) (Report, error) {
	if len(engines) == 0 {
		return Report{}, fmt.Errorf("backtest: at least one engine is required")
	}

	report := Report{Split: h.split, Modes: make([]ModeReport, 0, len(targets))}
	for _, target := range targets {
		inputs, err := h.source.ListInputs(ctx, target.GameID, target.ModeKey)
		if err != nil {
			return Report{}, fmt.Errorf("backtest: list inputs for %s/%s: %w", target.GameID, target.ModeKey, err)
		}

		train := h.split.trainCount(len(inputs))
		mode := ModeReport{
			Target:                target,
			Matches:               len(inputs),
			TrainMatches:          train,
			HeldOutMatches:        len(inputs) - train,
			IdentifiabilityReport: rating.Identifiability(inputs),
			Engines:               make([]EngineScore, 0, len(engines)),
		}

		for _, engine := range engines {
			score, err := scoreEngine(engine, inputs, train)
			if err != nil {
				return Report{}, fmt.Errorf("backtest: score %s over %s/%s: %w", engine.ID(), target.GameID, target.ModeKey, err)
			}
			mode.Engines = append(mode.Engines, EngineScore{EngineID: engine.ID(), Score: score})
		}

		report.Modes = append(report.Modes, mode)
	}
	return report, nil
}

// scoreEngine walks one mode's history in order, learning from the prefix and
// forecasting-then-learning through the suffix.
//
// Determinism has the same basis as rating.ReplayMode's: everything that
// affects an engine call, or the order of engine calls, iterates slices. The
// accumulating ratings map is lookup-only, and the floating-point sums below
// accumulate in match order, so repeated runs agree bit for bit rather than
// merely closely.
func scoreEngine(engine rating.Engine, inputs []rating.Input, train int) (Score, error) {
	predictor, canPredict := engine.(rating.Predictor)
	score := Score{Probabilistic: canPredict}

	ratings := make(map[string]rating.Rating)
	var correct, logLoss, brier float64

	for i, in := range inputs {
		sides := make([]rating.Side, len(in.Sides))
		for j, side := range in.Sides {
			entrants := make([]rating.Entrant, len(side.Entrants))
			for k, e := range side.Entrants {
				rt, ok := ratings[e.Key]
				if !ok {
					rt = engine.Prior()
				}
				entrants[k] = rating.Entrant{Key: e.Key, Rating: rt}
			}
			sides[j] = rating.Side{Entrants: entrants, Rank: side.Rank}
		}

		// Score before rating: the forecast has to be made from what the
		// engine knew going in, never from the match it is predicting.
		if i >= train {
			score.HeldOut++

			pairs, hits := orderingScore(sides)
			score.Pairs += pairs
			correct += hits

			if canPredict {
				winner, ok := outrightWinner(sides)
				if ok {
					probs, err := predictor.WinProbabilities(sides)
					if err != nil {
						return Score{}, fmt.Errorf("predict session %s: %w", in.SessionID, err)
					}
					if len(probs) != len(sides) {
						return Score{}, fmt.Errorf("predict session %s: got %d probabilities, want %d", in.SessionID, len(probs), len(sides))
					}
					score.ProbMatches++
					logLoss += -math.Log(math.Max(probs[winner], logLossFloor))
					for s, p := range probs {
						outcome := 0.0
						if s == winner {
							outcome = 1.0
						}
						brier += (p - outcome) * (p - outcome)
					}
				}
			}
		}

		updated, err := engine.Rate(sides)
		if err != nil {
			return Score{}, fmt.Errorf("rate session %s: %w", in.SessionID, err)
		}
		if len(updated) != len(sides) {
			return Score{}, fmt.Errorf("rate session %s: engine returned %d sides, want %d", in.SessionID, len(updated), len(sides))
		}
		for j, side := range sides {
			if len(updated[j]) != len(side.Entrants) {
				return Score{}, fmt.Errorf("rate session %s: engine returned %d ratings for side %d, want %d", in.SessionID, len(updated[j]), j, len(side.Entrants))
			}
			for k, entrant := range side.Entrants {
				ratings[entrant.Key] = updated[j][k]
			}
		}
	}

	if score.Pairs > 0 {
		score.Accuracy = correct / float64(score.Pairs)
	}
	if score.ProbMatches > 0 {
		score.LogLoss = logLoss / float64(score.ProbMatches)
		score.Brier = brier / float64(score.ProbMatches)
	}
	return score, nil
}

// orderingScore compares the engine's ordering of the sides against the actual
// finishing order, over every pair of sides that actually placed differently.
//
// Pairwise rather than "did it name the winner" so that a mode with three or
// more sides is scored on the whole result: an engine that puts the runner-up
// last is wrong about something, and naming the winner correctly should not
// hide it. Pairs that tied are skipped — there is no ordering to get right.
func orderingScore(sides []rating.Side) (pairs int, hits float64) {
	strengths := make([]float64, len(sides))
	for i, side := range sides {
		strengths[i] = sideStrength(side)
	}

	for i := 0; i < len(sides); i++ {
		for j := i + 1; j < len(sides); j++ {
			if sides[i].Rank == sides[j].Rank {
				continue
			}
			pairs++
			switch {
			case strengths[i] == strengths[j]:
				hits += 0.5
			case (strengths[i] > strengths[j]) == (sides[i].Rank < sides[j].Rank):
				hits++
			}
		}
	}
	return pairs, hits
}

// sideStrength is the sum of the side's entrants' rating centres.
//
// Summing — rather than averaging — is what makes a numerical advantage count
// as an advantage, and it is how the Weng-Lin model itself composes a team. It
// deliberately reads Mu alone and ignores Sigma: this is the ordering the
// engine's own point estimates imply, and applying a conservative discount
// here would score the harness's opinion about uncertainty rather than the
// engine's opinion about skill.
//
// Modifier entrants — a seat, a scenario, a rated pre-queue option — count
// exactly like players, because the side really does carry that advantage into
// the match. Their identifiability is what ModeReport.Modifiers is for.
func sideStrength(side rating.Side) float64 {
	var total float64
	for _, e := range side.Entrants {
		total += e.Rating.Mu
	}
	return total
}

// outrightWinner returns the index of the single best-placed side, and whether
// there was exactly one. Fewer than two sides, or a shared first place, has no
// unique outcome for a win-probability forecast to be scored against.
func outrightWinner(sides []rating.Side) (int, bool) {
	if len(sides) < 2 {
		return 0, false
	}
	best, tied := 0, false
	for i := 1; i < len(sides); i++ {
		switch {
		case sides[i].Rank < sides[best].Rank:
			best, tied = i, false
		case sides[i].Rank == sides[best].Rank:
			tied = true
		}
	}
	return best, !tied
}
