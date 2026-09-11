package partytree

import (
	"encoding/json"
	"strings"
)

// Node is a party layout tree aligned with seatTemplate dimensions.
// Root role may be empty and is ignored during placement.
type Node struct {
	Role     string   `json:"role,omitempty"`
	Children []Node   `json:"children,omitempty"`
	Members  []string `json:"members,omitempty"`
	// TableID links a table-backfill party to its room table (forming assignment metadata).
	TableID string `json:"tableId,omitempty"`
}

// PinnedSeat binds a user to an exact seat key (table seating).
type PinnedSeat struct {
	UserID  string
	SeatKey string
}

// Member is a flat party member for queue rows.
type Member struct {
	UserID    string
	QueuePath string
}

// IsRootRole returns true when the role is empty or a conventional root placeholder.
func IsRootRole(role string) bool {
	switch strings.TrimSpace(role) {
	case "", "Root", "root":
		return true
	default:
		return false
	}
}

// EffectiveRole returns the role unless it is a root placeholder.
func EffectiveRole(role string) string {
	if IsRootRole(role) {
		return ""
	}
	return strings.TrimSpace(role)
}

// AllMembers returns every user id in the tree.
func (n Node) AllMembers() []Member {
	var out []Member
	n.collectMembers("", &out)
	return out
}

func (n Node) collectMembers(parentRole string, out *[]Member) {
	role := EffectiveRole(n.Role)
	if role == "" {
		role = parentRole
	}
	for _, userID := range n.Members {
		if strings.TrimSpace(userID) == "" {
			continue
		}
		*out = append(*out, Member{UserID: userID, QueuePath: role})
	}
	for _, child := range n.Children {
		child.collectMembers(role, out)
	}
}

// MarshalJSON stores the tree; omits empty root role.
func (n Node) MarshalJSON() ([]byte, error) {
	type alias Node
	return json.Marshal(alias(n))
}

// SoloNode is a single-player party for catalog solo join.
func SoloNode(userID, queuePath string) Node {
	return Node{Role: queuePath, Members: []string{userID}}
}

// WithoutMember returns the tree with one user removed, dropping any node it leaves
// empty. Used when a player walks out of a group that is already queued: the party
// carries on without them, so the layout it is placed from must stop naming them.
func WithoutMember(n Node, userID string) Node {
	want := strings.TrimSpace(userID)
	out := n
	out.Members = nil
	for _, member := range n.Members {
		if strings.TrimSpace(member) != want {
			out.Members = append(out.Members, member)
		}
	}
	out.Children = nil
	for _, child := range n.Children {
		pruned := WithoutMember(child, userID)
		// A role node with nobody left in it is not a request for that role — it is
		// nothing, and leaving it in would ask placement for a seat the party no
		// longer needs.
		if len(pruned.Members) == 0 && len(pruned.Children) == 0 {
			continue
		}
		out.Children = append(out.Children, pruned)
	}
	return out
}
