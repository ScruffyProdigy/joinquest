// Package testutil provides fixture game-server handlers for local dev and integration tests.
package testutil

import (
	"encoding/json"
	"net/http"
	"strings"
)

// EligibilityFixtureHandler serves GET /api/v1/players/{lobbyUserId}/mode-eligibility with
// canned data for the fixture game seeded by migration 000038. The response depends on
// whether lobbyUserId ends in "-locked" or "-unlocked" (defaulting to locked), so tests and
// local dev can exercise both states without mutating real stats.
func EligibilityFixtureHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasPrefix(r.URL.Path, "/api/v1/players/") || !strings.HasSuffix(r.URL.Path, "/mode-eligibility") {
			http.NotFound(w, r)
			return
		}

		// Extract lobbyUserId from path: /api/v1/players/{lobbyUserId}/mode-eligibility
		lobbyUserId := strings.TrimPrefix(r.URL.Path, "/api/v1/players/")
		lobbyUserId = strings.TrimSuffix(lobbyUserId, "/mode-eligibility")

		unlocked := strings.HasSuffix(lobbyUserId, "-unlocked")

		deckBuilder := "deck-builder"
		modes := map[string]any{
			"arena": map[string]any{
				"accessible": unlocked,
				"reason":     reasonUnless(unlocked, "Complete the tutorial to unlock."),
			},
			"legendary": map[string]any{
				"accessible": unlocked,
				"reason":     reasonUnless(unlocked, "Complete 50 Ranked matches to unlock."),
				"requirement": map[string]any{
					"kind": "leaf", "label": "Ranked matches",
					"current": currentUnless(unlocked, 12, 50), "target": 50,
				},
			},
			"standard": map[string]any{
				"accessible":    unlocked,
				"reason":        reasonUnless(unlocked, "You have no Standard-legal decks."),
				"unlockModeKey": unlockKeyUnless(unlocked, &deckBuilder),
				"requirement": map[string]any{
					"kind": "leaf", "label": "Standard-legal decks",
					"current": currentUnless(unlocked, 0, 1), "target": 1,
				},
			},
			"commander": map[string]any{
				"accessible": unlocked,
				"reason":     reasonUnless(unlocked, "Requires 25 Standard wins and 5 unique decks used."),
				"requirement": map[string]any{
					"kind": "group", "label": "Commander requirements", "operator": "all",
					"children": []map[string]any{
						{"kind": "leaf", "label": "Standard wins", "current": currentUnless(unlocked, 18, 25), "target": 25},
						{"kind": "leaf", "label": "Unique decks used", "current": currentUnless(unlocked, 3, 5), "target": 5},
					},
				},
			},
			"deck-builder": map[string]any{
				"accessible": true,
			},
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]any{"modes": modes})
	})
}

func reasonUnless(unlocked bool, reason string) any {
	if unlocked {
		return nil
	}
	return reason
}

func currentUnless(unlocked bool, lockedValue, unlockedValue int) int {
	if unlocked {
		return unlockedValue
	}
	return lockedValue
}

func unlockKeyUnless(unlocked bool, key *string) any {
	if unlocked {
		return nil
	}
	return key
}
