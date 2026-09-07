package store

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/google/uuid"
)

// ParticipantIsActive reports whether the user is a seated, not-left participant.
func (s *Store) ParticipantIsActive(ctx context.Context, sessionID, userID uuid.UUID) error {
	var one int
	err := s.db.QueryRowContext(ctx, `
		SELECT 1
		FROM game_session_participants
		WHERE session_id = $1 AND user_id = $2 AND left_at IS NULL
	`, sessionID, userID).Scan(&one)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ErrNotFound
		}
		return err
	}
	return nil
}

// GetParticipantReturnContext loads per-player return routing for a session.
func (s *Store) GetParticipantReturnContext(ctx context.Context, sessionID, userID uuid.UUID) (ReturnContext, error) {
	var raw []byte
	err := s.db.QueryRowContext(ctx, `
		SELECT return_context
		FROM game_session_participants
		WHERE session_id = $1 AND user_id = $2 AND left_at IS NULL
	`, sessionID, userID).Scan(&raw)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ReturnContext{}, ErrNotFound
		}
		return ReturnContext{}, err
	}
	return decodeReturnContext(raw)
}

// MarkParticipantFinished records that a player is done with the match for themselves.
func (s *Store) MarkParticipantFinished(ctx context.Context, sessionID, userID uuid.UUID, finishedAt time.Time) error {
	result, err := s.db.ExecContext(ctx, `
		UPDATE game_session_participants
		SET finished_at = $3
		WHERE session_id = $1 AND user_id = $2 AND left_at IS NULL AND finished_at IS NULL
	`, sessionID, userID, finishedAt)
	if err != nil {
		return err
	}
	return ensureRowsAffected(result, ErrNotFound)
}

// AcknowledgePlayerReturn clears the user's matched queue row when they land on the
// return hub after a game. Complements reportMatchResult from the game server.
func (s *Store) AcknowledgePlayerReturn(ctx context.Context, sessionID, userID uuid.UUID, returnedAt time.Time) error {
	session, err := s.GetSessionByID(ctx, sessionID)
	if err != nil {
		return err
	}
	if err := s.MarkParticipantFinished(ctx, sessionID, userID, returnedAt); err != nil && !errors.Is(err, ErrNotFound) {
		return err
	}
	if session.ModeQueueID != nil {
		if err := s.ReleaseUserMatchedQueue(ctx, *session.ModeQueueID, userID); err != nil {
			return err
		}
	}
	return nil
}

// ReleaseUserMatchedQueue clears the user's matched row for a mode queue so they can queue again.
func (s *Store) ReleaseUserMatchedQueue(ctx context.Context, modeQueueID, userID uuid.UUID) error {
	_, err := s.db.ExecContext(ctx, `
		UPDATE game_queues
		SET status = 'cancelled'
		WHERE mode_queue_id = $1 AND user_id = $2 AND status = 'matched'
	`, modeQueueID, userID)
	return err
}

// CompleteSession marks a session ended and releases matched queue rows for all seated players.
func (s *Store) CompleteSession(ctx context.Context, sessionID uuid.UUID, endedAt time.Time) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	if err := completeSessionTx(ctx, tx, sessionID, endedAt); err != nil {
		return err
	}

	return tx.Commit()
}

// completeSessionTx is the whole completion — status, participant queue-row cancellation
// and room-table reset — inside a caller's transaction.
//
// Every path that ends a session goes through here. A path that flipped only the status
// left its participants' `matched` game_queues rows behind with no live session, which is
// exactly the orphan condition JQ-133's sweep exists to clear — and a raw partial UPDATE
// in a CronJob was manufacturing them on a schedule (JQ-171).
//
// Returns ErrNotFound when the session is not 'active', so a caller racing another
// completion can tell "already done" from a real failure.
func completeSessionTx(ctx context.Context, tx *sql.Tx, sessionID uuid.UUID, endedAt time.Time) error {
	var modeQueueID sql.NullString
	err := tx.QueryRowContext(ctx, `
		UPDATE game_sessions
		SET status = 'completed', ended_at = $2
		WHERE id = $1 AND status = 'active'
		RETURNING mode_queue_id
	`, sessionID, endedAt).Scan(&modeQueueID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ErrNotFound
		}
		return err
	}

	if modeQueueID.Valid {
		mqID, parseErr := uuid.Parse(modeQueueID.String)
		if parseErr != nil {
			return parseErr
		}
		rows, qErr := tx.QueryContext(ctx, `
			SELECT user_id FROM game_session_participants
			WHERE session_id = $1 AND left_at IS NULL
		`, sessionID)
		if qErr != nil {
			return qErr
		}
		var participantIDs []uuid.UUID
		for rows.Next() {
			var uid uuid.UUID
			if scanErr := rows.Scan(&uid); scanErr != nil {
				_ = rows.Close()
				return scanErr
			}
			participantIDs = append(participantIDs, uid)
		}
		if err := rows.Close(); err != nil {
			return err
		}
		if err := rows.Err(); err != nil {
			return err
		}
		for _, uid := range participantIDs {
			if _, execErr := tx.ExecContext(ctx, `
				UPDATE game_queues
				SET status = 'cancelled'
				WHERE mode_queue_id = $1 AND user_id = $2 AND status = 'matched'
			`, mqID, uid); execErr != nil {
				return execErr
			}
		}
	}

	return resetRoomTableAfterSessionTx(ctx, tx, sessionID)
}

// completeEmptiedSessionsForUserTx completes the user's active sessions that no longer
// hold anyone — every seated player has finished or left.
//
// This is what re-queueing needs, and it is deliberately not "end the other sessions on
// this mode queue". A queue is catalog configuration shared by every match on a mode, so
// keying on it ended other players' live games (JQ-171); a session emptying out is a fact
// about that session alone. A partner still playing keeps their match, and the session
// completes on whoever leaves last.
func completeEmptiedSessionsForUserTx(ctx context.Context, tx *sql.Tx, userID uuid.UUID, endedAt time.Time) error {
	rows, err := tx.QueryContext(ctx, `
		SELECT DISTINCT gs.id
		FROM game_sessions gs
		INNER JOIN game_session_participants gsp ON gsp.session_id = gs.id
		WHERE gsp.user_id = $1
		  AND gs.status = 'active'
		  AND NOT EXISTS (
		    SELECT 1
		    FROM game_session_participants other
		    WHERE other.session_id = gs.id
		      AND other.left_at IS NULL
		      AND other.finished_at IS NULL
		  )
	`, userID)
	if err != nil {
		return err
	}
	var sessionIDs []uuid.UUID
	for rows.Next() {
		var id uuid.UUID
		if scanErr := rows.Scan(&id); scanErr != nil {
			_ = rows.Close()
			return scanErr
		}
		sessionIDs = append(sessionIDs, id)
	}
	if err := rows.Close(); err != nil {
		return err
	}
	if err := rows.Err(); err != nil {
		return err
	}

	for _, id := range sessionIDs {
		if err := completeSessionTx(ctx, tx, id, endedAt); err != nil && !errors.Is(err, ErrNotFound) {
			return err
		}
	}
	return nil
}

// CountActiveParticipants returns seated players who have not finished individually.
func (s *Store) CountActiveParticipants(ctx context.Context, sessionID uuid.UUID) (int, error) {
	var n int
	err := s.db.QueryRowContext(ctx, `
		SELECT COUNT(*)
		FROM game_session_participants
		WHERE session_id = $1 AND left_at IS NULL AND finished_at IS NULL
	`, sessionID).Scan(&n)
	return n, err
}
