package store

import (
	"context"
	"database/sql"
	"errors"

	"github.com/google/uuid"
	"github.com/lib/pq"
	"github.com/scruffyprodigy/joinquest/internal/queuewait"
)

// RecentQueueFlow measures each line's recent dynamics: arrivals and fills
// inside the window, and how many players are waiting right now.
//
// Three counts rather than one because they answer different questions.
// Consumption sets how fast a line drains, which is what a wait estimate
// divides by. Arrival says whether another player is likely to turn up, which
// is what holding a dequeue for skill matching turns on — and which consumption
// cannot stand in for, since a line draining fast and a line nobody is joining
// both read low.
//
// One query for every line asked about, never one per line, following
// RecentModeQueueFills next door. Depth is deliberately not windowed: a player
// who joined yesterday and is still there is still ahead of everyone behind
// them.
//
// Deliberately no rates: dividing by the window is the caller's business, in
// internal/queuewait, for the same reason the median lives there rather than in
// SQL. See that package's doc comment.
func (s *Store) RecentQueueFlow(ctx context.Context, q queuewait.FlowQuery) (map[queuewait.QueueKey]queuewait.Flow, error) {
	// matched_at rather than status = 'matched', matching the fill-history read:
	// the question is how fast players leave the line for a match, not whether
	// the handoff afterwards worked.
	//
	// The three legs are OR'd so one scan serves all of them. Postgres does not
	// turn that OR into a bitmap union over the three partial indexes — checked,
	// rather than assumed — and applies it as a heap filter instead. What keeps
	// that cheap is the narrowing above it: 000066's index leads with
	// mode_queue_id, so naming queues plans as an index scan and the filter runs
	// over one line's rows rather than the table.
	//
	// The consequence is worth knowing before adding a caller: asking about
	// every queue at once has no such narrowing and degrades to a full scan.
	// Nothing does that today — the per-player path always names one queue. A
	// whole-catalog reader wants this split into two batch queries, rates and
	// depth, each on its own index.
	rows, err := s.db.QueryContext(ctx, `
		SELECT
			mode_queue_id,
			COALESCE(queue_path, '') AS queue_path,
			COUNT(*) FILTER (WHERE joined_at >= $1) AS arrivals,
			COUNT(*) FILTER (WHERE matched_at IS NOT NULL AND matched_at >= $1) AS fills,
			COUNT(*) FILTER (WHERE status = 'waiting') AS depth
		FROM game_queues
		WHERE mode_queue_id IS NOT NULL
		  AND (cardinality($2::uuid[]) = 0 OR mode_queue_id = ANY($2::uuid[]))
		  AND (joined_at >= $1 OR matched_at >= $1 OR status = 'waiting')
		GROUP BY mode_queue_id, COALESCE(queue_path, '')
	`, q.Since, pq.Array(queueIDStrings(q.ModeQueueIDs)))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	window := q.Window()
	byQueue := make(map[queuewait.QueueKey]queuewait.Flow)
	for rows.Next() {
		var key queuewait.QueueKey
		flow := queuewait.Flow{Window: window}
		if err := rows.Scan(&key.ModeQueueID, &key.QueuePath, &flow.Arrivals, &flow.Fills, &flow.Depth); err != nil {
			return nil, err
		}
		byQueue[key] = flow
	}
	return byQueue, rows.Err()
}

// PositionOnLine reports where one waiting player sits in their line, and which
// line that is. The bool is false when the row is not waiting — matched, left,
// or never existed — which is not an error.
//
// Ahead counts only the same (mode_queue_id, queue_path): a composition mode's
// roles are separate lines, and a tank ahead of you does not lengthen a support
// wait. Ties on joined_at break by id so two players who joined in the same
// instant get a stable order rather than both claiming the same place.
func (s *Store) PositionOnLine(ctx context.Context, queueID uuid.UUID) (queuewait.Position, bool, error) {
	var position queuewait.Position
	err := s.db.QueryRowContext(ctx, `
		SELECT
			me.mode_queue_id,
			COALESCE(me.queue_path, '') AS queue_path,
			COUNT(w.id) FILTER (WHERE (w.joined_at, w.id) < (me.joined_at, me.id)) AS ahead,
			COUNT(w.id) AS depth
		FROM game_queues me
		LEFT JOIN game_queues w
			ON w.mode_queue_id = me.mode_queue_id
			AND COALESCE(w.queue_path, '') = COALESCE(me.queue_path, '')
			AND w.status = 'waiting'
		WHERE me.id = $1
		  AND me.status = 'waiting'
		  AND me.mode_queue_id IS NOT NULL
		GROUP BY me.mode_queue_id, COALESCE(me.queue_path, ''), me.joined_at, me.id
	`, queueID).Scan(&position.Key.ModeQueueID, &position.Key.QueuePath, &position.Ahead, &position.Depth)
	if errors.Is(err, sql.ErrNoRows) {
		return queuewait.Position{}, false, nil
	}
	if err != nil {
		return queuewait.Position{}, false, err
	}
	return position, true, nil
}
