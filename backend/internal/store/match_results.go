package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"log"
	"time"

	"github.com/google/uuid"

	"github.com/scruffyprodigy/joinquest/internal/prequeue"
	"github.com/scruffyprodigy/joinquest/internal/rating"
	"github.com/scruffyprodigy/joinquest/internal/seattemplate"
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
	SessionID uuid.UUID
	GameID    uuid.UUID
	// ModeID is the catalog mode the session was played in. Nullable in the schema, and
	// the row it points at may since have been removed, so callers must handle both.
	ModeID        *uuid.UUID
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

	if err := s.appendRatingInputForResultTx(ctx, tx, sessionID, status, winnerUserIDs, at); err != nil {
		return err
	}

	return tx.Commit()
}

// appendRatingInputForResultTx records a rating input for a just-finalized
// match, in the same transaction as the result write. An outcome that cannot
// be rated (an abandoned match with no winners or placements, a session whose
// mode or seat template no longer resolves, and so on) is a normal occurrence
// — it is logged and skipped, never treated as a failure of the transaction:
// the game server has already applied its result report, and failing here
// would make it retry a write that has already landed.
func (s *Store) appendRatingInputForResultTx(ctx context.Context, tx *sql.Tx, sessionID uuid.UUID, status string, winnerUserIDs []uuid.UUID, at time.Time) error {
	var (
		gameID       uuid.UUID
		modeKey      sql.NullString
		seatTemplate []byte
		socialMode   sql.NullString
	)
	err := tx.QueryRowContext(ctx, `
		SELECT gs.game_id, gm.mode_key, gm.seat_template, gm.social_mode
		FROM game_sessions gs
		LEFT JOIN game_modes gm ON gm.id = gs.mode_id
		WHERE gs.id = $1
	`, sessionID).Scan(&gameID, &modeKey, &seatTemplate, &socialMode)
	if err != nil {
		return err
	}

	// No mode row (mode_id was never set, or the mode has since been deleted)
	// or no seat template means there is nothing to build a ModeShape from.
	if !modeKey.Valid || len(seatTemplate) == 0 {
		log.Printf("rating: session %s has no mode or seat template; skipping rating input", sessionID)
		return nil
	}

	leaves, err := seattemplate.Expand(seatTemplate)
	if err != nil {
		log.Printf("rating: session %s seat template did not expand: %v; skipping rating input", sessionID, err)
		return nil
	}
	seatClasses := make(map[string]string, len(leaves))
	affinityBySeat := make(map[string]string, len(leaves))
	for _, leaf := range leaves {
		seatClasses[leaf.SeatKey] = leaf.NamePath
		affinityBySeat[leaf.SeatKey] = leaf.AffinityKey
	}

	cooperative := socialMode.Valid && socialMode.String == "co-op"
	shape := rating.ModeShape{SeatClasses: seatClasses, Cooperative: cooperative}

	rows, err := tx.QueryContext(ctx, `
		SELECT p.user_id, p.role, p.placement, p.is_winner, p.queue_options
		FROM game_session_participants p
		WHERE p.session_id = $1
	`, sessionID)
	if err != nil {
		return err
	}
	defer rows.Close()

	var participants []rating.Participant
	// queueOptionsByPlayer captures, per participant, exactly what they had
	// stored in game_session_participants.queue_options at match time — the
	// pre-queue picks that were locked in before matchmaking, as opposed to
	// anything decided afterward. It rides into RatingInput.QueueOptions as a
	// JSON object keyed by player id (the raw user id, not the "player:"
	// prefixed form used in RatingEntrantRow.Key), with each value being that
	// participant's stored selections array, unchanged from the column's own
	// shape: [{"groupKey":"...","optionIds":[...]}, ...]. It is retained for
	// backtesting, not read by the engine today — see migration 000055.
	queueOptionsByPlayer := make(map[string]json.RawMessage)
	for rows.Next() {
		var (
			userID       uuid.UUID
			role         sql.NullString
			placement    *int
			isWinner     bool
			queueOptions []byte
		)
		if err := rows.Scan(&userID, &role, &placement, &isWinner, &queueOptions); err != nil {
			return err
		}
		// role is nullable (migration 000001). NULL or empty means this
		// participant has no seat key; leave SeatKey empty rather than
		// inventing one — unlike session_participants.go's COALESCE to
		// 'player', a literal seat key here would match no seat class and
		// be worse than none. outcome.go already tolerates a seat key
		// absent from ModeShape.SeatClasses.
		seatKey := role.String

		selections, err := decodeQueueOptions(queueOptions)
		if err != nil {
			// Unreachable today (queue_options is NOT NULL DEFAULT '[]' and
			// every writer goes through encodeQueueOptions), but this is an
			// unrateable-match condition like the others in this function,
			// not a transaction failure: log and skip the rating input so
			// the match result itself still commits.
			log.Printf("rating: session %s participant %s queue_options failed to decode: %v; skipping rating input", sessionID, userID, err)
			return nil
		}
		participants = append(participants, rating.Participant{
			PlayerID:  userID.String(),
			SeatKey:   seatKey,
			TeamKey:   affinityBySeat[seatKey],
			Placement: placement,
			IsWinner:  isWinner,
			PreQueue:  preQueueByGroup(selections),
		})

		if len(queueOptions) == 0 {
			queueOptions = []byte("[]")
		}
		queueOptionsByPlayer[userID.String()] = json.RawMessage(queueOptions)
	}
	if err := rows.Err(); err != nil {
		return err
	}

	queueOptionsJSON, err := json.Marshal(queueOptionsByPlayer)
	if err != nil {
		return err
	}

	outcome := rating.MatchOutcome{
		Participants:       participants,
		CooperativeSuccess: cooperativeSuccess(cooperative, status, winnerUserIDs),
	}

	sides, err := rating.BuildSides(shape, outcome)
	if err != nil {
		log.Printf("rating: session %s outcome is not rateable: %v; skipping rating input", sessionID, err)
		return nil
	}

	ratingSides := make([]RatingSideRow, len(sides))
	for i, side := range sides {
		entrants := make([]RatingEntrantRow, len(side.Entrants))
		for j, e := range side.Entrants {
			entrants[j] = RatingEntrantRow{Key: e.Key}
		}
		ratingSides[i] = RatingSideRow{Rank: side.Rank, Entrants: entrants}
	}

	return s.AppendRatingInputTx(ctx, tx, RatingInput{
		SessionID:    sessionID,
		GameID:       gameID,
		ModeKey:      modeKey.String,
		Sides:        ratingSides,
		QueueOptions: queueOptionsJSON,
		RatedAt:      at,
	})
}

// preQueueByGroup turns a participant's stored queue selections into the
// groupKey -> optionIds map rating.Participant.PreQueue expects. Returns nil
// for no selections so an empty PreQueue is indistinguishable from one that
// was never set.
func preQueueByGroup(selections []prequeue.Selection) map[string][]string {
	if len(selections) == 0 {
		return nil
	}
	out := make(map[string][]string, len(selections))
	for _, sel := range selections {
		out[sel.GroupKey] = sel.OptionIDs
	}
	return out
}

// cooperativeSuccess derives rating.MatchOutcome.CooperativeSuccess for a
// cooperative mode. The reported MatchResultStatus enum only distinguishes
// COMPLETED / CANCELLED / ABANDONED — it has no dedicated "the crew failed"
// value, so a CANCELLED or ABANDONED match is left unrated (nil) exactly like
// an unrateable competitive match. For COMPLETED, the only other signal
// RecordMatchResult receives is winnerUserIDs, which every mode already uses
// to flag is_winner; a co-op game is expected to pass every crew member's id
// there on success and an empty/nil list on failure. This mirrors existing
// "winner" semantics rather than inventing a new field, but it is a judgment
// call, not a documented contract — see the Task 6 report for the caveat.
func cooperativeSuccess(cooperative bool, status string, winnerUserIDs []uuid.UUID) *bool {
	if !cooperative || status != "COMPLETED" {
		return nil
	}
	success := len(winnerUserIDs) > 0
	return &success
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
		SELECT game_id, mode_id, result_status, result_metadata, result_reported_at, status, ended_at
		FROM game_sessions
		WHERE id = $1
	`, sessionID).Scan(&out.GameID, &out.ModeID, &out.Status, &metadata, &out.ReportedAt, &out.SessionStatus, &out.EndedAt)
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
