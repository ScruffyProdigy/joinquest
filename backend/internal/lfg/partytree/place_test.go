package partytree

import (
	"testing"

	"github.com/scruffyprodigy/playhub/internal/lfg"
)

func codenamesSlots() []lfg.SeatSlot {
	keys := []string{
		"Team-1-SpyMaster",
		"Team-1-Guesser-1", "Team-1-Guesser-2", "Team-1-Guesser-3",
		"Team-2-SpyMaster",
		"Team-2-Guesser-1", "Team-2-Guesser-2", "Team-2-Guesser-3",
	}
	slots := make([]lfg.SeatSlot, len(keys))
	for i, key := range keys {
		qp := "Guesser"
		if stringsContains(key, "SpyMaster") {
			qp = "SpyMaster"
		}
		aff := "Team:1"
		if stringsContains(key, "Team-2-") {
			aff = "Team:2"
		}
		slots[i] = lfg.SeatSlot{
			SeatKey:     key,
			QueuePath:   qp,
			AffinityKey: aff,
		}
	}
	return slots
}

func stringsContains(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || len(sub) == 0 || indexOf(s, sub) >= 0)
}

func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}

func TestSeatMatchesPrefix(t *testing.T) {
	if !seatMatchesPrefix("Team-1-Guesser-1", "Team-1", "Guesser") {
		t.Fatal("expected match")
	}
	if !seatMatchesPrefix("Team-1-SpyMaster", "Team-1", "SpyMaster") {
		t.Fatal("expected spymaster match")
	}
	if !seatMatchesPrefix("Team-1-DPS-1", "", "DPS") {
		t.Fatal("expected nested role match with empty prefix")
	}
}

func TestPlaceTree_CompetingSpymasters(t *testing.T) {
	slots := codenamesSlots()
	party := Node{
		Children: []Node{
			{Role: "Team", Children: []Node{{Role: "SpyMaster", Members: []string{"A"}}}},
			{Role: "Team", Children: []Node{{Role: "SpyMaster", Members: []string{"B"}}}},
		},
	}
	seatByUser, ok := PlaceTree(slots, party)
	if !ok {
		t.Fatal("expected placement")
	}
	if seatByUser["A"] == seatByUser["B"] {
		t.Fatalf("expected different seats, got %v", seatByUser)
	}
	if seatByUser["A"] != "Team-1-SpyMaster" && seatByUser["A"] != "Team-2-SpyMaster" {
		t.Fatalf("A = %q", seatByUser["A"])
	}
}

func TestPlaceTree_SameTeamPair(t *testing.T) {
	slots := codenamesSlots()
	party := Node{
		Children: []Node{{
			Role: "Team",
			Children: []Node{
				{Role: "SpyMaster", Members: []string{"A"}},
				{Role: "Guesser", Members: []string{"B"}},
			},
		}},
	}
	open := openSlotsForRole(slots, "Team-1", "SpyMaster")
	if len(open) != 1 {
		t.Fatalf("spymaster open = %v", open)
	}
	openG := openSlotsForRole(slots, "Team-1", "Guesser")
	if len(openG) != 3 {
		t.Fatalf("guesser open = %v", openG)
	}
	seatByUser, ok := PlaceTree(slots, party)
	if !ok {
		t.Fatal("expected placement")
	}
	prefix := seatPrefix(seatByUser["A"])
	if seatPrefix(seatByUser["B"]) != prefix {
		t.Fatalf("expected same team, got A=%q B=%q", seatByUser["A"], seatByUser["B"])
	}
}

func TestPlaceTree_TwoGuessersSameTeam(t *testing.T) {
	slots := codenamesSlots()
	party := Node{
		Children: []Node{{
			Role:     "Team",
			Children: []Node{{Role: "Guesser", Members: []string{"A", "B"}}},
		}},
	}
	seatByUser, ok := PlaceTree(slots, party)
	if !ok {
		t.Fatal("expected placement")
	}
	if seatPrefix(seatByUser["A"]) != seatPrefix(seatByUser["B"]) {
		t.Fatalf("expected same team, got %v", seatByUser)
	}
}

func TestPlaceTree_SoloDuelDefaultPath(t *testing.T) {
	slots := []lfg.SeatSlot{
		{SeatKey: "1", QueuePath: ""},
		{SeatKey: "2", QueuePath: ""},
	}
	party := SoloNode("user-a", "")
	seatByUser, ok := PlaceTree(slots, party)
	if !ok {
		t.Fatal("expected solo placement on flat duel map")
	}
	if seatByUser["user-a"] != "1" {
		t.Fatalf("seat = %q, want 1", seatByUser["user-a"])
	}

	slots[0].OccupantID = "user-a"
	partyB := SoloNode("user-b", "")
	seatByUser, ok = PlaceTree(slots, partyB)
	if !ok {
		t.Fatal("expected second solo placement")
	}
	if seatByUser["user-b"] != "2" {
		t.Fatalf("seat = %q, want 2", seatByUser["user-b"])
	}
}

func TestPlaceTree_EmptyChildrenWithoutMembersFails(t *testing.T) {
	slots := []lfg.SeatSlot{{SeatKey: "1", QueuePath: ""}}
	_, ok := PlaceTree(slots, Node{})
	if ok {
		t.Fatal("expected failure when tree assigns nobody")
	}
}

func TestBuildFromPinnedSeats_Codenames(t *testing.T) {
	roles := map[string]string{
		"Team-1-SpyMaster": "SpyMaster",
		"Team-1-Guesser-1": "Guesser",
	}
	tree := BuildFromPinnedSeats([]PinnedSeat{
		{UserID: "A", SeatKey: "Team-1-SpyMaster"},
		{UserID: "B", SeatKey: "Team-1-Guesser-1"},
	}, roles)
	if len(tree.Children) != 1 || EffectiveRole(tree.Children[0].Role) != "Team" {
		t.Fatalf("unexpected tree: %+v", tree)
	}
	if len(tree.Children[0].Children) != 2 {
		t.Fatalf("expected 2 role children, got %+v", tree.Children[0].Children)
	}
}

func TestPlacePinned(t *testing.T) {
	slots := codenamesSlots()
	seatByUser, ok := PlacePinned(slots, []PinnedSeat{
		{UserID: "A", SeatKey: "Team-1-SpyMaster"},
		{UserID: "B", SeatKey: "Team-1-Guesser-1"},
	})
	if !ok {
		t.Fatal("expected pinned placement")
	}
	if seatByUser["A"] != "Team-1-SpyMaster" {
		t.Fatalf("got %v", seatByUser)
	}
}

func seatPrefix(seatKey string) string {
	parts := splitSeatKey(seatKey)
	if len(parts) >= 2 && isInstanceToken(parts[1]) {
		return parts[0] + "-" + parts[1]
	}
	return ""
}
