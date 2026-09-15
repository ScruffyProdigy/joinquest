package store

import (
	"context"

	"github.com/google/uuid"
)

// SetSessionParticipantLaunchURLBases persists game-minted launch URL bases (no JWT) per seated user.
func (s *Store) SetSessionParticipantLaunchURLBases(ctx context.Context, sessionID uuid.UUID, bases map[uuid.UUID]string) error {
	if len(bases) == 0 {
		return nil
	}
	for userID, base := range bases {
		if _, err := s.db.ExecContext(ctx, `
			UPDATE game_session_participants
			SET launch_url_base = $3
			WHERE session_id = $1 AND user_id = $2 AND left_at IS NULL
		`, sessionID, userID, base); err != nil {
			return err
		}
	}

	// Provisioning is recorded here, at the write that completes it, and NOT at
	// SessionProvisionComplete (JQ-143). That function reads as the obvious hook and is
	// the wrong one: it is a predicate, polled on every provision retry and again on
	// every handoff finalize, so emitting there would count one event per CHECK rather
	// than one per provisioning and silently inflate the funnel.
	s.recordMatchProvisioned(sessionID, len(bases))
	return nil
}

// GetSessionParticipantLaunchURLBase returns a stored game-minted URL base for a seated user.
func (s *Store) GetSessionParticipantLaunchURLBase(ctx context.Context, sessionID, userID uuid.UUID) (string, error) {
	var base *string
	err := s.db.QueryRowContext(ctx, `
		SELECT launch_url_base
		FROM game_session_participants
		WHERE session_id = $1 AND user_id = $2 AND left_at IS NULL
	`, sessionID, userID).Scan(&base)
	if err != nil {
		return "", err
	}
	if base == nil {
		return "", nil
	}
	// The closest the platform gets to "the player launched", and no closer: a URL
	// handed over is not a URL opened, and an opened URL is not a game entered
	// (JQ-143). Nothing downstream may read this as confirmed entry.
	s.recordLaunchURLRequested(sessionID, userID)
	return *base, nil
}

// ListSessionParticipantLaunchURLBases returns stored URL bases for all seated users in a session.
func (s *Store) ListSessionParticipantLaunchURLBases(ctx context.Context, sessionID uuid.UUID) (map[uuid.UUID]string, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT user_id, launch_url_base
		FROM game_session_participants
		WHERE session_id = $1 AND left_at IS NULL AND launch_url_base IS NOT NULL
	`, sessionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make(map[uuid.UUID]string)
	for rows.Next() {
		var userID uuid.UUID
		var base string
		if err := rows.Scan(&userID, &base); err != nil {
			return nil, err
		}
		out[userID] = base
	}
	return out, rows.Err()
}
