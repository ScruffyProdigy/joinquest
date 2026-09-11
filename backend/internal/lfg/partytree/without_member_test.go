package partytree

import (
	"encoding/json"
	"testing"
)

func TestWithoutMemberKeepsTheRestOfTheBranch(t *testing.T) {
	tree := Node{Children: []Node{
		{Role: "Team", Children: []Node{{Role: "Seat", Members: []string{"a", "b", "c"}}}},
	}}
	got, _ := json.Marshal(WithoutMember(tree, "b"))
	want := `{"children":[{"role":"Team","children":[{"role":"Seat","members":["a","c"]}]}]}`
	if string(got) != want {
		t.Fatalf("got %s, want %s", got, want)
	}
}

func TestWithoutMemberDropsTheBranchItEmpties(t *testing.T) {
	// The lone player on the far side leaves: the party stops asking for that side
	// rather than asking for a side with nobody in it.
	tree := Node{Children: []Node{
		{Role: "Team", Children: []Node{{Role: "Seat", Members: []string{"a", "b"}}}},
		{Role: "Team", Children: []Node{{Role: "Seat", Members: []string{"c"}}}},
	}}
	got, _ := json.Marshal(WithoutMember(tree, "c"))
	want := `{"children":[{"role":"Team","children":[{"role":"Seat","members":["a","b"]}]}]}`
	if string(got) != want {
		t.Fatalf("got %s, want %s", got, want)
	}
}

func TestWithoutMemberOfASoloParty(t *testing.T) {
	got := WithoutMember(SoloNode("a", "Guesser"), "a")
	if len(got.AllMembers()) != 0 {
		t.Fatalf("expected an empty tree, got %+v", got)
	}
}

func TestWithoutMemberLeavesAStrangerAlone(t *testing.T) {
	tree := Node{Children: []Node{{Role: "Seat", Members: []string{"a", "b"}}}}
	if len(WithoutMember(tree, "zz").AllMembers()) != 2 {
		t.Fatal("removing a user who is not in the tree changed it")
	}
}
