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
		}
		if !result.Acted {
			// A reconnect, a deliberate leave, or a match forming got here first.
			// Not an error — the guard doing its job.
			return
		}
		// Acted survives an error: the cancel is committed by the time anything
		// downstream of it can fail, so returning early here would drop the publish
		// for a removal that already happened.
		//
		// The publish is the same event a deliberate leave sends, and reaches the
		// same audience: publishQueueLeft addresses only the leaving player's own
		// channel, and this path fires precisely when they hold no socket, so in
		// practice nobody receives it. It is kept for parity — a returning player's
		// LEFT initial payload is what actually tells them, and other players learn
		// the new count from the next forming reconcile, not from here.
		if err := r.publishQueueLeft(ctx, result.GameID, result.ModeQueueID, userID, result.QueuedCount, ""); err != nil {
			log.Printf("disconnect expiry: publish for %s: %v", userID, err)
		}
	}
}
