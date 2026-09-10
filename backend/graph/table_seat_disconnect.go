package graph

import (
	"context"
	"log"
	"time"

	"github.com/google/uuid"
)

// OnTableSeatGraceExpired runs when a disconnected player's seat window elapses without a
// reconnect, and gives the seat back to the table.
//
// The third expiry off one socket edge, and the shortest. See
// store.DefaultTableSeatDisconnectGrace for why a seat's 30s is not the room's 5m: a room
// that waits costs the people left behind nothing, while a held seat is the one thing at a
// forming table somebody else actively wants. A player 40s into a disconnect has lost their
// seat, kept their room, and kept their queue place — three answers to the same disconnect,
// and a reconnect cancels all of them.
func (r *Resolver) OnTableSeatGraceExpired(ctx context.Context, userID uuid.UUID, stamp time.Time) {
	st, err := r.requireStore()
	if err != nil {
		log.Printf("table seat expiry: store unavailable for %s: %v", userID, err)
		return
	}

	release, err := st.ReleaseDisconnectedTableSeat(ctx, userID, stamp)
	if err != nil {
		log.Printf("table seat expiry: release for %s: %v", userID, err)
	}
	if !release.Acted {
		// A reconnect, a deliberate leave, or the table starting got here first. Not an
		// error — the guard doing its job.
		return
	}

	// Acted survives an error, for the reason OnRoomGraceExpired gives: the delete is
	// committed by the time anything downstream of it can fail, so returning early would
	// drop the publish for a seat that is already free.
	//
	// The publish is most of the point. The whole reason the seat goes at 30s rather than
	// 5m is so the friends still at the table can take it — and they are watching
	// tableUpdated, so a silent release would free the seat in Postgres while every client
	// still renders it occupied. It is the same event a deliberate leave publishes, so the
	// client needs no idea this departure was involuntary.
	if err := r.publishTableUpdated(ctx, release.RoomID, release.TableID); err != nil {
		log.Printf("table seat expiry: publish for %s: %v", userID, err)
	}
}
