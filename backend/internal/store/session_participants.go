package store

import (
	"context"

	"github.com/google/uuid"
)

// SessionParticipant is a user seated in a lobby session with an assigned seat key.
type SessionParticipant struct {
	UserID      uuid.UUID
	SeatKey     string
	DisplayName string
}

// ListSessionSeatAssignments returns seated users ordered by join time (seat key in role).
func (s *Store) ListSessionSeatAssignments(ctx context.Context, sessionID uuid.UUID) ([]SessionParticipant, error) {
	rows, err := s.db.QueryContext(ctx, `
		-- Display name only. Falling back to username or email would hand a
		-- third-party game an identifier the player never chose to show — an
		-- email address, in the worst case. An empty name is a bug for the
		-- handoff to reject (JQ-124), not a hole for this query to paper over.
		SELECT u.id, COALESCE(NULLIF(p.role, ''), 'player'), COALESCE(NULLIF(u.display_name, ''), '')
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
		if err := rows.Scan(&p.UserID, &p.SeatKey, &p.DisplayName); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}
