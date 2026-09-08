// Package prequeue models a game mode's pre-queue option groups: what a mode
// declares in its manifest, and whether a player's selections are allowed.
//
// A group is a roster the player picks from before entering matchmaking — a
// champion, a loadout, a deck. Groups are declared per mode; the choices inside
// them are per player and come from the game at request time, never from the
// manifest. See docs/composition-and-join-options.md.
package prequeue

import (
	"encoding/json"
	"fmt"
	"strings"
)

// Kind tells the UI what sort of thing a group holds, so a champion picker can
// look different from a brush picker without the lobby knowing what either is.
type Kind string

const (
	KindCharacter Kind = "Character"
	KindLoadout   Kind = "Loadout"
	KindDeck      Kind = "Deck"
)

func (k Kind) valid() bool {
	switch k {
	case KindCharacter, KindLoadout, KindDeck:
		return true
	}
	return false
}

// Group is one roster a mode asks the player to pick from.
type Group struct {
	Key   string `json:"key"`
	Kind  Kind   `json:"kind"`
	Label string `json:"label"`
	Min   int    `json:"min"`
	Max   int    `json:"max"`
}

// Optional reports whether a player may skip this group entirely.
func (g Group) Optional() bool { return g.Min == 0 }

// Declaration is a mode's whole pre-queue declaration.
type Declaration struct {
	Groups []Group `json:"groups"`
}

// Parse reads a mode manifest's preQueue block. A mode without one parses to a
// nil declaration, not an error — most modes have no options.
func Parse(raw json.RawMessage) (*Declaration, error) {
	if len(raw) == 0 || string(raw) == "null" {
		return nil, nil
	}
	var decl Declaration
	if err := json.Unmarshal(raw, &decl); err != nil {
		return nil, fmt.Errorf("prequeue: decode: %w", err)
	}
	if len(decl.Groups) == 0 {
		return nil, fmt.Errorf("prequeue: preQueue declares no groups; omit preQueue entirely instead")
	}

	seen := make(map[string]struct{}, len(decl.Groups))
	for i := range decl.Groups {
		g := &decl.Groups[i]
		g.Key = strings.TrimSpace(g.Key)
		g.Label = strings.TrimSpace(g.Label)
		if g.Key == "" {
			return nil, fmt.Errorf("prequeue: group %d: key is required", i)
		}
		if _, dup := seen[g.Key]; dup {
			return nil, fmt.Errorf("prequeue: duplicate group key %q", g.Key)
		}
		seen[g.Key] = struct{}{}
		if !g.Kind.valid() {
			return nil, fmt.Errorf("prequeue: group %q: unknown kind %q", g.Key, g.Kind)
		}
		if g.Label == "" {
			return nil, fmt.Errorf("prequeue: group %q: label is required", g.Key)
		}
		// A group that says nothing about bounds means "pick exactly one" —
		// the shape every character select in the prototype uses.
		if g.Min == 0 && g.Max == 0 {
			g.Min, g.Max = 1, 1
		}
		if g.Min < 0 {
			return nil, fmt.Errorf("prequeue: group %q: min must not be negative", g.Key)
		}
		if g.Max < 1 {
			return nil, fmt.Errorf("prequeue: group %q: max must be at least 1", g.Key)
		}
		if g.Max < g.Min {
			return nil, fmt.Errorf("prequeue: group %q: max %d is below min %d", g.Key, g.Max, g.Min)
		}
	}
	return &decl, nil
}
