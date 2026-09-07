package store

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// DefaultStaleSessionAge is how long a session may sit `active` before the sweep
// treats it as stuck rather than in play.
//
// The only thing that legitimately leaves a session active forever is a failed
// reportMatchResult, so this is a "no match runs this long" bound, not a guess at
// match length. Nothing in the catalog currently pins down the longest legitimate
// match, so it is deliberately far above any of them: a stuck session waiting one
// extra tick costs a player nothing, while ending a live one loses their game.
const DefaultStaleSessionAge = 6 * time.Hour

// staleSessionPredicate matches sessions that have been `active` longer than the age
// guard ($1, a Postgres interval).
//
// Age is the whole test. An earlier version of this sweep keyed on mode_queue_id and
// completed every session but the newest on a queue — but mode_queues is catalog
// configuration created once per (mode, name), not a match, and concurrent matches on
// one queue are normal and supported. Keying on it ended live games (JQ-171).
//
// started_at is nullable, and a NULL is deliberately not stale: without a start time
// there is no evidence the session is stuck, and the safe direction is to leave it for
// a human rather than end what might be a live match.
const staleSessionPredicate = `
	gs.status = 'active'
	  AND gs.started_at IS NOT NULL
	  AND gs.started_at < NOW() - $1::interval`

// SessionSweepResult reports what one pass of the stale session sweep saw and did.
type SessionSweepResult struct {
	// StaleBefore is how many stuck sessions the sweep found.
	StaleBefore int
	// Completed is how many it actually took through CompleteSession.
	Completed int
	// StaleAfter is how many remain. Anything above zero means sessions resisted
	// the sweep and a human should look.
	StaleAfter int
}

// CountStaleSessions returns how many sessions have been active longer than olderThan.
// It writes nothing, so it is safe to call from a health check or an alert probe.
func (s *Store) CountStaleSessions(ctx context.Context, olderThan time.Duration) (int, error) {
	var count int
	err := s.db.QueryRowContext(ctx, `
		SELECT count(*) FROM game_sessions gs WHERE `+staleSessionPredicate,
		pgInterval(olderThan),
	).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("count stale sessions: %w", err)
	}
	return count, nil
}

// ListStaleSessions returns the ids of sessions active longer than olderThan, oldest
// first so a truncated or timed-out sweep still makes progress on the worst offenders.
func (s *Store) ListStaleSessions(ctx context.Context, olderThan time.Duration) ([]uuid.UUID, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT gs.id FROM game_sessions gs
		WHERE `+staleSessionPredicate+`
		ORDER BY gs.started_at ASC`,
		pgInterval(olderThan),
	)
	if err != nil {
		return nil, fmt.Errorf("list stale sessions: %w", err)
	}
	defer rows.Close()

	var ids []uuid.UUID
	for rows.Next() {
		var id uuid.UUID
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("scan stale session: %w", err)
		}
		ids = append(ids, id)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list stale sessions: %w", err)
	}
	return ids, nil
}

// SweepStaleSessions completes sessions that have been active past olderThan.
//
// It delegates to CompleteSession rather than issuing its own UPDATE, so a swept
// session gets the whole completion — status, participant queue-row cancellation and
// room-table reset — in one transaction. The previous raw-SQL job did only the status
// flip, which is precisely how it manufactured the orphaned `matched` rows that
// JQ-133's sweep exists to clean up.
//
// The sweep is idempotent: a second pass matches nothing, because the predicate only
// selects sessions still 'active'. A session that another path completes between the
// list and the write comes back as ErrNotFound and is skipped, not failed.
func (s *Store) SweepStaleSessions(ctx context.Context, olderThan time.Duration) (SessionSweepResult, error) {
	var result SessionSweepResult

	ids, err := s.ListStaleSessions(ctx, olderThan)
	if err != nil {
		return result, err
	}
	result.StaleBefore = len(ids)

	now := time.Now()
	for _, id := range ids {
		err := s.CompleteSession(ctx, id, now)
		if errors.Is(err, ErrNotFound) {
			// Raced with reportMatchResult or reportPlayerFinished. That path did the
			// full completion, which is the outcome we wanted anyway.
			continue
		}
		if err != nil {
			return result, fmt.Errorf("complete stale session %s: %w", id, err)
		}
		result.Completed++
	}

	after, err := s.CountStaleSessions(ctx, olderThan)
	if err != nil {
		return result, err
	}
	result.StaleAfter = after

	return result, nil
}
