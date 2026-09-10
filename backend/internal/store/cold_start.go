package store

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/lib/pq"

	"github.com/scruffyprodigy/joinquest/internal/coldstart"
)

// ListGamesWithRatings returns every game holding at least one cached player
// rating, in a stable order.
//
// It asks player_ratings rather than rating_match_inputs because seeding is
// fitted over ratings, not over match history: a game whose replay has not
// run yet has nothing to correlate even though its log is full. Ordering is
// fixed so a recompute covers games the same way on every pass.
func (s *Store) ListGamesWithRatings(ctx context.Context) ([]uuid.UUID, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT DISTINCT game_id FROM player_ratings ORDER BY game_id
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []uuid.UUID
	for rows.Next() {
		var id uuid.UUID
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		out = append(out, id)
	}
	return out, rows.Err()
}

// ListConvergedRatings returns every player rating in one game whose sigma has
// fallen to maxSigma or below, across all of the game's modes.
//
// The filter is in SQL rather than in the caller because on a popular game the
// unconverged ratings are most of the table and none of them are evidence:
// a rating still sitting near the prior correlates with another such rating
// through the prior they share rather than through anything either player has
// shown, which inflates the fit in the direction that causes harm. See
// coldstart.ConvergedSigma.
//
// The order is total so that a refit over unchanged data produces an
// identical measurement — coldstart.Fit sorts by player key before splitting
// folds, but a stable read makes the whole path reproducible rather than
// reproducible-because-one-function-remembers-to-sort.
func (s *Store) ListConvergedRatings(ctx context.Context, gameID uuid.UUID, maxSigma float64) ([]coldstart.ConvergedRating, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT user_id, mode_key, mu
		FROM player_ratings
		WHERE game_id = $1 AND sigma <= $2
		ORDER BY mode_key, user_id
	`, gameID, maxSigma)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []coldstart.ConvergedRating
	for rows.Next() {
		var r coldstart.ConvergedRating
		if err := rows.Scan(&r.UserID, &r.ModeKey, &r.Mu); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// SaveModePairStats replaces every measured mode pair for one game in a single
// transaction.
//
// Replace rather than upsert, for the same reason SaveRatings clears before it
// writes: a recompute produces the complete set of pairs derivable from the
// ratings as they now stand, so a pair that has stopped being derivable —
// a mode retired, or its rated population fallen away — must stop being served
// too. Upserting would leave it in place indefinitely, seeding players from a
// measurement nothing supports any more.
func (s *Store) SaveModePairStats(ctx context.Context, gameID uuid.UUID, at time.Time, stats []coldstart.PairStats) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	if _, err := tx.ExecContext(ctx, `DELETE FROM mode_pair_correlations WHERE game_id = $1`, gameID); err != nil {
		return err
	}

	for _, p := range stats {
		if p.SourceMode == p.TargetMode {
			return fmt.Errorf("store: SaveModePairStats got a self-pair for mode %q", p.SourceMode)
		}
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO mode_pair_correlations (
				game_id, source_mode_key, target_mode_key,
				paired_players, correlation,
				source_mean, source_sd, target_mean, target_sd,
				residual_sd, flat_residual_sd, bias,
				abs_error_p50, abs_error_p90, computed_at
			) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15)
		`,
			gameID, p.SourceMode, p.TargetMode,
			p.PairedPlayers, p.Correlation,
			p.SourceMean, p.SourceSD, p.TargetMean, p.TargetSD,
			p.ResidualSD, p.FlatResidualSD, p.Bias,
			p.AbsErrorP50, p.AbsErrorP90, at,
		); err != nil {
			return err
		}
	}

	return tx.Commit()
}

// LoadModePairStats returns every measured pair that can seed targetMode,
// keyed by source mode key.
//
// Unusable pairs are returned alongside usable ones rather than filtered in
// SQL. The gate is coldstart.PairStats.Usable, and keeping it in one place is
// what stops the serve path and the recompute from drifting into two
// different opinions about what counts as enough evidence.
func (s *Store) LoadModePairStats(ctx context.Context, gameID uuid.UUID, targetMode string) (map[string]coldstart.PairStats, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT source_mode_key, target_mode_key,
		       paired_players, correlation,
		       source_mean, source_sd, target_mean, target_sd,
		       residual_sd, flat_residual_sd, bias,
		       abs_error_p50, abs_error_p90
		FROM mode_pair_correlations
		WHERE game_id = $1 AND target_mode_key = $2
	`, gameID, targetMode)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make(map[string]coldstart.PairStats)
	for rows.Next() {
		var p coldstart.PairStats
		if err := rows.Scan(
			&p.SourceMode, &p.TargetMode,
			&p.PairedPlayers, &p.Correlation,
			&p.SourceMean, &p.SourceSD, &p.TargetMean, &p.TargetSD,
			&p.ResidualSD, &p.FlatResidualSD, &p.Bias,
			&p.AbsErrorP50, &p.AbsErrorP90,
		); err != nil {
			return nil, err
		}
		out[p.SourceMode] = p
	}
	return out, rows.Err()
}

// ListSeedSources returns, for each of userIDs, the converged ratings they
// hold in other modes of this game — everything a seed for excludeMode could
// be built from.
//
// excludeMode is left out because a player who has a rating in the mode being
// seeded is not being seeded at all; the caller has already established there
// is no row there, and returning one would mean the two reads disagreed.
//
// Sigma is filtered here rather than by the caller so that a roster read
// fetches only rows that can actually be used. coldstart.SeedFor checks the
// same threshold again on what it is handed — it is the package that owns the
// rule, and it must hold whoever calls it.
func (s *Store) ListSeedSources(ctx context.Context, gameID uuid.UUID, excludeMode string, userIDs []uuid.UUID, maxSigma float64) (map[uuid.UUID][]coldstart.SourceRating, error) {
	out := make(map[uuid.UUID][]coldstart.SourceRating, len(userIDs))
	if len(userIDs) == 0 {
		return out, nil
	}

	ids := make([]string, 0, len(userIDs))
	for _, id := range userIDs {
		ids = append(ids, id.String())
	}

	rows, err := s.db.QueryContext(ctx, `
		SELECT user_id, mode_key, mu, sigma
		FROM player_ratings
		WHERE game_id = $1
		  AND mode_key <> $2
		  AND sigma <= $3
		  AND user_id = ANY($4::uuid[])
		ORDER BY user_id, mode_key
	`, gameID, excludeMode, maxSigma, pq.Array(ids))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var userID uuid.UUID
		var src coldstart.SourceRating
		if err := rows.Scan(&userID, &src.ModeKey, &src.Rating.Mu, &src.Rating.Sigma); err != nil {
			return nil, err
		}
		out[userID] = append(out[userID], src)
	}
	return out, rows.Err()
}

// RecordRatingSeed writes the audit row for a seed served to a player, and
// does nothing if one is already there.
//
// First-write-wins rather than last: the row exists to say what the player's
// earliest matches in this mode were built on, and every subsequent read
// before their first result returns the same numbers anyway. Making it
// idempotent is also what lets the serve path record unconditionally without
// a read-back, so a roster of seeded players costs one insert each and no
// extra query.
func (s *Store) RecordRatingSeed(ctx context.Context, userID, gameID uuid.UUID, modeKey string, seed coldstart.Seed, at time.Time) error {
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO rating_seed_events (
			user_id, game_id, mode_key,
			source_mode_key, source_mu,
			seeded_mu, seeded_sigma,
			correlation, paired_players, seeded_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
		ON CONFLICT (user_id, game_id, mode_key) DO NOTHING
	`,
		userID, gameID, modeKey,
		seed.SourceMode, seed.SourceMu,
		seed.Rating.Mu, seed.Rating.Sigma,
		seed.Correlation, seed.PairedPlayers, at,
	)
	return err
}

// CountRatingSeeds returns how many players have been seeded into one mode.
// Used by tests and by anyone reading the audit trail to size it before
// pulling it.
func (s *Store) CountRatingSeeds(ctx context.Context, gameID uuid.UUID, modeKey string) (int, error) {
	var n int
	err := s.db.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM rating_seed_events WHERE game_id = $1 AND mode_key = $2
	`, gameID, modeKey).Scan(&n)
	return n, err
}
