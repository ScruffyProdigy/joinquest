package queuewait

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
)

// fakeFlow serves fixed dynamics per line and records the query it was asked,
// so a test can assert on the window a live estimator requested and on how many
// round trips answering one player took.
type fakeFlow struct {
	byQueue map[QueueKey]Flow
	err     error
	got     FlowQuery
	calls   int
}

func (f *fakeFlow) RecentFlow(_ context.Context, q FlowQuery) (map[QueueKey]Flow, error) {
	f.got = q
	f.calls++
	if f.err != nil {
		return nil, f.err
	}
	return f.byQueue, nil
}

// fakePositions answers for exactly one queue row; anything else is not
// waiting, which is how a player who already left is represented.
type fakePositions struct {
	queueID uuid.UUID
	pos     Position
	err     error
}

func (f *fakePositions) PositionOf(_ context.Context, queueID uuid.UUID) (Position, bool, error) {
	if f.err != nil {
		return Position{}, false, f.err
	}
	if queueID != f.queueID {
		return Position{}, false, nil
	}
	return f.pos, true, nil
}

// fakeEstimator stands in for MedianEstimator as the historical fallback.
type fakeEstimator struct {
	byQueue map[QueueKey]time.Duration
	err     error
	calls   int
}

func (f *fakeEstimator) EstimateByQueue(_ context.Context, _ []uuid.UUID, _ time.Time) (map[QueueKey]time.Duration, error) {
	f.calls++
	if f.err != nil {
		return nil, f.err
	}
	return f.byQueue, nil
}

// liveFixture wires one waiting player on one line, with the line's recent
// dynamics and a historical fallback that always has an answer. Tests vary the
// one field they are about.
type liveFixture struct {
	queueID  uuid.UUID
	key      QueueKey
	flow     *fakeFlow
	fallback *fakeEstimator
	est      LiveEstimator
}

func newLiveFixture(ahead int, flow Flow) *liveFixture {
	queueID := uuid.New()
	key := QueueKey{ModeQueueID: uuid.New(), QueuePath: "support"}

	f := &liveFixture{
		queueID: queueID,
		key:     key,
		flow:    &fakeFlow{byQueue: map[QueueKey]Flow{key: flow}},
		fallback: &fakeEstimator{byQueue: map[QueueKey]time.Duration{
			key: 90 * time.Second,
		}},
	}
	f.est = LiveEstimator{
		Flow:        f.flow,
		Positions:   &fakePositions{queueID: queueID, pos: Position{Key: key, Ahead: ahead, Depth: ahead + 1}},
		Fallback:    f.fallback,
		Window:      2 * time.Minute,
		MinFills:    3,
		MaxEstimate: 10 * time.Minute,
	}
	return f
}

func (f *liveFixture) estimate(t *testing.T) (time.Duration, bool) {
	t.Helper()
	got, ok, err := f.est.EstimateForPlayer(context.Background(), f.queueID, time.Now())
	if err != nil {
		t.Fatalf("EstimateForPlayer: %v", err)
	}
	return got, ok
}

// The player is matched once λ has consumed everyone ahead of them plus
// themselves: 3 players at 6 fills per 2 minutes is 60 seconds.
func TestLiveEstimatorDividesPositionByConsumptionRate(t *testing.T) {
	f := newLiveFixture(2, Flow{Fills: 6, Window: 2 * time.Minute})

	got, ok := f.estimate(t)
	if !ok {
		t.Fatal("got no estimate, want a live one")
	}
	if got != 60*time.Second {
		t.Errorf("got %v, want 1m", got)
	}
}

// The player's own position is what makes this per-player rather than a line
// aggregate: deeper in the same line, at the same rate, is a longer wait.
func TestLiveEstimatorGrowsWithPositionAtTheSameRate(t *testing.T) {
	shallow, _ := newLiveFixture(1, Flow{Fills: 6, Window: 2 * time.Minute}).estimate(t)
	deep, _ := newLiveFixture(5, Flow{Fills: 6, Window: 2 * time.Minute}).estimate(t)

	if !(deep > shallow) {
		t.Errorf("got deep %v and shallow %v, want deep to be longer", deep, shallow)
	}
}

// Crossover, lower edge: one fill short of the floor is not enough throughput
// to divide by, so the answer is the historical median.
func TestLiveEstimatorFallsBackToHistoricalJustBelowTheFillFloor(t *testing.T) {
	f := newLiveFixture(2, Flow{Fills: 2, Window: 2 * time.Minute})

	got, ok := f.estimate(t)
	if !ok {
		t.Fatal("got no estimate, want the historical fallback")
	}
	if got != 90*time.Second {
		t.Errorf("got %v, want the fallback's 1m30s", got)
	}
}

// Crossover, upper edge: exactly at the floor the live number takes over. Two
// fills per 2 minutes over 3 players is 180 seconds — deliberately different
// from the fallback's 90s, so a test cannot pass by reading the wrong source.
func TestLiveEstimatorGoesLiveExactlyAtTheFillFloor(t *testing.T) {
	f := newLiveFixture(2, Flow{Fills: 3, Window: 3 * time.Minute})

	got, ok := f.estimate(t)
	if !ok {
		t.Fatal("got no estimate, want a live one")
	}
	if got != 180*time.Second {
		t.Errorf("got %v, want 3m", got)
	}
	if f.fallback.calls != 0 {
		t.Errorf("fallback consulted %d times, want 0 once live", f.fallback.calls)
	}
}

// A line with no fills at all reports the fallback rather than dividing by a
// zero rate.
func TestLiveEstimatorFallsBackWhenTheLineHasNotFilledAtAll(t *testing.T) {
	f := newLiveFixture(2, Flow{Fills: 0, Window: 2 * time.Minute})

	got, ok := f.estimate(t)
	if !ok {
		t.Fatal("got no estimate, want the historical fallback")
	}
	if got != 90*time.Second {
		t.Errorf("got %v, want the fallback's 1m30s", got)
	}
}

// A line absent from the flow map has no measured dynamics at all — the same
// situation as no fills, and it must not be read as a zero-value live estimate.
func TestLiveEstimatorFallsBackWhenTheLineIsAbsentFromTheFlowMap(t *testing.T) {
	f := newLiveFixture(2, Flow{Fills: 6, Window: 2 * time.Minute})
	f.flow.byQueue = map[QueueKey]Flow{}

	got, ok := f.estimate(t)
	if !ok {
		t.Fatal("got no estimate, want the historical fallback")
	}
	if got != 90*time.Second {
		t.Errorf("got %v, want the fallback's 1m30s", got)
	}
}

// The ceiling is the guard the fill floor cannot provide: three fills in the
// window is "measurable", but on a deep enough line it computes to a number no
// player should be shown. An honest null beats a confident absurdity.
func TestLiveEstimatorReportsNoEstimateAboveTheCeiling(t *testing.T) {
	f := newLiveFixture(199, Flow{Fills: 3, Window: 3 * time.Minute})

	_, ok := f.estimate(t)
	if ok {
		t.Error("got an estimate, want none above the ceiling")
	}
}

// Above the ceiling the live path declines rather than deferring — a stale
// median is not a better answer about a line this backed up.
func TestLiveEstimatorDoesNotFallBackAboveTheCeiling(t *testing.T) {
	f := newLiveFixture(199, Flow{Fills: 3, Window: 3 * time.Minute})

	f.estimate(t)
	if f.fallback.calls != 0 {
		t.Errorf("fallback consulted %d times, want 0 above the ceiling", f.fallback.calls)
	}
}

// A player who is no longer waiting has no place in line to divide, which is
// not an error.
func TestLiveEstimatorReportsNoEstimateForAPlayerNotWaiting(t *testing.T) {
	f := newLiveFixture(2, Flow{Fills: 6, Window: 2 * time.Minute})

	_, ok, err := f.est.EstimateForPlayer(context.Background(), uuid.New(), time.Now())
	if err != nil {
		t.Fatalf("EstimateForPlayer: %v", err)
	}
	if ok {
		t.Error("got an estimate, want none for a player who is not waiting")
	}
}

// Neither source has anything to say: no badge, and still not an error.
func TestLiveEstimatorReportsNoEstimateWhenTheFallbackIsAlsoSilent(t *testing.T) {
	f := newLiveFixture(2, Flow{Fills: 0, Window: 2 * time.Minute})
	f.fallback.byQueue = map[QueueKey]time.Duration{}

	_, ok := f.estimate(t)
	if ok {
		t.Error("got an estimate, want none when neither source has one")
	}
}

// A broken query stays distinguishable from a cold line, the way JQ-58 settled
// it for the historical path.
func TestLiveEstimatorPropagatesFlowErrors(t *testing.T) {
	f := newLiveFixture(2, Flow{Fills: 6, Window: 2 * time.Minute})
	f.flow.err = errors.New("boom")

	if _, _, err := f.est.EstimateForPlayer(context.Background(), f.queueID, time.Now()); err == nil {
		t.Error("got nil error, want the flow error propagated")
	}
}

func TestLiveEstimatorPropagatesFallbackErrors(t *testing.T) {
	f := newLiveFixture(2, Flow{Fills: 0, Window: 2 * time.Minute})
	f.fallback.err = errors.New("boom")

	if _, _, err := f.est.EstimateForPlayer(context.Background(), f.queueID, time.Now()); err == nil {
		t.Error("got nil error, want the fallback error propagated")
	}
}

// One player costs one flow read, scoped to their own line rather than the
// whole catalog.
func TestLiveEstimatorAsksOnlyForThePlayersOwnLine(t *testing.T) {
	f := newLiveFixture(2, Flow{Fills: 6, Window: 2 * time.Minute})

	f.estimate(t)
	if f.flow.calls != 1 {
		t.Errorf("flow read %d times, want 1", f.flow.calls)
	}
	if len(f.flow.got.ModeQueueIDs) != 1 || f.flow.got.ModeQueueIDs[0] != f.key.ModeQueueID {
		t.Errorf("got ModeQueueIDs %v, want just %v", f.flow.got.ModeQueueIDs, f.key.ModeQueueID)
	}
}

// The live window is short and measured back from the caller's now, so the
// estimate reacts to conditions rather than to the median's multi-day history.
func TestLiveEstimatorAsksForTheConfiguredWindowEndingNow(t *testing.T) {
	f := newLiveFixture(2, Flow{Fills: 6, Window: 2 * time.Minute})
	now := time.Date(2026, 9, 10, 12, 0, 0, 0, time.UTC)

	if _, _, err := f.est.EstimateForPlayer(context.Background(), f.queueID, now); err != nil {
		t.Fatalf("EstimateForPlayer: %v", err)
	}
	if !f.flow.got.Since.Equal(now.Add(-2 * time.Minute)) {
		t.Errorf("got Since %v, want %v", f.flow.got.Since, now.Add(-2*time.Minute))
	}
	if !f.flow.got.Now.Equal(now) {
		t.Errorf("got Now %v, want %v", f.flow.got.Now, now)
	}
}

func TestLiveEstimatorDefaultsAreUsedWhenFieldsAreZero(t *testing.T) {
	f := newLiveFixture(2, Flow{Fills: 6, Window: 2 * time.Minute})
	f.est.Window = 0
	f.est.MinFills = 0
	f.est.MaxEstimate = 0
	now := time.Date(2026, 9, 10, 12, 0, 0, 0, time.UTC)

	if _, _, err := f.est.EstimateForPlayer(context.Background(), f.queueID, now); err != nil {
		t.Fatalf("EstimateForPlayer: %v", err)
	}
	if !f.flow.got.Since.Equal(now.Add(-DefaultLiveWindow)) {
		t.Errorf("got Since %v, want the default window back from now", f.flow.got.Since)
	}
}

// The floor's clamp is what makes reading an absent line's zero Flow safe: with
// a floor of zero, a line nobody has matched in would divide by a rate nobody
// observed. Pins the invariant the absent-line path relies on.
func TestLiveEstimatorDoesNotGoLiveOnAnUnmeasuredLineWhenMinFillsIsZero(t *testing.T) {
	f := newLiveFixture(2, Flow{Fills: 6, Window: 2 * time.Minute})
	f.flow.byQueue = map[QueueKey]Flow{}
	f.est.MinFills = 0

	got, ok := f.estimate(t)
	if !ok {
		t.Fatal("got no estimate, want the historical fallback")
	}
	if got != 90*time.Second {
		t.Errorf("got %v, want the fallback's 1m30s", got)
	}
}
