package store

import (
	"context"
	"strings"

	"github.com/google/uuid"
)

// ArrivalParty is who a player came into a match with, and it is the unit everything
// post-match is scoped to: the regroup roster they see, and the room they go back to
// (JQ-291).
//
// A party is a room. Everyone who queued from the same room table shares its id; a player
// who joined the catalog queue on their own shares it with nobody. That is the whole rule —
// there is no separate party table to consult, and deliberately so:
//
//	parties.id       exists, but is never written to game_session_participants. It lives on
//	                 the forming map and the queue rows, both finished with the instant the
//	                 match fires, so by the regroup screen there is no record of it.
//	return_context   is stamped once when the session starts and never rewritten, which is
//	                 what makes it safe to partition on. room_tables.session_id is cleared by
//	                 the post-match reset and game_sessions.regroup_table_id is stamped by
//	                 whichever player claims first, so neither can answer this per player.
//
// The proxy is faithful because a group can only reach a match through a room: "Play with
// friends" opens an implicit room and table (JQ-131/JQ-132), and a table that pulls in
// strangers carries its room id through the fire (formingReturnContextTx).
type ArrivalParty string

// NoArrivalParty is the party of a player who arrived alone. It is not an id and never
// matches another player's — two solo joiners in one match are two parties of one, not one
// party of two. Every comparison goes through SameArrivalParty so that cannot be forgotten.
const NoArrivalParty ArrivalParty = ""

// ArrivalPartyOf reads the party out of a stamped return context.
func ArrivalPartyOf(rc ReturnContext) ArrivalParty {
	return ArrivalParty(strings.TrimSpace(rc.RoomID))
}

// ArrivalPartyOfRoom is the party a room's members form, for callers holding a room id
// rather than a return context — the regroup table resolving its own roster, say.
func ArrivalPartyOfRoom(roomID uuid.UUID) ArrivalParty {
	return ArrivalParty(roomID.String())
}

// SameArrivalParty reports whether two players arrived together. Two empty parties are NOT
// the same party: "I came alone" is not a group anyone else is in, so a solo player's roster
// is empty rather than every other solo in the match.
func SameArrivalParty(a, b ArrivalParty) bool {
	return a != NoArrivalParty && a == b
}

// GetArrivalParties maps every participant of a session to the party they arrived with.
// Players who left are included: the caller decides whether a departure hides someone,
// and the two existing roster builders already make that call for themselves.
func (s *Store) GetArrivalParties(ctx context.Context, sessionID uuid.UUID) (map[uuid.UUID]ArrivalParty, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT user_id, return_context
		FROM game_session_participants
		WHERE session_id = $1
	`, sessionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	parties := make(map[uuid.UUID]ArrivalParty)
	for rows.Next() {
		var (
			userID uuid.UUID
			raw    []byte
		)
		if err := rows.Scan(&userID, &raw); err != nil {
			return nil, err
		}
		rc, err := decodeReturnContext(raw)
		if err != nil {
			return nil, err
		}
		parties[userID] = ArrivalPartyOf(rc)
	}
	return parties, rows.Err()
}
