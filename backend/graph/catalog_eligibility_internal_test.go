package graph

import (
	"testing"

	"github.com/scruffyprodigy/joinquest/graph/model"
	"github.com/scruffyprodigy/joinquest/internal/gameclient"
)

func TestToGraphQLModeEligibilityBooleanGate(t *testing.T) {
	in := gameclient.ModeEligibility{Accessible: false, Reason: "Complete the tutorial to unlock."}
	out := toGraphQLModeEligibility(in)
	if out.Accessible {
		t.Fatal("expected inaccessible")
	}
	if out.Reason == nil || *out.Reason != "Complete the tutorial to unlock." {
		t.Fatalf("unexpected reason: %+v", out.Reason)
	}
	if out.Requirement != nil {
		t.Fatalf("expected nil requirement for boolean gate, got %+v", out.Requirement)
	}
}

func TestToGraphQLModeEligibilityLeaf(t *testing.T) {
	in := gameclient.ModeEligibility{
		Accessible: false,
		Requirement: &gameclient.RequirementNode{
			Kind: "leaf", Label: "Ranked matches", Current: 12, Target: 50,
		},
	}
	out := toGraphQLModeEligibility(in)
	leaf, ok := out.Requirement.(*model.RequirementLeaf)
	if !ok {
		t.Fatalf("expected *model.RequirementLeaf, got %T", out.Requirement)
	}
	if leaf.Current != 12 || leaf.Target != 50 {
		t.Fatalf("unexpected leaf values: %+v", leaf)
	}
}

func TestToGraphQLModeEligibilityGroup(t *testing.T) {
	in := gameclient.ModeEligibility{
		Accessible: false,
		Requirement: &gameclient.RequirementNode{
			Kind: "group", Label: "Commander requirements", Operator: "all",
			Children: []gameclient.RequirementNode{
				{Kind: "leaf", Label: "Standard wins", Current: 18, Target: 25},
				{Kind: "leaf", Label: "Unique decks used", Current: 3, Target: 5},
			},
		},
	}
	out := toGraphQLModeEligibility(in)
	group, ok := out.Requirement.(*model.RequirementGroup)
	if !ok {
		t.Fatalf("expected *model.RequirementGroup, got %T", out.Requirement)
	}
	if group.Operator != model.RequirementOperatorAll {
		t.Fatalf("expected ALL operator, got %v", group.Operator)
	}
	if len(group.Children) != 2 {
		t.Fatalf("expected 2 children, got %d", len(group.Children))
	}
	firstLeaf, ok := group.Children[0].(*model.RequirementLeaf)
	if !ok || firstLeaf.Current != 18 {
		t.Fatalf("unexpected first child: %+v", group.Children[0])
	}
}

func TestToGraphQLModeEligibilityUnlockModeKey(t *testing.T) {
	key := "deck-builder"
	in := gameclient.ModeEligibility{Accessible: false, UnlockModeKey: &key}
	out := toGraphQLModeEligibility(in)
	if out.UnlockModeKey == nil || *out.UnlockModeKey != "deck-builder" {
		t.Fatalf("expected unlockModeKey to pass through, got %+v", out.UnlockModeKey)
	}
}
