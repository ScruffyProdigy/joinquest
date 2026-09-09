package store

import (
	"context"

	"github.com/google/uuid"
	"github.com/lib/pq"
	"github.com/scruffyprodigy/joinquest/internal/queuewait"
)

// RecentModeQueueFills returns recently observed waits keyed by the line they
// were observed in, most recent first within each line. An empty
// FillQuery.ModeQueueIDs reads every queue that has fills in the window.
//
// A line is a mode queue plus, for composition modes, the queue path the player
// joined on. Those move at genuinely different speeds — a scarce role is picked
// up quickly while a crowded one waits — so bucketing them together would
// produce a median describing no actual player.
//
// One query for every queue asked about, never one per queue: a catalog page
// renders many mode cards, and this is read behind a whole-catalog snapshot.
// The cap is applied by ROW_NUMBER rather than in Go so a single busy queue or
// role cannot crowd the others out of the result — and so the rows a quiet one
// needs are never fetched and thrown away.
//
// Deliberately no aggregation: the median (or whatever a later strategy wants)
// is computed in internal/queuewait, so changing how an estimate is derived
// does not mean changing SQL. See that package's doc comment.
func (s *Store) RecentModeQueueFills(ctx context.Context, q queuewait.FillQuery) (map[queuewait.QueueKey][]queuewait.Fill, error) {
	// matched_at IS NOT NULL rather than status = 'matched': the question is how
	// long a player waits to be matched, not whether the handoff afterwards
	// worked. That keeps rows the stale-matched sweep later cancelled, and drops
	// players who gave up while waiting — and rolled-back matches, which
	// match_rollback nulls the column for — without naming either case.
	rows, err := s.db.QueryContext(ctx, `
		SELECT mode_queue_id, queue_path, joined_at, matched_at
		FROM (
			SELECT
				mode_queue_id,
				COALESCE(queue_path, '') AS queue_path,
				joined_at,
				matched_at,
				ROW_NUMBER() OVER (
					PARTITION BY mode_queue_id, COALESCE(queue_path, '')
					ORDER BY matched_at DESC
				) AS recency
			FROM game_queues
			WHERE mode_queue_id IS NOT NULL
			  AND matched_at IS NOT NULL
			  AND matched_at >= $1
			  AND (cardinality($2::uuid[]) = 0 OR mode_queue_id = ANY($2::uuid[]))
		) ranked
		WHERE recency <= $3
		ORDER BY mode_queue_id, queue_path, matched_at DESC
	`, q.Since, pq.Array(queueIDStrings(q.ModeQueueIDs)), q.LimitPerQueue)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	byQueue := make(map[queuewait.QueueKey][]queuewait.Fill)
	for rows.Next() {
		var key queuewait.QueueKey
		var f queuewait.Fill
		if err := rows.Scan(&key.ModeQueueID, &key.QueuePath, &f.JoinedAt, &f.MatchedAt); err != nil {
			return nil, err
		}
		byQueue[key] = append(byQueue[key], f)
	}
	return byQueue, rows.Err()
}

// queueIDStrings renders queue ids for pq.Array, which has no uuid.UUID case.
// A nil slice becomes an empty array rather than NULL, which is what the
// cardinality check above reads as "every queue".
func queueIDStrings(ids []uuid.UUID) []string {
	out := make([]string, 0, len(ids))
	for _, id := range ids {
		out = append(out, id.String())
	}
	return out
}
