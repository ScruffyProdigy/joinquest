// Package queuewait turns observed matchmaking history into the wait estimate a
// mode card paints as "~15 sec wait".
//
// The split here is deliberate: the store returns observations, and an Estimator
// decides what they mean. Aggregating in SQL would have been cheaper, but it
// would also have pinned the meaning of the estimate to a query — and the
// meaning is the part expected to change. A busy game's wait fluctuates through
// the day, and representing that should cost a new Estimator in this package,
// not a new migration.
//
// Everything here is batch-shaped: estimates are produced for a set of queues in
// one fetch, never one queue at a time. A catalog page renders many mode cards
// at once, so a per-queue interface would be an N+1 waiting to happen — and one
// that is far more awkward to remove later than to avoid now.
package queuewait

import (
	"context"
	"slices"
	"time"

	"github.com/google/uuid"
)

// Defaults used by MedianEstimator when a field is left at its zero value.
const (
	// DefaultWindow is how far back a fill still counts. Long enough to gather
	// samples on a quiet queue, short enough that a game's matchmaking changing
	// shape works its way out within a week.
	DefaultWindow = 7 * 24 * time.Hour
	// DefaultMinSamples is the fewest fills worth reporting a median from.
	DefaultMinSamples = 5
	// DefaultLimitPerQueue caps how many of each queue's fills are pulled into
	// memory. Per queue rather than overall, so one busy queue cannot crowd
	// every other queue out of the sample.
	DefaultLimitPerQueue = 500
)

// Fill is one observed wait: a player joined a queue, and later got matched.
type Fill struct {
	JoinedAt  time.Time
	MatchedAt time.Time
}

// Wait is how long the player sat in the queue before being matched.
func (f Fill) Wait() time.Duration { return f.MatchedAt.Sub(f.JoinedAt) }

// FillQuery asks for several queues' recent fills at once.
//
// It is a struct rather than positional arguments so a later strategy can ask
// for samples differently without breaking every implementation of Samples.
// That matters sooner than it looks: a time-of-day strategy buckets fills by
// hour, and on a busy queue "the most recent LimitPerQueue fills" can all land
// inside a few hours and starve every other bucket. Widening the request is an
// added field here; changing the signature would be a rewrite.
type FillQuery struct {
	// ModeQueueIDs are the queues to sample. Empty means every queue that has
	// recent fills — the whole-catalog snapshot Cache is built on.
	ModeQueueIDs []uuid.UUID
	Since        time.Time
	// LimitPerQueue caps the fills returned for each queue independently.
	LimitPerQueue int
}

// Samples supplies observed fills, keyed by queue. The store implements it
// against Postgres; tests implement it with a map, which is why no Estimator
// needs a database.
//
// Queues with no fills in the window are absent from the map rather than
// present and empty.
type Samples interface {
	RecentFills(ctx context.Context, q FillQuery) (map[uuid.UUID][]Fill, error)
}

// Estimator turns observed fills into the numbers mode cards paint.
//
// A queue missing from the returned map has no estimate — the card shows no
// badge rather than a guess. That is distinct from an error, which means we
// could not look at all.
//
// now is a parameter rather than time.Now() inside, so a strategy that depends
// on the time of day stays testable.
type Estimator interface {
	EstimateByModeQueue(ctx context.Context, modeQueueIDs []uuid.UUID, now time.Time) (map[uuid.UUID]time.Duration, error)
}

// MedianEstimator is unconvinced by outliers: it reports the median of each
// queue's recent fills, and declines to answer for a queue until it has seen
// enough of them.
type MedianEstimator struct {
	Samples       Samples
	Window        time.Duration
	MinSamples    int
	LimitPerQueue int
}

// EstimateByModeQueue reports the median recent wait for each queue that has
// filled often enough recently to say anything honest. Queues that have not are
// left out of the map.
func (e MedianEstimator) EstimateByModeQueue(ctx context.Context, modeQueueIDs []uuid.UUID, now time.Time) (map[uuid.UUID]time.Duration, error) {
	byQueue, err := e.Samples.RecentFills(ctx, FillQuery{
		ModeQueueIDs:  modeQueueIDs,
		Since:         now.Add(-e.window()),
		LimitPerQueue: e.limitPerQueue(),
	})
	if err != nil {
		return nil, err
	}

	estimates := make(map[uuid.UUID]time.Duration, len(byQueue))
	for queueID, fills := range byQueue {
		if len(fills) < e.minSamples() {
			continue
		}
		waits := make([]time.Duration, len(fills))
		for i, f := range fills {
			waits[i] = f.Wait()
		}
		estimates[queueID] = medianDuration(waits)
	}
	return estimates, nil
}

func (e MedianEstimator) window() time.Duration {
	if e.Window <= 0 {
		return DefaultWindow
	}
	return e.Window
}

func (e MedianEstimator) minSamples() int {
	if e.MinSamples <= 0 {
		return DefaultMinSamples
	}
	return e.MinSamples
}

func (e MedianEstimator) limitPerQueue() int {
	if e.LimitPerQueue <= 0 {
		return DefaultLimitPerQueue
	}
	return e.LimitPerQueue
}

// medianDuration sorts a copy of waits and returns the middle one, averaging
// the two middle values when the count is even. Callers guarantee a non-empty
// slice.
func medianDuration(waits []time.Duration) time.Duration {
	sorted := slices.Clone(waits)
	slices.Sort(sorted)

	mid := len(sorted) / 2
	if len(sorted)%2 == 1 {
		return sorted[mid]
	}
	// Averaged as halves first: two durations near the max of int64 would
	// overflow if summed, and the loss from integer division is a nanosecond.
	return sorted[mid-1]/2 + sorted[mid]/2
}
