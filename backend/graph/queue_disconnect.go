package graph

import (
	"context"
	"log"
	"time"

	"github.com/google/uuid"
)

// DisconnectOutcome is what happens to a waiting player whose grace window expired.
//
// Removal is the only outcome today. A second one — keep the player queued and push
// them when the match forms — must be addable here without restructuring this path,
// which is why expiry is a named decision rather than a hardcoded removal.
//
// Matched rows never reach this decision at all. EvictDisconnectedWaitingEntry can
// only ever modify a row that is still 'waiting', which is what keeps a held seat
// safe while its own hold window runs. That guard is structural, not an outcome this
// enum returns, and must not be "generalised" into one: impossible beats configurable
// for a rule whose violation looks correct in review.
type DisconnectOutcome int

const (
	DisconnectOutcomeRemove DisconnectOutcome = iota
)

// decideDisconnectOutcome is the seam. It takes no arguments today because removal is
// unconditional; a second outcome will need the player's push reachability here.
func decideDisconnectOutcome() DisconnectOutcome {
	return DisconnectOutcomeRemove
}

// OnGraceExpired runs when a disconnected player's window elapses without a
// reconnect. Every safety condition lives in the store's UPDATE, so this only has to
// decide and publish.
func (r *Resolver) OnGraceExpired(ctx context.Context, userID uuid.UUID, stamp time.Time) {
	st, err := r.requireStore()
	if err != nil {
		log.Printf("disconnect expiry: store unavailable for %s: %v", userID, err)
		return
	}

	switch decideDisconnectOutcome() {
	case DisconnectOutcomeRemove:
		result, err := st.EvictDisconnectedWaitingEntry(ctx, userID, stamp)
		if err != nil {
			log.Printf("disconnect expiry: evict %s: %v", userID, err)
			return
		}
		if !result.Acted {
			// A reconnect, a deliberate leave, or a match forming got here first.
			// Not an error — the guard doing its job.
			return
		}
		// The same event a deliberate leave publishes, so other players' queued
		// counts and forming gaps stay correct either way.
		if err := r.publishQueueLeft(ctx, result.GameID, result.ModeQueueID, userID, result.QueuedCount, ""); err != nil {
			log.Printf("disconnect expiry: publish for %s: %v", userID, err)
		}
	}
}
