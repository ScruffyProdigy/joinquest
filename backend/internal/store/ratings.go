package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

// sqlExecContext is satisfied by both *sql.DB and *sql.Tx, letting
// appendRatingInput run against either a standalone connection or a
// caller-supplied transaction.
type sqlExecContext interface {
	ExecContext(context.Context, string, ...any) (sql.Result, error)
}

// playerRatingKeyPrefix namespaces a player_ratings row's user id inside the
// flat string-keyed maps the rating engine works in. Non-player entities
// (scenarios, seat classes, pre-queue options) use their own namespaced keys
// and live in nonplayer_ratings instead — see the migration comment on that
// table for the entity_key convention.
const playerRatingKeyPrefix = "player:"

// PlayerRatingKey returns the map key LoadPlayerRatings/SaveRatings use for a
// user, so callers never have to build the "player:<uuid>" string by hand.
func PlayerRatingKey(userID uuid.UUID) string {
	return playerRatingKeyPrefix + userID.String()
}

// RatingValue is one entrant's current skill estimate, as cached in
// player_ratings or nonplayer_ratings.
type RatingValue struct {
	Mu            float64
	Sigma         float64
	MatchesPlayed int
}

// RatingEntrantRow is one entrant on a side of a rated match. Key follows the
// same "player:<uuid>" / "seat:<name>" / "prequeue:<name>" namespacing as
// nonplayer_ratings.entity_key.
type RatingEntrantRow struct {
	Key string `json:"key"`
}

// RatingSideRow is one side of a rated match (a team, a seat, or a solo
// entrant), ranked against the match's other sides. Rank 0 is the winner;
// ties share a rank.
type RatingSideRow struct {
	Rank     int                `json:"rank"`
	Entrants []RatingEntrantRow `json:"entrants"`
}

// RatingInput is one match's worth of input to the rating engine: who played,
// how their sides placed, and when. There is at most one row per session (see
// AppendRatingInput, which updates the row in place on a corrected report),
// and rows are replayed in order (see ListRatingInputs) to produce the
// player_ratings and nonplayer_ratings caches.
type RatingInput struct {
	SessionID     uuid.UUID
	GameID        uuid.UUID
	ModeKey       string
	Sides         []RatingSideRow
	QueueOptions  json.RawMessage
	InputsVersion int
	RatedAt       time.Time
}

// AppendRatingInput records a match's rating input, keyed by session_id.
// Retrying the same report is idempotent: the row ends up holding the same
// values either way. But a *corrected* report — a game server that reports
// different winners, or upgrades ABANDONED to COMPLETED — updates the
// existing row rather than being silently dropped, so this log tracks the
// latest reported result for a session, not the first one recorded. The
// session_id primary key still guarantees at most one row per match.
func (s *Store) AppendRatingInput(ctx context.Context, in RatingInput) error {
	return appendRatingInput(ctx, s.db, in)
}

// AppendRatingInputTx is AppendRatingInput run against a caller-supplied
// transaction, so a rating input can be recorded atomically alongside the
// match result that produced it.
func (s *Store) AppendRatingInputTx(ctx context.Context, tx *sql.Tx, in RatingInput) error {
	return appendRatingInput(ctx, tx, in)
}

func appendRatingInput(ctx context.Context, exec sqlExecContext, in RatingInput) error {
	sidesJSON, err := json.Marshal(in.Sides)
	if err != nil {
		return fmt.Errorf("store: marshal rating sides: %w", err)
	}

	queueOptions := []byte(in.QueueOptions)
	if len(queueOptions) == 0 {
		queueOptions = []byte("[]")
	}

	inputsVersion := in.InputsVersion
	if inputsVersion == 0 {
		inputsVersion = 1
	}

	_, err = exec.ExecContext(ctx, `
		INSERT INTO rating_match_inputs (session_id, game_id, mode_key, sides, queue_options, inputs_version, rated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		ON CONFLICT (session_id) DO UPDATE SET
			sides          = EXCLUDED.sides,
			queue_options  = EXCLUDED.queue_options,
			inputs_version = EXCLUDED.inputs_version
			-- rated_at is deliberately NOT refreshed here. It orders replay
			-- (see ListRatingInputs' ORDER BY rated_at ASC, session_id ASC).
			-- A correction is a restatement of a match that already
			-- happened, not a new event: bumping rated_at to the
			-- correction's later report timestamp would move the match to a
			-- new position in history, changing the relative replay order
			-- of everything reported in between and potentially producing
			-- different recomputed ratings. Keep the original rated_at so a
			-- correction stays in its original chronological slot.
	`, in.SessionID, in.GameID, in.ModeKey, sidesJSON, queueOptions, inputsVersion, in.RatedAt)
	return err
}

// ListRatingInputs returns every rating input for a game/mode in replay
// order. The ORDER BY tiebreak on session_id is load-bearing, not cosmetic:
// rated_at alone can tie, and without a total order the replay that recomputes
// ratings from this log would not be reproducible.
func (s *Store) ListRatingInputs(ctx context.Context, gameID uuid.UUID, modeKey string) ([]RatingInput, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT session_id, game_id, mode_key, sides, queue_options, inputs_version, rated_at
		FROM rating_match_inputs
		WHERE game_id = $1 AND mode_key = $2
		ORDER BY rated_at ASC, session_id ASC
	`, gameID, modeKey)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []RatingInput
	for rows.Next() {
		var in RatingInput
		var sides, queueOptions []byte
		if err := rows.Scan(&in.SessionID, &in.GameID, &in.ModeKey, &sides, &queueOptions, &in.InputsVersion, &in.RatedAt); err != nil {
			return nil, err
		}
		if err := json.Unmarshal(sides, &in.Sides); err != nil {
			return nil, fmt.Errorf("store: unmarshal rating sides for session %s: %w", in.SessionID, err)
		}
		in.QueueOptions = json.RawMessage(queueOptions)
		out = append(out, in)
	}
	return out, rows.Err()
}

// LoadPlayerRatings returns every cached player rating for a game/mode, keyed
// by PlayerRatingKey(user_id) — the same "player:<uuid>" form used in
// RatingEntrantRow.Key and SaveRatings' players map, so callers never build
// that key twice.
func (s *Store) LoadPlayerRatings(ctx context.Context, gameID uuid.UUID, modeKey string) (map[string]RatingValue, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT user_id, mu, sigma, matches_played
		FROM player_ratings
		WHERE game_id = $1 AND mode_key = $2
	`, gameID, modeKey)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make(map[string]RatingValue)
	for rows.Next() {
		var userID uuid.UUID
		var v RatingValue
		if err := rows.Scan(&userID, &v.Mu, &v.Sigma, &v.MatchesPlayed); err != nil {
			return nil, err
		}
		out[PlayerRatingKey(userID)] = v
	}
	return out, rows.Err()
}

// LoadNonPlayerRatings returns every cached non-player rating (scenarios, seat
// classes, rated pre-queue options) for a game/mode, keyed by entity_key.
func (s *Store) LoadNonPlayerRatings(ctx context.Context, gameID uuid.UUID, modeKey string) (map[string]RatingValue, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT entity_key, mu, sigma, matches_played
		FROM nonplayer_ratings
		WHERE game_id = $1 AND mode_key = $2
	`, gameID, modeKey)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make(map[string]RatingValue)
	for rows.Next() {
		var key string
		var v RatingValue
		if err := rows.Scan(&key, &v.Mu, &v.Sigma, &v.MatchesPlayed); err != nil {
			return nil, err
		}
		out[key] = v
	}
	return out, rows.Err()
}

// SaveRatings writes a replay's output for a game/mode in one transaction.
//
// It first clears every existing row for this game/mode from both tables,
// then writes exactly the rows passed in, so the result is always a full
// recompute rather than an upsert onto whatever was cached before. This is
// what makes SaveRatings safe to call with a replay's output: replay always
// produces the complete set of entrants derived from the current input log
// (see rating.Replayer.ReplayMode), so an entrant that no longer appears in
// that log — because its input row was corrected or removed — must not
// survive here either, or the cache silently stops equalling the replay,
// which is the one invariant this whole path exists to uphold. players is
// keyed by PlayerRatingKey(user_id) ("player:<uuid>"); nonPlayers is keyed
// directly by nonplayer_ratings.entity_key. Either map may be nil.
func (s *Store) SaveRatings(ctx context.Context, gameID uuid.UUID, modeKey, engineID string, at time.Time, players, nonPlayers map[string]RatingValue) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	if err := clearRatingsTx(ctx, tx, gameID, modeKey); err != nil {
		return err
	}

	for key, v := range players {
		userID, ok := parsePlayerRatingKey(key)
		if !ok {
			return fmt.Errorf("store: SaveRatings player key %q is not %q<uuid>", key, playerRatingKeyPrefix)
		}
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO player_ratings (user_id, game_id, mode_key, mu, sigma, matches_played, engine_id, last_rated_at)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
			ON CONFLICT (user_id, game_id, mode_key) DO UPDATE SET
				mu = EXCLUDED.mu,
				sigma = EXCLUDED.sigma,
				matches_played = EXCLUDED.matches_played,
				engine_id = EXCLUDED.engine_id,
				last_rated_at = EXCLUDED.last_rated_at
		`, userID, gameID, modeKey, v.Mu, v.Sigma, v.MatchesPlayed, engineID, at); err != nil {
			return err
		}
	}

	for entityKey, v := range nonPlayers {
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO nonplayer_ratings (game_id, mode_key, entity_key, mu, sigma, matches_played, engine_id, last_rated_at)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
			ON CONFLICT (game_id, mode_key, entity_key) DO UPDATE SET
				mu = EXCLUDED.mu,
				sigma = EXCLUDED.sigma,
				matches_played = EXCLUDED.matches_played,
				engine_id = EXCLUDED.engine_id,
				last_rated_at = EXCLUDED.last_rated_at
		`, gameID, modeKey, entityKey, v.Mu, v.Sigma, v.MatchesPlayed, engineID, at); err != nil {
			return err
		}
	}

	return tx.Commit()
}

// ClearRatings deletes both cached rating tables for a game/mode. Used when
// modes merge or are retired; a from-scratch replay no longer needs to call
// this itself since SaveRatings clears in the same transaction as it writes.
func (s *Store) ClearRatings(ctx context.Context, gameID uuid.UUID, modeKey string) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	if err := clearRatingsTx(ctx, tx, gameID, modeKey); err != nil {
		return err
	}
	return tx.Commit()
}

// clearRatingsTx deletes both cached rating tables for a game/mode against a
// caller-supplied transaction, following the ...Tx naming used elsewhere in
// this package for transaction-scoped variants (e.g. AppendRatingInputTx).
func clearRatingsTx(ctx context.Context, tx *sql.Tx, gameID uuid.UUID, modeKey string) error {
	if _, err := tx.ExecContext(ctx, `DELETE FROM player_ratings WHERE game_id = $1 AND mode_key = $2`, gameID, modeKey); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM nonplayer_ratings WHERE game_id = $1 AND mode_key = $2`, gameID, modeKey); err != nil {
		return err
	}
	return nil
}

// ListModesNeedingReplay returns every (game, mode) whose newest rating input
// is newer than the ratings computed from it — modes whose replay was
// scheduled but never ran, typically because the process restarted between
// the result committing and the worker's next tick.
//
// The comparison is exact because SaveRatings stamps last_rated_at with the
// newest input the replay consumed, not with wall-clock time. A correction to
// an older session keeps its original rated_at and so is invisible here; it
// is scheduled directly by the result path instead.
//
// The join is against player_ratings only, not nonplayer_ratings because
// every stored input carries at least one player: entrant (BuildSides/buildSide),
// so any replay of a mode in this query writes at least one player_ratings row;
// a NULL therefore means "not currently cached" — never replayed, or invalidated
// by ClearRatings or a user merge — and one replay clears it either way.
func (s *Store) ListModesNeedingReplay(ctx context.Context) ([]RatedMode, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT i.game_id, i.mode_key
		FROM (
			SELECT game_id, mode_key, MAX(rated_at) AS last_input
			FROM rating_match_inputs
			GROUP BY game_id, mode_key
		) i
		LEFT JOIN (
			SELECT game_id, mode_key, MAX(last_rated_at) AS last_rated
			FROM player_ratings
			GROUP BY game_id, mode_key
		) r ON r.game_id = i.game_id AND r.mode_key = i.mode_key
		WHERE r.last_rated IS NULL OR r.last_rated < i.last_input
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []RatedMode
	for rows.Next() {
		var m RatedMode
		if err := rows.Scan(&m.GameID, &m.ModeKey); err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

func parsePlayerRatingKey(key string) (uuid.UUID, bool) {
	rest, ok := strings.CutPrefix(key, playerRatingKeyPrefix)
	if !ok {
		return uuid.UUID{}, false
	}
	id, err := uuid.Parse(rest)
	if err != nil {
		return uuid.UUID{}, false
	}
	return id, true
}
