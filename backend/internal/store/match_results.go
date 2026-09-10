package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"log"
	"strings"
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
	// User is the player record this row joins to. It is carried here because every caller
	// that renders a participant needs it, and fetching it separately meant a second pass
	// over the same join on every live push (JQ-177).
	User User
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

// RatedMode names the (game, mode) whose rating history just changed, so the
// caller can schedule a replay once the transaction has committed. Replaying
// inside the transaction would hold it open across a recompute that grows
// with the mode's whole history.
type RatedMode struct {
	GameID  uuid.UUID
	ModeKey string
}

// scenarioKeysFromMetadata reads the cooperative scenario identifiers a game
// reported alongside its match result. A single string and an array of
// strings are both accepted, since "scenarios": "hard" is the shape an
// integrator writes by hand first. Anything else yields no keys, which leaves
// a cooperative match unrated rather than inventing a difficulty for it.
func scenarioKeysFromMetadata(metadata map[string]any) []string {
	raw, ok := metadata["scenarios"]
	if !ok {
		return nil
	}
	switch v := raw.(type) {
	case string:
		if s := strings.TrimSpace(v); s != "" {
			return []string{s}
		}
		return nil
	case []any:
		out := make([]string, 0, len(v))
		for _, item := range v {
			s, ok := item.(string)
			if !ok {
				continue
			}
			if s = strings.TrimSpace(s); s != "" {
				out = append(out, s)
			}
		}
		return out
	default:
		return nil
	}
}

// excludedFinishReasons names the finish reasons that remove a participant
// from rating entirely. A disconnect says nothing about skill — the player
// lost their connection, not the match — and the OpenSkill paper leaves
// partial play and contribution weighting unimplemented, so the honest
// handling is to leave that player's rating untouched rather than to invent a
// weight. A FORFEIT is deliberately absent for the opposite reason: the
// reason itself carries no verdict, so the forfeiter stays in the match and
// is rated on whatever outcome the game separately reports for them — a
// trailing placement, or their absence from winnerLobbyUserIds. A game that
// wants a forfeit to cost rating reports it that way; see the Rating section
// of docs/match-lifecycle-callbacks.md, which publishes this contract.
var excludedFinishReasons = map[string]bool{"DISCONNECT": true}

// RecordPlayerFinish stores what the game reported about one player finishing.
// Side effects (marking finished, releasing queue rows) stay in match_lifecycle.go.
//
// It runs in a transaction because a finish reported *after* the match result
// has to rebuild that match's rating input alongside it. That order is not an
// edge case: docs/match-lifecycle-callbacks.md tells games to report the
// result when the match ends and to call reportPlayerFinished at the expiry
// of a disconnect grace period, which lands afterward — and without the
// rebuild the player the lobby promised to leave unrated would be rated as a
// loser. The rebuild is a correction, mechanically identical to a re-reported
// result: appendRatingInput preserves the row's original rated_at, so it
// lands back in its original chronological slot. The returned *RatedMode
// (nil when the rating log did not change) tells the caller to schedule a
// replay after commit, exactly as RecordMatchResult's does.
func (s *Store) RecordPlayerFinish(ctx context.Context, sessionID, userID uuid.UUID, reason string, placement *int, metadata map[string]any) (*RatedMode, error) {
	raw, err := marshalMetadata(metadata)
	if err != nil {
		return nil, err
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()

	result, err := tx.ExecContext(ctx, `
		UPDATE game_session_participants
		SET finish_reason = $3, placement = $4, finish_metadata = $5
		WHERE session_id = $1 AND user_id = $2
	`, sessionID, userID, reason, placement, raw)
	if err != nil {
		return nil, err
	}
	if err := ensureRowsAffected(result, ErrNotFound); err != nil {
		return nil, err
	}

	rated, err := s.rebuildRatingInputForReportedResultTx(ctx, tx, sessionID)
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return rated, nil
}

// rebuildRatingInputForReportedResultTx re-runs the rating-input build for a
// session whose result the game has already reported. A session with no
// result yet has nothing to rebuild — its rating input gets written when the
// result lands — so this is a no-op on the common ordering.
//
// Nothing is re-sent by the game: every input the build needs is already
// persisted by RecordMatchResult. The reported status and scenario metadata
// come off the session row, the winners off game_session_participants
// .is_winner, and the reporting instant off result_reported_at, so the
// rebuilt input restates the same result the game reported, differing only in
// what this finish just changed.
func (s *Store) rebuildRatingInputForReportedResultTx(ctx context.Context, tx *sql.Tx, sessionID uuid.UUID) (*RatedMode, error) {
	var (
		status      sql.NullString
		reportedAt  sql.NullTime
		rawMetadata []byte
	)
	if err := tx.QueryRowContext(ctx, `
		SELECT result_status, result_reported_at, result_metadata
		FROM game_sessions
		WHERE id = $1
	`, sessionID).Scan(&status, &reportedAt, &rawMetadata); err != nil {
		return nil, err
	}
	if !status.Valid || !reportedAt.Valid {
		return nil, nil
	}

	var metadata map[string]any
	if len(rawMetadata) > 0 {
		if err := json.Unmarshal(rawMetadata, &metadata); err != nil {
			// Unreachable today (the column only ever holds what
			// marshalMetadata wrote), and an unrateable-match condition like
			// the ones in appendRatingInputForResultTx rather than a
			// transaction failure: the finish itself must still commit.
			log.Printf("rating: session %s stored result_metadata did not decode: %v; skipping rating input rebuild", sessionID, err)
			return nil, nil
		}
	}

	winners, err := reportedWinnersTx(ctx, tx, sessionID)
	if err != nil {
		return nil, err
	}
	return s.appendRatingInputForResultTx(ctx, tx, sessionID, status.String, winners, metadata, reportedAt.Time)
}

// reportedWinnersTx reads back the winner ids RecordMatchResult persisted for
// a session. Only cooperativeSuccess consults this list — a competitive
// match's ranking comes from each participant's own is_winner — but a co-op
// crew's success has nowhere else to live, so a rebuild has to recover it
// from the same flags rather than assume failure.
func reportedWinnersTx(ctx context.Context, tx *sql.Tx, sessionID uuid.UUID) ([]uuid.UUID, error) {
	rows, err := tx.QueryContext(ctx, `
		SELECT user_id
		FROM game_session_participants
		WHERE session_id = $1 AND is_winner
	`, sessionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var winners []uuid.UUID
	for rows.Next() {
		var id uuid.UUID
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		winners = append(winners, id)
	}
	return winners, rows.Err()
}

// RecordMatchResult stores the match outcome and flags winners. Winner ids that do not
// belong to the session are silently ignored — the UPDATE simply matches no rows.
// It returns the (game, mode) whose rating log this changed — by appending an
// input, or by removing one a correction has voided — so a caller can schedule
// a replay after commit, and nil when the log is untouched.
func (s *Store) RecordMatchResult(ctx context.Context, sessionID uuid.UUID, status string, winnerUserIDs []uuid.UUID, metadata map[string]any, at time.Time) (*RatedMode, error) {
	raw, err := marshalMetadata(metadata)
	if err != nil {
		return nil, err
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()

	if _, err := tx.ExecContext(ctx, `
		UPDATE game_sessions
		SET result_status = $2, result_metadata = $3, result_reported_at = $4
		WHERE id = $1
	`, sessionID, status, raw, at); err != nil {
		return nil, err
	}

	if _, err := tx.ExecContext(ctx, `
		UPDATE game_session_participants SET is_winner = false WHERE session_id = $1
	`, sessionID); err != nil {
		return nil, err
	}

	for _, winnerID := range winnerUserIDs {
		if _, err := tx.ExecContext(ctx, `
			UPDATE game_session_participants
			SET is_winner = true
			WHERE session_id = $1 AND user_id = $2
		`, sessionID, winnerID); err != nil {
			return nil, err
		}
	}

	rated, err := s.appendRatingInputForResultTx(ctx, tx, sessionID, status, winnerUserIDs, metadata, at)
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return rated, nil
}

// appendRatingInputForResultTx records a rating input for a just-finalized
// match, in the same transaction as the result write. An outcome that cannot
// be rated (an abandoned match with no winners or placements, a session whose
// mode or seat template no longer resolves, and so on) is a normal occurrence
// — it is logged and skipped, never treated as a failure of the transaction:
// the game server has already applied its result report, and failing here
// would make it retry a write that has already landed. Once the mode is
// known, "skipped" also means dropping any input a correction has voided —
// see dropRatingInputTx.
func (s *Store) appendRatingInputForResultTx(ctx context.Context, tx *sql.Tx, sessionID uuid.UUID, status string, winnerUserIDs []uuid.UUID, metadata map[string]any, at time.Time) (*RatedMode, error) {
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
		return nil, err
	}

	// No mode row (mode_id was never set, or the mode has since been deleted)
	// or no seat template means there is nothing to build a ModeShape from.
	if !modeKey.Valid || len(seatTemplate) == 0 {
		log.Printf("rating: session %s has no mode or seat template; skipping rating input", sessionID)
		return nil, nil
	}

	leaves, err := seattemplate.Expand(seatTemplate)
	if err != nil {
		log.Printf("rating: session %s seat template did not expand: %v; skipping rating input", sessionID, err)
		return nil, nil
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
		SELECT p.user_id, p.role, p.placement, p.is_winner, p.queue_options, p.finish_reason
		FROM game_session_participants p
		WHERE p.session_id = $1
	`, sessionID)
	if err != nil {
		return nil, err
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
			finishReason sql.NullString
		)
		if err := rows.Scan(&userID, &role, &placement, &isWinner, &queueOptions, &finishReason); err != nil {
			return nil, err
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
			return dropRatingInputTx(ctx, tx, sessionID, gameID, modeKey.String)
		}
		participants = append(participants, rating.Participant{
			PlayerID:  userID.String(),
			SeatKey:   seatKey,
			TeamKey:   affinityBySeat[seatKey],
			Placement: placement,
			IsWinner:  isWinner,
			PreQueue:  preQueueByGroup(selections),
			Excluded:  excludedFinishReasons[finishReason.String],
		})

		if len(queueOptions) == 0 {
			queueOptions = []byte("[]")
		}
		queueOptionsByPlayer[userID.String()] = json.RawMessage(queueOptions)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	queueOptionsJSON, err := json.Marshal(queueOptionsByPlayer)
	if err != nil {
		return nil, err
	}

	outcome := rating.MatchOutcome{
		Participants:       participants,
		CooperativeSuccess: cooperativeSuccess(cooperative, status, winnerUserIDs),
		ScenarioKeys:       scenarioKeysFromMetadata(metadata),
	}

	sides, err := rating.BuildSides(shape, outcome)
	if err != nil {
		log.Printf("rating: session %s outcome is not rateable: %v; skipping rating input", sessionID, err)
		return dropRatingInputTx(ctx, tx, sessionID, gameID, modeKey.String)
	}

	ratingSides := make([]RatingSideRow, len(sides))
	for i, side := range sides {
		entrants := make([]RatingEntrantRow, len(side.Entrants))
		for j, e := range side.Entrants {
			entrants[j] = RatingEntrantRow{Key: e.Key}
		}
		ratingSides[i] = RatingSideRow{Rank: side.Rank, Entrants: entrants}
	}

	if err := s.AppendRatingInputTx(ctx, tx, RatingInput{
		SessionID: sessionID,
		GameID:    gameID,
		ModeKey:   modeKey.String,
		Sides:     ratingSides,
		// Stamped from the package that built the sides, so the row records
		// which emission rules produced it rather than a number kept in sync
		// by hand here.
		InputsVersion: rating.InputsVersion,
		QueueOptions:  queueOptionsJSON,
		RatedAt:       at,
	}); err != nil {
		return nil, err
	}
	return &RatedMode{GameID: gameID, ModeKey: modeKey.String}, nil
}

// dropRatingInputTx removes any rating input already recorded for a session
// that is no longer rateable, and reports the mode as needing a replay when
// it actually removed one.
//
// A match that was never rateable has nothing to remove and schedules
// nothing. But the same path now also covers a *correction*, and a
// correction can void a match that used to be rateable: COMPLETED with a
// winner restated as ABANDONED, a winner list that grew to cover everybody,
// a co-op result that dropped its scenarios, a disconnect reported late that
// empties out the second side. The log is meant to say what the game
// currently says, so the superseded row has to go — leaving it in place
// keeps the voided match moving ratings forever, since the session's status,
// is_winner flags and standings all follow the correction while the ratings
// do not.
//
// Scheduling the replay from here is not optional either: a corrected row
// keeps its original rated_at (see appendRatingInput) and a removed row
// lowers the mode's max(rated_at), so neither is visible to
// ListModesNeedingReplay's max-input-against-last-rated comparison.
func dropRatingInputTx(ctx context.Context, tx *sql.Tx, sessionID, gameID uuid.UUID, modeKey string) (*RatedMode, error) {
	result, err := tx.ExecContext(ctx, `DELETE FROM rating_match_inputs WHERE session_id = $1`, sessionID)
	if err != nil {
		return nil, err
	}
	removed, err := result.RowsAffected()
	if err != nil {
		return nil, err
	}
	if removed == 0 {
		return nil, nil
	}
	log.Printf("rating: session %s is no longer rateable; removed its rating input and scheduling a replay of %s/%s", sessionID, gameID, modeKey)
	return &RatedMode{GameID: gameID, ModeKey: modeKey}, nil
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
		SELECT `+strings.ReplaceAll(userColumns, "id,", "u.id,")+`,
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
		user, err := scanUser(trailingColumns{rows: rows, rest: []any{
			&p.DisplayName, &p.Role,
			&p.FinishedAt, &p.LeftAt, &p.Reason, &p.Placement, &p.IsWinner,
		}})
		if err != nil {
			return nil, err
		}
		p.User = *user
		p.UserID = user.ID
		out.Participants = append(out.Participants, p)
	}
	return &out, rows.Err()
}

// trailingColumns lets scanUser consume the leading userColumns of a wider row while the
// caller scans the rest, so a join can answer "the participant and the player" in one pass.
type trailingColumns struct {
	rows *sql.Rows
	rest []any
}

func (t trailingColumns) Scan(dest ...any) error {
	return t.rows.Scan(append(dest, t.rest...)...)
}
