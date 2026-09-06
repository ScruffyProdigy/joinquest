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
		if _, err := s.sitAtTableTx(ctx, tx, table.ID, userID, seatKey); err != nil {
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

// loadFormingRegroupTableTx returns the claimed table only if it still exists and is
// forming. A swept or started table reads as unclaimed so the caller creates a fresh one.
func (s *Store) loadFormingRegroupTableTx(ctx context.Context, tx *sql.Tx, regroupID *uuid.UUID) (*RoomTable, error) {
	if regroupID == nil {
		return nil, nil
	}
	row := tx.QueryRowContext(ctx, `
		SELECT `+roomTableColumns+`
		FROM room_tables
		WHERE id = $1 AND status = $2
	`, *regroupID, TableStatusForming)
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

// DeclineRegroup records that a participant is not playing again and frees the seat they
// may be holding. A room-table group is re-seated automatically when the match completes,
// so someone who says no is still sitting there — leaving them seated would block the
// king's Look for group backfill from filling the seat.
func (s *Store) DeclineRegroup(ctx context.Context, sessionID, userID uuid.UUID, at time.Time) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	result, err := tx.ExecContext(ctx, `
		UPDATE game_session_participants
		SET regroup_declined_at = $3, regroup_opted_in_at = NULL
		WHERE session_id = $1 AND user_id = $2
	`, sessionID, userID, at)
	if err != nil {
		return err
	}
	if err := ensureRowsAffected(result, ErrNotFound); err != nil {
		return err
	}

	if _, err := tx.ExecContext(ctx, `
		DELETE FROM table_seats
		WHERE user_id = $2
		  AND table_id = (SELECT regroup_table_id FROM game_sessions WHERE id = $1)
	`, sessionID, userID); err != nil {
		return err
	}

	return tx.Commit()
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
