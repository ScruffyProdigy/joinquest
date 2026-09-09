package rating

import "testing"

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

// Teammates share a side; the seat class entity appears once, not per player.
func TestBuildSidesGroupsTeammatesAndAddsClassOnce(t *testing.T) {
	shape := ModeShape{SeatClasses: map[string]string{
		"Team-1-SpyMaster": "Team/SpyMaster",
		"Team-1-Guesser-1": "Team/Guesser",
		"Team-2-SpyMaster": "Team/SpyMaster",
		"Team-2-Guesser-1": "Team/Guesser",
	}}
	out := MatchOutcome{
		Participants: []Participant{
			{PlayerID: "a", SeatKey: "Team-1-SpyMaster", TeamKey: "Team:1", IsWinner: true},
			{PlayerID: "b", SeatKey: "Team-1-Guesser-1", TeamKey: "Team:1", IsWinner: true},
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
		if seen["seat:Team/Guesser"] > 1 {
			t.Errorf("seat class entity duplicated within a side: %v", side.Entrants)
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
	if len(scenario.Entrants) != 1 || scenario.Entrants[0].Key != "scenario" {
		t.Fatalf("opposing side = %v, want a single scenario entrant", scenario.Entrants)
	}
	if crew.Rank >= scenario.Rank {
		t.Errorf("crew succeeded, so its rank %d must beat the scenario's %d",
			crew.Rank, scenario.Rank)
	}
}

func TestBuildSidesCooperativeFailureFlipsRanks(t *testing.T) {
	shape := ModeShape{SeatClasses: map[string]string{"1": ""}, Cooperative: true}
	out := MatchOutcome{
		CooperativeSuccess: boolp(false),
		Participants:       []Participant{{PlayerID: "a", SeatKey: "1"}},
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
