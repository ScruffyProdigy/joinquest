package rating

import (
	"context"
	"fmt"
	"math"
	"strings"
)

// seatCategory is the modifier category seat classes fall into, as produced
// by splitModifierKey from a "seat:<NamePath>" key.
const seatCategory = "seat"

// Input is one match as replay sees it: the store's row, minus storage
// detail. Sides carry entrant keys only — ratings are populated by the
// replay's accumulating map, never by the store.
type Input struct {
	SessionID string
	Sides     []Side
}

// Store is the narrow port replay needs. The real implementation lives in
// internal/store, adapting the database tables to this shape; tests use an
// in-memory fake, which is how the determinism guarantee below is tested
// without Postgres. The dependency runs store -> rating, never the reverse.
type Store interface {
	// ListInputs returns every rating input for a game/mode in replay order.
	// The order must be total (the database adapter relies on
	// rated_at, then session_id) since it drives the sequence of Engine.Rate
	// calls: a weaker order makes replay silently unreproducible.
	ListInputs(ctx context.Context, gameID, modeKey string) ([]Input, error)

	// SaveAll writes one replay's output for a game/mode. players is keyed by
	// "player:<uuid>"; entities holds every other entrant key (modifiers).
	SaveAll(ctx context.Context, gameID, modeKey, engineID string, players, entities map[string]Rating) error
}

// ModifierReport says whether a modifier is identifiable from the history.
//
// A modifier only carries meaning if it varies within players. If every
// player always appears with the same value, that player's skill and the
// modifier are perfectly confounded and the split between them is arbitrary
// — the failure is silent, producing plausible numbers that are wrong. This
// is reported rather than acted on: a modifier switching on partway through a
// replay would make the resulting ratings hard to reason about, so a human
// reads this and decides.
type ModifierReport struct {
	// Matches is the number of matches this exact modifier key appeared in.
	Matches int
	// DistinctValues is the number of distinct values seen for this
	// modifier's category across the whole replay (e.g. "seat:White" and
	// "seat:Black" both report 2, the size of the "seat" category).
	DistinctValues int
	// DistinctPlayers is the number of players who ever carried this
	// modifier key. It is what separates thin from confident: a seat class
	// with 500 matches behind it looks well-evidenced until you see that
	// three people account for all of them, at which point its rating is
	// mostly a statement about those three.
	DistinctPlayers int
	// PlayersWithMultipleValues is the number of players who appeared with
	// this modifier key at least once, and who are also known — from
	// elsewhere in the same replay — to have appeared under a different
	// value of the same category. Zero means every player who ever carried
	// this value carried only this value: skill and modifier are
	// confounded for all of them, and the split is arbitrary.
	PlayersWithMultipleValues int
}

// Identifiable reports whether this modifier is separable from the skill of
// the players who carried it.
//
// It needs two things at once. A category with a single value has nothing to
// vary against, so its modifier is a constant added to every side and is
// unidentifiable by construction. And with no player who has carried more
// than one value, every player's skill moves in lockstep with their one
// value, so the split between the two is arbitrary — the numbers come out
// looking ordinary, which is what makes this worth reporting rather than
// leaving to be noticed.
func (r ModifierReport) Identifiable() bool {
	return r.DistinctValues > 1 && r.PlayersWithMultipleValues > 0
}

// CategoryReport is the identifiability picture for a whole modifier
// category — "seat", "scenario", "prequeue:color" — rather than for one of
// its values.
//
// This is the level the question is really asked at. Whether seat class is
// separable from player skill in a mode is one fact about that mode; asking
// it per seat invites reading "seat:Clue Giver looks fine" off a mode where
// the split is arbitrary for everybody.
type CategoryReport struct {
	// DistinctValues is how many values of this category the replay saw.
	DistinctValues int
	// Players is how many players carried any value of it.
	Players int
	// PlayersWithMultipleValues is how many of those carried more than one.
	PlayersWithMultipleValues int
}

// SpecializationRate is the share of players who only ever carried a single
// value of this category — 1 means everybody specializes and 0 means
// everybody moves around. It is reported rather than acted on, and it is the
// number that says whether a category's main effect and the per-player
// interaction on it (JQ-229) can be told apart at all: at a rate of 1 they
// are perfectly confounded and only their sum is estimated, however
// confident either number looks on its own.
//
// A category no player has carried reports 0, not a division by zero. There
// is no specialization to measure, and Players == 0 already says so.
func (r CategoryReport) SpecializationRate() float64 {
	if r.Players == 0 {
		return 0
	}
	return float64(r.Players-r.PlayersWithMultipleValues) / float64(r.Players)
}

// Identifiable reports whether this category's main effect is separable from
// the skill of the players who carried its values, on the same two counts as
// ModifierReport.Identifiable.
func (r CategoryReport) Identifiable() bool {
	return r.DistinctValues > 1 && r.PlayersWithMultipleValues > 0
}

// IdentifiabilityReport is the full identifiability picture for one history,
// at both granularities.
type IdentifiabilityReport struct {
	// Modifiers is keyed by modifier entrant key ("seat:Team/Guesser").
	Modifiers map[string]ModifierReport
	// Categories is keyed by category ("seat", "prequeue:color").
	Categories map[string]CategoryReport
}

// Report summarizes one ReplayMode run.
type Report struct {
	Matches int
	IdentifiabilityReport
}

// Replayer recomputes ratings by walking a mode's rating_match_inputs log in
// order, from scratch, through a single Engine.
type Replayer struct {
	engine Engine
	store  Store
}

// NewReplayer builds a Replayer over the given engine and store port.
func NewReplayer(engine Engine, store Store) *Replayer {
	return &Replayer{engine: engine, store: store}
}

// ReplayMode recomputes every rating for one game/mode from its full input
// log and writes the result in a single SaveAll call.
//
// Determinism depends entirely on iterating Input.Sides and Side.Entrants as
// the slices they are — never ranging over a map — for anything that affects
// an Engine.Rate call or the order those calls happen in. The accumulating
// ratings map below is fine as a map because it is used only for lookup by
// key; nothing about map iteration order leaks into an engine call.
//
// One ordering inside a match is load-bearing and is not obvious from the
// loop: every entrant's rating is read before any is written back. seed reads
// a player's mode-level rating to size the uncertainty on a seat they have
// not played before, so writing updates as each entrant is visited would make
// that seed depend on where in the side the player happened to sit. Keep the
// read pass and the write-back pass separate.
func (r *Replayer) ReplayMode(ctx context.Context, gameID, modeKey string) (Report, error) {
	inputs, err := r.store.ListInputs(ctx, gameID, modeKey)
	if err != nil {
		return Report{}, fmt.Errorf("rating: list inputs for %s/%s: %w", gameID, modeKey, err)
	}

	ratings := make(map[string]Rating)

	for _, in := range inputs {
		sides := make([]Side, len(in.Sides))
		for i, side := range in.Sides {
			entrants := make([]Entrant, len(side.Entrants))
			for j, e := range side.Entrants {
				rt, ok := ratings[e.Key]
				if !ok {
					rt = r.seed(e.Key, ratings)
				}
				entrants[j] = Entrant{Key: e.Key, Rating: rt}
			}
			sides[i] = Side{Entrants: entrants, Rank: side.Rank}
		}

		updated, err := r.engine.Rate(sides)
		if err != nil {
			return Report{}, fmt.Errorf("rating: rate session %s: %w", in.SessionID, err)
		}
		if len(updated) != len(sides) {
			return Report{}, fmt.Errorf("rating: engine returned %d sides for session %s, want %d", len(updated), in.SessionID, len(sides))
		}
		for i, side := range sides {
			if len(updated[i]) != len(side.Entrants) {
				return Report{}, fmt.Errorf("rating: engine returned %d ratings for side %d of session %s, want %d", len(updated[i]), i, in.SessionID, len(side.Entrants))
			}
			for j, entrant := range side.Entrants {
				ratings[entrant.Key] = updated[i][j]
			}
		}
	}

	players := make(map[string]Rating)
	entities := make(map[string]Rating)
	for key, rt := range ratings {
		playerID, seatClass, isPlayer := ParsePlayerKey(key)
		switch {
		case !isPlayer:
			entities[key] = rt
		case seatClass == "":
			players[key] = rt
		default:
			// What the engine holds for an interaction key is a deviation
			// centred on zero, which is not a skill estimate and must not be
			// stored as though it were: a caller reading player_ratings finds
			// mode-level rows near the prior and would have no way to know
			// this one is on a different scale. Project it onto the same
			// scale as everything else in the table before it leaves.
			mode, ok := ratings[PlayerKey(playerID)]
			if !ok {
				// Unreachable: buildSide emits the mode-level entrant for
				// every player it emits an interaction for. Fall back to the
				// prior rather than dropping the row, so a hand-written or
				// future input log that breaks that pairing still round-trips.
				mode = r.engine.Prior()
			}
			players[key] = combineSeat(mode, rt)
		}
	}

	if err := r.store.SaveAll(ctx, gameID, modeKey, r.engine.ID(), players, entities); err != nil {
		return Report{}, fmt.Errorf("rating: save replay results for %s/%s: %w", gameID, modeKey, err)
	}

	return Report{Matches: len(inputs), IdentifiabilityReport: Identifiability(inputs)}, nil
}

// InteractionSigmaRatio sets how much uncertainty a player's per-seat rating
// starts with, as a multiple of the uncertainty already on their mode-level
// estimate.
//
// At 1 the interaction and the mode-level term carry equal variance, which
// says we have no prior belief about whether a surprising result means the
// player is better than we thought in general or better than we thought in
// this seat specifically — an update splits evenly between the two. That is
// the honest default in the absence of calibration, and it puts the player's
// first appearance in a new seat at their mode-level rating widened by a
// factor of sqrt(2), around 41%.
//
// Both directions off this default are wrong in a way worth naming. Too low
// and the interaction never moves, so a specialist reads as equally
// competent in every seat — the failure JQ-229 exists to fix. Too high and
// the interaction absorbs everything, so the mode-level estimate a new seat
// inherits does no work and the player is effectively unrated again in each
// seat. JQ-154 replaces this with a prior calibrated by backtesting; until
// then it is a stated guess, not a measured value.
const InteractionSigmaRatio = 1.0

// seed is the starting rating for an entrant appearing for the first time in
// a replay.
//
// Everything except a per-seat interaction starts from the engine's prior.
// An interaction starts centred on zero — no deviation from the player's own
// average is the correct default belief, and it is what makes a player's
// first match in a seat rate them at their mode-level estimate rather than
// at the global prior or at another seat's number.
//
// Its uncertainty is derived from the player's mode-level uncertainty as it
// stands at this point in the replay, so an established player entering a new
// seat gets a modestly widened estimate while an unknown one gets a wide one.
// Reading a value out of the accumulating map keeps this deterministic:
// ReplayMode reads every entrant's rating for a match before it writes any of
// them back, so the value here is always the one from before this match, and
// never depends on entrant order within it.
func (r *Replayer) seed(key string, ratings map[string]Rating) Rating {
	playerID, seatClass, isPlayer := ParsePlayerKey(key)
	if !isPlayer || seatClass == "" {
		return r.engine.Prior()
	}

	mode, ok := ratings[PlayerKey(playerID)]
	if !ok {
		mode = r.engine.Prior()
	}
	return Rating{Mu: 0, Sigma: mode.Sigma * InteractionSigmaRatio}
}

// combineSeat projects a player's mode-level rating and one seat interaction
// into the rating for that player in that seat.
//
// Means add, because the two are additive terms of one side's strength.
// Uncertainties add in quadrature, because they are the standard deviations
// of two estimates being summed — adding them directly would overstate the
// spread of the total.
func combineSeat(mode, interaction Rating) Rating {
	return Rating{
		Mu:    mode.Mu + interaction.Mu,
		Sigma: math.Hypot(mode.Sigma, interaction.Sigma),
	}
}

// Identifiability reports whether each modifier — and each modifier category
// — is separable from player skill in the given history.
//
// It is exported and takes inputs directly because replay is not the only
// caller that needs it: the backtest harness (internal/ratingbacktest) reports
// the same statistic alongside every score, since a comparison of two engines
// over history where a modifier is confounded is a comparison of two numbers
// that both mean something other than what they appear to. Sharing the
// function rather than the reasoning is what keeps the two reports from
// drifting into disagreement about the same history.
//
// Nothing here is order-sensitive: it is built from sets and counts, and
// nothing it computes feeds back into an Engine.Rate call.
func Identifiability(inputs []Input) IdentifiabilityReport {
	acc := newModifierAccumulator()
	for _, in := range inputs {
		acc.observe(in.SessionID, in.Sides)
	}
	return IdentifiabilityReport{Modifiers: acc.report(), Categories: acc.categories()}
}

// modifierAccumulator tallies identifiability bookkeeping while replay walks
// the input log. It is built entirely from sets and counts, so — unlike the
// engine-call path above — nothing here is order-sensitive: nothing it
// computes feeds back into an Engine.Rate call, so using maps freely here
// does not put determinism at risk.
type modifierAccumulator struct {
	// sessionsByKey counts, per modifier key, the distinct sessions it
	// appeared in (Matches).
	sessionsByKey map[string]map[string]bool
	// valuesByCategory tracks, per category, the distinct values seen
	// anywhere in the replay (DistinctValues).
	valuesByCategory map[string]map[string]bool
	// playersByKey tracks, per modifier key, every player ever seen sharing
	// a side with it.
	playersByKey map[string]map[string]bool
	// valuesByCategoryPlayer tracks, per category then player, every value
	// of that category the player has ever been seen with — the basis for
	// PlayersWithMultipleValues.
	valuesByCategoryPlayer map[string]map[string]map[string]bool
	// playersByCategory tracks, per category, every player who carried any
	// value of it — the denominator of the specialization rate.
	playersByCategory map[string]map[string]bool
}

func newModifierAccumulator() *modifierAccumulator {
	return &modifierAccumulator{
		sessionsByKey:          make(map[string]map[string]bool),
		valuesByCategory:       make(map[string]map[string]bool),
		playersByKey:           make(map[string]map[string]bool),
		valuesByCategoryPlayer: make(map[string]map[string]map[string]bool),
		playersByCategory:      make(map[string]map[string]bool),
	}
}

func (m *modifierAccumulator) observe(sessionID string, sides []Side) {
	for _, side := range sides {
		var players []string
		var modKeys []string
		// seatOf records which seat class each player on this side actually
		// held, read straight off the interaction entrants (JQ-229). It is
		// empty for a version 1 input row, which did not carry them.
		seatOf := map[string]string{}

		for _, e := range side.Entrants {
			playerID, seatClass, isPlayer := ParsePlayerKey(e.Key)
			switch {
			case !isPlayer:
				modKeys = append(modKeys, e.Key)
			case seatClass == "":
				players = append(players, e.Key)
			default:
				seatOf[PlayerKey(playerID)] = seatClass
			}
		}

		for _, key := range modKeys {
			category, value := splitModifierKey(key)

			if m.sessionsByKey[key] == nil {
				m.sessionsByKey[key] = make(map[string]bool)
			}
			m.sessionsByKey[key][sessionID] = true

			if m.valuesByCategory[category] == nil {
				m.valuesByCategory[category] = make(map[string]bool)
			}
			m.valuesByCategory[category][value] = true

			if m.playersByKey[key] == nil {
				m.playersByKey[key] = make(map[string]bool)
			}
			if m.valuesByCategoryPlayer[category] == nil {
				m.valuesByCategoryPlayer[category] = make(map[string]map[string]bool)
			}
			if m.playersByCategory[category] == nil {
				m.playersByCategory[category] = make(map[string]bool)
			}

			for _, p := range attributedPlayers(category, value, players, seatOf) {
				m.playersByKey[key][p] = true
				m.playersByCategory[category][p] = true
				if m.valuesByCategoryPlayer[category][p] == nil {
					m.valuesByCategoryPlayer[category][p] = make(map[string]bool)
				}
				m.valuesByCategoryPlayer[category][p][value] = true
			}
		}
	}
}

// attributedPlayers answers which players on a side should count as having
// carried a given modifier value.
//
// For most categories the only available answer is co-presence: a pre-queue
// option or a scenario is recorded per side, not per player, so every player
// on the side is credited with it. That is exact for those categories, since
// a scenario really is a property of the whole side.
//
// Seats are the exception, and getting them wrong silently breaks the
// statistic this whole report exists to provide. On a side holding a Clue
// Giver and two Guessers, co-presence credits all three players with both
// seat classes, so all three read as having varied across seats and the seat
// modifier reads as identifiable — when in fact none of them ever moved.
// That is the precise failure mode JQ-229 is about, reported backwards.
// Version 2 input rows carry an interaction entrant naming the seat each
// player actually held, so where they are present the attribution is exact.
// Version 1 rows never recorded it, so they keep the old co-presence
// behaviour rather than inventing an answer.
func attributedPlayers(category, value string, players []string, seatOf map[string]string) []string {
	if category != seatCategory || len(seatOf) == 0 {
		return players
	}
	held := make([]string, 0, len(players))
	for _, p := range players {
		if seatOf[p] == value {
			held = append(held, p)
		}
	}
	return held
}

func (m *modifierAccumulator) report() map[string]ModifierReport {
	out := make(map[string]ModifierReport, len(m.sessionsByKey))
	for key, sessions := range m.sessionsByKey {
		category, _ := splitModifierKey(key)
		withMultiple := 0
		for p := range m.playersByKey[key] {
			if len(m.valuesByCategoryPlayer[category][p]) > 1 {
				withMultiple++
			}
		}
		out[key] = ModifierReport{
			Matches:                   len(sessions),
			DistinctValues:            len(m.valuesByCategory[category]),
			DistinctPlayers:           len(m.playersByKey[key]),
			PlayersWithMultipleValues: withMultiple,
		}
	}
	return out
}

// categories rolls the per-key tallies up to one report per category, which
// is the level the identifiability question is actually asked at: whether
// "seat" is separable from player skill in this mode is one fact about the
// mode, not one fact per seat.
func (m *modifierAccumulator) categories() map[string]CategoryReport {
	out := make(map[string]CategoryReport, len(m.playersByCategory))
	for category, players := range m.playersByCategory {
		withMultiple := 0
		for p := range players {
			if len(m.valuesByCategoryPlayer[category][p]) > 1 {
				withMultiple++
			}
		}
		out[category] = CategoryReport{
			DistinctValues:            len(m.valuesByCategory[category]),
			Players:                   len(players),
			PlayersWithMultipleValues: withMultiple,
		}
	}
	return out
}

// splitModifierKey splits a modifier's namespaced key into a category and a
// value.
//
// Most namespaces are single-dimension: everything after the colon is the
// value, e.g. "seat:White" -> ("seat", "White"). This holds even when that
// remainder itself contains "/" — "seat:Team/Guesser" -> ("seat",
// "Team/Guesser") — because a seat's NamePath (see internal/seattemplate)
// joins nested template segments with "/" to name one value of the "seat"
// dimension, not a further split within it.
//
// "prequeue" is the one namespace with a real second dimension inside it:
// its keys are "prequeue:<groupKey>/<optionID>" (see BuildSides in
// internal/rating/outcome.go), where groupKey names a distinct pre-queue
// option group — e.g. "color" and "map" are unrelated dimensions that must
// not be lumped into one "prequeue" category, or a player locked to one
// color across many maps would be misreported as having a varying,
// identifiable color. For that namespace only, the category includes the
// group key: "prequeue:color/white" -> ("prequeue:color", "white").
//
// A key with no ":" is its own single-value category (e.g. "scenario"),
// which is intrinsically unidentifiable — there is no other value to vary
// against.
func splitModifierKey(key string) (category, value string) {
	idx := strings.Index(key, ":")
	if idx < 0 {
		return key, key
	}
	namespace, rest := key[:idx], key[idx+1:]
	if namespace == "prequeue" {
		if slash := strings.Index(rest, "/"); slash >= 0 {
			return namespace + ":" + rest[:slash], rest[slash+1:]
		}
	}
	return namespace, rest
}
