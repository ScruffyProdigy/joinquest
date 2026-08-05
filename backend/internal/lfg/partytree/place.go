package partytree

import (
	"strings"

	"github.com/scruffyprodigy/playhub/internal/lfg"
)

// PlacePinned assigns users to exact seat keys on the forming map.
func PlacePinned(slots []lfg.SeatSlot, pinned []PinnedSeat) (map[string]string, bool) {
	if len(pinned) == 0 {
		return nil, false
	}
	byKey := make(map[string]int, len(slots))
	for i, slot := range slots {
		byKey[slot.SeatKey] = i
	}
	working := append([]lfg.SeatSlot(nil), slots...)
	seatByUser := make(map[string]string, len(pinned))
	for _, pin := range pinned {
		idx, ok := byKey[pin.SeatKey]
		if !ok {
			return nil, false
		}
		if working[idx].OccupantID != "" && working[idx].OccupantID != pin.UserID {
			return nil, false
		}
	}
	for _, pin := range pinned {
		idx := byKey[pin.SeatKey]
		working[idx].OccupantID = pin.UserID
		seatByUser[pin.UserID] = pin.SeatKey
	}
	return seatByUser, true
}

// PlaceTree assigns a party tree onto open forming-map slots.
func PlaceTree(slots []lfg.SeatSlot, party Node) (map[string]string, bool) {
	instances := collectBranchInstances(slots)
	working := append([]lfg.SeatSlot(nil), slots...)
	seatByUser := map[string]string{}
	if !placeTree(working, normalizeRoot(party), "", instances, seatByUser) {
		return nil, false
	}
	if len(seatByUser) == 0 {
		return nil, false
	}
	return seatByUser, true
}

// placeTree places the current node, then recurses into children the same way.
func placeTree(slots []lfg.SeatSlot, node Node, prefix string, instances map[string][]string, out map[string]string) bool {
	role := EffectiveRole(node.Role)

	if len(node.Members) > 0 {
		open := openSlotsForRole(slots, prefix, role)
		if len(open) < len(node.Members) {
			return false
		}
		for i, userID := range node.Members {
			idx := open[i]
			if slots[idx].OccupantID != "" {
				return false
			}
			slots[idx].OccupantID = userID
			out[userID] = slots[idx].SeatKey
		}
	}

	if len(node.Children) == 0 {
		return len(node.Members) > 0 || role == ""
	}

	if role != "" && len(instances[role]) > 0 {
		for _, inst := range instances[role] {
			backup := cloneSlots(slots)
			backupOut := copyStringMap(out)
			instPrefix := joinPrefix(prefix, inst)
			local := copyStringMap(backupOut)
			if placeTreeChildren(slots, node.Children, instPrefix, instances, local) {
				for k, v := range local {
					out[k] = v
				}
				return true
			}
			restoreSlots(slots, backup)
			for k := range out {
				delete(out, k)
			}
			for k, v := range backupOut {
				out[k] = v
			}
		}
		return false
	}

	nextPrefix := prefix
	if role != "" {
		nextPrefix = extendPrefix(prefix, role)
	}
	return placeTreeChildren(slots, node.Children, nextPrefix, instances, out)
}

// placeTreeChildren dispatches a sibling list: branch permutation or per-child placeTree.
func placeTreeChildren(slots []lfg.SeatSlot, children []Node, prefix string, instances map[string][]string, out map[string]string) bool {
	if len(children) == 0 {
		return true
	}
	if isBranchSiblingGroup(children, instances) {
		return placeBranchSiblings(slots, children, prefix, instances, out)
	}
	for _, child := range children {
		if !placeTree(slots, child, prefix, instances, out) {
			return false
		}
	}
	return true
}

func isBranchSiblingGroup(children []Node, instances map[string][]string) bool {
	if len(children) <= 1 {
		return false
	}
	role := EffectiveRole(children[0].Role)
	if role == "" || len(instances[role]) == 0 {
		return false
	}
	for _, child := range children[1:] {
		if EffectiveRole(child.Role) != role {
			return false
		}
	}
	return true
}

func placeBranchSiblings(slots []lfg.SeatSlot, children []Node, prefix string, instances map[string][]string, out map[string]string) bool {
	role := EffectiveRole(children[0].Role)
	available := branchInstancesForPrefix(instances, role, prefix)
	if len(available) < len(children) {
		return false
	}
	return permuteBranchSiblings(slots, children, prefix, available, 0, out)
}

func permuteBranchSiblings(slots []lfg.SeatSlot, children []Node, prefix string, available []string, start int, out map[string]string) bool {
	if len(children) == 0 {
		return true
	}
	backup := cloneSlots(slots)
	backupOut := copyStringMap(out)

	for i := start; i <= len(available)-len(children); i++ {
		restoreSlots(slots, backup)
		local := copyStringMap(backupOut)
		ok := true
		for j, child := range children {
			instPrefix := joinPrefix(prefix, available[i+j])
			if !placeTree(slots, child, instPrefix, collectBranchInstances(slots), local) {
				ok = false
				break
			}
		}
		if ok {
			for k, v := range local {
				out[k] = v
			}
			return true
		}
	}
	restoreSlots(slots, backup)
	return false
}

func cloneSlots(slots []lfg.SeatSlot) []lfg.SeatSlot {
	return append([]lfg.SeatSlot(nil), slots...)
}

func restoreSlots(dst, src []lfg.SeatSlot) {
	copy(dst, src)
}

func copyStringMap(in map[string]string) map[string]string {
	out := make(map[string]string, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}

func collectBranchInstances(slots []lfg.SeatSlot) map[string][]string {
	type set map[string]struct{}
	raw := map[string]set{}
	for _, slot := range slots {
		parts := splitSeatKey(slot.SeatKey)
		if len(parts) < 2 || !isInstanceToken(parts[1]) {
			continue
		}
		branch := parts[0]
		inst := parts[0] + "-" + parts[1]
		if raw[branch] == nil {
			raw[branch] = set{}
		}
		raw[branch][inst] = struct{}{}
	}
	out := make(map[string][]string, len(raw))
	for branch, instSet := range raw {
		list := make([]string, 0, len(instSet))
		for inst := range instSet {
			list = append(list, inst)
		}
		sortStrings(list)
		out[branch] = list
	}
	return out
}

func branchInstancesForPrefix(instances map[string][]string, branchRole, prefix string) []string {
	all := instances[branchRole]
	if prefix == "" {
		return all
	}
	out := make([]string, 0)
	for _, inst := range all {
		if strings.HasPrefix(inst, prefix) || strings.HasPrefix(prefix, inst) {
			out = append(out, inst)
		}
	}
	if len(out) > 0 {
		return out
	}
	return all
}

func openSlotsForRole(slots []lfg.SeatSlot, prefix, role string) []int {
	var indices []int
	for i, slot := range slots {
		if slot.OccupantID != "" {
			continue
		}
		if prefix == "" && role == "" {
			indices = append(indices, i)
			continue
		}
		if !pathsMatch(slot.QueuePath, role) && !pathsMatch(leafRoleFromSeatKey(slot.SeatKey), role) {
			continue
		}
		if !seatMatchesPrefix(slot.SeatKey, prefix, role) {
			continue
		}
		indices = append(indices, i)
	}
	return indices
}

func seatMatchesPrefix(seatKey, prefix, role string) bool {
	if prefix == "" {
		if role == "" {
			return true
		}
		if strings.HasPrefix(seatKey, role+"-") || seatKey == role || strings.HasSuffix(seatKey, "-"+role) {
			return true
		}
		return leafRoleFromSeatKey(seatKey) == role
	}
	if !strings.HasPrefix(seatKey, prefix) {
		return false
	}
	if seatKey == prefix {
		return false
	}
	if len(seatKey) > len(prefix) && seatKey[len(prefix)] != '-' {
		return false
	}
	rest := strings.TrimPrefix(seatKey, prefix)
	rest = strings.TrimPrefix(rest, "-")
	if rest == role {
		return true
	}
	return strings.HasPrefix(rest, role+"-")
}

func extendPrefix(prefix, segment string) string {
	if segment == "" {
		return prefix
	}
	if prefix == "" {
		return segment
	}
	return prefix + "-" + segment
}

func joinPrefix(prefix, segment string) string {
	if prefix == "" {
		return segment
	}
	if strings.HasPrefix(segment, prefix) {
		return segment
	}
	return prefix + "-" + strings.TrimPrefix(segment, prefix+"-")
}

func pathsMatch(a, b string) bool {
	return strings.TrimSpace(a) == strings.TrimSpace(b)
}
