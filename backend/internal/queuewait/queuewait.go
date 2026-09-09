// Package queuewait turns observed matchmaking history into the wait estimate a
// mode card paints as "~15 sec wait".
//
// The split here is deliberate: the store returns observations, and an Estimator
// decides what they mean. Aggregating in SQL would have been cheaper, but it
// would also have pinned the meaning of the estimate to a query — and the
// meaning is the part expected to change. A busy game's wait fluctuates through
// the day, and representing that should cost a new Estimator in this package,
// not a new migration.
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
	// DefaultLimit caps how many fills are pulled into memory per estimate.
	DefaultLimit = 500
)

// Fill is one observed wait: a player joined a queue, and later got matched.
type Fill struct {
	JoinedAt  time.Time
	MatchedAt time.Time
}

// Wait is how long the player sat in the queue before being matched.
func (f Fill) Wait() time.Duration { return f.MatchedAt.Sub(f.JoinedAt) }

// FillQuery asks for one queue's recent fills.
//
// It is a struct rather than positional arguments so a later strategy can ask
// for samples differently without breaking every implementation of Samples.
// That matters sooner than it looks: a time-of-day strategy buckets fills by
// hour, and on a busy game "the most recent Limit fills" can all land inside a
// few hours and starve every other bucket. Widening the request is an added
// field here; changing the signature would be a rewrite.
type FillQuery struct {
	ModeQueueID uuid.UUID
	Since       time.Time
	Limit       int
}

// Samples supplies observed fills. The store implements it against Postgres;
// tests implement it with a slice, which is why no Estimator needs a database.
type Samples interface {
	RecentFills(ctx context.Context, q FillQuery) ([]Fill, error)
}

// Estimator turns observed fills into the number a mode card paints.
//
// A nil duration means "not willing to say" — the card shows no badge rather
// than a guess. That is distinct from an error, which means we could not look.
//
// now is a parameter rather than time.Now() inside, so a strategy that depends
// on the time of day stays testable.
type Estimator interface {
	Estimate(ctx context.Context, modeQueueID uuid.UUID, now time.Time) (*time.Duration, error)
}

// MedianEstimator is unconvinced by outliers: it reports the median of a
// queue's recent fills, and declines to answer at all until it has seen enough
// of them.
type MedianEstimator struct {
	Samples    Samples
	Window     time.Duration
	MinSamples int
	Limit      int
}

// Estimate reports the median recent wait for one queue, or nil when the queue
// has not filled often enough recently to say anything honest.
func (e MedianEstimator) Estimate(ctx context.Context, modeQueueID uuid.UUID, now time.Time) (*time.Duration, error) {
	fills, err := e.Samples.RecentFills(ctx, FillQuery{
		ModeQueueID: modeQueueID,
		Since:       now.Add(-e.window()),
		Limit:       e.limit(),
	})
	if err != nil {
		return nil, err
	}
	if len(fills) < e.minSamples() {
		return nil, nil
	}

	waits := make([]time.Duration, len(fills))
	for i, f := range fills {
		waits[i] = f.Wait()
	}
	median := medianDuration(waits)
	return &median, nil
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

func (e MedianEstimator) limit() int {
	if e.Limit <= 0 {
		return DefaultLimit
	}
	return e.Limit
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
