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

// DefaultTableSeatDisconnectGrace is how long a player's socket may stay down before the
// forming-table seat they are holding is released.
//
// 30s, and deliberately NOT DefaultRoomDisconnectGrace's 5m, even though both windows run
// off the same socket edge for the same player. They are not the same question, because a
// room and a seat cost the people left behind completely different amounts.
//
// A room that waits costs nobody anything: its members are the only people who can ever
// use it and they are the same people coming back, which is why it waits five minutes. A
// held seat is the opposite — it is the one thing at a forming table another player
// actively wants, and while it is held the table cannot fill and the king cannot start.
// Bob should not be staring at a seat he is not allowed to take because Alice's phone died.
//
// The asymmetry costs the disconnected player almost nothing, which is what makes it the
// right trade rather than merely a defensible one: coming back at two minutes, Alice still
// has her room, her friends and the chat, and re-takes a seat with one tap. Coming back to
// no room at all is what actually hurts. So the seat goes early and the room waits.
//
// This is NOT the seat-hold window for an already-formed match (JQ-199), which answers how
// long a match that has already been made waits for a player who is not there. Do not
// borrow one for the other.
const DefaultTableSeatDisconnectGrace = 30 * time.Second

// DefaultRoomRosterPresenceGrace is how long the roster keeps calling a member present
// after their last socket closed.
//
// It changes nothing about what the player holds — a member past it still has their
// membership, their seat is governed by DefaultTableSeatDisconnectGrace and their room by
// DefaultRoomDisconnectGrace. It governs one thing only: what the roster asserts.
//
// Named for the claim it governs rather than for the window it lives inside, because
// DefaultRoomDisconnectGrace is the constant it must never be mistaken for. A room waits
// 5m for good reasons (see there), and a roster that stayed silent for all of it would
// spend those five minutes telling everyone else in the room that a player whose battery
// died is sitting right there. Holding someone's place and claiming they are present are
// different promises, and only the first one is worth five minutes.
//
// 30s, which is DefaultTableSeatDisconnectGrace's value and deliberately not a borrow of
// its identifier. Both answer "do we still believe this person is at their device", so
// they agree today; they are separate constants because they act on that belief for
// different people. The seat window spends Alice's seat on Bob's behalf, so it is bounded
// by Bob's patience at a table that cannot fill. This window spends nothing — it only
// makes the roster honest — so it is free to move if a mobile audience turns out to want a
// beat longer before their friends see them greyed out. One identifier for both would make
// that a change to seat availability too.
//
// The window only ever starts for a socket that actually closed: presence counts
// subscriptions, so a backgrounded tab that keeps its socket never reaches here at all.
// 30s is then short enough that a two-person room learns the truth quickly, and long
// enough that an ordinary reload or a passing tunnel is invisible to everybody else.
//
// What this measures is disconnection, and only disconnection. It is NOT UserIsAway
// (visibility.go), which is the opposite evidence — a live socket with no visible
// document — and returns false for exactly the players this window catches. The roster
// draws both as "away" to a player, and JQ-179 is where the second signal joins the
// first, but at this layer they stay two readings that can disagree.
const DefaultRoomRosterPresenceGrace = 30 * time.Second

// DefaultClosedRoomRetention is how long a closed room's row survives before the
// sweep deletes it outright.
//
// Closing is the player-visible removal — a closed room is unreachable by invite code
// (GetRoomByInviteCode), invisible to GetUserRoom, and refuses new members — so
// nothing here is about what players see. This is purely about not accumulating a
// table of rooms nobody has opened in years, which is the real cost of making every
// Play with friends click mint a fresh room.
//
// The delay is not caution for its own sake, but it is NOT about preserving regroup, and
// this comment used to say it was. The claim of record was that loadFormingRegroupTableTx
// "reads a forming table through a closed room on purpose", so deleting a room at close
// time would take "play again with the same group" down with it. It does the opposite: a
// closed room's table reads as unclaimed (liveRegroupTableClause) precisely so the next
// claim builds a fresh one, because adopting a table in a closed room is the
// bricked-forever bug. Closing a room already ends its offer; deleting it later takes
// nothing further away.
//
// What the delay actually buys is that nothing has to be careful about ordering. Rooms
// close on a 5-minute presence window while sessions, tables and seats hang off them by
// foreign key, so a row deleted the instant its room closed would be deleted underneath
// whatever was still reading it.
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
	//
	// In the ordinary case this now finds nothing to do: the seat went at
	// DefaultTableSeatDisconnectGrace, ten times earlier in the same disconnect. It stays
	// here because it is cheap and because "a member is removed" must never be able to
	// leave a seat standing, whatever path got here — a seat timer that died with its pod,
	// a membership removed by something other than the disconnect window, or a future
	// caller that does not know about the seat window at all.
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

// TableSeatRelease reports whether an expiry actually freed a seat, and names the table it
// freed it at so the caller can tell the room's watchers.
//
// Everyone at /room/{code} and /group watches tableUpdated, not roomUpdated, so without the
// ids there is nothing to address the publish to and the seat stays visibly occupied until
// some unrelated table event fires. Same reason RegroupSeatRelease carries them.
type TableSeatRelease struct {
	Acted   bool
	TableID uuid.UUID
	RoomID  uuid.UUID
}

// ReleaseDisconnectedTableSeat frees the forming-table seat a user is holding if and only
// if their socket is still down and the disconnect stamp still matches the one the caller
// armed its timer on.
//
// Every safety condition is a predicate on the DELETE rather than a preceding SELECT, for
// the reason EvictDisconnectedRoomMember gives: a read-then-write is check-then-act, and a
// reconnect landing in the gap would cost a returned player the seat they are sitting in.
// Zero rows affected means somebody got there first — a reconnect, a deliberate leave, a
// table that started — which is exactly the outcome we want.
//
// It deliberately does NOT touch room membership. This window answers "should someone else
// be allowed to take this seat", not "is this player still in the room", and the room's own
// window is ten times longer on purpose (see DefaultTableSeatDisconnectGrace). A player who
// comes back at two minutes finds their room, their friends and their chat exactly as they
// left them, and re-takes a seat with one tap.
//
// Scoped to forming tables. A seat at a STARTED table is not a seat anyone is waiting for —
// the match is under way and its players' sockets are on the game, not the lobby, so every
// one of them looks disconnected from here. Freeing those would tear down the seating of a
// live match, which is the same trap roomHasLivePlayClause exists to avoid.
func (s *Store) ReleaseDisconnectedTableSeat(ctx context.Context, userID uuid.UUID, stamp time.Time) (TableSeatRelease, error) {
	var out TableSeatRelease

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return out, fmt.Errorf("begin table seat release: %w", err)
	}
	defer tx.Rollback()

	err = tx.QueryRowContext(ctx, `
		DELETE FROM table_seats ts
		USING user_presence up, room_tables rt
		WHERE ts.user_id = $1
		  AND up.user_id = ts.user_id
		  AND up.connection_count = 0
		  AND up.disconnected_at = $2
		  AND rt.id = ts.table_id
		  AND rt.status = $3
		RETURNING ts.table_id, rt.room_id
	`, userID, stamp, TableStatusForming).Scan(&out.TableID, &out.RoomID)
	if errors.Is(err, sql.ErrNoRows) {
		return TableSeatRelease{}, nil
	}
	if err != nil {
		return TableSeatRelease{}, fmt.Errorf("release disconnected table seat: %w", err)
	}
	out.Acted = true

	// The table's own row carries the change, exactly as leaveTableSeatTx and LeaveTable
	// do. A client that reconciles on updated_at would otherwise never see the seat go.
	if _, err := tx.ExecContext(ctx, `
		UPDATE room_tables SET updated_at = NOW() WHERE id = $1
	`, out.TableID); err != nil {
		return TableSeatRelease{}, fmt.Errorf("touch table on seat release: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return TableSeatRelease{}, fmt.Errorf("commit table seat release: %w", err)
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
