package rating

import (
	"context"
	"fmt"
	"strings"
)

// playerKeyPrefix marks an Entrant.Key as a player rather than a modifier
// entity (a seat class, a scenario, a rated pre-queue option). It mirrors the
// "player:<uuid>" convention documented on Entrant and on the store's
// nonplayer_ratings.entity_key column.
const playerKeyPrefix = "player:"

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
	// PlayersWithMultipleValues is the number of players who appeared with
	// this modifier key at least once, and who are also known — from
	// elsewhere in the same replay — to have appeared under a different
	// value of the same category. Zero means every player who ever carried
	// this value carried only this value: skill and modifier are
	// confounded for all of them, and the split is arbitrary.
	PlayersWithMultipleValues int
}

// Report summarizes one ReplayMode run.
type Report struct {
	Matches   int
	Modifiers map[string]ModifierReport
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
					rt = r.engine.Prior()
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
		if strings.HasPrefix(key, playerKeyPrefix) {
			players[key] = rt
		} else {
			entities[key] = rt
		}
	}

	if err := r.store.SaveAll(ctx, gameID, modeKey, r.engine.ID(), players, entities); err != nil {
		return Report{}, fmt.Errorf("rating: save replay results for %s/%s: %w", gameID, modeKey, err)
	}

	return Report{Matches: len(inputs), Modifiers: ModifierIdentifiability(inputs)}, nil
}

// ModifierIdentifiability reports, per modifier key, whether that modifier is
// separable from player skill in the given history.
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
func ModifierIdentifiability(inputs []Input) map[string]ModifierReport {
	acc := newModifierAccumulator()
	for _, in := range inputs {
		acc.observe(in.SessionID, in.Sides)
	}
	return acc.report()
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
}

func newModifierAccumulator() *modifierAccumulator {
	return &modifierAccumulator{
		sessionsByKey:          make(map[string]map[string]bool),
		valuesByCategory:       make(map[string]map[string]bool),
		playersByKey:           make(map[string]map[string]bool),
		valuesByCategoryPlayer: make(map[string]map[string]map[string]bool),
	}
}

func (m *modifierAccumulator) observe(sessionID string, sides []Side) {
	for _, side := range sides {
		var players []string
		var modKeys []string
		for _, e := range side.Entrants {
			if strings.HasPrefix(e.Key, playerKeyPrefix) {
				players = append(players, e.Key)
			} else {
				modKeys = append(modKeys, e.Key)
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

			for _, p := range players {
				m.playersByKey[key][p] = true
				if m.valuesByCategoryPlayer[category][p] == nil {
					m.valuesByCategoryPlayer[category][p] = make(map[string]bool)
				}
				m.valuesByCategoryPlayer[category][p][value] = true
			}
		}
	}
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
