package store

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/google/uuid"
)

// ActiveSessionParticipation is the user's seat in an active game session.
type ActiveSessionParticipation struct {
	SessionID uuid.UUID
	GameID    uuid.UUID
	GameName  string
	ModeID    uuid.UUID
	ModeName  string
	SeatKey   string
}

// GetUserActiveSessionParticipation returns the user's in-progress game session, if any.
func (s *Store) GetUserActiveSessionParticipation(ctx context.Context, userID uuid.UUID) (*ActiveSessionParticipation, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT gs.id, gs.game_id, g.name, gm.id, gm.display_name, gsp.role
		FROM game_session_participants gsp
		INNER JOIN game_sessions gs ON gs.id = gsp.session_id AND gs.status = 'active'
		INNER JOIN games g ON g.id = gs.game_id
		INNER JOIN game_modes gm ON gm.id = gs.mode_id
		WHERE gsp.user_id = $1
		  AND gsp.left_at IS NULL
		  AND gsp.finished_at IS NULL
		ORDER BY gs.started_at DESC
		LIMIT 1
	`, userID)
	var view ActiveSessionParticipation
	if err := row.Scan(
		&view.SessionID, &view.GameID, &view.GameName, &view.ModeID, &view.ModeName, &view.SeatKey,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return &view, nil
}

// finishReturnedQueueSessionsForRequeueTx marks catalog queue sessions finished when the
// user already left the matched queue (return hub) but the session is still active.
func finishReturnedQueueSessionsForRequeueTx(ctx context.Context, tx *sql.Tx, userID uuid.UUID) error {
	_, err := tx.ExecContext(ctx, `
		UPDATE game_session_participants gsp
		SET finished_at = NOW()
		FROM game_sessions gs
		WHERE gsp.session_id = gs.id
		  AND gsp.user_id = $1
		  AND gsp.left_at IS NULL
		  AND gsp.finished_at IS NULL
		  AND gs.status = 'active'
		  AND gs.mode_queue_id IS NOT NULL
		  AND NOT EXISTS (
		    SELECT 1
		    FROM game_queues gq
		    WHERE gq.user_id = gsp.user_id
		      AND gq.mode_queue_id = gs.mode_queue_id
		      AND gq.status IN ('waiting', 'matched')
		  )
	`, userID)
	return err
}

func ensureNotInActiveGameTx(ctx context.Context, tx *sql.Tx, userID uuid.UUID) error {
	var exists bool
	if err := tx.QueryRowContext(ctx, `
		SELECT EXISTS(
			SELECT 1
			FROM game_session_participants gsp
			INNER JOIN game_sessions gs ON gs.id = gsp.session_id AND gs.status = 'active'
			WHERE gsp.user_id = $1
			  AND gsp.left_at IS NULL
			  AND gsp.finished_at IS NULL
		)
	`, userID).Scan(&exists); err != nil {
		return err
	}
	if exists {
		return ErrActiveGame
	}
	return nil
}

// ResetRoomTableAfterSession returns a started room table to forming and re-seats players.
func (s *Store) ResetRoomTableAfterSession(ctx context.Context, sessionID uuid.UUID) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err := resetRoomTableAfterSessionTx(ctx, tx, sessionID); err != nil {
		return err
	}
	return tx.Commit()
}

func resetRoomTableAfterSessionTx(ctx context.Context, tx *sql.Tx, sessionID uuid.UUID) error {
	var tableID uuid.UUID
	err := tx.QueryRowContext(ctx, `
		SELECT id
		FROM room_tables
		WHERE session_id = $1 AND status = $2
	`, sessionID, TableStatusStarted).Scan(&tableID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil
		}
		return err
	}

	participants, err := listSessionParticipantsTx(ctx, tx, sessionID)
	if err != nil {
		return err
	}

	if _, err := tx.ExecContext(ctx, `DELETE FROM table_seats WHERE table_id = $1`, tableID); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `
		UPDATE room_tables
		SET status = $2, session_id = NULL, updated_at = NOW()
		WHERE id = $1
	`, tableID, TableStatusForming); err != nil {
		return err
	}

	for _, p := range participants {
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO table_seats (table_id, user_id, seat_key)
			VALUES ($1, $2, $3)
		`, tableID, p.UserID, p.SeatKey); err != nil {
			return err
		}
	}

	return nil
}

func listSessionParticipantsTx(ctx context.Context, tx *sql.Tx, sessionID uuid.UUID) ([]SessionParticipant, error) {
	rows, err := tx.QueryContext(ctx, `
		SELECT u.id, COALESCE(NULLIF(p.role, ''), 'player'), COALESCE(NULLIF(u.display_name, ''), u.username, u.email)
		FROM game_session_participants p
		JOIN users u ON u.id = p.user_id
		WHERE p.session_id = $1 AND p.left_at IS NULL AND p.finished_at IS NULL
		ORDER BY p.joined_at ASC
	`, sessionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []SessionParticipant
	for rows.Next() {
		var p SessionParticipant
		if err := rows.Scan(&p.UserID, &p.SeatKey, &p.DisplayName); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

// LeaveActiveGame clears the user's playing intent before the game reports a result.
func (s *Store) LeaveActiveGame(ctx context.Context, userID uuid.UUID) (*uuid.UUID, error) {
	participation, err := s.GetUserActiveSessionParticipation(ctx, userID)
	if err != nil {
		return nil, err
	}
	if participation == nil {
		return nil, ErrNotFound
	}

	now := time.Now()
	session, err := s.GetSessionByID(ctx, participation.SessionID)
	if err != nil {
		return nil, err
	}

	if err := s.MarkParticipantFinished(ctx, participation.SessionID, userID, now); err != nil {
		return nil, err
	}
	if session.ModeQueueID != nil {
		if err := s.ReleaseUserMatchedQueue(ctx, *session.ModeQueueID, userID); err != nil {
			return nil, err
		}
	}

	remaining, err := s.CountActiveParticipants(ctx, participation.SessionID)
	if err != nil {
		return nil, err
	}
	if remaining == 0 {
		if err := s.CompleteSession(ctx, participation.SessionID, now); err != nil && !errors.Is(err, ErrNotFound) {
			return nil, err
		}
	}

	var tableID uuid.UUID
	if table, tableErr := s.GetRoomTableBySessionID(ctx, participation.SessionID); tableErr == nil && table != nil {
		tableID = table.ID
	}
	return &tableID, nil
}

func (s *Store) GetRoomTableBySessionID(ctx context.Context, sessionID uuid.UUID) (*RoomTable, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT `+roomTableColumns+`
		FROM room_tables
		WHERE session_id = $1
		LIMIT 1
	`, sessionID)
	return scanRoomTable(row)
}
