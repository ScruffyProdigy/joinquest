package graph

import (
	"strings"

	"github.com/scruffyprodigy/playhub/graph/model"
	"github.com/scruffyprodigy/playhub/internal/gameclient"
)

// toGraphQLRequirementNode converts a gameclient requirement tree node into
// the corresponding generated GraphQL ModeRequirementNode implementation
// (either *model.RequirementLeaf or *model.RequirementGroup).
func toGraphQLRequirementNode(n *gameclient.RequirementNode) model.ModeRequirementNode {
	if n == nil {
		return nil
	}
	if n.Kind == "group" {
		children := make([]model.ModeRequirementNode, 0, len(n.Children))
		for i := range n.Children {
			if child := toGraphQLRequirementNode(&n.Children[i]); child != nil {
				children = append(children, child)
			}
		}
		operator := model.RequirementOperatorAll
		if strings.EqualFold(n.Operator, "any") {
			operator = model.RequirementOperatorAny
		}
		return &model.RequirementGroup{
			Label:    n.Label,
			Operator: operator,
			Children: children,
		}
	}
	return &model.RequirementLeaf{
		Label:   n.Label,
		Current: n.Current,
		Target:  n.Target,
	}
}

// toGraphQLModeEligibility converts a gameclient.ModeEligibility (as fetched
// from a game server and cached) into the generated GraphQL model type.
func toGraphQLModeEligibility(e gameclient.ModeEligibility) *model.ModeEligibility {
	out := &model.ModeEligibility{
		Accessible:    e.Accessible,
		Requirement:   toGraphQLRequirementNode(e.Requirement),
		UnlockModeKey: e.UnlockModeKey,
	}
	if e.Reason != "" {
		reason := e.Reason
		out.Reason = &reason
	}
	return out
}
