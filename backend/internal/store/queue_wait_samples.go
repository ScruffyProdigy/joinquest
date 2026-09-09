package store

import (
	"context"

	"github.com/scruffyprodigy/joinquest/internal/queuewait"
)

// RecentModeQueueFills returns one queue's recently observed waits, most recent
// first.
//
// Deliberately no aggregation: the median (or whatever a later strategy wants)
// is computed in internal/queuewait, so changing how an estimate is derived
// does not mean changing SQL. See that package's doc comment.
func (s *Store) RecentModeQueueFills(ctx context.Context, q queuewait.FillQuery) ([]queuewait.Fill, error) {
	// matched_at IS NOT NULL rather than status = 'matched': the question is how
	// long a player waits to be matched, not whether the handoff afterwards
	// worked. That keeps rows the stale-matched sweep later cancelled, and drops
	// players who gave up while waiting — and rolled-back matches, which
	// match_rollback nulls the column for — without naming either case.
	rows, err := s.db.QueryContext(ctx, `
		SELECT joined_at, matched_at
		FROM game_queues
		WHERE mode_queue_id = $1
		  AND matched_at IS NOT NULL
		  AND matched_at >= $2
		ORDER BY matched_at DESC
		LIMIT $3
	`, q.ModeQueueID, q.Since, q.Limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var fills []queuewait.Fill
	for rows.Next() {
		var f queuewait.Fill
		if err := rows.Scan(&f.JoinedAt, &f.MatchedAt); err != nil {
			return nil, err
		}
		fills = append(fills, f)
	}
	return fills, rows.Err()
}
