package store

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/google/uuid"
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

// ClaimRegroupTable returns the single forming table for a finished match, creating it on
// first call, and seats the caller. The SELECT ... FOR UPDATE is what makes every player
// converge on one table instead of each creating their own.
func (s *Store) ClaimRegroupTable(ctx context.Context, sessionID, userID uuid.UUID) (*RoomTable, *Room, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, nil, err
	}
	defer func() { _ = tx.Rollback() }()

	var (
		status    string
		gameID    *uuid.UUID
		modeID    *uuid.UUID
		regroupID *uuid.UUID
	)
	err = tx.QueryRowContext(ctx, `
		SELECT status, game_id, mode_id, regroup_table_id
		FROM game_sessions
		WHERE id = $1
		FOR UPDATE
	`, sessionID).Scan(&status, &gameID, &modeID, &regroupID)
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

	table, err := s.loadFormingRegroupTableTx(ctx, tx, regroupID)
	if err != nil {
		return nil, nil, err
	}
	if table == nil {
		room, err := s.getUserRoomTx(ctx, tx, userID)
		if err != nil {
			if !errors.Is(err, ErrNotFound) {
				return nil, nil, err
			}
			room, err = s.createRoomTx(ctx, tx, userID)
			if err != nil {
				return nil, nil, err
			}
		}
		table, err = s.createTableTx(ctx, tx, room.ID, *gameID, *modeID)
		if err != nil {
			return nil, nil, err
		}
		if _, err := tx.ExecContext(ctx, `
			UPDATE game_sessions SET regroup_table_id = $2 WHERE id = $1
		`, sessionID, table.ID); err != nil {
			return nil, nil, err
		}
	}

	if err := s.ensureRoomMemberTx(ctx, tx, table.RoomID, userID); err != nil {
		return nil, nil, err
	}

	// A full table is an error, not a silent seatless opt-in: the stamp below must only
	// ever mark a real seat holder, or the roster reads IN for someone the king cannot
	// actually start with.
	seatKey, err := s.firstOpenSeatKeyTx(ctx, tx, table, userID)
	if err != nil {
		return nil, nil, err
	}
	if seatKey != "" {
		// A regroup seats the player where they already were; they re-open the
		// picker themselves before the table starts.
		if _, err := s.sitAtTableTx(ctx, tx, table.ID, userID, seatKey, nil); err != nil {
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

// loadFormingRegroupTableTx returns the claimed table only if it still exists, is forming,
// and lives in a room that is still open. A swept or started table reads as unclaimed so
// the caller creates a fresh one.
//
// The room-status join is load-bearing, not defensive. leaveRoomTx closes a room once the
// last member leaves but the table survives, and regroup_table_id still points at it.
// Adopting that table would insert the claimant into a closed room, and sitAtTableTx's
// isRoomMemberTx requires r.status = open — so the claim fails with ErrNotFound, which
// regroupClientError reports as "you did not play in this match", permanently, with no
// path to a fresh table. Treating it as unclaimed makes the next claim build a new one.
func (s *Store) loadFormingRegroupTableTx(ctx context.Context, tx *sql.Tx, regroupID *uuid.UUID) (*RoomTable, error) {
	if regroupID == nil {
		return nil, nil
	}
	row := tx.QueryRowContext(ctx, `
		SELECT `+roomTableColumnsT+`
		FROM room_tables t
		INNER JOIN rooms r ON r.id = t.room_id
		WHERE t.id = $1 AND t.status = $2 AND r.status = $3
	`, *regroupID, TableStatusForming, RoomStatusOpen)
	table, err := scanRoomTable(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) || errors.Is(err, ErrNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return table, nil
}

// firstOpenSeatKeyTx picks the first unoccupied seat key.
//
// The two "no seat key to take" outcomes are deliberately distinct. A caller who already
// holds a seat here gets ("", nil) — a room-table group is re-seated by
// resetRoomTableAfterSessionTx, so opting in must not move anyone. A caller who cannot be
// seated because every seat is taken gets ErrTableFull, because the regroup table lives in
// a pre-existing room whose other members can take its seats through SitAtTable.
func (s *Store) firstOpenSeatKeyTx(ctx context.Context, tx *sql.Tx, table *RoomTable, userID uuid.UUID) (string, error) {
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

	var release RegroupSeatRelease
	err = tx.QueryRowContext(ctx, `
		DELETE FROM table_seats
		WHERE user_id = $2
		  AND table_id = (SELECT regroup_table_id FROM game_sessions WHERE id = $1)
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

// GetRegroupTableID returns the claimed regroup table, or nil when none exists yet.
func (s *Store) GetRegroupTableID(ctx context.Context, sessionID uuid.UUID) (*uuid.UUID, error) {
	var id *uuid.UUID
	err := s.db.QueryRowContext(ctx, `
		SELECT regroup_table_id FROM game_sessions WHERE id = $1
	`, sessionID).Scan(&id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return id, nil
}

// GetSessionIDByRegroupTable is the reverse of GetRegroupTableID: given a table, find the
// finished match it originated from. Returns nil, not an error, when no session points at
// this table — an ordinary table (created directly, never reached via playAgain) is the
// normal case, not a failure.
//
// regroup_table_id is not unique and is never cleared, so a room that plays more than once
// at its persistent table accumulates one matching row per match: CompleteSession stamps
// the finished session via resetRoomTableAfterSessionTx without touching the previous
// ones. The most recently started match is the one this table is regrouping from, so the
// ordering is load-bearing — without it Postgres is free to return the oldest row, which
// would name players who already left and omit anyone who backfilled since. The id
// tiebreak only keeps the answer stable if two sessions somehow share a started_at.
func (s *Store) GetSessionIDByRegroupTable(ctx context.Context, tableID uuid.UUID) (*uuid.UUID, error) {
	var id uuid.UUID
	err := s.db.QueryRowContext(ctx, `
		SELECT id
		FROM game_sessions
		WHERE regroup_table_id = $1
		ORDER BY started_at DESC NULLS LAST, id DESC
		LIMIT 1
	`, tableID).Scan(&id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return &id, nil
}
