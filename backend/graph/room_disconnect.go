package graph

import (
	"context"
	"log"
	"time"

	"github.com/google/uuid"
)

// OnRoomGraceExpired runs when a disconnected player's room window elapses without a
// reconnect, and takes them out of the room they are no longer in.
//
// It is a separate expiry from OnGraceExpired on a separate clock — see
// store.DefaultRoomDisconnectGrace for why a room's 5m is not the queue's 90s — so a player
// two minutes into a disconnect has kept their room and lost their queue place. All three
// expiries fire from the one socket edge, and a reconnect cancels all of them.
func (r *Resolver) OnRoomGraceExpired(ctx context.Context, userID uuid.UUID, stamp time.Time) {
	st, err := r.requireStore()
	if err != nil {
		log.Printf("room disconnect expiry: store unavailable for %s: %v", userID, err)
		return
	}

	result, err := st.EvictDisconnectedRoomMember(ctx, userID, stamp)
	if err != nil {
		log.Printf("room disconnect expiry: evict %s: %v", userID, err)
	}
	if !result.Acted {
		// A reconnect, a deliberate leave, or a move into another room got here
		// first. Not an error — the guard doing its job.
		return
	}

	// Acted survives an error, for the reason OnGraceExpired gives: the removal is
	// committed by the time anything downstream of it can fail, so returning early
	// would drop the publish for a departure that already happened.
	//
	// This publish matters in a way the queue's counterpart does not. publishQueueLeft
	// addresses only the leaving player's own channel, so in practice nobody receives
	// it; roomUpdated addresses the ROOM, and the people still in it are connected and
	// looking at a roster with a departed player on it. Skipping it is how rosters go
	// stale. It is the same event a deliberate leave publishes, so the client needs no
	// idea that this departure was involuntary.
	if err := r.publishRoomUpdated(ctx, result.RoomID); err != nil {
		log.Printf("room disconnect expiry: publish for %s: %v", userID, err)
	}
}
