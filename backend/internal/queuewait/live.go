package queuewait

import (
	"context"
	"time"

	"github.com/google/uuid"
)

// Defaults used by LiveEstimator when a field is left at its zero value.
const (
	// DefaultLiveWindow is how far back throughput is measured. Deliberately
	// short, and deliberately unrelated to DefaultWindow: the median asks what
	// this line usually does, the live estimate asks what it is doing now.
	DefaultLiveWindow = 15 * time.Minute
	// DefaultMinFills is the fewest fills in the live window worth dividing by.
	// Below it the line falls back to its historical median.
	//
	// The honest caveat: this number cannot be tuned from evidence until there
	// is enough traffic for the rate to be stable. What is defensible today is
	// the mechanism and a conservative default — which is why it is a knob.
	DefaultMinFills = 3
	// DefaultMaxEstimate is the longest live estimate worth showing a player.
	//
	// The fill floor alone does not prevent an absurd answer: three fills in a
	// quarter hour is "measurable", but on a deep enough line it still computes
	// to hours. Past this the line reports no estimate at all, following the
	// JQ-58 decision that a missing badge beats a confident wrong one.
	DefaultMaxEstimate = time.Hour
)

// PlayerEstimator answers for one queued player rather than for a line.
//
// It is a separate contract from Estimator on purpose. Estimator is
// batch-and-aggregate by construction — it exists so a catalog page can paint
// many mode cards from one fetch — and a per-player question does not fit
// through it. The historical strategy keeps its shape; this is an addition
// beside it, not a replacement.
//
// The bool is false when there is no estimate worth showing, which is distinct
// from an error: a player who is not waiting, a line with nothing measurable
// and no history, or a number too large to be useful.
type PlayerEstimator interface {
	EstimateForPlayer(ctx context.Context, queueID uuid.UUID, now time.Time) (time.Duration, bool, error)
}

// LiveEstimator answers from throughput when the line has enough of it, and
// defers to a historical estimator when it does not.
//
// The model is Little's Law read the short way. A line's throughput is the rate
// it consumes players — measured directly, rather than reconstructed as a match
// rate times a party size — and a player waits for it to get through everyone
// ahead of them plus themselves.
type LiveEstimator struct {
	Flow      FlowSource
	Positions PositionSource
	// Fallback answers for lines whose throughput is not measurable. Normally
	// MedianEstimator.
	Fallback    Estimator
	Window      time.Duration
	MinFills    int
	MaxEstimate time.Duration
}

// EstimateForPlayer reports how much longer this player is likely to wait.
func (e LiveEstimator) EstimateForPlayer(ctx context.Context, queueID uuid.UUID, now time.Time) (time.Duration, bool, error) {
	position, waiting, err := e.Positions.PositionOf(ctx, queueID)
	if err != nil {
		return 0, false, err
	}
	if !waiting {
		return 0, false, nil
	}

	byQueue, err := e.Flow.RecentFlow(ctx, FlowQuery{
		ModeQueueIDs: []uuid.UUID{position.Key.ModeQueueID},
		Since:        now.Add(-e.window()),
		Now:          now,
	})
	if err != nil {
		return 0, false, err
	}

	// A line absent from the map reads as a zero Flow, which is the right
	// answer rather than a lucky one: minFills() clamps the floor to at least
	// one, so no-fills-observed and too-few-fills-observed take the same branch
	// by construction. Do not lower that clamp without restoring an explicit
	// presence check here.
	flow := byQueue[position.Key]
	if flow.Fills >= e.minFills() {
		return e.live(position, flow)
	}
	return e.historical(ctx, position, now)
}

// live divides the player's place in line by the rate the line is draining at.
//
// Ahead+1 counts the player themselves, so the numerator is never zero and the
// result is never negative. The rate is guaranteed positive here: reaching this
// branch requires Fills at or above the floor, which is at least one.
func (e LiveEstimator) live(position Position, flow Flow) (time.Duration, bool, error) {
	rate := flow.ConsumptionRate()
	if rate <= 0 {
		return 0, false, nil
	}

	seconds := float64(position.Ahead+1) / rate
	estimate := time.Duration(seconds * float64(time.Second))
	if estimate > e.maxEstimate() {
		// Deliberately no fallback here. The line is measurably backed up; a
		// median drawn from calmer days would be a worse answer, not a safer
		// one.
		return 0, false, nil
	}
	return estimate, true, nil
}

// historical asks the fallback about this one line. Absent from its map means
// no estimate, the same as it means for a mode card.
func (e LiveEstimator) historical(ctx context.Context, position Position, now time.Time) (time.Duration, bool, error) {
	if e.Fallback == nil {
		return 0, false, nil
	}
	byQueue, err := e.Fallback.EstimateByQueue(ctx, []uuid.UUID{position.Key.ModeQueueID}, now)
	if err != nil {
		return 0, false, err
	}
	estimate, ok := byQueue[position.Key]
	return estimate, ok, nil
}

func (e LiveEstimator) window() time.Duration {
	if e.Window <= 0 {
		return DefaultLiveWindow
	}
	return e.Window
}

func (e LiveEstimator) minFills() int {
	if e.MinFills <= 0 {
		return DefaultMinFills
	}
	return e.MinFills
}

func (e LiveEstimator) maxEstimate() time.Duration {
	if e.MaxEstimate <= 0 {
		return DefaultMaxEstimate
	}
	return e.MaxEstimate
}
