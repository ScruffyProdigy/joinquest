package rating

import (
	"reflect"
	"strings"
	"testing"

	"github.com/scruffyprodigy/joinquest/internal/prequeue"
)

func keysOf(sides []Side) [][]string {
	out := make([][]string, len(sides))
	for i, s := range sides {
		for _, e := range s.Entrants {
			out[i] = append(out[i], e.Key)
		}
	}
	return out
}

// A symmetric mode has one seat class, so no modifier entities appear at all.
func TestBuildSidesSymmetricOneVersusOneHasNoModifiers(t *testing.T) {
	shape := ModeShape{SeatClasses: map[string]string{"1": "", "2": ""}}
	out := MatchOutcome{
		Participants: []Participant{
			{PlayerID: "a", SeatKey: "1", Placement: intp(1)},
			{PlayerID: "b", SeatKey: "2", Placement: intp(2)},
		},
	}

	sides, err := BuildSides(shape, out)
	if err != nil {
		t.Fatalf("BuildSides: %v", err)
	}
	if got := keysOf(sides); len(got) != 2 || len(got[0]) != 1 || got[0][0] != "player:a" {
		t.Fatalf("sides = %v, want two solo player sides", got)
	}
	if sides[0].Rank >= sides[1].Rank {
		t.Errorf("placement 1 rank %d not better than placement 2 rank %d",
			sides[0].Rank, sides[1].Rank)
	}
}

// More than one seat class means asymmetry: each class joins its own side.
func TestBuildSidesAsymmetricAddsSeatClassEntrants(t *testing.T) {
	shape := ModeShape{SeatClasses: map[string]string{"White": "White", "Black": "Black"}}
	out := MatchOutcome{
		Participants: []Participant{
			{PlayerID: "w", SeatKey: "White", Placement: intp(2)},
			{PlayerID: "b", SeatKey: "Black", Placement: intp(1)},
		},
	}

	sides, err := BuildSides(shape, out)
	if err != nil {
		t.Fatalf("BuildSides: %v", err)
	}
	got := keysOf(sides)
	if len(got) != 2 {
		t.Fatalf("sides = %v, want 2", got)
	}
	for _, side := range got {
		if len(side) != 2 {
			t.Fatalf("side %v should hold the player and its seat-class entity", side)
		}
	}
}

// Teammates share a side; the seat class entity appears once, not per player
// — three Guessers on one side still add a single seat:Team/Guesser entrant.
func TestBuildSidesGroupsTeammatesAndAddsClassOnce(t *testing.T) {
	shape := ModeShape{SeatClasses: map[string]string{
		"Team-1-SpyMaster": "Team/SpyMaster",
		"Team-1-Guesser-1": "Team/Guesser",
		"Team-1-Guesser-2": "Team/Guesser",
		"Team-1-Guesser-3": "Team/Guesser",
		"Team-2-SpyMaster": "Team/SpyMaster",
		"Team-2-Guesser-1": "Team/Guesser",
	}}
	out := MatchOutcome{
		Participants: []Participant{
			{PlayerID: "a", SeatKey: "Team-1-SpyMaster", TeamKey: "Team:1", IsWinner: true},
			{PlayerID: "b", SeatKey: "Team-1-Guesser-1", TeamKey: "Team:1", IsWinner: true},
			{PlayerID: "e", SeatKey: "Team-1-Guesser-2", TeamKey: "Team:1", IsWinner: true},
			{PlayerID: "f", SeatKey: "Team-1-Guesser-3", TeamKey: "Team:1", IsWinner: true},
			{PlayerID: "c", SeatKey: "Team-2-SpyMaster", TeamKey: "Team:2"},
			{PlayerID: "d", SeatKey: "Team-2-Guesser-1", TeamKey: "Team:2"},
		},
	}

	sides, err := BuildSides(shape, out)
	if err != nil {
		t.Fatalf("BuildSides: %v", err)
	}
	if len(sides) != 2 {
		t.Fatalf("sides = %d, want 2", len(sides))
	}
	for _, side := range sides {
		seen := map[string]int{}
		for _, e := range side.Entrants {
			seen[e.Key]++
		}
		if seen["seat:Team/Guesser"] != 1 {
			t.Errorf("side %v: seat:Team/Guesser entrant count = %d, want exactly 1", side.Entrants, seen["seat:Team/Guesser"])
		}
	}
}

// Co-op: one player side against a scenario entity that is nobody's teammate.
func TestBuildSidesCooperativeAddsScenarioOpponent(t *testing.T) {
	shape := ModeShape{
		SeatClasses: map[string]string{"1": "", "2": "", "3": ""},
		Cooperative: true,
	}
	out := MatchOutcome{
		CooperativeSuccess: boolp(true),
		Participants: []Participant{
			{PlayerID: "a", SeatKey: "1"},
			{PlayerID: "b", SeatKey: "2"},
			{PlayerID: "c", SeatKey: "3"},
		},
		ScenarioKeys: []string{"standard"},
	}

	sides, err := BuildSides(shape, out)
	if err != nil {
		t.Fatalf("BuildSides: %v", err)
	}
	if len(sides) != 2 {
		t.Fatalf("sides = %d, want the crew and the scenario", len(sides))
	}
	crew, scenario := sides[0], sides[1]
	if len(crew.Entrants) != 3 {
		t.Errorf("crew = %v, want 3 players", crew.Entrants)
	}
	if len(scenario.Entrants) != 1 || scenario.Entrants[0].Key != "scenario:standard" {
		t.Fatalf("opposing side = %v, want a single scenario entrant", scenario.Entrants)
	}
	if crew.Rank >= scenario.Rank {
		t.Errorf("crew succeeded, so its rank %d must beat the scenario's %d",
			crew.Rank, scenario.Rank)
	}
}

// A cooperative match's participants can carry non-empty TeamKeys (e.g. from
// a grouped seat template). Cooperative grouping must win over team
// grouping: everyone still lands on one crew side, not split by team.
func TestBuildSidesCooperativeIgnoresTeamKeyGrouping(t *testing.T) {
	shape := ModeShape{
		SeatClasses: map[string]string{"1": "", "2": "", "3": ""},
		Cooperative: true,
	}
	out := MatchOutcome{
		CooperativeSuccess: boolp(true),
		Participants: []Participant{
			{PlayerID: "a", SeatKey: "1", TeamKey: "Team:1"},
			{PlayerID: "b", SeatKey: "2", TeamKey: "Team:1"},
			{PlayerID: "c", SeatKey: "3", TeamKey: "Team:2"},
		},
		ScenarioKeys: []string{"standard"},
	}

	sides, err := BuildSides(shape, out)
	if err != nil {
		t.Fatalf("BuildSides: %v", err)
	}
	if len(sides) != 2 {
		t.Fatalf("sides = %d, want the crew and the scenario, got %v", len(sides), keysOf(sides))
	}
	crew, scenario := sides[0], sides[1]
	if len(crew.Entrants) != 3 {
		t.Errorf("crew = %v, want all 3 players on one crew side despite differing TeamKeys", crew.Entrants)
	}
	if len(scenario.Entrants) != 1 || scenario.Entrants[0].Key != "scenario:standard" {
		t.Fatalf("opposing side = %v, want a single scenario entrant", scenario.Entrants)
	}
}

func TestBuildSidesCooperativeFailureFlipsRanks(t *testing.T) {
	shape := ModeShape{SeatClasses: map[string]string{"1": ""}, Cooperative: true}
	out := MatchOutcome{
		CooperativeSuccess: boolp(false),
		Participants:       []Participant{{PlayerID: "a", SeatKey: "1"}},
		ScenarioKeys:       []string{"standard"},
	}

	sides, err := BuildSides(shape, out)
	if err != nil {
		t.Fatalf("BuildSides: %v", err)
	}
	if sides[0].Rank <= sides[1].Rank {
		t.Errorf("crew failed, so the scenario must rank better: %d vs %d",
			sides[0].Rank, sides[1].Rank)
	}
}

func TestBuildSidesEqualPlacementsAreATie(t *testing.T) {
	shape := ModeShape{SeatClasses: map[string]string{"1": "", "2": ""}}
	out := MatchOutcome{
		Participants: []Participant{
			{PlayerID: "a", SeatKey: "1", Placement: intp(1)},
			{PlayerID: "b", SeatKey: "2", Placement: intp(1)},
		},
	}

	sides, err := BuildSides(shape, out)
	if err != nil {
		t.Fatalf("BuildSides: %v", err)
	}
	if sides[0].Rank != sides[1].Rank {
		t.Errorf("equal placements must share a rank: %d vs %d", sides[0].Rank, sides[1].Rank)
	}
}

// No placements reported: fall back to is_winner. Games report one or the other.
func TestBuildSidesFallsBackToIsWinnerWhenNoPlacements(t *testing.T) {
	shape := ModeShape{SeatClasses: map[string]string{"1": "", "2": ""}}
	out := MatchOutcome{
		Participants: []Participant{
			{PlayerID: "a", SeatKey: "1", IsWinner: true},
			{PlayerID: "b", SeatKey: "2"},
		},
	}

	sides, err := BuildSides(shape, out)
	if err != nil {
		t.Fatalf("BuildSides: %v", err)
	}
	if sides[0].Rank >= sides[1].Rank {
		t.Errorf("winner rank %d must beat loser rank %d", sides[0].Rank, sides[1].Rank)
	}
}

// An unrateable match must be refused, not silently rated as a draw.
func TestBuildSidesRejectsOutcomeWithNoSignal(t *testing.T) {
	shape := ModeShape{SeatClasses: map[string]string{"1": "", "2": ""}}
	out := MatchOutcome{
		Participants: []Participant{
			{PlayerID: "a", SeatKey: "1"},
			{PlayerID: "b", SeatKey: "2"},
		},
	}

	if _, err := BuildSides(shape, out); err == nil {
		t.Fatal("BuildSides with no placements, no winners and no co-op result = nil error, want an error")
	}
}

func intp(i int) *int    { return &i }
func boolp(b bool) *bool { return &b }

// A group that asks for exactly one pick is a single categorical value —
// structurally the same thing as a seat class, and ratable the same way.
func TestBuildSidesRatesSinglePickPreQueueGroup(t *testing.T) {
	shape := ModeShape{
		SeatClasses:         map[string]string{"1": "", "2": ""},
		RatedPreQueueGroups: []string{"color"},
	}
	out := MatchOutcome{
		Participants: []Participant{
			{PlayerID: "a", SeatKey: "1", IsWinner: true,
				PreQueue: map[string][]string{"color": {"white"}}},
			{PlayerID: "b", SeatKey: "2",
				PreQueue: map[string][]string{"color": {"black"}}},
		},
	}

	sides, err := BuildSides(shape, out)
	if err != nil {
		t.Fatalf("BuildSides: %v", err)
	}
	want := map[string]bool{"prequeue:color/white": false, "prequeue:color/black": false}
	for _, side := range sides {
		for _, e := range side.Entrants {
			if _, ok := want[e.Key]; ok {
				want[e.Key] = true
			}
		}
	}
	for key, found := range want {
		if !found {
			t.Errorf("missing entrant %q", key)
		}
	}
}

// A group not listed as rated contributes nothing, even when it has one pick.
func TestBuildSidesIgnoresUnratedPreQueueGroup(t *testing.T) {
	shape := ModeShape{SeatClasses: map[string]string{"1": "", "2": ""}}
	out := MatchOutcome{
		Participants: []Participant{
			{PlayerID: "a", SeatKey: "1", IsWinner: true,
				PreQueue: map[string][]string{"color": {"white"}}},
			{PlayerID: "b", SeatKey: "2",
				PreQueue: map[string][]string{"color": {"black"}}},
		},
	}

	sides, err := BuildSides(shape, out)
	if err != nil {
		t.Fatalf("BuildSides: %v", err)
	}
	for _, side := range sides {
		for _, e := range side.Entrants {
			if strings.HasPrefix(e.Key, "prequeue:") {
				t.Errorf("unrated group produced entrant %q", e.Key)
			}
		}
	}
}

// A multi-pick group is a combination, and strength lives in how picks
// interact — which an additive per-option entity cannot express at any volume.
func TestRatedPreQueueGroupsRejectsMultiPickGroup(t *testing.T) {
	groups := []prequeue.Group{
		{Key: "color", Kind: prequeue.KindCharacter, Min: 1, Max: 1},
		{Key: "loadout", Kind: prequeue.KindLoadout, Min: 1, Max: 3},
		{Key: "deck", Kind: prequeue.KindDeck, Min: 0, Max: 1},
	}

	got := RatedPreQueueGroups(groups)

	if len(got) != 1 || got[0] != "color" {
		t.Errorf("RatedPreQueueGroups = %v, want only [color]", got)
	}
}

func TestBuildSidesCooperativeAddsOneEntrantPerScenarioKey(t *testing.T) {
	success := true
	shape := ModeShape{
		SeatClasses: map[string]string{"s1": "Crew", "s2": "Crew"},
		Cooperative: true,
	}
	out := MatchOutcome{
		Participants: []Participant{
			{PlayerID: "a", SeatKey: "s1"},
			{PlayerID: "b", SeatKey: "s2"},
		},
		CooperativeSuccess: &success,
		ScenarioKeys:       []string{"night", "hard", "night"},
	}

	sides, err := BuildSides(shape, out)
	if err != nil {
		t.Fatalf("BuildSides: %v", err)
	}
	if len(sides) != 2 {
		t.Fatalf("got %d sides, want 2", len(sides))
	}

	scenario := sides[len(sides)-1]
	var keys []string
	for _, e := range scenario.Entrants {
		keys = append(keys, e.Key)
	}
	want := []string{"scenario:hard", "scenario:night"}
	if !reflect.DeepEqual(keys, want) {
		t.Fatalf("scenario entrants = %v, want %v (deduplicated and sorted)", keys, want)
	}
	if scenario.Rank != 1 {
		t.Fatalf("scenario rank = %d, want 1 on a successful clear", scenario.Rank)
	}
}

func TestBuildSidesCooperativeWithoutScenarioKeysIsUnrateable(t *testing.T) {
	success := true
	shape := ModeShape{
		SeatClasses: map[string]string{"s1": "Crew", "s2": "Crew"},
		Cooperative: true,
	}
	out := MatchOutcome{
		Participants: []Participant{
			{PlayerID: "a", SeatKey: "s1"},
			{PlayerID: "b", SeatKey: "s2"},
		},
		CooperativeSuccess: &success,
	}

	if _, err := BuildSides(shape, out); err == nil {
		t.Fatal("BuildSides succeeded without scenario keys; want an error rather than a guessed opponent")
	}
}
