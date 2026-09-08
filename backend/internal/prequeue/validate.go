package prequeue

import (
	"fmt"
	"sort"
	"strings"
)

// Choice is the validation view of one option the game offered this player.
// The wire type in gameclient carries the label and unlock requirement too;
// validation only needs to know what exists and what is out of reach.
type Choice struct {
	ID     string
	Label  string
	Locked bool
}

// RosterGroup is the set of choices the game served for one declared group.
type RosterGroup struct {
	Key     string
	Choices []Choice
}

// Selection is what a player picked in one group.
type Selection struct {
	GroupKey  string   `json:"groupKey"`
	OptionIDs []string `json:"optionIds"`
	// Labels are the game's names for OptionIDs, in the same order, stamped by
	// Resolve at pick time. Never read from a client: a player must not be able
	// to caption their own choice. Stored so a waiting player still reads
	// "Good Old Rock" when the game has since gone quiet.
	Labels []string `json:"labels,omitempty"`
}

// Validate checks a player's selections against the mode's declaration and the
// roster the game actually served for them. Every rejection here is one a
// client could otherwise have talked its way past: unknown options, options the
// game locked, and counts outside what the mode asked for.
func Validate(groups []Group, roster []RosterGroup, selections []Selection) error {
	if len(groups) == 0 {
		if len(selections) > 0 {
			return fmt.Errorf("prequeue: this mode does not use pre-queue options")
		}
		return nil
	}

	declared := make(map[string]Group, len(groups))
	for _, g := range groups {
		declared[g.Key] = g
	}
	offered := make(map[string]map[string]Choice, len(roster))
	for _, rg := range roster {
		byID := make(map[string]Choice, len(rg.Choices))
		for _, c := range rg.Choices {
			byID[c.ID] = c
		}
		offered[rg.Key] = byID
	}

	chosen := make(map[string]int, len(selections))
	for _, sel := range selections {
		key := strings.TrimSpace(sel.GroupKey)
		group, ok := declared[key]
		if !ok {
			return fmt.Errorf("prequeue: this mode has no option group %q", sel.GroupKey)
		}
		if _, dup := chosen[key]; dup {
			return fmt.Errorf("prequeue: group %q was chosen more than once", key)
		}
		chosen[key] = len(sel.OptionIDs)

		choices, served := offered[key]
		if !served {
			return fmt.Errorf("prequeue: the game offered no choices for %q", key)
		}
		if len(sel.OptionIDs) < group.Min || len(sel.OptionIDs) > group.Max {
			return fmt.Errorf("prequeue: %s takes %s, got %d",
				group.Label, boundsPhrase(group), len(sel.OptionIDs))
		}
		seen := make(map[string]struct{}, len(sel.OptionIDs))
		for _, id := range sel.OptionIDs {
			id = strings.TrimSpace(id)
			if _, dup := seen[id]; dup {
				return fmt.Errorf("prequeue: %q was chosen twice in group %q", id, key)
			}
			seen[id] = struct{}{}
			choice, exists := choices[id]
			if !exists {
				return fmt.Errorf("prequeue: %q is not an option this game offered in group %q", id, key)
			}
			if choice.Locked {
				return fmt.Errorf("prequeue: %q is locked for this player", id)
			}
		}
	}

	for _, g := range groups {
		if _, ok := chosen[g.Key]; ok {
			continue
		}
		if g.Optional() {
			continue
		}
		return fmt.Errorf("prequeue: %s is required (group %q)", g.Label, g.Key)
	}
	return nil
}

func boundsPhrase(g Group) string {
	if g.Min == g.Max {
		return fmt.Sprintf("exactly %d", g.Min)
	}
	return fmt.Sprintf("between %d and %d", g.Min, g.Max)
}

// Resolve turns validated selections into the form that gets stored: groups in
// declaration order, ids sorted, and each id captioned with the game's own label
// from the roster the player was actually shown.
//
// Sorting and ordering mean the same picks always store as the same JSON.
// Labels come from the roster rather than the request so nothing a client sends
// can rename what it chose.
func Resolve(groups []Group, roster []RosterGroup, selections []Selection) []Selection {
	labels := make(map[string]map[string]string, len(roster))
	for _, rg := range roster {
		byID := make(map[string]string, len(rg.Choices))
		for _, c := range rg.Choices {
			byID[c.ID] = c.Label
		}
		labels[rg.Key] = byID
	}

	byKey := make(map[string][]string, len(selections))
	for _, sel := range selections {
		ids := make([]string, 0, len(sel.OptionIDs))
		for _, id := range sel.OptionIDs {
			ids = append(ids, strings.TrimSpace(id))
		}
		sort.Strings(ids)
		byKey[strings.TrimSpace(sel.GroupKey)] = ids
	}

	out := make([]Selection, 0, len(selections))
	for _, g := range groups {
		ids, ok := byKey[g.Key]
		if !ok {
			continue
		}
		captions := make([]string, len(ids))
		for i, id := range ids {
			label := labels[g.Key][id]
			if strings.TrimSpace(label) == "" {
				// A game that shipped no label leaves the id showing, which is
				// worse-looking but still true.
				label = id
			}
			captions[i] = label
		}
		out = append(out, Selection{GroupKey: g.Key, OptionIDs: ids, Labels: captions})
	}
	return out
}
