package store

import (
	"context"

	"github.com/google/uuid"
	"github.com/scruffyprodigy/joinquest/internal/prequeue"
)

// SessionParticipant is a user seated in a lobby session with an assigned seat key.
type SessionParticipant struct {
	UserID      uuid.UUID
	SeatKey     string
	DisplayName string
	// QueueOptions is what this player picked before queueing or sitting down.
	// It rides to the game in the provision payload so the match can be set up
	// with the loadout, champion or deck the player actually chose.
	QueueOptions []prequeue.Selection
}

// ListSessionSeatAssignments returns seated users ordered by join time (seat key in role).
func (s *Store) ListSessionSeatAssignments(ctx context.Context, sessionID uuid.UUID) ([]SessionParticipant, error) {
	rows, err := s.db.QueryContext(ctx, `
		-- Display name only. Falling back to username or email would hand a
		-- third-party game an identifier the player never chose to show — an
		-- email address, in the worst case. An empty name is a bug for the
		-- handoff to reject (JQ-124), not a hole for this query to paper over.
		SELECT u.id, COALESCE(NULLIF(p.role, ''), 'player'), COALESCE(NULLIF(u.display_name, ''), ''), p.queue_options
		FROM game_session_participants p
		JOIN users u ON u.id = p.user_id
		WHERE p.session_id = $1 AND p.left_at IS NULL
		ORDER BY p.joined_at ASC
	`, sessionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []SessionParticipant
	for rows.Next() {
		var p SessionParticipant
		var queueOptions []byte
		if err := rows.Scan(&p.UserID, &p.SeatKey, &p.DisplayName, &queueOptions); err != nil {
			return nil, err
		}
		selections, err := decodeQueueOptions(queueOptions)
		if err != nil {
			return nil, err
		}
		p.QueueOptions = selections
		out = append(out, p)
	}
	return out, rows.Err()
}
