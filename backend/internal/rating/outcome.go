package rating

import (
	"fmt"
	"sort"
	"strings"
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
}

// Participant is one player's outcome, as the game reported it.
type Participant struct {
	PlayerID  string
	SeatKey   string
	TeamKey   string // affinity key; empty means the player is their own side
	Placement *int   // 1-based; equal placements are a tie
	IsWinner  bool
}

// MatchOutcome is one finished match.
type MatchOutcome struct {
	Participants []Participant
	// CooperativeSuccess is set only for cooperative modes. Nil elsewhere.
	CooperativeSuccess *bool
}

// scenarioEntrantKey names the single opponent entity a cooperative match's
// crew is rated against.
const scenarioEntrantKey = "scenario"

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

	asymmetric := distinctCount(shape.SeatClasses) > 1
	groups := groupParticipants(shape, outcome.Participants)

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
		sides = append(sides, buildSide(shape.SeatClasses, asymmetric, participants, ranks[key]))
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
			Entrants: []Entrant{{Key: scenarioEntrantKey}},
			Rank:     scenarioRank,
		})
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
	if p.TeamKey != "" {
		return "team:" + p.TeamKey
	}
	if shape.Cooperative {
		return "crew"
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
		for key, ps := range groups {
			rank := 1
			for _, p := range ps {
				if p.IsWinner {
					rank = 0
					break
				}
			}
			ranks[key] = rank
		}

	default:
		return nil, fmt.Errorf("rating: outcome has no placements, winners, or cooperative result to rank sides by")
	}

	return ranks, nil
}

// buildSide turns one group of teammates into a Side: one entrant per
// player, plus — in an asymmetric mode — one entrant per distinct seat class
// held by the group, deduplicated so three Guessers still add a single
// seat:Team/Guesser entity rather than three.
func buildSide(seatClasses map[string]string, asymmetric bool, participants []Participant, rank int) Side {
	entrantSet := map[string]Entrant{}
	for _, p := range participants {
		key := "player:" + p.PlayerID
		entrantSet[key] = Entrant{Key: key}
		if asymmetric {
			if namePath, ok := seatClasses[p.SeatKey]; ok {
				seatKey := "seat:" + namePath
				entrantSet[seatKey] = Entrant{Key: seatKey}
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
