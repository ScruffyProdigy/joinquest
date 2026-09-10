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
// The catalog mode is joined loosely on purpose: game_sessions.mode_id is nullable, and a
// session that outlives its mode row must still be visible here. Joining it strictly hid
// such sessions from both the intent banner and LeaveActiveGame, stranding the player
// on a playing banner nothing could clear (JQ-134).
func (s *Store) GetUserActiveSessionParticipation(ctx context.Context, userID uuid.UUID) (*ActiveSessionParticipation, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT gs.id, gs.game_id, g.name,
		       COALESCE(gm.id, '00000000-0000-0000-0000-000000000000'::uuid),
		       COALESCE(gm.display_name, ''),
		       gsp.role
		FROM game_session_participants gsp
		INNER JOIN game_sessions gs ON gs.id = gsp.session_id AND gs.status = 'active'
		INNER JOIN games g ON g.id = gs.game_id
		LEFT JOIN game_modes gm ON gm.id = gs.mode_id
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

	var modeID uuid.UUID
	if err := tx.QueryRowContext(ctx, `
		SELECT mode_id FROM room_tables WHERE id = $1
	`, tableID).Scan(&modeID); err != nil {
		return err
	}
	mode, err := getGameModeByID(ctx, tx, modeID)
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

	// Re-seating a returning group pre-empts the choice they came back to make: who
	// plays spymaster this round, which character to bring. So it happens only where
	// there is nothing to choose at all — one seat class and no pre-queue options — and
	// is then pure convenience (JQ-232). Everywhere else the table comes back empty and
	// every returner appears on the "Picking a seat" card until they answer.
	if !ModeOffersPreMatchChoice(mode) {
		for _, p := range participants {
			if _, err := tx.ExecContext(ctx, `
				INSERT INTO table_seats (table_id, user_id, seat_key)
				VALUES ($1, $2, $3)
			`, tableID, p.UserID, p.SeatKey); err != nil {
				return err
			}
		}
	}

	// A room-table group regroups at the table they already have, so record it as the
	// one table the finished match converges on (JQ-135). Both directions are stamped:
	// the table needs to name its originating match without reading game_sessions
	// backwards on every render, and a room's persistent table is reused match after
	// match, so the newer stamp replacing the older is exactly the wanted answer (JQ-177).
	if _, err := tx.ExecContext(ctx, `
		UPDATE game_sessions SET regroup_table_id = $2 WHERE id = $1
	`, sessionID, tableID); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `
		UPDATE room_tables SET regroup_session_id = $2 WHERE id = $1
	`, tableID, sessionID); err != nil {
		return err
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
// It returns ErrNotFound only when the user had nothing to leave.
func (s *Store) LeaveActiveGame(ctx context.Context, userID uuid.UUID) (*uuid.UUID, error) {
	participation, err := s.GetUserActiveSessionParticipation(ctx, userID)
	if err != nil {
		return nil, err
	}
	if participation == nil {
		return s.leaveMatchedModeQueueWithoutSession(ctx, userID)
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

// leaveMatchedModeQueueWithoutSession unwinds a matched game_queues row that has no
// session participation behind it. GetUserActiveIntent reports such a row as MATCHED,
// so without this the leave-game button had nothing to act on (JQ-134).
func (s *Store) leaveMatchedModeQueueWithoutSession(ctx context.Context, userID uuid.UUID) (*uuid.UUID, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT mode_queue_id
		FROM game_queues
		WHERE user_id = $1 AND status = 'matched' AND mode_queue_id IS NOT NULL
		ORDER BY matched_at DESC NULLS LAST, joined_at DESC
		LIMIT 1
	`, userID)
	var modeQueueID uuid.UUID
	if err := row.Scan(&modeQueueID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	if err := s.ReleaseUserMatchedQueue(ctx, modeQueueID, userID); err != nil {
		return nil, err
	}
	var tableID uuid.UUID
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
