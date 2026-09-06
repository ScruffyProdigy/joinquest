package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"time"

	"github.com/google/uuid"
)

// MatchParticipantResult is one player's outcome in a finished match.
type MatchParticipantResult struct {
	UserID      uuid.UUID
	DisplayName string
	Role        string
	FinishedAt  *time.Time
	LeftAt      *time.Time
	Reason      *string
	Placement   *int
	IsWinner    bool
}

// MatchResult is the stored outcome of a match, as reported by the game.
type MatchResult struct {
	SessionID     uuid.UUID
	GameID        uuid.UUID
	Status        *string
	Metadata      json.RawMessage
	ReportedAt    *time.Time
	SessionStatus string
	EndedAt       *time.Time
	Participants  []MatchParticipantResult
}

// marshalMetadata returns a driver value suitable for a nullable JSONB column.
// It must return a bare untyped nil (not a nil []byte) so lib/pq sends SQL NULL
// instead of an empty string, which Postgres rejects as invalid JSON.
func marshalMetadata(metadata map[string]any) (any, error) {
	if metadata == nil {
		return nil, nil
	}
	return json.Marshal(metadata)
}

// RecordPlayerFinish stores what the game reported about one player finishing.
// Side effects (marking finished, releasing queue rows) stay in match_lifecycle.go.
func (s *Store) RecordPlayerFinish(ctx context.Context, sessionID, userID uuid.UUID, reason string, placement *int, metadata map[string]any) error {
	raw, err := marshalMetadata(metadata)
	if err != nil {
		return err
	}
	result, err := s.db.ExecContext(ctx, `
		UPDATE game_session_participants
		SET finish_reason = $3, placement = $4, finish_metadata = $5
		WHERE session_id = $1 AND user_id = $2
	`, sessionID, userID, reason, placement, raw)
	if err != nil {
		return err
	}
	return ensureRowsAffected(result, ErrNotFound)
}

// RecordMatchResult stores the match outcome and flags winners. Winner ids that do not
// belong to the session are silently ignored — the UPDATE simply matches no rows.
func (s *Store) RecordMatchResult(ctx context.Context, sessionID uuid.UUID, status string, winnerUserIDs []uuid.UUID, metadata map[string]any, at time.Time) error {
	raw, err := marshalMetadata(metadata)
	if err != nil {
		return err
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	if _, err := tx.ExecContext(ctx, `
		UPDATE game_sessions
		SET result_status = $2, result_metadata = $3, result_reported_at = $4
		WHERE id = $1
	`, sessionID, status, raw, at); err != nil {
		return err
	}

	if _, err := tx.ExecContext(ctx, `
		UPDATE game_session_participants SET is_winner = false WHERE session_id = $1
	`, sessionID); err != nil {
		return err
	}

	for _, winnerID := range winnerUserIDs {
		if _, err := tx.ExecContext(ctx, `
			UPDATE game_session_participants
			SET is_winner = true
			WHERE session_id = $1 AND user_id = $2
		`, sessionID, winnerID); err != nil {
			return err
		}
	}

	return tx.Commit()
}

// GetMatchResult loads the stored outcome plus the full participant roster.
func (s *Store) GetMatchResult(ctx context.Context, sessionID uuid.UUID) (*MatchResult, error) {
	var out MatchResult
	out.SessionID = sessionID
	// Scan the nullable jsonb column into a plain []byte: json.RawMessage is a named
	// slice type, so database/sql's NULL-to-*[]byte fast path doesn't recognize it and
	// scanning NULL directly into *json.RawMessage fails with "unsupported Scan".
	var metadata []byte
	err := s.db.QueryRowContext(ctx, `
		SELECT game_id, result_status, result_metadata, result_reported_at, status, ended_at
		FROM game_sessions
		WHERE id = $1
	`, sessionID).Scan(&out.GameID, &out.Status, &metadata, &out.ReportedAt, &out.SessionStatus, &out.EndedAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	out.Metadata = json.RawMessage(metadata)

	rows, err := s.db.QueryContext(ctx, `
		SELECT p.user_id,
		       COALESCE(NULLIF(u.display_name, ''), u.username, u.email, ''),
		       COALESCE(NULLIF(p.role, ''), 'player'),
		       p.finished_at, p.left_at, p.finish_reason, p.placement, p.is_winner
		FROM game_session_participants p
		JOIN users u ON u.id = p.user_id
		WHERE p.session_id = $1
		ORDER BY p.placement NULLS LAST, p.joined_at ASC
	`, sessionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var p MatchParticipantResult
		if err := rows.Scan(
			&p.UserID, &p.DisplayName, &p.Role,
			&p.FinishedAt, &p.LeftAt, &p.Reason, &p.Placement, &p.IsWinner,
		); err != nil {
			return nil, err
		}
		out.Participants = append(out.Participants, p)
	}
	return &out, rows.Err()
}
