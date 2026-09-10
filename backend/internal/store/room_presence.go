package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// DefaultRoomDisconnectGrace is how long a room member's socket may stay down before
// the room treats the disconnect as a departure.
//
// 5m, and deliberately LONGER than DefaultQueueDisconnectGrace's 90s. That ordering
// was the other way round until this window was re-derived, and the inversion is the
// point rather than an oversight, so do not "restore" it.
//
// The old reasoning held that a room should expire faster than a queue place because a
// stale roster makes the product look asleep, while a lost queue place costs a wait you
// already served. What that missed is who pays. A queue place is rivalrous: every
// second you hold one, strangers behind you wait, so 90s is a fairness ceiling imposed
// by people who are not you. A private room is not rivalrous at all — its members are
// the only people who can ever use it, and they are the same people coming back. Nobody
// is kept waiting by a room that waits. So a room has no reason to be the impatient one,
// and 30s turned ordinary behaviour into a departure: tab away to read a rule, take a
// call, let a laptop sleep for a minute, and the room you were standing in is gone.
//
// 5m rather than something larger because that is where the client stops trying. The
// room subscription retries 65 times with min(500ms * n, 5s) backoff
// (frontend/src/lib/rooms.js), which is 297.5s of reconnect attempts before it gives
// up. Anything longer would hold a room open for a browser that has already given up on
// it, which is not a held place but a lie with a longer lifetime; anything shorter would
// evict a player whose own client still believes it is coming back, so a reload or a
// tunnel would read as leaving. So the rule this number encodes is unchanged, only its
// value: we hold your place for exactly as long as your client is still asking for it,
// and not one window longer.
//
// (The retry budget is 297.5s and not 302.5s because graphql-ws passes retryWait a
// 0-based count — the first retry waits min(500*0, 5000) = 0ms. The 27.5s the previous
// derivation quoted for 10 attempts was the 1-based reading of the same formula; the
// real figure then was 22.5s. The conclusion it drew was unaffected.)
//
// Moving it means re-deriving it from that backoff, not nudging the constant: cut the
// retry budget and this should follow it down; raise the retries and this has to
// follow up or reconnects start losing rooms.
const DefaultRoomDisconnectGrace = 5 * time.Minute

// DefaultClosedRoomRetention is how long a closed room's row survives before the
// sweep deletes it outright.
//
// Closing is the player-visible removal — a closed room is unreachable by invite code
// (GetRoomByInviteCode), invisible to GetUserRoom, and refuses new members — so
// nothing here is about what players see. This is purely about not accumulating a
// table of rooms nobody has opened in years, which is the real cost of making every
// Play with friends click mint a fresh room.
//
// The delay is not caution for its own sake. A closed room's tables deliberately
// outlive it: game_sessions.regroup_table_id points at a room_table, room_tables
// cascade-deletes with its room, and loadFormingRegroupTableTx reads a forming table
// through a closed room on purpose (see its comment — adopting one through an open-room
// join is a bricked-forever bug). Deleting the room at close time would therefore take
// "play again with the same group" down with it.
//
// So the floor is "past any possible regroup", and regroup is a post-match flow measured
// in minutes — the session sweep completes an abandoned session at
// DefaultStaleMatchedQueueAge, and nobody returns to a rematch prompt hours later. Six
// hours clears that by two orders of magnitude while still bounding the table to a day's
// worth of rooms rather than a year's, which is the point: every Play with friends click
// mints a fresh room now, so the row count is a rate, not a population.
const DefaultClosedRoomRetention = 6 * time.Hour

// roomHasLivePlayClause is the "this room is busy, not merely unattended" test — the one
// thing that outlives an empty roster and must still keep a room open.
//
// A started table with an active session means the players are IN the game. Their
// sockets are on the game, not on the lobby, so presence reads every one of them as
// gone and the room they came from looks abandoned. That is the case where closing it
// would destroy an in-flight match rather than tidy up after one.
//
// A FORMING table is deliberately NOT live play, which is worth stating because the
// opposite reads as obviously correct. A forming table's seats look like a roster worth
// protecting, but a seat is only claimable by a room member (sitAtTableTx's
// isRoomMemberTx), so while anybody is legitimately seated the room has members and this
// clause is never consulted — every caller tests it only after finding room_members
// empty. Seats surviving an empty roster are therefore seats nobody is entitled to, and
// two paths produce them routinely: completeSessionTx returns a started table to forming
// and re-seats its participants (resetRoomTableAfterSessionTx), and any seat outliving
// its membership. Counting those would pin a room open permanently on behalf of players
// who are gone, with nothing left that could ever clear them.
//
// The tables themselves are fine left behind — loadFormingRegroupTableTx reads a forming
// table through a closed room on purpose, and sweepStaleEmptyTablesTx collects the
// unseated ones.
//
// One constant rather than a copy per caller, for the reason
// activeSessionParticipationClause gives: two hand-maintained copies of a liveness test
// drift, and the drift reads as correct in review. REQUIRES the caller to alias rooms
// as `r`, so the correlation cannot be rewritten differently at each site.
const roomHasLivePlayClause = `
	EXISTS (
	    SELECT 1
	    FROM room_tables rt
	    WHERE rt.room_id = r.id
	      AND rt.status = 'started'
	      AND EXISTS (
	        SELECT 1 FROM game_sessions gs
	        WHERE gs.id = rt.session_id AND gs.status = 'active'
	      )
	  )`

// closeRoomIfEmptyTx closes the room when nothing is left in it — no members, and no
// live play holding it open.
//
// Every condition is a predicate on the UPDATE rather than a preceding SELECT, for the
// reason EvictDisconnectedWaitingEntry gives: a read-then-write is check-then-act, and
// somebody joining by invite code between the read and the write would be inserted into
// a room this call then closes. Zero rows affected means the room is still wanted.
func (s *Store) closeRoomIfEmptyTx(ctx context.Context, tx *sql.Tx, roomID uuid.UUID) (bool, error) {
	res, err := tx.ExecContext(ctx, `
		UPDATE rooms r
		SET status = $2, updated_at = NOW()
		WHERE r.id = $1
		  AND r.status = $3
		  AND NOT EXISTS (SELECT 1 FROM room_members rm WHERE rm.room_id = r.id)
		  AND NOT `+roomHasLivePlayClause,
		roomID, RoomStatusClosed, RoomStatusOpen)
	if err != nil {
		return false, fmt.Errorf("close room if empty: %w", err)
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return false, fmt.Errorf("close room if empty rows affected: %w", err)
	}
	return affected > 0, nil
}

// RoomEviction reports whether an expiry actually removed a member, and what the caller
// needs in order to publish the same room update a deliberate leave publishes.
type RoomEviction struct {
	Acted bool
	// RoomID is the room the player was removed from. Meaningful only when Acted.
	RoomID uuid.UUID
	// RoomClosed says the removal emptied the room and closed it, so the caller can
	// tell "the roster changed" from "the room is gone".
	RoomClosed bool
}

// EvictDisconnectedRoomMember removes a user from their room if and only if their
// socket is still down and the disconnect stamp still matches the one the caller armed
// its timer on, then closes the room if that emptied it.
//
// The stamp guard is what makes a reconnect inside the window keep the player in the
// room: a returning player owns the presence row now, its disconnected_at is NULL, and
// this matches nothing. A backgrounded phone that comes back is not someone who left,
// and the guard — not a second lookup — is what enforces that.
//
// It deliberately does NOT reuse LeaveRoom. That path is unconditional, so calling it
// from an expiry timer would evict a player who had already reconnected, and the
// resulting bug would read as correct at the call site.
func (s *Store) EvictDisconnectedRoomMember(ctx context.Context, userID uuid.UUID, stamp time.Time) (RoomEviction, error) {
	var out RoomEviction

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return out, fmt.Errorf("begin room eviction: %w", err)
	}
	defer tx.Rollback()

	err = tx.QueryRowContext(ctx, `
		DELETE FROM room_members rm
		USING user_presence up
		WHERE rm.user_id = $1
		  AND up.user_id = rm.user_id
		  AND up.connection_count = 0
		  AND up.disconnected_at = $2
		RETURNING rm.room_id
	`, userID, stamp).Scan(&out.RoomID)
	if errors.Is(err, sql.ErrNoRows) {
		// A reconnect, a deliberate leave, or a move to another room got here
		// first. Not an error — the guard doing its job.
		return RoomEviction{}, nil
	}
	if err != nil {
		return RoomEviction{}, fmt.Errorf("evict disconnected room member: %w", err)
	}
	out.Acted = true

	// Vacating the seat is part of the removal, not a separate courtesy. A seat is
	// only claimable by a room member (sitAtTableTx's isRoomMemberTx), so a seat left
	// behind by an evicted member is a seat no rule allows — it blocks its seat_key
	// against the friends still in the room, and it counts as live play in
	// roomHasLivePlayClause, which would pin the room open on the strength of a
	// player who is gone.
	if _, _, err := s.leaveTableSeatTx(ctx, tx, userID); err != nil {
		return RoomEviction{}, fmt.Errorf("vacate seat on room eviction: %w", err)
	}

	closed, err := s.closeRoomIfEmptyTx(ctx, tx, out.RoomID)
	if err != nil {
		return RoomEviction{}, err
	}
	out.RoomClosed = closed

	if err := tx.Commit(); err != nil {
		return RoomEviction{}, fmt.Errorf("commit room eviction: %w", err)
	}
	return out, nil
}

// staleDisconnectedRoomMemberPredicate matches room_members rows whose player's last
// socket closed longer ago than the room grace window.
//
// This is the crash backstop, not the normal path: when the API process survives, an
// in-process timer removes the member at exactly the window. This catches the rows
// whose timer died with their pod, in the manner of staleDisconnectedWaitingPredicate.
const staleDisconnectedRoomMemberPredicate = `
	EXISTS (
	    SELECT 1
	    FROM user_presence up
	    WHERE up.user_id = rm.user_id
	      AND up.connection_count = 0
	      AND up.disconnected_at IS NOT NULL
	      AND up.disconnected_at < NOW() - $1::interval
	  )`

// RoomSweepResult reports what one pass of the room sweep saw and did.
type RoomSweepResult struct {
	// MembersBefore is how many memberships had outlived their player's window.
	MembersBefore int
	// MembersRemoved is how many of those the sweep actually removed.
	MembersRemoved int
	// MembersAfter is how many remain. Above zero means rows resisted the sweep and
	// a human should look.
	MembersAfter int
	// RoomsClosed is how many rooms the removals emptied.
	RoomsClosed int
	// RoomsDeleted is how many long-closed rooms passed the retention age and were
	// deleted outright.
	RoomsDeleted int
}

// CountStaleDisconnectedRoomMembers reports how many memberships have outlived their
// player's grace window. Writes nothing, so it is safe from a health check.
func (s *Store) CountStaleDisconnectedRoomMembers(ctx context.Context, olderThan time.Duration) (int, error) {
	var count int
	err := s.db.QueryRowContext(ctx, `
		SELECT count(*) FROM room_members rm WHERE `+staleDisconnectedRoomMemberPredicate,
		pgInterval(olderThan),
	).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("count stale disconnected room members: %w", err)
	}
	return count, nil
}

// SweepStaleDisconnectedRoomMembers removes memberships abandoned by a disconnected
// player across all users, then closes whichever rooms that emptied.
//
// Unlike the timer path it publishes nothing — this is a separate binary with no
// pubsub, exactly like the queue sweeps. Acceptable because it only ever runs on rows
// orphaned by a dead pod, whose sockets went with it.
func (s *Store) SweepStaleDisconnectedRoomMembers(ctx context.Context, olderThan time.Duration) (RoomSweepResult, error) {
	var result RoomSweepResult
	interval := pgInterval(olderThan)

	before, err := s.CountStaleDisconnectedRoomMembers(ctx, olderThan)
	if err != nil {
		return result, err
	}
	result.MembersBefore = before

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return result, fmt.Errorf("begin room sweep: %w", err)
	}
	defer tx.Rollback()

	// RETURNING the users and rooms rather than counting rows: each removal has the
	// same two consequences the timer path has — a seat to vacate and a room that may
	// now be empty — and neither is expressible in the DELETE.
	rows, err := tx.QueryContext(ctx, `
		DELETE FROM room_members rm
		WHERE `+staleDisconnectedRoomMemberPredicate+`
		RETURNING rm.user_id, rm.room_id`, interval)
	if err != nil {
		return result, fmt.Errorf("sweep stale disconnected room members: %w", err)
	}
	type removal struct{ userID, roomID uuid.UUID }
	var swept []removal
	for rows.Next() {
		var r removal
		if err := rows.Scan(&r.userID, &r.roomID); err != nil {
			rows.Close()
			return result, fmt.Errorf("sweep stale disconnected room members scan: %w", err)
		}
		swept = append(swept, r)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return result, fmt.Errorf("sweep stale disconnected room members rows: %w", err)
	}
	if err := rows.Close(); err != nil {
		return result, fmt.Errorf("sweep stale disconnected room members close: %w", err)
	}
	result.MembersRemoved = len(swept)

	seen := make(map[uuid.UUID]bool, len(swept))
	for _, r := range swept {
		if _, _, err := s.leaveTableSeatTx(ctx, tx, r.userID); err != nil {
			return result, fmt.Errorf("vacate seat on room sweep: %w", err)
		}
		seen[r.roomID] = true
	}
	// Rooms after seats, not interleaved: two members of one room swept together
	// would otherwise have the first one's close decision made while the second still
	// held a seat, and the room would survive an emptying that did happen.
	for roomID := range seen {
		closed, err := s.closeRoomIfEmptyTx(ctx, tx, roomID)
		if err != nil {
			return result, err
		}
		if closed {
			result.RoomsClosed++
		}
	}

	if err := tx.Commit(); err != nil {
		return result, fmt.Errorf("commit room sweep: %w", err)
	}

	after, err := s.CountStaleDisconnectedRoomMembers(ctx, olderThan)
	if err != nil {
		return result, err
	}
	result.MembersAfter = after
	return result, nil
}

// CountEmptyOpenRooms reports how many open rooms have nobody in them and nothing
// holding them open. Writes nothing, so it is safe from a health check.
func (s *Store) CountEmptyOpenRooms(ctx context.Context) (int, error) {
	var count int
	err := s.db.QueryRowContext(ctx, `
		SELECT count(*) FROM rooms r
		WHERE r.status = $1
		  AND NOT EXISTS (SELECT 1 FROM room_members rm WHERE rm.room_id = r.id)
		  AND NOT `+roomHasLivePlayClause, RoomStatusOpen).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("count empty open rooms: %w", err)
	}
	return count, nil
}

// SweepEmptyRooms closes open rooms that nothing is left in.
//
// This is not a duplicate of the membership sweep, and the gap it fills is the whole
// reason live play keeps a room open. A room whose players are IN a game is deliberately
// left open with an empty roster — see roomHasLivePlayClause — and when that match ends
// there is no member left to remove, so no removal path runs and nothing reconsiders the
// room. Closing is driven by the last member leaving, and by then the last member has
// already left. Without this pass a room outlives every match played in it, and the
// retention sweep never reaches it either, because that one only deletes CLOSED rooms.
//
// Same predicate as closeRoomIfEmptyTx, and deliberately the same constant behind it:
// this is that decision made late, for the rooms whose moment to make it has passed.
func (s *Store) SweepEmptyRooms(ctx context.Context) (int, error) {
	res, err := s.db.ExecContext(ctx, `
		UPDATE rooms r
		SET status = $2, updated_at = NOW()
		WHERE r.status = $1
		  AND NOT EXISTS (SELECT 1 FROM room_members rm WHERE rm.room_id = r.id)
		  AND NOT `+roomHasLivePlayClause, RoomStatusOpen, RoomStatusClosed)
	if err != nil {
		return 0, fmt.Errorf("sweep empty rooms: %w", err)
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("sweep empty rooms rows affected: %w", err)
	}
	return int(affected), nil
}

// CountRetiredRooms reports how many closed rooms have passed the retention age.
// Writes nothing, so it is safe from a health check.
func (s *Store) CountRetiredRooms(ctx context.Context, olderThan time.Duration) (int, error) {
	var count int
	err := s.db.QueryRowContext(ctx, `
		SELECT count(*) FROM rooms r
		WHERE r.status = $1 AND r.updated_at < NOW() - $2::interval
	`, RoomStatusClosed, pgInterval(olderThan)).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("count retired rooms: %w", err)
	}
	return count, nil
}

// DeleteRetiredRooms deletes closed rooms whose last change is older than olderThan,
// and with them the tables, seats and messages that cascade from the row.
//
// This is the half of removal that closing does not do. Closing is immediate and
// player-visible; this is the row actually going away, deferred long enough that the
// regroup path — which reads a forming table through its closed room on purpose — has
// no possible remaining interest in it. See DefaultClosedRoomRetention.
//
// updated_at rather than created_at is the age that matters: a long-lived room that
// closed an hour ago has been finished for an hour, not for however long it existed,
// and leaveRoomTx stamps updated_at on the way through.
func (s *Store) DeleteRetiredRooms(ctx context.Context, olderThan time.Duration) (int, error) {
	res, err := s.db.ExecContext(ctx, `
		DELETE FROM rooms r
		WHERE r.status = $1 AND r.updated_at < NOW() - $2::interval
	`, RoomStatusClosed, pgInterval(olderThan))
	if err != nil {
		return 0, fmt.Errorf("delete retired rooms: %w", err)
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("delete retired rooms rows affected: %w", err)
	}
	return int(affected), nil
}
