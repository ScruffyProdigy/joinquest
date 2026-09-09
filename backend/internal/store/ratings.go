package store

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

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
// how their sides placed, and when. It is appended once per session (see
// AppendRatingInput) and replayed in order (see ListRatingInputs) to produce
// the player_ratings and nonplayer_ratings caches.
type RatingInput struct {
	SessionID     uuid.UUID
	GameID        uuid.UUID
	ModeKey       string
	Sides         []RatingSideRow
	QueueOptions  json.RawMessage
	InputsVersion int
	RatedAt       time.Time
}

// AppendRatingInput records a match's rating input. It is idempotent on
// session_id: a game server that retries its result report must not cause the
// same match to be rated twice.
func (s *Store) AppendRatingInput(ctx context.Context, in RatingInput) error {
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

	_, err = s.db.ExecContext(ctx, `
		INSERT INTO rating_match_inputs (session_id, game_id, mode_key, sides, queue_options, inputs_version, rated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		ON CONFLICT (session_id) DO NOTHING
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
// Both tables are upserted with ON CONFLICT DO UPDATE, so calling this
// repeatedly with the same keys (as a re-replay does) overwrites rather than
// duplicates. players is keyed by PlayerRatingKey(user_id) ("player:<uuid>");
// nonPlayers is keyed directly by nonplayer_ratings.entity_key. Either map may
// be nil.
func (s *Store) SaveRatings(ctx context.Context, gameID uuid.UUID, modeKey, engineID string, at time.Time, players, nonPlayers map[string]RatingValue) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

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

// ClearRatings deletes both cached rating tables for a game/mode. Used before
// a from-scratch replay, and when modes merge or are retired.
func (s *Store) ClearRatings(ctx context.Context, gameID uuid.UUID, modeKey string) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	if _, err := tx.ExecContext(ctx, `DELETE FROM player_ratings WHERE game_id = $1 AND mode_key = $2`, gameID, modeKey); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM nonplayer_ratings WHERE game_id = $1 AND mode_key = $2`, gameID, modeKey); err != nil {
		return err
	}
	return tx.Commit()
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
