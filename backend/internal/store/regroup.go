package store

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/scruffyprodigy/joinquest/internal/prequeue"
)

// ErrNoRegroupMode is returned when a finished session has no mode to rebuild a table from.
// game_sessions.mode_id is nullable and sessions outlive their modes (JQ-134).
var ErrNoRegroupMode = errors.New("store: session has no mode to regroup into")

// ErrSessionNotFinished is returned when a claim arrives before the match is over. Claiming
// early would race CompleteSession: resetRoomTableAfterSessionTx overwrites regroup_table_id
// with the original room table, so the early claimant would be stranded on a table of their
// own while everyone else adopts the original (JQ-135).
var ErrSessionNotFinished = errors.New("store: session is still in progress")

// ErrTableFull is returned when the regroup table has no seat left for the caller. The
// caller is not opted in: regroup_opted_in_at must only ever mark a real seat holder.
var ErrTableFull = errors.New("store: table has no open seat")

// ClaimRegroupTable returns the forming table the caller's arrival party regroups at,
// creating it on first call, and seats the caller. The SELECT ... FOR UPDATE is what makes
// the party converge on one table instead of each member creating their own.
//
// The unit is the party, not the match (JQ-291). A 3v3 is ordinarily two groups of three
// who each came in through their own room, and converging the whole session on one table
// merged two sets of strangers into a room neither agreed to — whichever player claimed
// first decided where five other people went. Each party now regroups in the room it
// arrived from, so the two groups go home separately and each leaves the seats the other
// group filled genuinely open for backfill.
func (s *Store) ClaimRegroupTable(ctx context.Context, sessionID, userID uuid.UUID) (*RoomTable, *Room, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, nil, err
	}
	defer func() { _ = tx.Rollback() }()

	var (
		status string
		gameID *uuid.UUID
		modeID *uuid.UUID
	)
	// The lock is still taken on the session rather than on the party's room, because it is
	// what orders two claims that will build the *same* party's table. Locking per party
	// would let two members of one party each create one.
	err = tx.QueryRowContext(ctx, `
		SELECT status, game_id, mode_id
		FROM game_sessions
		WHERE id = $1
		FOR UPDATE
	`, sessionID).Scan(&status, &gameID, &modeID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil, ErrNotFound
		}
		return nil, nil, err
	}

	var participates bool
	if err := tx.QueryRowContext(ctx, `
		SELECT EXISTS(SELECT 1 FROM game_session_participants WHERE session_id = $1 AND user_id = $2)
	`, sessionID, userID).Scan(&participates); err != nil {
		return nil, nil, err
	}
	if !participates {
		return nil, nil, ErrNotFound
	}
	// Only a finished match regroups. The row lock serializes concurrent claims but does
	// not order this transaction against CompleteSession, so a claim on a still-active
	// session could stamp regroup_table_id only for CompleteSession to overwrite it with
	// the original room table, splitting the players across two tables. 'cancelled' is not
	// a match to replay either.
	if status != "completed" {
		return nil, nil, ErrSessionNotFinished
	}
	if modeID == nil || gameID == nil {
		return nil, nil, ErrNoRegroupMode
	}

	previous, err := loadParticipantSeatingTx(ctx, tx, sessionID, userID)
	if err != nil {
		return nil, nil, err
	}

	table, err := s.partyRegroupTableTx(ctx, tx, sessionID, userID, *gameID, *modeID, previous)
	if err != nil {
		return nil, nil, err
	}
	// regroup_table_id is one column and a session can now have as many regroup tables as
	// it had arrival parties, so it records the first only and no longer answers "where
	// does this player go" — GetRegroupTableIDForUser does, per party. It is still stamped
	// because resetRoomTableAfterSessionTx writes it for the room-table path and a column
	// that is sometimes written and sometimes not is worse than one that means "the first
	// table this match produced". Guarded on NULL so a second party cannot overwrite it.
	if _, err := tx.ExecContext(ctx, `
		UPDATE game_sessions SET regroup_table_id = $2 WHERE id = $1 AND regroup_table_id IS NULL
	`, sessionID, table.ID); err != nil {
		return nil, nil, err
	}
	// The forward pointer is stamped whether the table was just created or adopted from an
	// earlier claimant: a table reached from resetRoomTableAfterSessionTx already carries
	// it, but one created here on first claim does not (JQ-177).
	if _, err := tx.ExecContext(ctx, `
		UPDATE room_tables SET regroup_session_id = $2 WHERE id = $1
	`, table.ID, sessionID); err != nil {
		return nil, nil, err
	}
	table.RegroupSessionID = &sessionID

	if err := s.ensureRoomMemberTx(ctx, tx, table.RoomID, userID); err != nil {
		return nil, nil, err
	}

	mode, err := getGameModeByID(ctx, tx, table.ModeID)
	if err != nil {
		return nil, nil, err
	}

	// Three rules, one principle: a selection is carried forward only where the player
	// would not plausibly re-make it (JQ-232).
	//
	//	nothing to choose	seat them — re-clicking an identical seat is friction
	//	a choice, group play	seat nobody — the group chooses again on /group
	//	a choice, solo play	replay the seat and the options they just had
	//
	// The solo and the group rule are different by design, not by oversight: a solo
	// player almost always wants exactly what they just had, while a group came back
	// to rotate the spymaster or bring a different character. Tests assert both, so
	// neither is later "fixed" into the other.
	switch {
	case !ModeOffersPreMatchChoice(mode):
		if err := s.seatRegroupClaimantTx(ctx, tx, table, userID, "", nil); err != nil {
			return nil, nil, err
		}
	case previous.GroupPlay:
		// Seatless on purpose, and the opt-in stamp below still records them as back.
		// An IN who holds no seat is not a broken state here — it is exactly "returned,
		// still picking", which is what the "Picking a seat" card renders. Starting is
		// gated on seats by canStart, never on this stamp, so the king cannot start on
		// the strength of someone who has not sat down.
	case !selectionsSatisfyMode(mode, previous.Options):
		// Seatless for a different reason with the same shape: the picks we would
		// replay no longer satisfy what this mode declares, because the manifest moved
		// between rounds. Replaying them anyway would seat a player with a selection
		// that cannot start the table, and the king would meet that error rather than
		// the player who can answer the picker (JQ-211).
	default:
		if err := s.seatRegroupClaimantTx(ctx, tx, table, userID, previous.SeatKey, previous.Options); err != nil {
			return nil, nil, err
		}
	}

	if _, err := tx.ExecContext(ctx, `
		UPDATE game_session_participants
		SET regroup_opted_in_at = NOW(), regroup_declined_at = NULL
		WHERE session_id = $1 AND user_id = $2
	`, sessionID, userID); err != nil {
		return nil, nil, err
	}

	room, err := s.getRoomByIDTx(ctx, tx, table.RoomID)
	if err != nil {
		return nil, nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, nil, err
	}
	return table, room, nil
}

// liveRegroupTableClause is the one test for "this regroup offer is still real": the table
// still exists, is still forming, and its room is still open.
//
// One constant rather than a copy per caller, for the reason roomHasLivePlayClause gives —
// two hand-maintained copies of a liveness test drift, and the drift reads as correct in
// review. That is not hypothetical here. The claim path applied this test and the read path
// did not, so a finished match went on handing out the invite code of a table its own
// playAgain would refuse to adopt: the code looked live, and following it failed. Anything
// that answers "where is this party's regroup offer" goes through here.
//
// REQUIRES the caller to alias room_tables as `t` and rooms as `r`, so the correlation
// cannot be rewritten differently at each site.
const liveRegroupTableClause = `
	t.status = 'forming'
	  AND r.status = 'open'`

// partyRegroupTableTx finds or builds the table the claimant's arrival party regroups at.
// Three steps, most-specific first:
//
//  1. a table this party already claimed for this match — the second and third members to
//     press "Another round" land here, which is what converges a party on one table;
//  2. the table the party arrived from, still sitting forming in their room. This is the
//     ordinary case for a room-table group: resetRoomTableAfterSessionTx hands their own
//     table back the moment the match completes, already stamped, so step 1 usually catches
//     it — but a group that reached the match through matchmaking never went through that
//     reset, and their table is still there unstamped;
//  3. a fresh table in the party's room.
//
// A player who arrived alone has no party room and no arrival table, so they fall to step 3
// in a room of their own. Two of them in one match get two tables: they did not arrive
// together, so they do not go back together (JQ-291).
func (s *Store) partyRegroupTableTx(
	ctx context.Context,
	tx *sql.Tx,
	sessionID, userID, gameID, modeID uuid.UUID,
	arrival *participantSeating,
) (*RoomTable, error) {
	room, err := s.regroupRoomTx(ctx, tx, userID, arrival)
	if err != nil {
		return nil, err
	}

	table, err := s.liveRegroupTableInRoomTx(ctx, tx, sessionID, room.ID)
	if err != nil {
		return nil, err
	}
	if table != nil {
		return table, nil
	}

	table, err = s.adoptArrivalTableTx(ctx, tx, room.ID, gameID, modeID, arrival)
	if err != nil {
		return nil, err
	}
	if table != nil {
		return table, nil
	}

	return s.createTableTx(ctx, tx, room.ID, gameID, modeID)
}

// regroupRoomTx answers which room this claimant regroups into: the one they arrived from
// while it is still open, and otherwise a room of their own.
//
// The fallback is not only for solo players. A room that emptied while the match ran is
// closed by leaveRoomTx, and sitAtTableTx's isRoomMemberTx requires an open room — so
// returning the arrival room regardless would fail every claim from that party with
// ErrNotFound, which regroupClientError reports as "you did not play in this match",
// permanently. A fresh room is a worse answer than their own room and a far better one than
// no way back at all.
func (s *Store) regroupRoomTx(ctx context.Context, tx *sql.Tx, userID uuid.UUID, arrival *participantSeating) (*Room, error) {
	if arrival.ArrivalRoomID != nil {
		room, err := s.getRoomByIDTx(ctx, tx, *arrival.ArrivalRoomID)
		if err != nil && !errors.Is(err, ErrNotFound) {
			return nil, err
		}
		if err == nil && room.Status == RoomStatusOpen {
			return room, nil
		}
	}
	room, err := s.getUserRoomTx(ctx, tx, userID)
	if err == nil {
		return room, nil
	}
	if !errors.Is(err, ErrNotFound) {
		return nil, err
	}
	return s.createRoomTx(ctx, tx, userID)
}

// liveRegroupTableInRoomTx returns this party's already-claimed table for the match, or nil.
// The (regroup_session_id, room_id) pair is the index that replaced the single
// game_sessions.regroup_table_id column: one row per party rather than one per session, with
// no migration, because both columns already existed.
//
// A swept, started or closed-room table reads as unclaimed so the caller builds a fresh one —
// see liveRegroupTableClause for why the room half of that test is load-bearing.
func (s *Store) liveRegroupTableInRoomTx(ctx context.Context, tx *sql.Tx, sessionID, roomID uuid.UUID) (*RoomTable, error) {
	row := tx.QueryRowContext(ctx, `
		SELECT `+roomTableColumnsT+`
		FROM room_tables t
		INNER JOIN rooms r ON r.id = t.room_id
		WHERE t.regroup_session_id = $1 AND t.room_id = $2 AND `+liveRegroupTableClause+`
		ORDER BY t.created_at DESC
		LIMIT 1
	`, sessionID, roomID)
	table, err := scanRoomTable(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) || errors.Is(err, ErrNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return table, nil
}

// adoptArrivalTableTx reclaims the table the party left to play the match, when it is still
// forming in the room they came back to and still set up for the same game and mode.
//
// Without this a group matched through the queue comes home to two tables: the one they were
// sitting at, which nothing reset because they never went through StartTable, and a brand new
// one beside it. Adopting theirs is both tidier and more honest — it is the table their room
// already shows.
//
// Its seats are cleared on the way in, for the reason resetRoomTableAfterSessionTx clears
// them: they are last round's, and a group came back to rotate the spymaster or bring a
// different character (JQ-232). The caller's seating rules then run per claimant, which
// re-seats everyone where the mode offers nothing to choose.
func (s *Store) adoptArrivalTableTx(
	ctx context.Context,
	tx *sql.Tx,
	roomID, gameID, modeID uuid.UUID,
	arrival *participantSeating,
) (*RoomTable, error) {
	if arrival.ArrivalTableID == nil {
		return nil, nil
	}
	row := tx.QueryRowContext(ctx, `
		SELECT `+roomTableColumnsT+`
		FROM room_tables t
		INNER JOIN rooms r ON r.id = t.room_id
		WHERE t.id = $1
		  AND t.room_id = $2
		  AND t.game_id = $3
		  AND t.mode_id = $4
		  AND t.session_id IS NULL
		  AND t.regroup_session_id IS NULL
		  AND `+liveRegroupTableClause, *arrival.ArrivalTableID, roomID, gameID, modeID)
	table, err := scanRoomTable(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) || errors.Is(err, ErrNotFound) {
			return nil, nil
		}
		return nil, err
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM table_seats WHERE table_id = $1`, table.ID); err != nil {
		return nil, err
	}
	return table, nil
}

// seatRegroupClaimantTx seats a claimant the rules say should be seated, preferring the
// seat they name. A caller who already holds a seat is left where they are.
func (s *Store) seatRegroupClaimantTx(ctx context.Context, tx *sql.Tx, table *RoomTable, userID uuid.UUID, preferred string, options []prequeue.Selection) error {
	seatKey, err := s.regroupSeatKeyTx(ctx, tx, table, userID, preferred)
	if err != nil {
		return err
	}
	if seatKey == "" {
		return nil
	}
	if _, err := s.sitAtTableTx(ctx, tx, table.ID, userID, seatKey, options); err != nil {
		return err
	}
	return nil
}

// regroupSeatKeyTx picks the seat a claimant takes: the one they ask for when it is still
// theirs to take, otherwise another seat in the same role, otherwise the first open one.
//
// The preference is what makes a solo replay honest. Falling back within the role first
// matters for a mode whose seats are per-role rather than pooled: "you had a Guesser seat"
// survives even though "you had Guesser-3" did not.
//
// The two "no seat key to take" outcomes are deliberately distinct. A caller who already
// holds a seat here gets ("", nil), so claiming twice never moves anyone. A caller who
// cannot be seated because every seat is taken gets ErrTableFull, because the regroup
// table lives in a pre-existing room whose other members can take its seats through
// SitAtTable.
func (s *Store) regroupSeatKeyTx(ctx context.Context, tx *sql.Tx, table *RoomTable, userID uuid.UUID, preferred string) (string, error) {
	modeSeats, err := listGameModeSeats(ctx, tx, table.ModeID)
	if err != nil {
		return "", err
	}
	seated, err := s.listTableSeatsTx(ctx, tx, table.ID)
	if err != nil {
		return "", err
	}
	taken := make(map[string]bool, len(seated))
	for _, seat := range seated {
		if seat.UserID == userID {
			return "", nil
		}
		taken[seat.SeatKey] = true
	}

	preferred = strings.TrimSpace(preferred)
	preferredPath := ""
	for _, seat := range modeSeats {
		if seat.SeatKey != preferred {
			continue
		}
		if !taken[seat.SeatKey] {
			return seat.SeatKey, nil
		}
		preferredPath = seatQueuePathValue(seat)
	}
	if preferredPath != "" {
		for _, seat := range modeSeats {
			if !taken[seat.SeatKey] && seatQueuePathValue(seat) == preferredPath {
				return seat.SeatKey, nil
			}
		}
	}

	for _, seat := range modeSeats {
		if !taken[seat.SeatKey] {
			return seat.SeatKey, nil
		}
	}
	return "", ErrTableFull
}

// RegroupState is a participant's answer to "playing again?".
type RegroupState string

const (
	RegroupIn      RegroupState = "IN"      // explicitly opted in via playAgain
	RegroupOut     RegroupState = "OUT"     // explicitly declined
	RegroupPending RegroupState = "PENDING" // neither — has not returned or has not chosen
)

// GetRegroupRoster derives each participant's regroup state from explicit markers.
// IN is NOT "seated": resetRoomTableAfterSessionTx re-seats a room-table group the moment
// their match completes, before anyone has chosen. Only regroup_opted_in_at means yes.
func (s *Store) GetRegroupRoster(ctx context.Context, sessionID uuid.UUID) (map[uuid.UUID]RegroupState, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT p.user_id,
		       p.regroup_opted_in_at IS NOT NULL AS opted_in,
		       p.regroup_declined_at IS NOT NULL AS declined
		FROM game_session_participants p
		WHERE p.session_id = $1
	`, sessionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	roster := make(map[uuid.UUID]RegroupState)
	for rows.Next() {
		var (
			userID   uuid.UUID
			optedIn  bool
			declined bool
		)
		if err := rows.Scan(&userID, &optedIn, &declined); err != nil {
			return nil, err
		}
		switch {
		case optedIn:
			roster[userID] = RegroupIn
		case declined:
			roster[userID] = RegroupOut
		default:
			roster[userID] = RegroupPending
		}
	}
	return roster, rows.Err()
}

// RegroupSeatRelease names the table a decline actually freed a seat at, so the caller can
// tell the room's watchers. Everyone sitting at /room/{code} watches tableUpdated, not
// matchResultUpdated, so without this the decliner stays visibly seated until some
// unrelated table event fires.
type RegroupSeatRelease struct {
	TableID uuid.UUID
	RoomID  uuid.UUID
}

// DeclineRegroup records that a participant is not playing again and frees the seat they
// may be holding. A room-table group is re-seated automatically when the match completes,
// so someone who says no is still sitting there — leaving them seated would block the
// king's Look for group backfill from filling the seat.
//
// The returned release is nil when no seat was actually freed (no regroup table yet, or the
// decliner was not sitting at it); a non-nil one is the caller's cue to publish tableUpdated.
func (s *Store) DeclineRegroup(ctx context.Context, sessionID, userID uuid.UUID, at time.Time) (*RegroupSeatRelease, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()

	result, err := tx.ExecContext(ctx, `
		UPDATE game_session_participants
		SET regroup_declined_at = $3, regroup_opted_in_at = NULL
		WHERE session_id = $1 AND user_id = $2
	`, sessionID, userID, at)
	if err != nil {
		return nil, err
	}
	if err := ensureRowsAffected(result, ErrNotFound); err != nil {
		return nil, err
	}

	// Any of this match's regroup tables, not game_sessions.regroup_table_id: that column
	// names the first party's table only, so reading it freed a seat for one group and left
	// every other group's decliner sitting there — visibly IN, blocking the backfill their
	// own king is waiting on (JQ-291). Matching on the seat's owner is exact anyway, since a
	// player holds at most one seat and only ever at their own party's table.
	var release RegroupSeatRelease
	err = tx.QueryRowContext(ctx, `
		DELETE FROM table_seats
		WHERE user_id = $2
		  AND table_id IN (SELECT id FROM room_tables WHERE regroup_session_id = $1)
		RETURNING table_id, (SELECT room_id FROM room_tables WHERE id = table_seats.table_id)
	`, sessionID, userID).Scan(&release.TableID, &release.RoomID)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return nil, err
	}
	freed := err == nil

	if err := tx.Commit(); err != nil {
		return nil, err
	}
	if !freed {
		return nil, nil
	}
	return &release, nil
}

// GetRegroupTableIDForUser returns the regroup table this player's arrival party claimed, or
// nil when their party has no live offer yet.
//
// Per viewer, because a match has one regroup table per arrival party and handing everyone
// the first one stamped is the merge this ticket exists to stop (JQ-291): the invite code on
// the results screen is a link people follow, so pointing the losing group at the winners'
// room puts them in it.
//
// Nil-versus-error is load-bearing and unchanged: a session that does not exist is
// ErrNotFound, a party with no live offer is (nil, nil), and the caller renders no invite
// code — exactly as it does before anyone claims.
func (s *Store) GetRegroupTableIDForUser(ctx context.Context, sessionID, userID uuid.UUID) (*uuid.UUID, error) {
	var participates bool
	if err := s.db.QueryRowContext(ctx, `
		SELECT EXISTS(SELECT 1 FROM game_session_participants WHERE session_id = $1 AND user_id = $2)
	`, sessionID, userID).Scan(&participates); err != nil {
		return nil, err
	}
	if !participates {
		return nil, ErrNotFound
	}

	// "A regroup table of this match, in a room I am in" — room membership is what makes
	// this the viewer's own party without re-reading their return context. ClaimRegroupTable
	// puts every claimant in their party's room (ensureRoomMemberTx) and a group never left
	// the room it queued from, so a member sees their group's table the moment any one of
	// them claims it, and the opposing group — members of a different room — sees nothing.
	//
	// It also keeps answering for a player who arrived alone, whose table is in a room of
	// their own: scoping on return_context.roomId instead would read nil for them and take
	// away the link back to the table they just claimed.
	var id *uuid.UUID
	err := s.db.QueryRowContext(ctx, `
		SELECT t.id
		FROM room_tables t
		INNER JOIN rooms r ON r.id = t.room_id
		INNER JOIN room_members rm ON rm.room_id = t.room_id AND rm.user_id = $2
		WHERE t.regroup_session_id = $1 AND `+liveRegroupTableClause+`
		ORDER BY t.created_at DESC
		LIMIT 1
	`, sessionID, userID).Scan(&id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return id, nil
}
