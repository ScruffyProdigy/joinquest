package partytree

import (
	"testing"

	"github.com/scruffyprodigy/joinquest/internal/lfg"
)

func TestNormalizeForPlacement_LegacyFlatDuelSeatRole(t *testing.T) {
	legacy := Node{
		TableID: "table-a",
		Children: []Node{{
			Role:    "1",
			Members: []string{"user-a"},
		}},
	}
	tree := NormalizeForPlacement(legacy)
	if EffectiveRole(tree.Role) != "" {
		t.Fatalf("role = %q, want empty default path", tree.Role)
	}
	if len(tree.Members) != 1 || tree.Members[0] != "user-a" {
		t.Fatalf("members = %v", tree.Members)
	}
	if tree.TableID != "table-a" {
		t.Fatalf("tableId = %q", tree.TableID)
	}
}

func TestPlaceTree_TwoLegacyFlatDuelPartiesMatch(t *testing.T) {
	slots := []lfg.SeatSlot{
		{SeatKey: "1", QueuePath: ""},
		{SeatKey: "2", QueuePath: ""},
	}

	treeA := NormalizeForPlacement(Node{Children: []Node{{Role: "1", Members: []string{"user-a"}}}})
	seatA, ok := PlaceTree(slots, treeA)
	if !ok || seatA["user-a"] != "1" {
		t.Fatalf("first placement = %v, ok=%v", seatA, ok)
	}

	for i, slot := range slots {
		if slot.SeatKey == seatA["user-a"] {
			slots[i].OccupantID = "user-a"
		}
	}

	treeB := NormalizeForPlacement(Node{Children: []Node{{Role: "1", Members: []string{"user-b"}}}})
	seatB, ok := PlaceTree(slots, treeB)
	if !ok {
		t.Fatal("expected second legacy party to place")
	}
	if seatB["user-b"] != "2" {
		t.Fatalf("seat = %q, want 2", seatB["user-b"])
	}
}
