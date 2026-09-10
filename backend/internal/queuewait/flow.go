package queuewait

import (
	"context"
	"time"

	"github.com/google/uuid"
)

// Flow is one line's measured dynamics over a recent window: who arrived, who
// was consumed by a match, and how many are waiting right now.
//
// It exists because two different questions need two different rates, and one
// cannot stand in for the other. Consumption answers "how fast is this line
// draining", which is what a wait estimate divides by. Arrival answers "is
// another player likely to show up", which is what deciding to hold a dequeue
// turns on. A line consuming fast and a line nobody is joining both read low on
// consumption alone.
type Flow struct {
	// Arrivals is rows that joined this line during the window.
	Arrivals int
	// Fills is rows on this line that were matched during the window.
	Fills int
	// Depth is rows waiting on this line right now — a level, not a rate, so
	// it is deliberately not windowed.
	Depth  int
	Window time.Duration
}

// ArrivalRate is players per second joining the line.
func (f Flow) ArrivalRate() float64 { return f.ratePerSecond(f.Arrivals) }

// ConsumptionRate is λ: players per second the line's matches consume.
func (f Flow) ConsumptionRate() float64 { return f.ratePerSecond(f.Fills) }

// ratePerSecond is where the divide-by-zero guard lives, rather than in every
// caller. A zero or negative window reports no rate at all, which reads
// downstream as "not measurable" and sends an estimate to its fallback — the
// honest answer, and the one that cannot produce an infinity.
func (f Flow) ratePerSecond(count int) float64 {
	if count <= 0 || f.Window <= 0 {
		return 0
	}
	return float64(count) / f.Window.Seconds()
}

// FlowQuery asks for several lines' recent dynamics at once.
//
// A struct rather than positional arguments, for the same reason FillQuery is
// one: a later consumer wanting dynamics sliced differently adds a field here
// instead of breaking every implementation.
//
// Since and Now bracket the window rather than a duration being passed, so the
// source can report the window it actually measured over and callers stay
// testable with a fixed now.
type FlowQuery struct {
	// ModeQueueIDs are the queues to measure. Empty means every queue with
	// activity, matching what FillQuery.ModeQueueIDs means.
	ModeQueueIDs []uuid.UUID
	Since        time.Time
	Now          time.Time
}

// Window is the span the query covers, or zero if the bracket is inverted.
func (q FlowQuery) Window() time.Duration { return q.Now.Sub(q.Since) }

// FlowSource supplies measured dynamics, keyed by the line they were measured
// in. The store implements it against Postgres; tests implement it with a map,
// which is why nothing consuming a rate needs a database.
//
// Lines with no arrivals, no fills and nobody waiting are absent from the map
// rather than present and zeroed.
type FlowSource interface {
	RecentFlow(ctx context.Context, q FlowQuery) (map[QueueKey]Flow, error)
}

// Position is where one waiting player sits in their line.
//
// Key travels with it because the caller starts from a queue row id and does
// not otherwise know which line that row is in — and the line is what every
// rate is keyed by.
type Position struct {
	Key QueueKey
	// Ahead is waiting players in the same line who joined earlier. The player
	// is matched once the line consumes Ahead+1 players, themselves included,
	// which is why an estimate built on this can never be negative.
	Ahead int
	Depth int
}

// PositionSource locates one player in their line. The bool is false when the
// player is not waiting — they left, or were already matched — which is not an
// error.
type PositionSource interface {
	PositionOf(ctx context.Context, queueID uuid.UUID) (Position, bool, error)
}
