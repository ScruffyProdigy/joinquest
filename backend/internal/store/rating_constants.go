package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// ModeRatingConstants is one mode's measured rating constants.
//
// Only beta lives here today. It is the constant that plainly is not one value
// across the catalog: it says how much a skill gap predicts the result, and an
// RPSLR duel and a high-luck party game do not agree about that (JQ-227). The
// table it comes from has room for the next such constant without another
// migration, but nothing is added to it before there is a way to measure it.
type ModeRatingConstants struct {
	// Beta is the mode's performance variance.
	Beta float64

	// EngineID is the engine identity these constants imply — what
	// rating.Engine.ID() returns for an engine built with this beta. It is
	// stored rather than recomputed on read so the replay sweep can compare
	// it against player_ratings.engine_id in one query: a mode whose cached
	// ratings were computed under a different beta is stale in exactly the
	// way a mode with newer inputs is stale, and both need the same full
	// replay. Ratings computed under different constants are not comparable,
	// and letting them mix would corrupt the mode's history silently.
	EngineID string

	// SampleSize is how many held-out matches the estimate was selected on.
	// It is required, and it is kept permanently rather than only in the
	// report that produced it: a beta fitted on 200 matches and one fitted on
	// 40,000 are the same number and not the same evidence, and whoever reads
	// this row months later has no other way to tell them apart.
	SampleSize int

	// EstimatedAt is when the fit was run, which is what says how much of the
	// mode's history it actually saw.
	EstimatedAt time.Time
}

// GetModeRatingConstants returns one mode's measured constants.
//
// The boolean is the whole point of the call: false means nothing has been
// measured for this mode and the caller should use the engine's default,
// which is deliberately a different state from a stored row that happens to
// hold the default value. The second claims a measurement; the first does not.
func (s *Store) GetModeRatingConstants(ctx context.Context, gameID uuid.UUID, modeKey string) (ModeRatingConstants, bool, error) {
	var c ModeRatingConstants
	err := s.db.QueryRowContext(ctx, `
		SELECT beta, engine_id, sample_size, estimated_at
		FROM mode_rating_constants
		WHERE game_id = $1 AND mode_key = $2
	`, gameID, modeKey).Scan(&c.Beta, &c.EngineID, &c.SampleSize, &c.EstimatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return ModeRatingConstants{}, false, nil
	}
	if err != nil {
		return ModeRatingConstants{}, false, err
	}
	return c, true, nil
}

// SetModeRatingConstants records a mode's measured constants, replacing any
// earlier measurement.
//
// It does not replay the mode. That is deliberate: the write and the
// recomputation are separated by the same mechanism that already repairs a
// mode whose replay was lost — SaveRatings stamps every cached rating with the
// engine id that produced it, and ListModesNeedingReplay now treats a mismatch
// against these constants as staleness. So a beta written here is picked up by
// the next sweep and the whole mode is replayed under it, without this call
// having to hold a replay open or a caller having to remember to trigger one.
//
// EngineID is supplied by the caller rather than derived here because the
// choice of model is the server's, not the data layer's: store would have to
// name "plackett-luce" to build an engine, and that name lives in one place.
func (s *Store) SetModeRatingConstants(ctx context.Context, gameID uuid.UUID, modeKey string, c ModeRatingConstants) error {
	if modeKey == "" {
		return fmt.Errorf("store: mode rating constants need a mode key")
	}
	if !(c.Beta > 0) {
		return fmt.Errorf("store: mode rating beta must be positive, got %v", c.Beta)
	}
	if c.EngineID == "" {
		return fmt.Errorf("store: mode rating constants need the engine id they imply")
	}
	if c.SampleSize <= 0 {
		// An estimate with no sample behind it is an assertion wearing a
		// measurement's clothes, and the whole reason this table records a
		// sample size is so nobody has to guess which one they are reading.
		return fmt.Errorf("store: mode rating constants need a positive sample size, got %d", c.SampleSize)
	}
	at := c.EstimatedAt
	if at.IsZero() {
		at = time.Now().UTC()
	}

	_, err := s.db.ExecContext(ctx, `
		INSERT INTO mode_rating_constants (game_id, mode_key, beta, engine_id, sample_size, estimated_at)
		VALUES ($1, $2, $3, $4, $5, $6)
		ON CONFLICT (game_id, mode_key) DO UPDATE SET
			beta = EXCLUDED.beta,
			engine_id = EXCLUDED.engine_id,
			sample_size = EXCLUDED.sample_size,
			estimated_at = EXCLUDED.estimated_at
	`, gameID, modeKey, c.Beta, c.EngineID, c.SampleSize, at)
	return err
}

// ClearModeRatingConstants returns a mode to the engine's default constants.
//
// Deleting the row rather than writing the default value back is what keeps
// "never measured" and "measured, and it came out at the default" distinct —
// and it puts the mode back on the same footing as one that has never been
// fitted, which is what a caller undoing a bad fit actually wants. The next
// sweep sees the mode's cached ratings carrying constants that no longer match
// and replays it, exactly as it would for a new measurement.
func (s *Store) ClearModeRatingConstants(ctx context.Context, gameID uuid.UUID, modeKey string) error {
	_, err := s.db.ExecContext(ctx, `
		DELETE FROM mode_rating_constants WHERE game_id = $1 AND mode_key = $2
	`, gameID, modeKey)
	return err
}

// ModeRatingConstantsRow is one row of ListModeRatingConstants.
type ModeRatingConstantsRow struct {
	GameID  uuid.UUID
	ModeKey string
	ModeRatingConstants
}

// ListModeRatingConstants returns every mode with measured constants, ordered
// so two listings can be diffed.
func (s *Store) ListModeRatingConstants(ctx context.Context) ([]ModeRatingConstantsRow, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT game_id, mode_key, beta, engine_id, sample_size, estimated_at
		FROM mode_rating_constants
		ORDER BY game_id, mode_key
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []ModeRatingConstantsRow
	for rows.Next() {
		var r ModeRatingConstantsRow
		if err := rows.Scan(&r.GameID, &r.ModeKey, &r.Beta, &r.EngineID, &r.SampleSize, &r.EstimatedAt); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}
