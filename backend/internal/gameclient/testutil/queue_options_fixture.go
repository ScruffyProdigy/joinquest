package testutil

import (
	"encoding/json"
	"net/http"
	"strings"
)

// QueueOptionsFixtureHandler serves GET /api/v1/players/{lobbyUserId}/queue-options
// with a canned roster, so the pre-queue picker can be exercised end to end
// without a real game.
//
// As with the eligibility fixture, a lobbyUserId ending in "-unlocked" gets the
// whole roster; anyone else sees the back half locked behind progress, which is
// the state worth looking at.
func QueueOptionsFixtureHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasPrefix(r.URL.Path, "/api/v1/players/") || !strings.HasSuffix(r.URL.Path, "/queue-options") {
			http.NotFound(w, r)
			return
		}
		lobbyUserID := strings.TrimPrefix(r.URL.Path, "/api/v1/players/")
		lobbyUserID = strings.TrimSuffix(lobbyUserID, "/queue-options")
		unlocked := strings.HasSuffix(lobbyUserID, "-unlocked")

		// A mode with no pre-queue step answers with an empty roster rather than
		// a 404, so "this mode has no options" and "this game is broken" stay
		// distinguishable.
		groups := []map[string]any{}
		if modeKey := r.URL.Query().Get("modeKey"); modeKey == "standard" || modeKey == "commander" {
			groups = append(groups, map[string]any{
				"key":     "deck",
				"choices": fixtureDeckChoices(unlocked),
			})
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"groups": groups})
	})
}

func fixtureDeckChoices(unlocked bool) []map[string]any {
	deckBuilder := "deck-builder"
	choices := []map[string]any{
		{"id": "starter", "label": "Starter Deck", "description": "The one everybody begins with", "locked": false},
		{"id": "aggro", "label": "Red Aggro", "description": "Fast, fragile, unsubtle", "locked": false},
		{"id": "control", "label": "Blue Control", "description": "Wins on turn nineteen", "locked": false},
	}
	locked := []map[string]any{
		{
			"id": "artifact", "label": "Artifact Ramp",
			"description": "Built, not bought",
			"locked":      !unlocked,
			"requirement": map[string]any{
				"kind": "leaf", "label": "Decks built",
				"current": currentUnless(unlocked, 1, 3), "target": 3,
			},
			"unlockModeKey": unlockKeyUnless(unlocked, &deckBuilder),
		},
		{
			"id": "legends", "label": "Legends",
			"description": "Unlocked by playing, not paying",
			"locked":      !unlocked,
			"requirement": map[string]any{
				"kind": "group", "label": "Both of", "operator": "all",
				"children": []map[string]any{
					{"kind": "leaf", "label": "Casual wins", "current": currentUnless(unlocked, 3, 5), "target": 5},
					{"kind": "leaf", "label": "Decks built", "current": currentUnless(unlocked, 1, 3), "target": 3},
				},
			},
		},
	}
	return append(choices, locked...)
}
