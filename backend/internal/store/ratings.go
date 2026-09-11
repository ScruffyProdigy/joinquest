package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/lib/pq"

	"github.com/scruffyprodigy/joinquest/internal/rating"
)

// sqlExecContext is satisfied by both *sql.DB and *sql.Tx, letting
// appendRatingInput run against either a standalone connection or a
// caller-supplied transaction.
type sqlExecContext interface {
	ExecContext(context.Context, string, ...any) (sql.Result, error)
}

// PlayerRatingKey returns the map key LoadPlayerRatings/SaveRatings use for a
// user's mode-level rating, so callers never have to build the
// "player:<uuid>" string by hand.
//
// The grammar for every entrant key — player, per-seat player, seat class,
// scenario, pre-queue option — lives in internal/rating, which is the package
// that produces them. It is deliberately not restated here: this table and
// that package have to agree exactly for a replay's output to land in the
// right rows, and two copies of the same string constant is how they would
// come to disagree.
func PlayerRatingKey(userID uuid.UUID) string {
	return rating.PlayerKey(userID.String())
}

// PlayerSeatRatingKey is PlayerRatingKey for a player's rating in one seat
// class. An empty seatClass gives back the mode-level key.
func PlayerSeatRatingKey(userID uuid.UUID, seatClass string) string {
	return rating.PlayerSeatKey(userID.String(), seatClass)
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
		SELECT user_id, seat_class, mu, sigma, matches_played
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
		var seatClass string
		var v RatingValue
		if err := rows.Scan(&userID, &seatClass, &v.Mu, &v.Sigma, &v.MatchesPlayed); err != nil {
			return nil, err
		}
		out[PlayerSeatRatingKey(userID, seatClass)] = v
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
		userID, seatClass, ok := parsePlayerRatingKey(key)
		if !ok {
			return fmt.Errorf("store: SaveRatings player key %q is not player:<uuid> or player:<uuid>@seat:<class>", key)
		}
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO player_ratings (user_id, game_id, mode_key, seat_class, mu, sigma, matches_played, engine_id, last_rated_at)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
			ON CONFLICT (user_id, game_id, mode_key, seat_class) DO UPDATE SET
				mu = EXCLUDED.mu,
				sigma = EXCLUDED.sigma,
				matches_played = EXCLUDED.matches_played,
				engine_id = EXCLUDED.engine_id,
				last_rated_at = EXCLUDED.last_rated_at
		`, userID, gameID, modeKey, seatClass, v.Mu, v.Sigma, v.MatchesPlayed, engineID, at); err != nil {
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

// ListRatedModes returns every (game, mode) that has any retained rating
// input, in a stable order.
//
// Unlike ListModesNeedingReplay this asks nothing about whether the cached
// ratings are current: the backtest harness (internal/ratingbacktest) reads
// the input log and never the cache, so a mode whose ratings are perfectly
// up to date is exactly as backtestable as one whose replay is pending. The
// ordering is fixed so that a report covering every mode lists them the same
// way on every run, and two reports can be diffed.
func (s *Store) ListRatedModes(ctx context.Context) ([]RatedMode, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT DISTINCT game_id, mode_key
		FROM rating_match_inputs
		ORDER BY game_id, mode_key
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

// ListModesNeedingReplay returns every (game, mode) whose cached ratings no
// longer follow from the inputs and constants they should have been computed
// from — modes whose replay was scheduled but never ran, typically because the
// process restarted between the result committing and the worker's next tick,
// and modes whose rating constants have since changed.
//
// Two kinds of staleness, one sweep, because they need the identical repair: a
// full replay of the mode.
//
// Newer inputs. The comparison is exact because SaveRatings stamps
// last_rated_at with the newest input the replay consumed, not with wall-clock
// time. A correction to an older session keeps its original rated_at and so is
// invisible here; it is scheduled directly by the result path instead.
//
// Changed constants. defaultEngineID is the engine a mode with no measured
// constants must be rated by, and mode_rating_constants.engine_id is the one a
// measured mode must be rated by; either way, cached ratings carrying a
// different engine id were computed under constants that no longer apply.
// Ratings produced under two different betas are not on one scale and cannot
// be compared, so a mode's whole history is replayed rather than continued —
// which also means a beta can be written without holding a replay open, since
// the next sweep picks it up. Both the minimum and the maximum engine id are
// checked so a mode that somehow holds a mixture is caught as well: a mode
// half-rated under old constants is the exact corruption this is here to
// prevent, and it would otherwise pass whichever single row the query happened
// to see.
//
// The join is against player_ratings only, not nonplayer_ratings because
// every stored input carries at least one player: entrant (BuildSides/buildSide),
// so any replay of a mode in this query writes at least one player_ratings row;
// a NULL therefore means "not currently cached" — never replayed, or invalidated
// by ClearRatings or a user merge — and one replay clears it either way.
func (s *Store) ListModesNeedingReplay(ctx context.Context, defaultEngineID string) ([]RatedMode, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT i.game_id, i.mode_key
		FROM (
			SELECT game_id, mode_key, MAX(rated_at) AS last_input
			FROM rating_match_inputs
			GROUP BY game_id, mode_key
		) i
		LEFT JOIN (
			SELECT game_id, mode_key,
			       MAX(last_rated_at) AS last_rated,
			       MIN(engine_id) AS engine_id_min,
			       MAX(engine_id) AS engine_id_max
			FROM player_ratings
			GROUP BY game_id, mode_key
		) r ON r.game_id = i.game_id AND r.mode_key = i.mode_key
		LEFT JOIN mode_rating_constants c
			ON c.game_id = i.game_id AND c.mode_key = i.mode_key
		WHERE r.last_rated IS NULL
		   OR r.last_rated < i.last_input
		   OR r.engine_id_min IS DISTINCT FROM COALESCE(c.engine_id, $1)
		   OR r.engine_id_max IS DISTINCT FROM COALESCE(c.engine_id, $1)
	`, defaultEngineID)
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

// parsePlayerRatingKey splits a player entrant key into the primary-key parts
// of a player_ratings row: the user, and the seat class the rating is scoped
// to (empty for the mode-level rating).
func parsePlayerRatingKey(key string) (uuid.UUID, string, bool) {
	playerID, seatClass, ok := rating.ParsePlayerKey(key)
	if !ok {
		return uuid.UUID{}, "", false
	}
	id, err := uuid.Parse(playerID)
	if err != nil {
		return uuid.UUID{}, "", false
	}
	return id, seatClass, true
}

// GetPlayerRatings returns the cached rating for each of userIDs in one game
// and mode, keyed by user id. A user with no rating row is simply absent from
// the map: what an unrated player should report to a game is an exposure
// decision (see rating.UnratedSkill), not something the cache invents.
//
// This is the read counterpart to LoadPlayerRatings, which pulls every rated
// player in a mode because a replay needs the whole population. Serving a
// roster — or a single lookup — does not, and on a popular mode the difference
// is the entire table versus a handful of rows.
func (s *Store) GetPlayerRatings(ctx context.Context, gameID uuid.UUID, modeKey string, userIDs []uuid.UUID) (map[uuid.UUID]RatingValue, error) {
	out := make(map[uuid.UUID]RatingValue, len(userIDs))
	if len(userIDs) == 0 {
		return out, nil
	}

	ids := make([]string, 0, len(userIDs))
	for _, id := range userIDs {
		ids = append(ids, id.String())
	}

	rows, err := s.db.QueryContext(ctx, `
		SELECT user_id, mu, sigma, matches_played
		FROM player_ratings
		WHERE game_id = $1 AND mode_key = $2 AND user_id = ANY($3::uuid[])
		  AND seat_class = ''
	`, gameID, modeKey, pq.Array(ids))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var userID uuid.UUID
		var v RatingValue
		if err := rows.Scan(&userID, &v.Mu, &v.Sigma, &v.MatchesPlayed); err != nil {
			return nil, err
		}
		out[userID] = v
	}
	return out, rows.Err()
}

// GameModeExists reports whether a game currently declares a mode under this
// key.
//
// The skill lookup asks first so a typo'd mode key answers null instead of a
// plausible-looking prior. A rating can outlive the mode row that produced it —
// manifest sync deletes and rewrites mode rows, and player_ratings is keyed by
// the mode_key string precisely so it survives that — but a mode the game no
// longer declares is one it should no longer be reading skill for either.
func (s *Store) GameModeExists(ctx context.Context, gameID uuid.UUID, modeKey string) (bool, error) {
	var exists bool
	err := s.db.QueryRowContext(ctx, `
		SELECT EXISTS (
			SELECT 1 FROM game_modes WHERE game_id = $1 AND mode_key = $2
		)
	`, gameID, modeKey).Scan(&exists)
	return exists, err
}
