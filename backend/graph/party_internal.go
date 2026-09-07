package graph

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/scruffyprodigy/joinquest/graph/model"
	"github.com/scruffyprodigy/joinquest/internal/lfg/partytree"
	"github.com/scruffyprodigy/joinquest/internal/store"
)

func toStorePartyInput(ctx context.Context, callerID uuid.UUID, input *model.PartyNodeInput) (*store.JoinPartyInput, error) {
	if input == nil {
		return nil, nil
	}

	tree, err := toPartyTreeNode(input)
	if err != nil {
		return nil, err
	}

	members := make([]store.JoinPartyMemberInput, 0, len(tree.AllMembers()))
	for _, m := range tree.AllMembers() {
		userID, err := parseUUID(m.UserID, "party member user id")
		if err != nil {
			return nil, err
		}
		members = append(members, store.JoinPartyMemberInput{UserID: userID, QueuePath: m.QueuePath})
	}
	if len(members) == 0 {
		return nil, nil
	}
	if members[0].UserID != callerID {
		return nil, fmt.Errorf("party leader must be the authenticated user")
	}

	return &store.JoinPartyInput{
		Tree:    tree,
		Members: members,
	}, nil
}

func toPartyTreeNode(input *model.PartyNodeInput) (partytree.Node, error) {
	if input == nil {
		return partytree.Node{}, nil
	}
	node := partytree.Node{}
	if input.Role != nil {
		node.Role = *input.Role
	}
	for _, child := range input.Children {
		c, err := toPartyTreeNode(child)
		if err != nil {
			return partytree.Node{}, err
		}
		node.Children = append(node.Children, c)
	}
	for _, member := range input.Members {
		userID, err := parseUUID(member.UserID, "party member user id")
		if err != nil {
			return partytree.Node{}, err
		}
		node.Members = append(node.Members, userID.String())
	}
	return node, nil
}
