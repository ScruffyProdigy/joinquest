package store

import (
	"context"

	"github.com/google/uuid"
)

// UnprovisionedSession is a matched session waiting for game-server provision.
type UnprovisionedSession struct {
	SessionID     uuid.UUID
	GameID        uuid.UUID
	ModeQueueID   uuid.UUID
	NotifyUserIDs []uuid.UUID
}

// ListSessionsNeedingProvision returns active sessions with seated players missing launch URL bases.
func (s *Store) ListSessionsNeedingProvision(ctx context.Context) ([]UnprovisionedSession, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT gs.id, gs.game_id, gs.mode_queue_id
		FROM game_sessions gs
		WHERE gs.status = 'active'
		  AND gs.mode_queue_id IS NOT NULL
		  AND EXISTS (
		    SELECT 1
		    FROM game_session_participants p
		    WHERE p.session_id = gs.id
		      AND p.left_at IS NULL
		      AND (p.launch_url_base IS NULL OR btrim(p.launch_url_base) = '')
		  )
		ORDER BY gs.started_at ASC
	`)
	if err != nil {
		return nil, err
	}

	type pendingSession struct {
		sessionID   uuid.UUID
		gameID      uuid.UUID
		modeQueueID uuid.UUID
	}
	var pending []pendingSession
	for rows.Next() {
		var item pendingSession
		if err := rows.Scan(&item.sessionID, &item.gameID, &item.modeQueueID); err != nil {
			_ = rows.Close()
			return nil, err
		}
		pending = append(pending, item)
	}
	if err := rows.Close(); err != nil {
		return nil, err
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	var out []UnprovisionedSession
	for _, item := range pending {
		notifyIDs, err := s.listMatchedQueueUserIDsForSession(ctx, item.sessionID, item.modeQueueID)
		if err != nil {
			return nil, err
		}
		out = append(out, UnprovisionedSession{
			SessionID:     item.sessionID,
			GameID:        item.gameID,
			ModeQueueID:   item.modeQueueID,
			NotifyUserIDs: notifyIDs,
		})
	}
	return out, nil
}

func (s *Store) listMatchedQueueUserIDsForSession(ctx context.Context, sessionID, modeQueueID uuid.UUID) ([]uuid.UUID, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT DISTINCT q.user_id
		FROM game_queues q
		INNER JOIN game_session_participants p
			ON p.user_id = q.user_id
			AND p.session_id = $1
			AND p.left_at IS NULL
		WHERE q.mode_queue_id = $2
		  AND q.status = 'matched'
	`, sessionID, modeQueueID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var ids []uuid.UUID
	for rows.Next() {
		var id uuid.UUID
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if len(ids) > 0 {
		return ids, nil
	}

	rows2, err := s.db.QueryContext(ctx, `
		SELECT user_id
		FROM game_session_participants
		WHERE session_id = $1 AND left_at IS NULL
		ORDER BY joined_at ASC
	`, sessionID)
	if err != nil {
		return nil, err
	}
	defer rows2.Close()
	for rows2.Next() {
		var id uuid.UUID
		if err := rows2.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows2.Err()
}

// SessionProvisionComplete reports whether every seated participant has a launch URL base stored.
func (s *Store) SessionProvisionComplete(ctx context.Context, sessionID uuid.UUID) (bool, error) {
	var pending int
	err := s.db.QueryRowContext(ctx, `
		SELECT COUNT(*)
		FROM game_session_participants
		WHERE session_id = $1
		  AND left_at IS NULL
		  AND (launch_url_base IS NULL OR btrim(launch_url_base) = '')
	`, sessionID).Scan(&pending)
	if err != nil {
		return false, err
	}
	if pending > 0 {
		return false, nil
	}
	var seated int
	err = s.db.QueryRowContext(ctx, `
		SELECT COUNT(*)
		FROM game_session_participants
		WHERE session_id = $1 AND left_at IS NULL
	`, sessionID).Scan(&seated)
	if err != nil {
		return false, err
	}
	return seated > 0, nil
}
