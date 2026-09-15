package store

import (
	"context"
	"time"
)

// DefaultActivityRetention is how long raw player activity events are kept.
//
// 180 days, chosen against the tension the ticket names: longitudinal analysis will
// be wanted at a point when the volume does not yet justify keeping everything, and
// deleted is deleted. Half a year is long enough to cover a game's whole life from
// first playtest to public release and still have the early rows to compare against,
// and short enough that an append-only table on an unmeasured platform stays bounded.
//
// What makes the window safe to enforce is that it does not take the answers with it:
// RollUpActivityDaily and RefreshFirstMatchSummary run first and keep the aggregates
// -- and the per-player first-match signal, the sharpest reading of a game's floor --
// indefinitely. Changing this constant is a product decision, not a tuning knob; the
// retention window is documented in docs/player-activity-events.md and the two should
// not drift apart.
const DefaultActivityRetention = 180 * 24 * time.Hour

// RollUpActivityDaily folds raw events into per-game, per-mode, per-day counts.
//
// Idempotent by construction: it recomputes each day from the raw rows still present
// and overwrites whatever the last run stored, so running it twice, or after a
// partial failure, converges on the same numbers rather than doubling them. That
// matters more than it sounds -- a retry that silently doubled a count would corrupt
// an aggregate nobody can rebuild once the raw rows are gone.
//
// Deliberately does not roll up days that still have raw rows arriving. through is
// the last day to fold, and a caller passing yesterday avoids storing a partial count
// for today that a later run would have to correct.
func (s *Store) RollUpActivityDaily(ctx context.Context, through time.Time) (int64, error) {
	result, err := s.db.ExecContext(ctx, `
		INSERT INTO player_activity_daily (day, game_id, mode_key, event_type, event_count, distinct_users)
		SELECT
			(occurred_at AT TIME ZONE 'UTC')::date AS day,
			game_id,
			COALESCE(mode_key, ''),
			event_type,
			COUNT(*),
			COUNT(DISTINCT user_id)
		FROM player_activity_events
		WHERE game_id IS NOT NULL
		  AND (occurred_at AT TIME ZONE 'UTC')::date <= ($1 AT TIME ZONE 'UTC')::date
		GROUP BY 1, 2, 3, 4
		ON CONFLICT (day, game_id, mode_key, event_type) DO UPDATE
		SET event_count    = EXCLUDED.event_count,
		    distinct_users = EXCLUDED.distinct_users
	`, through.UTC())
	if err != nil {
		return 0, err
	}
	return result.RowsAffected()
}

// RefreshFirstMatchSummary materialises the first-match view into a table that
// outlives the raw events behind it.
//
// The view is the definition and this is a snapshot of it, which is why the refresh
// overwrites rather than merges: a player whose second match happened since the last
// run must have second_started_at filled in, and a row that once said NULL there was
// reporting "not yet", not "never".
//
// A player's first match is a fact about the past, so first_started_at may move
// EARLIER but never later. Once the raw event behind a stored first match has been
// swept, the view starts reporting that player's second match as their first; without
// the guard below, a refresh would promote it and silently understate how long the
// player had been bouncing off the game.
//
// Which row's finish reason to keep follows from the same comparison, and getting it
// wrong is the subtle failure here. When the view reports a LATER first match than the
// stored one, it is describing a DIFFERENT match -- so its outcome must be discarded
// along with its timestamp, not merged in. Coalescing it onto the stored row would
// attach the second match's finish reason to the first, which is precisely the number
// a floor analysis is most interested in and would be quietly wrong.
//
// The three cases, all expressed by comparing EXCLUDED against the stored row:
//
//	earlier -- a first match we had not seen; take all of its fields
//	equal   -- the same match; take its fields, since the game may have reported since
//	later   -- a different match; keep the stored row's fields untouched
//
// second_started_at is LEAST in every case: the earliest observed return is the right
// one, and Postgres's LEAST ignores NULLs, so "not yet" never overwrites a real date.
func (s *Store) RefreshFirstMatchSummary(ctx context.Context) (int64, error) {
	result, err := s.db.ExecContext(ctx, `
		INSERT INTO player_first_match_summary (
			user_id, game_id, first_session_id, first_started_at,
			first_outcome_observed, first_finish_reason, second_started_at, refreshed_at
		)
		SELECT
			user_id, game_id, first_session_id, first_started_at,
			first_outcome_observed, first_finish_reason, second_started_at, NOW()
		FROM player_activity_first_match
		ON CONFLICT (user_id, game_id) DO UPDATE
		SET first_session_id = CASE
		        WHEN EXCLUDED.first_started_at <= player_first_match_summary.first_started_at
		        THEN EXCLUDED.first_session_id
		        ELSE player_first_match_summary.first_session_id END,
		    first_started_at = LEAST(player_first_match_summary.first_started_at,
		                             EXCLUDED.first_started_at),
		    first_outcome_observed = CASE
		        WHEN EXCLUDED.first_started_at <= player_first_match_summary.first_started_at
		        THEN EXCLUDED.first_outcome_observed
		        ELSE player_first_match_summary.first_outcome_observed END,
		    first_finish_reason = CASE
		        WHEN EXCLUDED.first_started_at <= player_first_match_summary.first_started_at
		        THEN EXCLUDED.first_finish_reason
		        ELSE player_first_match_summary.first_finish_reason END,
		    second_started_at = LEAST(player_first_match_summary.second_started_at,
		                              EXCLUDED.second_started_at),
		    refreshed_at = NOW()
	`)
	if err != nil {
		return 0, err
	}
	return result.RowsAffected()
}

// CountActivityEventsOlderThan reports how many raw events the cutoff would remove.
// Used by the sweep's dry run, so an operator can see the size of a deletion before
// it happens rather than after.
func (s *Store) CountActivityEventsOlderThan(ctx context.Context, cutoff time.Time) (int64, error) {
	var n int64
	err := s.db.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM player_activity_events WHERE occurred_at < $1
	`, cutoff.UTC()).Scan(&n)
	return n, err
}

// DeleteActivityEventsOlderThan removes raw events past the retention window.
//
// Batched, and returning whether more remain, because this table is append-only and
// the first run after the window is first reached may have a very large backlog. One
// unbounded DELETE would hold a long transaction and a great many row locks against
// the same table live traffic is inserting into; a caller looping over bounded
// batches leaves gaps for that traffic between them.
func (s *Store) DeleteActivityEventsOlderThan(ctx context.Context, cutoff time.Time, batchSize int) (deleted int64, more bool, err error) {
	if batchSize <= 0 {
		batchSize = 10000
	}

	result, err := s.db.ExecContext(ctx, `
		DELETE FROM player_activity_events
		WHERE id IN (
			SELECT id FROM player_activity_events
			WHERE occurred_at < $1
			ORDER BY id
			LIMIT $2
		)
	`, cutoff.UTC(), batchSize)
	if err != nil {
		return 0, false, err
	}

	deleted, err = result.RowsAffected()
	if err != nil {
		return 0, false, err
	}
	// A full batch means there is probably another one behind it.
	return deleted, deleted == int64(batchSize), nil
}
