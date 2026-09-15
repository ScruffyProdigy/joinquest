package graph

import (
	"context"
	"errors"
	"log"
	"time"

	"github.com/google/uuid"
	"github.com/scruffyprodigy/joinquest/internal/store"
)

// OnRoomRosterPresenceExpired runs when a disconnected member's roster window elapses, and
// tells their room that the roster has stopped claiming they are there.
//
// The fourth expiry off one socket edge, and the only one that takes nothing away. The
// other three end something — a queue place, a room membership, a seat — and publish so
// that everyone sees what was ended. This one ends nothing at all: RoomMember.Disconnected
// is derived on read from user_presence, so by the time this runs the roster already reads
// correctly, and has since the instant the window passed. What is missing is an event, and
// a derived field only reaches a client when something re-sends the room.
//
// That is the whole bug this closes. Time passing has no event of its own, so the one
// moment the reading changes was the one moment nothing published, and a dropped player
// stayed drawn as fully present until an unrelated join or leave happened to fire. Nobody
// noticed sooner because the seat window is the same 30s and does publish — a seated player
// visibly loses their seat on time, which makes the missing dimming look like a seat bug
// rather than a roster one. It is not about seats: a member who never sat down anywhere had
// exactly the same problem.
//
// A separate expiry rather than a line inside OnTableSeatGraceExpired, though the two fire
// at the same 30s today. The seat's window is bounded by another player's patience at a
// table that cannot fill, and this one spends nothing and is free to move if a mobile
// audience wants a beat longer before their friends see them greyed out — see
// store.DefaultRoomRosterPresenceGrace. Hanging the publish off the seat timer would make
// them one number again silently, and would miss every member who was not seated.
//
// The stamp the tracker armed on is not re-checked here, unlike the expiries that remove
// something: this publishes only when the roster's own predicate says the reading has
// flipped (RoomForDisconnectedMember), which is a stricter guard for this purpose than a
// matching stamp. A player who reconnected in the race between the timer firing and this
// running is present again by that predicate, and gets no publish — and would lose nothing
// if they did, since a roomUpdated asserts nothing except "read the room again".
func (r *Resolver) OnRoomRosterPresenceExpired(ctx context.Context, userID uuid.UUID, stamp time.Time) {
	st, err := r.requireStore()
	if err != nil {
		log.Printf("roster presence expiry: store unavailable for %s: %v", userID, err)
		return
	}

	roomID, away, err := st.RoomForDisconnectedMember(ctx, userID, store.DefaultRoomRosterPresenceGrace)
	if err != nil {
		log.Printf("roster presence expiry: room for %s: %v", userID, err)
		return
	}
	if !away {
		// Back already, never in a room, or in one that has since closed. Not an
		// error — the guard doing its job.
		return
	}

	if err := r.publishRoomUpdated(ctx, roomID); err != nil {
		log.Printf("roster presence expiry: publish for %s: %v", userID, err)
	}
}

// OnRoomRosterPresenceRestored runs when a room member's first socket opens after their
// last one closed, and tells their room to read the roster again.
//
// The same mechanism as the expiry above, on the other edge, and that is the point rather
// than tidiness. The away reading is cleared in user_presence by the reconnect itself — one
// write, no timer to beat — so the room is already correct the instant the player is back.
// What is missing is again the event: everyone else is holding a roster they were told about
// once, and nothing re-reads it on its own. A player who came back would otherwise stay
// greyed out on their friends' screens until an unrelated mutation fired, which is the bug
// this ticket is about, pointing the other way.
//
// It publishes for every returning member, not only for one who was drawn as away. The
// reading is derived, so this cannot tell — the stamp that would have said how long they
// were gone is the one the reconnect just cleared — and the two ways to find out are both
// worse than the extra publish: keeping a per-player "we said they were away" flag is a
// second answer to a question user_presence already answers, and comparing the cleared
// stamp against the window means the undim rides on a clock comparison that, if it lands
// the wrong side, leaves a returned player greyed out with nothing left to correct it.
// A roomUpdated asserts nothing except "read the room again", so the cost of one that was
// not needed is a re-resolve, and it stays bounded: one per connect edge, which is one per
// disconnect the player actually had, however many sockets the tab opens.
func (r *Resolver) OnRoomRosterPresenceRestored(ctx context.Context, userID uuid.UUID) {
	st, err := r.requireStore()
	if err != nil {
		log.Printf("roster presence return: store unavailable for %s: %v", userID, err)
		return
	}

	room, err := st.GetUserRoom(ctx, userID)
	if err != nil {
		if !errors.Is(err, store.ErrNotFound) {
			log.Printf("roster presence return: room for %s: %v", userID, err)
		}
		// Not in a room. Most reconnects are this, and none of them is an error.
		return
	}

	if err := r.publishRoomUpdated(ctx, room.ID); err != nil {
		log.Printf("roster presence return: publish for %s: %v", userID, err)
	}
}
