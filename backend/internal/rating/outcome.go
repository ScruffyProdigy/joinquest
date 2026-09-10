package rating

import (
	"fmt"
	"sort"
	"strings"

	"github.com/scruffyprodigy/joinquest/internal/prequeue"
)

// ModeShape is everything about a mode the translation needs, resolved once by
// the caller from the mode's seat template and manifest.
type ModeShape struct {
	// SeatClasses maps seat key to NamePath. More than one distinct value means
	// the mode is asymmetric and each class becomes a rated entity; exactly one
	// means symmetric, and no modifier entities are produced at all.
	SeatClasses map[string]string

	// Cooperative means the players win or lose together against the mode
	// itself, so a scenario entrant supplies the opponent Weng-Lin needs.
	Cooperative bool

	// RatedPreQueueGroups names the pre-queue option groups (by Group.Key)
	// whose selections carry a rating of their own, as returned by
	// RatedPreQueueGroups. A group not listed here contributes no entrant
	// even if a participant reports a selection for it. Enabling a group is
	// a human decision made after reading the identifiability report — this
	// field is never populated automatically.
	RatedPreQueueGroups []string
}

// Participant is one player's outcome, as the game reported it.
type Participant struct {
	PlayerID  string
	SeatKey   string
	TeamKey   string // affinity key; empty means the player is their own side
	Placement *int   // 1-based; equal placements are a tie
	IsWinner  bool

	// PreQueue holds the player's pre-queue selections, keyed by
	// prequeue.Group.Key, as reported by the game at request time. Only
	// groups named in ModeShape.RatedPreQueueGroups produce entrants; other
	// entries here are ignored.
	PreQueue map[string][]string

	// Excluded marks a participant who took part but must not be rated —
	// a player the lobby knows dropped out for a reason that says nothing
	// about skill. They are removed before sides are formed, so they neither
	// gain nor lose rating and neither help nor hurt the side they were on.
	// The caller decides which reasons qualify; this package does not know
	// what a disconnect is.
	Excluded bool
}

// MatchOutcome is one finished match.
type MatchOutcome struct {
	Participants []Participant
	// CooperativeSuccess is set only for cooperative modes. Nil elsewhere.
	CooperativeSuccess *bool
	// ScenarioKeys names what the crew faced, as the game reported it —
	// "hard", "night", "wave-12". Each becomes one entrant on the opposing
	// side, and the engine learns each key's strength from how crews fare
	// against it; the platform assigns the ratings, the game only supplies
	// the identifiers. Several keys compose the way teammates do, so a game
	// can report orthogonal facets and each carries its own contribution.
	// Required for a cooperative mode and ignored elsewhere.
	ScenarioKeys []string
}

// scenarioEntrantPrefix namespaces a cooperative scenario key inside
// nonplayer_ratings.entity_key, alongside "seat:" and "prequeue:".
const scenarioEntrantPrefix = "scenario:"

// scenarioEntrants turns the reported scenario keys into the entrants of the
// crew's opposing side, deduplicated and sorted so the output is stable for
// replay regardless of the order the game listed them in.
func scenarioEntrants(keys []string) []Entrant {
	seen := make(map[string]bool, len(keys))
	entrants := make([]Entrant, 0, len(keys))
	for _, k := range keys {
		key := scenarioEntrantPrefix + k
		if seen[key] {
			continue
		}
		seen[key] = true
		entrants = append(entrants, Entrant{Key: key})
	}
	sort.Slice(entrants, func(i, j int) bool { return entrants[i].Key < entrants[j].Key })
	return entrants
}

// BuildSides translates a finished match into the sides the rating engine
// consumes. It is the only place that knows what a game, a seat, a team or a
// cooperative match is; the engine below it only sees keyed entrants and a
// rank per side.
//
// Every returned Entrant carries a zero-valued Rating — this function does
// not know current ratings, the caller loads them from the store.
//
// Output is fully deterministic for a given (shape, outcome): the sides
// holding real participants are sorted by rank then a side key, entrants
// within a side are sorted by key, and (for a cooperative match) the
// scenario side is always the last element. Later tasks replay this output
// to recompute ratings, so relying on Go's randomised map iteration order
// anywhere here would make that replay non-reproducible.
func BuildSides(shape ModeShape, outcome MatchOutcome) ([]Side, error) {
	if len(outcome.Participants) == 0 {
		return nil, fmt.Errorf("rating: match outcome has no participants")
	}
	if shape.Cooperative && outcome.CooperativeSuccess == nil {
		return nil, fmt.Errorf("rating: cooperative match outcome has no success result")
	}
	if shape.Cooperative && len(outcome.ScenarioKeys) == 0 {
		return nil, fmt.Errorf("rating: cooperative match outcome reported no scenario keys")
	}

	rated := make([]Participant, 0, len(outcome.Participants))
	for _, p := range outcome.Participants {
		if p.Excluded {
			continue
		}
		rated = append(rated, p)
	}
	if len(rated) == 0 {
		return nil, fmt.Errorf("rating: every participant is excluded from rating")
	}

	asymmetric := distinctCount(shape.SeatClasses) > 1
	groups := groupParticipants(shape, rated)

	var ranks map[string]int
	if shape.Cooperative {
		// The scenario opponent's success/failure is the only signal that
		// matters for a co-op crew's rank; placements and is_winner (if the
		// game even sends them for a co-op mode) are not consulted.
		crewRank := 0
		if !*outcome.CooperativeSuccess {
			crewRank = 1
		}
		ranks = make(map[string]int, len(groups))
		for key := range groups {
			ranks[key] = crewRank
		}
	} else {
		var err error
		ranks, err = deriveRanks(groups)
		if err != nil {
			return nil, err
		}
	}

	sides := make([]Side, 0, len(groups)+1)
	for key, participants := range groups {
		sides = append(sides, buildSide(shape, asymmetric, participants, ranks[key]))
	}
	sortSides(sides)

	if shape.Cooperative {
		scenarioRank := 1
		if !*outcome.CooperativeSuccess {
			scenarioRank = 0
		}
		// Appended after the sort, not merged into it: the scenario side is
		// always the last element regardless of whether it out-ranks the
		// crew, so a failed match doesn't reorder which side is "the crew".
		sides = append(sides, Side{
			Entrants: scenarioEntrants(outcome.ScenarioKeys),
			Rank:     scenarioRank,
		})
	}

	// Fewer than two sides is not a match the engine can learn anything from:
	// Weng-Lin rates sides against each other, and a lone side has no
	// opponent. This is reachable whenever exclusions empty out every side
	// but one — a 1v1 where the loser disconnected, say.
	if len(sides) < 2 {
		return nil, fmt.Errorf("rating: match has %d rateable side(s), want at least 2", len(sides))
	}

	return sides, nil
}

// groupParticipants clusters participants into the raw material for sides.
// Explicit teammates share a TeamKey and land on the same side. Without one,
// a competitive participant is its own side; a cooperative participant joins
// the shared crew, since a co-op match has no notion of solo sides on the
// crew's side of the table — they all live or die together.
func groupParticipants(shape ModeShape, participants []Participant) map[string][]Participant {
	groups := map[string][]Participant{}
	for _, p := range participants {
		key := groupKey(shape, p)
		groups[key] = append(groups[key], p)
	}
	return groups
}

func groupKey(shape ModeShape, p Participant) string {
	if shape.Cooperative {
		return "crew"
	}
	if p.TeamKey != "" {
		return "team:" + p.TeamKey
	}
	return "solo:" + p.PlayerID
}

// deriveRanks ranks each group of a competitive (non-cooperative) match:
// dense-ranked placements when any are present, else is_winner, else an
// error — an unrateable outcome must be refused, not silently treated as a
// draw.
func deriveRanks(groups map[string][]Participant) (map[string]int, error) {
	hasPlacement, hasWinner := false, false
	for _, ps := range groups {
		for _, p := range ps {
			if p.Placement != nil {
				hasPlacement = true
			}
			if p.IsWinner {
				hasWinner = true
			}
		}
	}

	ranks := make(map[string]int, len(groups))

	switch {
	case hasPlacement:
		placementOf := make(map[string]int, len(groups))
		for key, ps := range groups {
			var best *int
			for _, p := range ps {
				if p.Placement == nil {
					continue
				}
				if best == nil || *p.Placement < *best {
					best = p.Placement
				}
			}
			if best == nil {
				return nil, fmt.Errorf("rating: side %q has no placement while other sides do", key)
			}
			placementOf[key] = *best
		}

		distinct := map[int]struct{}{}
		for _, v := range placementOf {
			distinct[v] = struct{}{}
		}
		values := make([]int, 0, len(distinct))
		for v := range distinct {
			values = append(values, v)
		}
		sort.Ints(values)
		dense := make(map[int]int, len(values))
		for i, v := range values {
			dense[v] = i
		}
		for key, v := range placementOf {
			ranks[key] = dense[v]
		}

	case hasWinner:
		winningSides := 0
		for key, ps := range groups {
			rank := 1
			for _, p := range ps {
				if p.IsWinner {
					rank = 0
					break
				}
			}
			if rank == 0 {
				winningSides++
			}
			ranks[key] = rank
		}
		// Unlike equal placements, which state a draw, "everybody won" states
		// no ordering: nobody was better than anybody. Rating it would move
		// sigma on a report that carries no information.
		//
		// Requiring more than one group keeps this from claiming the far
		// commoner single-side case, where it would send an operator hunting
		// a phantom all-winner report: exclusions routinely leave one
		// winner-marked group (a 1v1 whose loser disconnected), and that is
		// BuildSides' side-count refusal below, which says so accurately.
		if len(groups) > 1 && winningSides == len(groups) {
			return nil, fmt.Errorf("rating: every side is marked a winner, so the outcome carries no ranking")
		}

	default:
		return nil, fmt.Errorf("rating: outcome has no placements, winners, or cooperative result to rank sides by")
	}

	return ranks, nil
}

// buildSide turns one group of teammates into a Side: one entrant per
// player, plus — in an asymmetric mode — one entrant per distinct seat class
// held by the group, deduplicated so three Guessers still add a single
// seat:Team/Guesser entity rather than three, plus one entrant per distinct
// rated pre-queue selection held by the group, deduplicated the same way.
func buildSide(shape ModeShape, asymmetric bool, participants []Participant, rank int) Side {
	entrantSet := map[string]Entrant{}
	for _, p := range participants {
		key := "player:" + p.PlayerID
		entrantSet[key] = Entrant{Key: key}
		if asymmetric {
			if namePath, ok := shape.SeatClasses[p.SeatKey]; ok {
				seatKey := "seat:" + namePath
				entrantSet[seatKey] = Entrant{Key: seatKey}
			}
		}
		for _, pqGroupKey := range shape.RatedPreQueueGroups {
			for _, optionID := range p.PreQueue[pqGroupKey] {
				key := "prequeue:" + pqGroupKey + "/" + optionID
				entrantSet[key] = Entrant{Key: key}
			}
		}
	}

	entrants := make([]Entrant, 0, len(entrantSet))
	for _, e := range entrantSet {
		entrants = append(entrants, e)
	}
	sort.Slice(entrants, func(i, j int) bool { return entrants[i].Key < entrants[j].Key })

	return Side{Entrants: entrants, Rank: rank}
}

// RatedPreQueueGroups returns the groups whose selections may carry a rating.
//
// Only a group that asks for exactly one required pick qualifies. Min == Max == 1
// makes the selection a single categorical value — a color, a faction, a
// character — which is structurally identical to a seat class. Anything else is
// a combination, and a combination's strength lives in how its picks interact,
// which an additive entity per option cannot represent no matter how much data
// it sees. An optional group is excluded for a different reason: players who
// skip it contribute nothing, so the entity would be estimated only from those
// who opted in.
func RatedPreQueueGroups(groups []prequeue.Group) []string {
	var out []string
	for _, g := range groups {
		if g.Min == 1 && g.Max == 1 {
			out = append(out, g.Key)
		}
	}
	sort.Strings(out)
	return out
}

func distinctCount(m map[string]string) int {
	set := map[string]struct{}{}
	for _, v := range m {
		set[v] = struct{}{}
	}
	return len(set)
}

// sortSides orders the participant-holding sides (never the scenario side,
// which BuildSides appends separately) deterministically: rank ascending,
// then by a side key derived from its entrants. This neutralizes the
// randomised iteration order of the groups map.
func sortSides(sides []Side) {
	sort.Slice(sides, func(i, j int) bool {
		if sides[i].Rank != sides[j].Rank {
			return sides[i].Rank < sides[j].Rank
		}
		return sideKey(sides[i]) < sideKey(sides[j])
	})
}

// sideKey is a deterministic fingerprint of a side's entrants, used only to
// order sides that tie on rank. Entrants are already sorted by key by
// buildSide.
func sideKey(s Side) string {
	keys := make([]string, len(s.Entrants))
	for i, e := range s.Entrants {
		keys[i] = e.Key
	}
	return strings.Join(keys, ",")
}
