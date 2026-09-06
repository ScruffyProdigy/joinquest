package store

import (
	"context"
	"database/sql"
	"errors"

	"github.com/google/uuid"
)

// UserActiveIntent is the user's catalog play intent: waiting or matched queue membership.
type UserActiveIntent struct {
	GameID      uuid.UUID
	GameName    string
	ModeQueueID uuid.UUID
	ModeID      uuid.UUID
	ModeName    string
	SeatKey     string
	Waiting     bool
	Matched     bool
	QueuedCount int
	QueuePath   *string
	SessionID   *uuid.UUID
}

// GetUserActiveIntent returns the user's catalog play intent. An in-progress game session
// takes priority over waiting or matched queue rows so the leave-game banner matches
// join blocking (ensureNotInActiveGame).
func (s *Store) GetUserActiveIntent(ctx context.Context, userID uuid.UUID) (*UserActiveIntent, error) {
	if err := s.reconcileStaleMatchedQueuesForUser(ctx, userID); err != nil {
		return nil, err
	}

	participation, err := s.GetUserActiveSessionParticipation(ctx, userID)
	if err != nil {
		return nil, err
	}
	if participation != nil {
		out := &UserActiveIntent{
			GameID:    participation.GameID,
			GameName:  participation.GameName,
			ModeID:    participation.ModeID,
			ModeName:  participation.ModeName,
			SeatKey:   participation.SeatKey,
			Matched:   true,
			SessionID: &participation.SessionID,
		}
		if session, sErr := s.GetSessionByID(ctx, participation.SessionID); sErr == nil && session.ModeQueueID != nil {
			out.ModeQueueID = *session.ModeQueueID
		}
		return out, nil
	}

	if waiting, err := s.getUserWaitingQueueAny(ctx, userID); err != nil {
		return nil, err
	} else if waiting != nil {
		entry, err := s.GetWaitingModeQueueEntry(ctx, waiting.ModeQueueID, userID)
		if err != nil && !errors.Is(err, ErrNotFound) {
			return nil, err
		}
		count, err := s.CountWaitingInModeQueue(ctx, waiting.ModeQueueID)
		if err != nil {
			return nil, err
		}
		game, err := s.GetGameByID(ctx, waiting.GameID)
		if err != nil {
			return nil, err
		}
		out := &UserActiveIntent{
			GameID:      waiting.GameID,
			GameName:    game.Name,
			ModeQueueID: waiting.ModeQueueID,
			Waiting:     true,
			QueuedCount: count,
		}
		if entry != nil {
			out.QueuePath = entry.QueuePath
		}
		return out, nil
	}

	session, modeQueueID, err := s.getUserMatchedModeQueueAny(ctx, userID)
	if err != nil {
		return nil, err
	}
	if session != nil {
		game, err := s.GetGameByID(ctx, session.GameID)
		if err != nil {
			return nil, err
		}
		out := &UserActiveIntent{
			GameID:      session.GameID,
			GameName:    game.Name,
			ModeQueueID: modeQueueID,
			Matched:     true,
			SessionID:   &session.ID,
		}
		if session.ModeID != nil {
			out.ModeID = *session.ModeID
			if mode, mErr := s.GetGameModeByID(ctx, *session.ModeID); mErr == nil {
				out.ModeName = mode.DisplayName
			}
		}
		return out, nil
	}

	return nil, nil
}

func (s *Store) getUserWaitingQueueAny(ctx context.Context, userID uuid.UUID) (*struct {
	GameID      uuid.UUID
	ModeQueueID uuid.UUID
}, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT game_id, mode_queue_id
		FROM game_queues
		WHERE user_id = $1 AND status = 'waiting' AND mode_queue_id IS NOT NULL
		ORDER BY joined_at DESC
		LIMIT 1
	`, userID)
	var gameID uuid.UUID
	var modeQueueID uuid.UUID
	if err := row.Scan(&gameID, &modeQueueID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return &struct {
		GameID      uuid.UUID
		ModeQueueID uuid.UUID
	}{GameID: gameID, ModeQueueID: modeQueueID}, nil
}

func (s *Store) getUserMatchedModeQueueAny(ctx context.Context, userID uuid.UUID) (*Session, uuid.UUID, error) {
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
			return nil, uuid.Nil, nil
		}
		return nil, uuid.Nil, err
	}
	session, err := s.GetMatchedSessionForUserAndModeQueue(ctx, modeQueueID, userID)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			_ = s.cancelUserMatchedModeQueue(ctx, modeQueueID, userID)
			return nil, uuid.Nil, nil
		}
		return nil, uuid.Nil, err
	}
	return session, modeQueueID, nil
}

// reconcileStaleMatchedQueuesForUserTx cancels matched queue rows that no longer have
// an active, unfinished session participation for the same catalog queue.
func (s *Store) reconcileStaleMatchedQueuesForUserTx(ctx context.Context, tx *sql.Tx, userID uuid.UUID) error {
	_, err := tx.ExecContext(ctx, `
		UPDATE game_queues gq
		SET status = 'cancelled'
		WHERE gq.user_id = $1
		  AND gq.status = 'matched'
		  AND gq.mode_queue_id IS NOT NULL
		  AND NOT EXISTS (
		    SELECT 1
		    FROM game_session_participants gsp
		    INNER JOIN game_sessions gs
		      ON gs.id = gsp.session_id
		     AND gs.status = 'active'
		     AND gs.game_id = gq.game_id
		     AND gs.mode_queue_id = gq.mode_queue_id
		    WHERE gsp.user_id = gq.user_id
		      AND gsp.left_at IS NULL
		      AND gsp.finished_at IS NULL
		  )
	`, userID)
	return err
}

func (s *Store) reconcileStaleMatchedQueuesForUser(ctx context.Context, userID uuid.UUID) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err := s.reconcileStaleMatchedQueuesForUserTx(ctx, tx, userID); err != nil {
		return err
	}
	return tx.Commit()
}

func getUserWaitingQueueSwitchInfoTx(ctx context.Context, tx *sql.Tx, userID, exceptModeQueueID uuid.UUID) (*SwitchedFromQueue, error) {
	row := tx.QueryRowContext(ctx, `
		SELECT gq.mode_queue_id, gq.game_id, g.name
		FROM game_queues gq
		INNER JOIN games g ON g.id = gq.game_id
		WHERE gq.user_id = $1 AND gq.status = 'waiting' AND gq.mode_queue_id IS NOT NULL
		  AND gq.mode_queue_id <> $2
		ORDER BY gq.joined_at DESC
		LIMIT 1
	`, userID, exceptModeQueueID)
	var info SwitchedFromQueue
	if err := row.Scan(&info.ModeQueueID, &info.GameID, &info.GameName); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return &info, nil
}

func getUserMatchedModeQueueIDTx(ctx context.Context, tx *sql.Tx, userID, exceptModeQueueID uuid.UUID) (*uuid.UUID, error) {
	row := tx.QueryRowContext(ctx, `
		SELECT mode_queue_id
		FROM game_queues
		WHERE user_id = $1 AND status = 'matched' AND mode_queue_id IS NOT NULL
		  AND mode_queue_id <> $2
		ORDER BY matched_at DESC NULLS LAST, joined_at DESC
		LIMIT 1
	`, userID, exceptModeQueueID)
	var id uuid.UUID
	if err := row.Scan(&id); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return &id, nil
}
